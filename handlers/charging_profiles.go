package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

// ChargingProfileResponseType — sync HTTP response "result" enum, per OCPI 2.2.1
// mod_charging_profiles §object description. Returned inside ChargingProfileResponse.
const (
	ChargingProfileRespAccepted       = "ACCEPTED"
	ChargingProfileRespNotSupported   = "NOT_SUPPORTED"
	ChargingProfileRespRejected       = "REJECTED"
	ChargingProfileRespTooOften       = "TOO_OFTEN"
	ChargingProfileRespUnknownSession = "UNKNOWN_SESSION"
)

// ChargingProfileResultType — async callback "result" enum, per OCPI 2.2.1.
// Used in ChargingProfileResult / ActiveChargingProfileResult / ClearProfileResult.
const (
	ChargingProfileResultAccepted = "ACCEPTED"
	ChargingProfileResultRejected = "REJECTED"
	ChargingProfileResultUnknown  = "UNKNOWN"
)

// chargingProfileTimeoutSeconds is the value we advertise in ChargingProfileResponse.
// The spec leaves this to implementations; 30s is a reasonable upper bound for the
// async callback to arrive before the eMSP may assume it never will.
const chargingProfileTimeoutSeconds = 30

// chargingProfileResponse is the sync HTTP response body returned by all three
// receiver endpoints (PUT/GET/DELETE). See mod_charging_profiles §ChargingProfileResponse.
type chargingProfileResponse struct {
	Result  string `json:"result"`
	Timeout int    `json:"timeout"`
}

// setChargingProfilePayload matches the SetChargingProfile object accepted by PUT.
type setChargingProfilePayload struct {
	ChargingProfile json.RawMessage `json:"charging_profile"`
	ResponseURL     string          `json:"response_url"`
}

// activeChargingProfile mirrors the ActiveChargingProfile data type.
type activeChargingProfile struct {
	StartDateTime   string          `json:"start_date_time"`
	ChargingProfile json.RawMessage `json:"charging_profile"`
}

// activeChargingProfileResult is POSTed to response_url after a GET.
type activeChargingProfileResult struct {
	Result  string                 `json:"result"`
	Profile *activeChargingProfile `json:"profile,omitempty"`
}

// resultOnly covers both ChargingProfileResult (PUT) and ClearProfileResult (DELETE).
type resultOnly struct {
	Result string `json:"result"`
}

// PutChargingProfile — receiver interface, SetChargingProfile.
// Sync: returns ChargingProfileResponse {result, timeout}.
// Async: POSTs ChargingProfileResult {result} to payload.response_url.
func (h *Handler) PutChargingProfile(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	sessionID := chi.URLParam(r, "sessionID")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}
	var payload setChargingProfilePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}
	if payload.ResponseURL == "" {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "response_url is required")
		return
	}

	session, err := store.GetSession(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to check session")
		return
	}
	// Per spec the missing-session case is reported via the domain result, not
	// as an OCPI-level error. Envelope stays at status_code 1000.
	if session == nil {
		ocpiutil.OK(w, r, chargingProfileResponse{
			Result:  ChargingProfileRespUnknownSession,
			Timeout: chargingProfileTimeoutSeconds,
		})
		return
	}

	if err := store.PutChargingProfile(sessionID, payload.ChargingProfile); err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to persist profile")
		return
	}

	ocpiutil.OK(w, r, chargingProfileResponse{
		Result:  ChargingProfileRespAccepted,
		Timeout: chargingProfileTimeoutSeconds,
	})

	go h.sendChargingProfileCallback(store, payload.ResponseURL, "set_charging_profile_callback",
		resultOnly{Result: ChargingProfileResultAccepted})
}

