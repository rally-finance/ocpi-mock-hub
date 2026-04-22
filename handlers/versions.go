package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetVersions(w http.ResponseWriter, r *http.Request) {
	if !h.verifyTokenA(w, r) {
		return
	}

	scheme := resolveScheme(r)
	host := resolveHost(r)
	base := fmt.Sprintf("%s://%s", scheme, host)

	versions := []map[string]string{
		{"version": "2.2.1", "url": base + "/ocpi/2.2.1"},
	}
	ocpiutil.OK(w, r, versions)
}

// SupportedOCPIVersion is the single OCPI protocol version served by the mock hub.
const SupportedOCPIVersion = "2.2.1"

func (h *Handler) GetVersionDetails(w http.ResponseWriter, r *http.Request) {
	if !h.verifyTokenA(w, r) {
		return
	}

	if version := chi.URLParam(r, "version"); version != "" && version != SupportedOCPIVersion {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnsupportedVersion,
			fmt.Sprintf("Version %q is not supported; this hub only serves %s", version, SupportedOCPIVersion))
		return
	}

	scheme := resolveScheme(r)
	host := resolveHost(r)
	base := fmt.Sprintf("%s://%s/ocpi/2.2.1", scheme, host)

	details := map[string]any{
		"version": "2.2.1",
		"endpoints": []map[string]string{
			{"identifier": "credentials", "role": "RECEIVER", "url": base + "/credentials"},
			{"identifier": "locations", "role": "SENDER", "url": base + "/sender/locations"},
			{"identifier": "tariffs", "role": "SENDER", "url": base + "/sender/tariffs"},
			{"identifier": "sessions", "role": "SENDER", "url": base + "/sender/sessions"},
			{"identifier": "cdrs", "role": "SENDER", "url": base + "/sender/cdrs"},
			{"identifier": "tokens", "role": "SENDER", "url": base + "/sender/tokens"},
			{"identifier": "tokens", "role": "RECEIVER", "url": base + "/receiver/tokens"},
			{"identifier": "commands", "role": "RECEIVER", "url": base + "/receiver/commands"},
			{"identifier": "sessions", "role": "RECEIVER", "url": base + "/receiver/sessions"},
			{"identifier": "cdrs", "role": "RECEIVER", "url": base + "/receiver/cdrs"},
			{"identifier": "chargingprofiles", "role": "RECEIVER", "url": base + "/receiver/chargingprofiles"},
			{"identifier": "hubclientinfo", "role": "SENDER", "url": base + "/sender/hubclientinfo"},
		},
	}
	ocpiutil.OK(w, r, details)
}

// verifyTokenA checks the Authorization header against Token A.
// Returns true if valid, writes an error response and returns false otherwise.
// Also accepts Token B from the active local session or from persisted
// correctness-session party state so follow-up discovery requests can succeed
// immediately across instances.
func (h *Handler) verifyTokenA(w http.ResponseWriter, r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if len(ocpiutil.AuthTokenCandidates(authHeader)) == 0 {
		ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Missing authorization token")
		return false
	}

	if ocpiutil.AuthHeaderMatchesToken(authHeader, h.Config.TokenA) {
		return true
	}

	// Also accept Token B for already-handshaked clients querying versions.
	tokenB, _ := h.storeForRequest(r).GetTokenB()
	if tokenB != "" && ocpiutil.AuthHeaderMatchesToken(authHeader, tokenB) {
		return true
	}
	if h != nil && h.Store != nil {
		for _, candidate := range ocpiutil.AuthTokenCandidates(authHeader) {
			party, _ := h.Store.GetPartyByTokenB(candidate)
			if isCorrectnessSessionParty(party) {
				return true
			}
		}
	}

	ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Invalid authorization token")
	return false
}

func isCorrectnessSessionParty(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}

	var party struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(raw, &party); err != nil {
		return false
	}

	return strings.HasPrefix(strings.TrimSpace(party.Key), "correctness/")
}

func resolveScheme(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		return fwd
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func resolveHost(r *http.Request) string {
	if h := r.Header.Get("X-Rally-Forwarded-Host"); h != "" {
		return h
	}
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return h
	}
	return r.Host
}
