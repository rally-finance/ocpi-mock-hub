package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
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

	from, to := ocpiutil.ParseDateRange(r)
	locations = ocpiutil.FilterByLastUpdated(locations, func(l fakegen.Location) string { return l.LastUpdated }, from, to)

	p := ocpiutil.ParsePaging(r, 50)
	total := len(locations)
	page := ocpiutil.PaginateSlice(locations, p)

	if page == nil {
		page = locations[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
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
