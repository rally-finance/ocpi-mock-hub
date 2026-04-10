package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) PostReceiverCDR(w http.ResponseWriter, r *http.Request) {
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

	id, _ := parsed["id"].(string)
	if id == "" {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Missing id field")
		return
	}

	h.Store.PutCDR(id, body)

	scheme := resolveScheme(r)
	host := resolveHost(r)
	w.Header().Set("Location", fmt.Sprintf("%s://%s/ocpi/2.2.1/receiver/cdrs/%s", scheme, host, id))
	ocpiutil.Write(w, r, http.StatusCreated, ocpiutil.Response{
		StatusCode: ocpiutil.StatusSuccess,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) GetReceiverCDR(w http.ResponseWriter, r *http.Request) {
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
	ocpiutil.OK(w, r, json.RawMessage(raw))
}

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

	p := h.parsePaging(r, 50)
	total := len(cdrs)
	page := ocpiutil.PaginateSlice(cdrs, p)

	if page == nil {
		page = cdrs[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}
