package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

// OCPI 2.2.1 Charging Preferences response enum (§11.5.2.1).
const (
	chargingPreferencesAccepted       = "ACCEPTED"
	chargingPreferencesDepartureReqd  = "DEPARTURE_REQUIRED"
	chargingPreferencesEnergyNeedReqd = "ENERGY_NEED_REQUIRED"
	chargingPreferencesNotPossible    = "NOT_POSSIBLE"
	chargingPreferencesProfileNotSup  = "PROFILE_TYPE_NOT_SUPPORTED"

	capabilityChargingPreferences = "CHARGING_PREFERENCES_CAPABLE"
)

// chargingPreferences mirrors the OCPI ChargingPreferences object (§11.4.1).
type chargingPreferences struct {
	ProfileType      string   `json:"profile_type"`
	DepartureTime    string   `json:"departure_time,omitempty"`
	EnergyNeed       *float64 `json:"energy_need,omitempty"`
	DischargeAllowed *bool    `json:"discharge_allowed,omitempty"`
}

var supportedProfileTypes = map[string]bool{
	"REGULAR": true,
	"GREEN":   true,
	"COMFORT": true,
	"CHEAP":   true,
}

// PutChargingPreferences handles
// PUT /ocpi/2.2.1/sender/sessions/{sessionID}/charging_preferences.
//
// The endpoint validates the target session is ACTIVE, confirms the associated
// EVSE advertises CHARGING_PREFERENCES_CAPABLE, performs the per-profile-type
// required-field checks from §11.5.2.1, and persists accepted preferences onto
// the session so the lifecycle simulation can consume them later.
func (h *Handler) PutChargingPreferences(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	sessionID := chi.URLParam(r, "sessionID")

	raw, err := store.GetSession(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to read session")
		return
	}
	if raw == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Session not found")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}
	var prefs chargingPreferences
	if err := json.Unmarshal(body, &prefs); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	profileType := strings.ToUpper(strings.TrimSpace(prefs.ProfileType))
	if profileType == "" {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "profile_type is required")
		return
	}
	if !supportedProfileTypes[profileType] {
		ocpiutil.OK(w, r, chargingPreferencesProfileNotSup)
		return
	}

	var session map[string]any
	_ = json.Unmarshal(raw, &session)

	status, _ := session["status"].(string)
	if strings.ToUpper(status) != "ACTIVE" {
		ocpiutil.OK(w, r, chargingPreferencesNotPossible)
		return
	}

	if !sessionEVSESupportsChargingPreferences(session, h.seedForRequest(r)) {
		ocpiutil.OK(w, r, chargingPreferencesNotPossible)
		return
	}

	// Per-profile-type required-field gating (§11.5.2.1).
	switch profileType {
	case "CHEAP", "COMFORT", "GREEN":
		if prefs.DepartureTime == "" {
			ocpiutil.OK(w, r, chargingPreferencesDepartureReqd)
			return
		}
	}
	if profileType == "CHEAP" || profileType == "COMFORT" {
		if prefs.EnergyNeed == nil {
			ocpiutil.OK(w, r, chargingPreferencesEnergyNeedReqd)
			return
		}
	}

	// Persist accepted preferences onto the session so the simulation layer
	// can factor them in (e.g. CHEAP could pick a cheaper tariff in Stage 4).
	session["charging_preferences"] = prefs
	merged, err := json.Marshal(session)
	if err == nil {
		_ = store.PutSession(sessionID, merged)
	}

	ocpiutil.OK(w, r, chargingPreferencesAccepted)
}

// sessionEVSESupportsChargingPreferences resolves the session's EVSE through
// the seed data and reports whether it advertises the required capability.
// Sessions that reference an unknown EVSE conservatively return false so the
// handler returns NOT_POSSIBLE rather than silently accepting.
func sessionEVSESupportsChargingPreferences(session map[string]any, seed *fakegen.SeedData) bool {
	if seed == nil {
		return false
	}
	evseUID, _ := session["evse_uid"].(string)
	locationID, _ := session["location_id"].(string)
	if evseUID == "" || locationID == "" {
		return false
	}
	_, evse := seed.EVSEByUID(locationID, evseUID)
	if evse == nil {
		return false
	}
	for _, c := range evse.Capabilities {
		if c == capabilityChargingPreferences {
			return true
		}
	}
	return false
}
