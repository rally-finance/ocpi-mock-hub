package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetCDRs(w http.ResponseWriter, r *http.Request) {
	raw, err := h.Store.ListCDRs()
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to list CDRs")
		return
	}

	from, to := ocpiutil.ParseDateRange(r)
	raw = ocpiutil.FilterRawByLastUpdated(raw, from, to)

	cdrs := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		cdrs = append(cdrs, json.RawMessage(b))
	}

	p := ocpiutil.ParsePaging(r, 50)
	total := len(cdrs)
	page := ocpiutil.PaginateSlice(cdrs, p)

	if page == nil {
		page = cdrs[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}
