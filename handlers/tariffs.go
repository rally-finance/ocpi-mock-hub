package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetTariffs(w http.ResponseWriter, r *http.Request) {
	seed := h.seedForRequest(r)
	tariffs := seed.Tariffs

	toCountry := strings.ToUpper(r.Header.Get("OCPI-To-Country-Code"))
	toParty := strings.ToUpper(r.Header.Get("OCPI-To-Party-Id"))
	if toCountry != "" && toParty != "" &&
		!(toCountry == h.Config.HubCountry && toParty == h.Config.HubParty) {
		tariffs = seed.TariffsByParty(toCountry, toParty)
	}

	from, to := ocpiutil.ParseDateRange(r)
	tariffs = ocpiutil.FilterByLastUpdated(tariffs, func(t fakegen.Tariff) string { return t.LastUpdated }, from, to)

	p := h.parsePaging(r, 50)
	total := len(tariffs)
	page := ocpiutil.PaginateSlice(tariffs, p)

	if page == nil {
		page = tariffs[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}

func (h *Handler) GetTariff(w http.ResponseWriter, r *http.Request) {
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	tid := chi.URLParam(r, "tariffID")

	t := h.seedForRequest(r).TariffByID(cc, pid, tid)
	if t == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Tariff not found")
		return
	}
	ocpiutil.OK(w, r, t)
}
