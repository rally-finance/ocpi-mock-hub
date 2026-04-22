package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/rally-finance/ocpi-mock-hub/simulation"
)

type connectionStatus struct {
	Connected        bool   `json:"connected"`
	TokenB           string `json:"token_b"`
	TokenBMasked     string `json:"token_b_masked"`
	EMSPCallbackURL  string `json:"emsp_callback_url"`
	EMSPVersionsURL  string `json:"emsp_versions_url"`
	EMSPOwnToken     string `json:"emsp_own_token_masked"`
	CanDeregister    bool   `json:"can_deregister"`
	DeregisterReason string `json:"deregister_reason"`
	HubCountry       string `json:"hub_country"`
	HubParty         string `json:"hub_party"`
	Mode             string `json:"mode"`
	SeedLocations    int    `json:"seed_locations"`
	SeedTariffs      int    `json:"seed_tariffs"`
	SeedCPOs         int    `json:"seed_cpos"`
	SessionCount     int    `json:"session_count"`
	CDRCount         int    `json:"cdr_count"`
	TokenCount       int    `json:"token_count"`
	ReservationCount int    `json:"reservation_count"`
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	tokenB, _ := h.Store.GetTokenB()
	callbackURL, _ := h.Store.GetEMSPCallbackURL()
	versionsURL, _ := h.Store.GetEMSPVersionsURL()
	emspOwnToken, _ := h.Store.GetEMSPOwnToken()
	mode, _ := h.Store.GetMode()
	sessions, _ := h.Store.ListSessions()
	cdrs, _ := h.Store.ListCDRs()
	tokens, _ := h.Store.ListTokens()
	reservations, _ := h.Store.ListReservations()
	_, deregisterReason := h.currentDeregisterState(h.Store)

	status := connectionStatus{
		Connected:        tokenB != "",
		TokenB:           tokenB,
		TokenBMasked:     maskToken(tokenB),
		EMSPCallbackURL:  callbackURL,
		EMSPVersionsURL:  versionsURL,
		EMSPOwnToken:     maskToken(emspOwnToken),
		CanDeregister:    deregisterReason == "",
		DeregisterReason: deregisterReason,
		HubCountry:       h.Config.HubCountry,
		HubParty:         h.Config.HubParty,
		Mode:             mode,
		SeedLocations:    len(h.Seed.Locations),
		SeedTariffs:      len(h.Seed.Tariffs),
		SeedCPOs:         len(h.Seed.CPOs),
		SessionCount:     len(sessions),
		CDRCount:         len(cdrs),
		TokenCount:       len(tokens),
		ReservationCount: len(reservations),
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) GetAdminSessions(w http.ResponseWriter, r *http.Request) {
	raw, _ := h.Store.ListSessions()
	sessions := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		sessions = append(sessions, json.RawMessage(b))
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *Handler) GetAdminCDRs(w http.ResponseWriter, r *http.Request) {
	raw, _ := h.Store.ListCDRs()
	cdrs := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		cdrs = append(cdrs, json.RawMessage(b))
	}
	writeJSON(w, http.StatusOK, cdrs)
}

func (h *Handler) GetAdminLocations(w http.ResponseWriter, r *http.Request) {
	type locationSummary struct {
		ID          string `json:"id"`
		CountryCode string `json:"country_code"`
		PartyID     string `json:"party_id"`
		Name        string `json:"name"`
		City        string `json:"city"`
		EVSECount   int    `json:"evse_count"`
	}

	seed := h.Seed
	locs := make([]locationSummary, 0, len(seed.Locations))
	for _, loc := range seed.Locations {
		locs = append(locs, locationSummary{
			ID:          loc.ID,
			CountryCode: loc.CountryCode,
			PartyID:     loc.PartyID,
			Name:        loc.Name,
			City:        loc.City,
			EVSECount:   len(loc.EVSEs),
		})
	}
	writeJSON(w, http.StatusOK, locs)
}

func (h *Handler) GetAdminTokens(w http.ResponseWriter, r *http.Request) {
	raw, _ := h.Store.ListTokens()
	tokens := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		tokens = append(tokens, json.RawMessage(b))
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *Handler) GetAdminReservations(w http.ResponseWriter, r *http.Request) {
	raw, _ := h.Store.ListReservations()
	reservations := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		reservations = append(reservations, json.RawMessage(b))
	}
	writeJSON(w, http.StatusOK, reservations)
}

