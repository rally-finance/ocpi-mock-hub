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

	from, to := ocpiutil.ParseDateRange(r)
	raw = ocpiutil.FilterRawByLastUpdated(raw, from, to)

	sessions := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		sessions = append(sessions, json.RawMessage(b))
	}

	p := ocpiutil.ParsePaging(r, 50)
	total := len(sessions)
	page := ocpiutil.PaginateSlice(sessions, p)

	if page == nil {
		page = sessions[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}
