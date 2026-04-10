package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetCDRByID(w http.ResponseWriter, r *http.Request) {
	countryCode := strings.ToUpper(chi.URLParam(r, "countryCode"))
	partyID := strings.ToUpper(chi.URLParam(r, "partyID"))
	cdrID := chi.URLParam(r, "cdrID")

	raw, err := h.Store.GetCDR(cdrID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to get CDR")
		return
	}
	if raw == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "CDR not found")
		return
	}

	var party struct {
		CountryCode string `json:"country_code"`
		PartyID     string `json:"party_id"`
	}
	if err := json.Unmarshal(raw, &party); err == nil {
		if strings.ToUpper(party.CountryCode) != countryCode || strings.ToUpper(party.PartyID) != partyID {
			ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "CDR not found")
			return
		}
	}

	ocpiutil.OK(w, r, json.RawMessage(raw))
}

func (h *Handler) GetCDRs(w http.ResponseWriter, r *http.Request) {
	raw, err := h.Store.ListCDRs()
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to list CDRs")
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

	cdrs := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		cdrs = append(cdrs, json.RawMessage(b))
	}

	p := ocpiutil.ParsePaging(r, 50)
	total := len(cdrs)
	page := ocpiutil.PaginateSlice(cdrs, p)

	if page == nil {
		page = cdrs[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}
