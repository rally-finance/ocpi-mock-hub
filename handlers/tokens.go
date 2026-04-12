package handlers

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) PutToken(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	if err := store.PutToken(cc, pid, uid, body); err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to store token")
		return
	}

	ocpiutil.OK(w, r, nil)
}