func (h *Handler) AdminAuthorize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CountryCode string `json:"country_code"`
		PartyID     string `json:"party_id"`
		UID         string `json:"uid"`
		Type        string `json:"type,omitempty"`
		LocationID  string `json:"location_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	storageUID := tokenStorageUID(req.UID, req.Type)

	mode, _ := h.Store.GetMode()
	if mode == "reject" {
		writeJSON(w, http.StatusOK, map[string]any{
			"allowed": "NOT_ALLOWED",
			"token":   map[string]string{"country_code": req.CountryCode, "party_id": req.PartyID, "uid": req.UID},
		})
		return
	}
	if mode == "auth-fail" {
		statuses := []string{"NOT_ALLOWED", "EXPIRED", "BLOCKED"}
		writeJSON(w, http.StatusOK, map[string]any{
			"allowed": statuses[rand.Intn(len(statuses))],
			"token":   map[string]string{"country_code": req.CountryCode, "party_id": req.PartyID, "uid": req.UID},
		})
		return
	}

	raw, _ := h.Store.GetToken(req.CountryCode, req.PartyID, storageUID)
	if raw == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"allowed": "NOT_ALLOWED",
			"token":   map[string]string{"country_code": req.CountryCode, "party_id": req.PartyID, "uid": req.UID},
		})
		return
	}

	result := map[string]any{
		"allowed": "ALLOWED",
		"token":   map[string]string{"country_code": req.CountryCode, "party_id": req.PartyID, "uid": req.UID},
	}

	if req.LocationID != "" {
		loc := h.Seed.LocationByID(req.LocationID)
		if loc != nil && len(loc.EVSEs) > 0 {
			evseUIDs := make([]string, 0, len(loc.EVSEs))
			for _, e := range loc.EVSEs {
				evseUIDs = append(evseUIDs, e.UID)
			}
			result["location"] = map[string]any{"evse_uids": evseUIDs}
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ResetConnection(w http.ResponseWriter, r *http.Request) {
	if err := h.resetHandshakeState(h.Store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear local handshake state: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (h *Handler) TriggerTick(w http.ResponseWriter, r *http.Request) {
	if err := h.tickAllStores(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "tick_complete"})
}

func (h *Handler) PushLocations(w http.ResponseWriter, r *http.Request) {
	authToken, emspURL, err := h.resolveEMSPPushTarget()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var cfg simulation.PushConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if cfg.Pattern == "" {
		cfg.Pattern = "burst"
	}
	if cfg.Count > 50 {
		cfg.Count = 50
	}

	summary := simulation.PushLocations(cfg, h.Seed, emspURL, authToken)
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) PushTariffs(w http.ResponseWriter, r *http.Request) {
	authToken, emspURL, err := h.resolveEMSPPushTarget()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var cfg simulation.PushConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if cfg.Pattern == "" {
		cfg.Pattern = "burst"
	}

	summary := simulation.PushTariffs(cfg, h.Seed, emspURL, authToken)
	writeJSON(w, http.StatusOK, summary)
}

// PushActiveProfile triggers an unsolicited ActiveChargingProfile PUT to an
// eMSP's sender chargingprofiles endpoint. This mirrors the CPO-side behavior
// in OCPI 2.2.1 mod_charging_profiles where the Receiver SHALL post an update
// when a local input influences the ActiveChargingProfile for an ongoing session.
//
// Request body: {"session_id": "...", "target_url": "https://..."}
// target_url is the full eMSP sender URL ({base}/chargingprofiles/{session_id}).
func (h *Handler) PushActiveProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string          `json:"session_id"`
		TargetURL string          `json:"target_url"`
		Profile   json.RawMessage `json:"profile,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.TargetURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_url is required"})
		return
	}

	// If a session is provided and we already have a profile stored for it,
	// use that as the body so the push reflects current state.
	profile := req.Profile
	if len(profile) == 0 && req.SessionID != "" {
		if stored, _ := h.Store.GetChargingProfile(req.SessionID); len(stored) > 0 {
			profile = stored
		}
	}

	if err := h.PushActiveChargingProfile(r.Context(), h.Store, req.TargetURL, profile); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "pushed",
		"session_id": req.SessionID,
		"target_url": req.TargetURL,
	})
}

