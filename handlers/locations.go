package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetLocations(w http.ResponseWriter, r *http.Request) {
	locations := h.Seed.Locations

	toCountry := strings.ToUpper(r.Header.Get("OCPI-To-Country-Code"))
	toParty := strings.ToUpper(r.Header.Get("OCPI-To-Party-Id"))
	if toCountry != "" && toParty != "" &&
		!(toCountry == h.Config.HubCountry && toParty == h.Config.HubParty) {
		locations = h.Seed.LocationsByParty(toCountry, toParty)
	}

	p := ocpiutil.ParsePaging(r, 50)
	page := ocpiutil.PaginateSlice(locations, p)

	if page == nil {
		page = h.Seed.Locations[:0]
	}

	var extra []http.Header
	if link := ocpiutil.BuildLinkHeader(r, p, len(page), len(locations)); link != nil {
		extra = append(extra, link)
	}

	ocpiutil.OK(w, r, page, extra...)
}

func (h *Handler) GetLocation(w http.ResponseWriter, r *http.Request) {
	locationID := chi.URLParam(r, "locationID")
	loc := h.Seed.LocationByID(locationID)
	if loc == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Location not found")
		return
	}
	ocpiutil.OK(w, r, loc)
}
