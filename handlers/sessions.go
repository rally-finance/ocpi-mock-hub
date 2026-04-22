package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) PutReceiverSession(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	sessionID := chi.URLParam(r, "sessionID")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	parsed["id"] = sessionID
	data, _ := json.Marshal(parsed)
	store.PutSession(sessionID, data)

	ocpiutil.OK(w, r, nil)
}

// PatchReceiverSession handles PATCH /ocpi/2.2.1/receiver/sessions/{cc}/{pid}/{sessionID}.
// The body is a partial Session object merged onto the existing one. Spec
// peculiarity: `charging_periods`, if present and non-empty, is *appended* to
// the existing list rather than replacing it. Missing or empty
// charging_periods leaves the list untouched.
func (h *Handler) PatchReceiverSession(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	countryCode := strings.ToUpper(chi.URLParam(r, "countryCode"))
	partyID := strings.ToUpper(chi.URLParam(r, "partyID"))
	sessionID := chi.URLParam(r, "sessionID")

	existing, err := store.GetSession(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to read session")
		return
	}
	if existing == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Session not found")
		return
	}

	// Enforce routing: PATCH must target the party that originally owns the session.
	var party struct {
		CountryCode string `json:"country_code"`
		PartyID     string `json:"party_id"`
	}
	if err := json.Unmarshal(existing, &party); err == nil {
		if strings.ToUpper(party.CountryCode) != countryCode || strings.ToUpper(party.PartyID) != partyID {
			ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Session not found")
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}
	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}
	if _, ok := patch["last_updated"]; !ok {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "PATCH body must include last_updated")
		return
	}

	var target map[string]any
	if err := json.Unmarshal(existing, &target); err != nil {
		target = map[string]any{}
	}

	// Pull charging_periods out for append semantics.
	var patchPeriods []any
	if raw, ok := patch["charging_periods"]; ok {
		if asList, ok := raw.([]any); ok {
			patchPeriods = asList
		}
		delete(patch, "charging_periods")
	}

	for k, v := range patch {
		target[k] = v
	}

	if len(patchPeriods) > 0 {
		var existingPeriods []any
		if raw, ok := target["charging_periods"]; ok {
			if asList, ok := raw.([]any); ok {
				existingPeriods = asList
			}
		}
		target["charging_periods"] = append(existingPeriods, patchPeriods...)
	}

	target["id"] = sessionID

	merged, err := json.Marshal(target)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to encode session")
		return
	}
	if err := store.PutSession(sessionID, merged); err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to store session")
		return
	}
	ocpiutil.OK(w, r, nil)
}

func (h *Handler) GetSessionByID(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	countryCode := strings.ToUpper(chi.URLParam(r, "countryCode"))
	partyID := strings.ToUpper(chi.URLParam(r, "partyID"))
	sessionID := chi.URLParam(r, "sessionID")

	raw, err := store.GetSession(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to get session")
		return
	}
	if raw == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Session not found")
		return
	}

	var party struct {
		CountryCode string `json:"country_code"`
		PartyID     string `json:"party_id"`
	}
	if err := json.Unmarshal(raw, &party); err == nil {
		if strings.ToUpper(party.CountryCode) != countryCode || strings.ToUpper(party.PartyID) != partyID {
			ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Session not found")
			return
		}
	}

	ocpiutil.OK(w, r, json.RawMessage(raw))
}

func (h *Handler) GetSessions(w http.ResponseWriter, r *http.Request) {
	raw, err := h.storeForRequest(r).ListSessions()
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to list sessions")
		return
	}

	toCountry := strings.ToUpper(r.Header.Get("OCPI-To-Country-Code"))
	toParty := strings.ToUpper(r.Header.Get("OCPI-To-Party-Id"))
	if toCountry != "" && toParty != "" &&
		!(toCountry == h.Config.HubCountry && toParty == h.Config.HubParty) {
		raw = filterRawByParty(raw, toCountry, toParty)
	}

	from, to := ocpiutil.ParseDateRange(r)
	raw = ocpiutil.FilterRawByLastUpdated(raw, from, to)

	sessions := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		sessions = append(sessions, json.RawMessage(b))
	}

	p := h.parsePaging(r, 50)
	total := len(sessions)
	page := ocpiutil.PaginateSlice(sessions, p)

	if page == nil {
		page = sessions[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}
