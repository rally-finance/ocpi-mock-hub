package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetTokens(w http.ResponseWriter, r *http.Request) {
	raw, err := h.Store.ListTokens()
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to list tokens")
		return
	}

	from, to := ocpiutil.ParseDateRange(r)
	raw = ocpiutil.FilterRawByLastUpdated(raw, from, to)

	tokens := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		tokens = append(tokens, json.RawMessage(b))
	}

	p := ocpiutil.ParsePaging(r, 50)
	total := len(tokens)
	page := ocpiutil.PaginateSlice(tokens, p)

	if page == nil {
		page = tokens[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}

func (h *Handler) GetTokenByID(w http.ResponseWriter, r *http.Request) {
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")

	raw, err := h.Store.GetToken(cc, pid, uid)
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
}

func (h *Handler) PostTokenAuthorize(w http.ResponseWriter, r *http.Request) {
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")

	body, _ := io.ReadAll(r.Body)
	var req authorizeRequest
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}

	mode, _ := h.Store.GetMode()
	if mode == "reject" {
		ocpiutil.OK(w, r, map[string]any{
			"allowed": "NOT_ALLOWED",
			"token":   map[string]string{"country_code": cc, "party_id": pid, "uid": uid},
		})
		return
	}

	raw, _ := h.Store.GetToken(cc, pid, uid)
	if raw == nil {
		ocpiutil.OK(w, r, map[string]any{
			"allowed": "NOT_ALLOWED",
			"token":   map[string]string{"country_code": cc, "party_id": pid, "uid": uid},
		})
		return
	}

	var tokenData map[string]any
	json.Unmarshal(raw, &tokenData)

	result := map[string]any{
		"allowed": "ALLOWED",
		"token":   map[string]string{"country_code": cc, "party_id": pid, "uid": uid},
	}

	if req.LocationID != "" {
		loc := h.Seed.LocationByID(req.LocationID)
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
