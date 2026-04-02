package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetTariffs(w http.ResponseWriter, r *http.Request) {
	tariffs := h.Seed.Tariffs

	toCountry := strings.ToUpper(r.Header.Get("OCPI-To-Country-Code"))
	toParty := strings.ToUpper(r.Header.Get("OCPI-To-Party-Id"))
	if toCountry != "" && toParty != "" &&
		!(toCountry == h.Config.HubCountry && toParty == h.Config.HubParty) {
		tariffs = h.Seed.TariffsByParty(toCountry, toParty)
	}

	p := ocpiutil.ParsePaging(r, 50)
	page := ocpiutil.PaginateSlice(tariffs, p)

	if page == nil {
		page = h.Seed.Tariffs[:0]
	}

	var extra []http.Header
	if link := ocpiutil.BuildLinkHeader(r, p, len(page), len(tariffs)); link != nil {
		extra = append(extra, link)
	}

	ocpiutil.OK(w, r, page, extra...)
}

func (h *Handler) GetTariff(w http.ResponseWriter, r *http.Request) {
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	tid := chi.URLParam(r, "tariffID")

	t := h.Seed.TariffByID(cc, pid, tid)
	if t == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Tariff not found")
		return
	}
	ocpiutil.OK(w, r, t)
}
