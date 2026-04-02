package handlers

import (
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetHubClientInfo(w http.ResponseWriter, r *http.Request) {
	ocpiutil.OK(w, r, h.Seed.HubClientInfo)
}
