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

	cdrs := make([]json.RawMessage, 0, len(raw))
	for _, b := range raw {
		cdrs = append(cdrs, json.RawMessage(b))
	}

	p := ocpiutil.ParsePaging(r, 50)
	page := ocpiutil.PaginateSlice(cdrs, p)

	if page == nil {
		page = cdrs[:0]
	}

	var extra []http.Header
	if link := ocpiutil.BuildLinkHeader(r, p, len(page), len(cdrs)); link != nil {
		extra = append(extra, link)
	}

	ocpiutil.OK(w, r, page, extra...)
}
