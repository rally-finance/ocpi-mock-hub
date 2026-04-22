package handlers

import (
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetTokens(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	raw, err := store.ListTokens()
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to list tokens")
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

	tokens := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		tokens = append(tokens, json.RawMessage(b))
	}

	p := h.parsePaging(r, 50)
	total := len(tokens)
	page := ocpiutil.PaginateSlice(tokens, p)

	if page == nil {
		page = tokens[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}

func (h *Handler) GetTokenByID(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")
	tokenType := tokenTypeFromRequest(r)

	raw, err := store.GetToken(cc, pid, tokenStorageUID(uid, tokenType))
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to get token")
		return
	}
	if raw == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Token not found")
		return
	}

	ocpiutil.OK(w, r, json.RawMessage(raw))
}

type authorizeRequest struct {
	LocationID string `json:"location_id,omitempty"`
	// OCPI spec field: full LocationReferences object. Presence is required
	// when the token's whitelist forbids offline authorization.
	LocationReferences *struct {
		LocationID string   `json:"location_id"`
		EVSEUIDs   []string `json:"evse_uids,omitempty"`
	} `json:"location_references,omitempty"`
}

func (h *Handler) PostTokenAuthorize(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")
	tokenType := tokenTypeFromRequest(r)
	storageUID := tokenStorageUID(uid, tokenType)

	body, _ := io.ReadAll(r.Body)
	var req authorizeRequest
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}
	// Backfill LocationReferences from the flat location_id field for backwards compat.
	if req.LocationReferences == nil && req.LocationID != "" {
		req.LocationReferences = &struct {
			LocationID string   `json:"location_id"`
			EVSEUIDs   []string `json:"evse_uids,omitempty"`
		}{LocationID: req.LocationID}
	}

	mode, _ := store.GetMode()
	if mode == "reject" {
		ocpiutil.OK(w, r, map[string]any{
			"allowed": "NOT_ALLOWED",
			"token":   map[string]string{"country_code": cc, "party_id": pid, "uid": uid},
		})
		return
	}

	if mode == "auth-fail" {
		statuses := []string{"NOT_ALLOWED", "EXPIRED", "BLOCKED"}
		ocpiutil.OK(w, r, map[string]any{
			"allowed": statuses[rand.Intn(len(statuses))],
			"token":   map[string]string{"country_code": cc, "party_id": pid, "uid": uid},
		})
		return
	}

	raw, _ := store.GetToken(cc, pid, storageUID)
	if raw == nil {
		ocpiutil.OK(w, r, map[string]any{
			"allowed": "NOT_ALLOWED",
			"token":   map[string]string{"country_code": cc, "party_id": pid, "uid": uid},
		})
		return
	}

	var tokenData map[string]any
	json.Unmarshal(raw, &tokenData)

	// Tokens with whitelist=NEVER require a LocationReferences object on every
	// real-time authorization request (spec 4.4.2). Missing body → 2002.
	if whitelist, _ := tokenData["whitelist"].(string); whitelist == "NEVER" && req.LocationReferences == nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusNotEnoughInfo,
			"Token requires LocationReferences for real-time authorization")
		return
	}

	locationID := ""
	if req.LocationReferences != nil {
		locationID = req.LocationReferences.LocationID
	}

	result := map[string]any{
		"allowed": "ALLOWED",
		"token":   map[string]string{"country_code": cc, "party_id": pid, "uid": uid},
	}

	if locationID != "" {
		loc := h.seedForRequest(r).LocationByID(locationID)
		if loc != nil && len(loc.EVSEs) > 0 {
			evseUIDs := make([]string, 0, len(loc.EVSEs))
			for _, e := range loc.EVSEs {
				evseUIDs = append(evseUIDs, e.UID)
			}
			result["location"] = map[string]any{"evse_uids": evseUIDs}
		}
	}

	ocpiutil.OK(w, r, result)
}