// IssueCreditCDR produces an OCPI 2.2.1 credit CDR for a previously stored
// CDR. Per spec (§9.2.3.3), a credit CDR corrects or reverses an original CDR
// and MUST carry credit=true plus credit_reference_id referencing the
// original CDR's id. The hub derives a credit CDR by copying the source CDR,
// generating a fresh id, negating all monetary totals and the total_energy,
// and bumping last_updated. The credit CDR is stored and, if the hub has an
// active handshake, pushed to the eMSP receiver/cdrs endpoint so the
// counterparty reconciles against it.
//
// Request body: {"cdr_id": "CDR-xyz", "push": true}
// `push` defaults to true — set false to store the credit CDR without sending.
func (h *Handler) IssueCreditCDR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CDRID string `json:"cdr_id"`
		Push  *bool  `json:"push,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.CDRID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cdr_id is required"})
		return
	}

	raw, err := h.Store.GetCDR(req.CDRID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read CDR: " + err.Error()})
		return
	}
	if raw == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "CDR not found: " + req.CDRID})
		return
	}

	var original map[string]any
	if err := json.Unmarshal(raw, &original); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse stored CDR: " + err.Error()})
		return
	}

	credit := buildCreditCDR(original, req.CDRID)
	creditID, _ := credit["id"].(string)

	encoded, err := json.Marshal(credit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encode credit CDR: " + err.Error()})
		return
	}
	if err := h.Store.PutCDR(creditID, encoded); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store credit CDR: " + err.Error()})
		return
	}

	push := true
	if req.Push != nil {
		push = *req.Push
	}

	result := map[string]any{
		"status":              "issued",
		"credit_cdr_id":       creditID,
		"credit_reference_id": req.CDRID,
		"pushed":              false,
	}

	if push {
		authToken, emspURL, err := h.resolveEMSPPushTarget()
		if err != nil {
			result["push_error"] = err.Error()
			writeJSON(w, http.StatusOK, result)
			return
		}
		target := strings.TrimRight(emspURL, "/") + "/receiver/cdrs"
		if err := h.postCreditCDR(r.Context(), target, authToken, encoded); err != nil {
			result["push_error"] = err.Error()
			writeJSON(w, http.StatusOK, result)
			return
		}
		result["pushed"] = true
		result["target_url"] = target
	}

	writeJSON(w, http.StatusOK, result)
}

// buildCreditCDR transforms an original CDR map into an OCPI 2.2.1 credit CDR
// payload. Non-monetary fields (location, token, auth method) are preserved
// so the credit CDR still identifies the original transaction; all monetary
// amounts are negated, total_energy is negated, and credit/credit_reference_id
// are set. A new id is generated to avoid collisions with the original.
func buildCreditCDR(original map[string]any, originalID string) map[string]any {
	credit := make(map[string]any, len(original)+2)
	for k, v := range original {
		credit[k] = v
	}

	creditID := "CDR-CREDIT-" + strings.ReplaceAll(originalID, "CDR-", "")
	credit["id"] = creditID
	credit["credit"] = true
	credit["credit_reference_id"] = originalID

	for _, field := range []string{
		"total_cost",
		"total_fixed_cost",
		"total_energy_cost",
		"total_time_cost",
		"total_parking_cost",
		"total_reservation_cost",
	} {
		if v, ok := credit[field]; ok {
			credit[field] = negatePriceShape(v)
		}
	}
	if v, ok := credit["total_energy"]; ok {
		credit["total_energy"] = negateFloat(v)
	}

	credit["last_updated"] = nowRFC3339()
	return credit
}

// negatePriceShape inverts the sign of an OCPI Price object ({excl_vat, incl_vat})
// while preserving its encoding (map vs JSON number). Any non-Price shape is
// returned untouched so malformed stored CDRs don't silently corrupt.
func negatePriceShape(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, inner := range typed {
			switch n := inner.(type) {
			case float64:
				out[k] = -n
			case int:
				out[k] = -n
			default:
				out[k] = inner
			}
		}
		return out
	case map[string]float64:
		out := make(map[string]float64, len(typed))
		for k, n := range typed {
			out[k] = -n
		}
		return out
	default:
		return v
	}
}

func negateFloat(v any) any {
	switch n := v.(type) {
	case float64:
		return -n
	case int:
		return -n
	default:
		return v
	}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// postCreditCDR POSTs an encoded credit CDR to the eMSP receiver/cdrs endpoint.
// Errors surface back to the admin caller so they can distinguish "stored but
// push failed" from "stored and delivered".
func (h *Handler) postCreditCDR(ctx context.Context, target, authToken string, body []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Token "+authToken)
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

func (h *Handler) resolveEMSPPushTarget() (authToken, emspURL string, err error) {
	tokenB, _ := h.Store.GetTokenB()
	if tokenB == "" {
		return "", "", fmt.Errorf("no handshake completed (Token B not set)")
	}
	authToken, _ = h.Store.GetEMSPOwnToken()
	if authToken == "" {
		return "", "", fmt.Errorf("no eMSP auth token available")
	}
	emspURL, _ = h.Store.GetEMSPCallbackURL()
	if emspURL == "" {
		emspURL = h.Config.EMSPCallbackURL
	}
	if emspURL == "" {
		return "", "", fmt.Errorf("no eMSP callback URL configured")
	}
	emspURL = normalizeCallbackURL(emspURL)
	return authToken, emspURL, nil
}

// normalizeCallbackURL strips trailing "/versions" from the OCPI credentials
// URL so it can be used as a base for receiver endpoint paths.
func normalizeCallbackURL(u string) string {
	u = strings.TrimRight(u, "/")
	if strings.HasSuffix(u, "/versions") {
		return strings.TrimSuffix(u, "/versions")
	}
	return u
}