// GetChargingProfile — receiver interface.
// Sync: returns ChargingProfileResponse {result, timeout}.
// Async: POSTs ActiveChargingProfileResult {result, profile?} to ?response_url.
func (h *Handler) GetChargingProfile(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	sessionID := chi.URLParam(r, "sessionID")

	responseURL := r.URL.Query().Get("response_url")
	if responseURL == "" {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "response_url is required")
		return
	}

	session, err := store.GetSession(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to check session")
		return
	}
	if session == nil {
		ocpiutil.OK(w, r, chargingProfileResponse{
			Result:  ChargingProfileRespUnknownSession,
			Timeout: chargingProfileTimeoutSeconds,
		})
		return
	}

	stored, _ := store.GetChargingProfile(sessionID)
	profileBody := stored
	if len(profileBody) == 0 {
		// If the EVSE has no explicit profile the spec still expects the
		// Charge Point to report one that reflects its current capabilities;
		// synthesize a trivially-valid empty profile so response_url always
		// receives a well-formed ActiveChargingProfile.
		profileBody, _ = json.Marshal(map[string]any{
			"charging_rate_unit":      "W",
			"charging_profile_period": []map[string]any{},
		})
	}
	active := &activeChargingProfile{
		StartDateTime:   time.Now().UTC().Format(time.RFC3339),
		ChargingProfile: json.RawMessage(profileBody),
	}

	ocpiutil.OK(w, r, chargingProfileResponse{
		Result:  ChargingProfileRespAccepted,
		Timeout: chargingProfileTimeoutSeconds,
	})

	go h.sendChargingProfileCallback(store, responseURL, "get_active_charging_profile_callback",
		activeChargingProfileResult{Result: ChargingProfileResultAccepted, Profile: active})
}

// DeleteChargingProfile — receiver interface, ClearChargingProfile.
// Sync: returns ChargingProfileResponse {result, timeout}.
// Async: POSTs ClearProfileResult {result} to ?response_url.
func (h *Handler) DeleteChargingProfile(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	sessionID := chi.URLParam(r, "sessionID")

	responseURL := r.URL.Query().Get("response_url")
	if responseURL == "" {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "response_url is required")
		return
	}

	session, err := store.GetSession(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to check session")
		return
	}
	if session == nil {
		ocpiutil.OK(w, r, chargingProfileResponse{
			Result:  ChargingProfileRespUnknownSession,
			Timeout: chargingProfileTimeoutSeconds,
		})
		return
	}

	existing, _ := store.GetChargingProfile(sessionID)
	clearResult := ChargingProfileResultAccepted
	if len(existing) == 0 {
		// Nothing to clear — distinct from a successful clear per spec enum.
		clearResult = ChargingProfileResultUnknown
	} else if err := store.DeleteChargingProfile(sessionID); err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to delete profile")
		return
	}

	ocpiutil.OK(w, r, chargingProfileResponse{
		Result:  ChargingProfileRespAccepted,
		Timeout: chargingProfileTimeoutSeconds,
	})

	go h.sendChargingProfileCallback(store, responseURL, "clear_charging_profile_callback",
		resultOnly{Result: clearResult})
}

// sendChargingProfileCallback POSTs the given payload to the eMSP response_url,
// using the eMSP's own token (if set) for authorization. Failures are swallowed
// — OCPI does not require retries on async result delivery; the timeout in the
// sync response is the eMSP's cue to move on.
func (h *Handler) sendChargingProfileCallback(store Store, responseURL, actionID string, payload any) {
	if responseURL == "" {
		return
	}
	if _, err := url.ParseRequestURI(responseURL); err != nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(h.outboundContext(actionID), http.MethodPost, responseURL, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if store != nil {
		if token, _ := store.GetEMSPOwnToken(); token != "" {
			req.Header.Set("Authorization", "Token "+token)
		}
	}
	resp, err := h.outboundClient().Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// PushActiveChargingProfile sends an unsolicited ActiveChargingProfile update to
// the eMSP's sender endpoint via HTTP PUT — as a CPO would when an EVSE notifies
// it of a profile change outside of the request/response flow.
// targetURL is the full URL of the eMSP's {chargingprofiles_endpoint_url}{session_id}.
// If profileJSON is empty a trivially-valid profile is synthesized.
func (h *Handler) PushActiveChargingProfile(ctx context.Context, store Store, targetURL string, profileJSON json.RawMessage) error {
	if store == nil {
		store = h.Store
	}
	if _, err := url.ParseRequestURI(targetURL); err != nil {
		return fmt.Errorf("invalid target_url: %w", err)
	}

	body := profileJSON
	if len(body) == 0 {
		body, _ = json.Marshal(map[string]any{
			"charging_rate_unit":      "W",
			"charging_profile_period": []map[string]any{},
		})
	}
	active := activeChargingProfile{
		StartDateTime:   time.Now().UTC().Format(time.RFC3339),
		ChargingProfile: body,
	}
	data, err := json.Marshal(active)
	if err != nil {
		return fmt.Errorf("marshal active profile: %w", err)
	}

	if ctx == nil {
		ctx = h.outboundContext("active_charging_profile_push")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, targetURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token, _ := store.GetEMSPOwnToken(); token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}
	resp, err := h.outboundClient().Do(req)
	if err != nil {
		return fmt.Errorf("push: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("push returned %d: %s", resp.StatusCode, string(snippet))
	}
	return nil
}
