package handlers

import (
	"encoding/json"
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

	p := h.parsePaging(r, 50)
	total := len(locations)
	page := ocpiutil.PaginateSlice(locations, p)

	if page == nil {
		page = locations[:0]
	}

	activeEVSEs := h.activeEVSESet()
	overlaid := make([]fakegen.Location, len(page))
	for i, loc := range page {
		overlaid[i] = applyEVSEOverlay(loc, activeEVSEs)
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, overlaid, headers)
}

func (h *Handler) GetLocation(w http.ResponseWriter, r *http.Request) {
	locationID := chi.URLParam(r, "locationID")
	loc := h.Seed.LocationByID(locationID)
	if loc == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Location not found")
		return
	}
	result := applyEVSEOverlay(*loc, h.activeEVSESet())
	ocpiutil.OK(w, r, result)
}

func (h *Handler) GetEVSE(w http.ResponseWriter, r *http.Request) {
	locationID := chi.URLParam(r, "locationID")
	evseUID := chi.URLParam(r, "evseUID")

	loc, evse := h.Seed.EVSEByUID(locationID, evseUID)
	if loc == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Location not found")
		return
	}
	if evse == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "EVSE not found")
		return
	}
	result := *evse
	if h.activeEVSESet()[evse.UID] {
		result.Status = "CHARGING"
	}
	ocpiutil.OK(w, r, result)
}

func (h *Handler) GetConnector(w http.ResponseWriter, r *http.Request) {
	locationID := chi.URLParam(r, "locationID")
	evseUID := chi.URLParam(r, "evseUID")
	connectorID := chi.URLParam(r, "connectorID")

	loc, evse, conn := h.Seed.ConnectorByID(locationID, evseUID, connectorID)
	if loc == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Location not found")
		return
	}
	if evse == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "EVSE not found")
		return
	}
	if conn == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Connector not found")
		return
	}
	ocpiutil.OK(w, r, conn)
}

// activeEVSESet returns the set of EVSE UIDs that have an active or pending session.
// Built once per request to avoid repeated ListSessions calls.
func (h *Handler) activeEVSESet() map[string]bool {
	raw, err := h.Store.ListSessions()
	if err != nil || len(raw) == 0 {
		return nil
	}
	active := make(map[string]bool)
	for _, b := range raw {
		var s struct {
			EvseUID string `json:"evse_uid"`
			Status  string `json:"status"`
		}
		if json.Unmarshal(b, &s) == nil && (s.Status == "ACTIVE" || s.Status == "PENDING") {
			active[s.EvseUID] = true
		}
	}
	return active
}

// applyEVSEOverlay clones a location and sets any EVSE with an active session to CHARGING.
func applyEVSEOverlay(loc fakegen.Location, activeEVSEs map[string]bool) fakegen.Location {
	if len(activeEVSEs) == 0 {
		return loc
	}
	evses := make([]fakegen.EVSE, len(loc.EVSEs))
	copy(evses, loc.EVSEs)
	for i := range evses {
		if activeEVSEs[evses[i].UID] {
			evses[i].Status = "CHARGING"
		}
	}
	loc.EVSEs = evses
	return loc
}
