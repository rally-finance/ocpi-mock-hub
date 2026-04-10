package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rally-finance/ocpi-mock-hub/simulation"
)

type connectionStatus struct {
	Connected        bool   `json:"connected"`
	TokenB           string `json:"token_b"`
	TokenBMasked     string `json:"token_b_masked"`
	EMSPCallbackURL  string `json:"emsp_callback_url"`
	EMSPVersionsURL  string `json:"emsp_versions_url"`
	EMSPOwnToken     string `json:"emsp_own_token_masked"`
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

func maskToken(t string) string {
	if len(t) <= 8 {
		return "****"
	}
	return t[:4] + "..." + t[len(t)-4:]
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

	status := connectionStatus{
		Connected:        tokenB != "",
		TokenB:           tokenB,
		TokenBMasked:     maskToken(tokenB),
		EMSPCallbackURL:  callbackURL,
		EMSPVersionsURL:  versionsURL,
		EMSPOwnToken:     maskToken(emspOwnToken),
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

	locs := make([]locationSummary, 0, len(h.Seed.Locations))
	for _, loc := range h.Seed.Locations {
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
		LocationID  string `json:"location_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	mode, _ := h.Store.GetMode()
	if mode == "reject" {
		writeJSON(w, http.StatusOK, map[string]any{
			"allowed": "NOT_ALLOWED",
			"token":   map[string]string{"country_code": req.CountryCode, "party_id": req.PartyID, "uid": req.UID},
		})
		return
	}

	raw, _ := h.Store.GetToken(req.CountryCode, req.PartyID, req.UID)
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
	h.Store.SetTokenB("")
	h.Store.SetEMSPCallbackURL("")
	h.Store.SetEMSPCredentials(nil)
	h.Store.SetEMSPOwnToken("")
	h.Store.SetEMSPVersionsURL("")

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (h *Handler) TriggerTick(w http.ResponseWriter, r *http.Request) {
	sim := simulation.New(
		h.Store,
		h.Seed,
		h.Config.EMSPCallbackURL,
		h.Config.CommandDelayMS,
		h.Config.SessionDurationS,
	)

	if err := sim.Tick(); err != nil {
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
