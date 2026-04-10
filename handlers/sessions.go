package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetSessionByID(w http.ResponseWriter, r *http.Request) {
	countryCode := strings.ToUpper(chi.URLParam(r, "countryCode"))
	partyID := strings.ToUpper(chi.URLParam(r, "partyID"))
	sessionID := chi.URLParam(r, "sessionID")

	raw, err := h.Store.GetSession(sessionID)
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
	raw, err := h.Store.ListSessions()
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

	p := ocpiutil.ParsePaging(r, 50)
	total := len(sessions)
	page := ocpiutil.PaginateSlice(sessions, p)

	if page == nil {
		page = sessions[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}
