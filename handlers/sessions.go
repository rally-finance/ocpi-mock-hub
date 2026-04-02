package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetSessions(w http.ResponseWriter, r *http.Request) {
	raw, err := h.Store.ListSessions()
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to list sessions")
		return
	}

	sessions := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		sessions = append(sessions, json.RawMessage(b))
	}

	p := ocpiutil.ParsePaging(r, 50)
	page := ocpiutil.PaginateSlice(sessions, p)

	if page == nil {
		page = sessions[:0]
	}

	var extra []http.Header
	if link := ocpiutil.BuildLinkHeader(r, p, len(page), len(sessions)); link != nil {
		extra = append(extra, link)
	}

	ocpiutil.OK(w, r, page, extra...)
}
