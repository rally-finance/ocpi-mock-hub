package handlers

import (
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetHubClientInfo(w http.ResponseWriter, r *http.Request) {
	items := h.Seed.HubClientInfo

	p := ocpiutil.ParsePaging(r, 50)
	total := len(items)
	page := ocpiutil.PaginateSlice(items, p)

	if page == nil {
		page = items[:0]
	}

	headers := ocpiutil.BuildPagingHeaders(r, p, len(page), total)
	ocpiutil.OK(w, r, page, headers)
}
