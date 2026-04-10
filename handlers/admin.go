package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) SetMode(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var payload struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Mode == "" {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid mode")
		return
	}

	valid := map[string]bool{
		"happy": true, "slow": true, "reject": true,
		"partial": true, "pagination-stress": true,
		"rate-limit": true, "random-500": true, "auth-fail": true,
	}
	if !valid[payload.Mode] {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams,
			"Mode must be: happy, slow, reject, partial, pagination-stress, rate-limit, random-500, auth-fail")
		return
	}

	h.Store.SetMode(payload.Mode)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mode": payload.Mode})
}

func (h *Handler) GetMode(w http.ResponseWriter, r *http.Request) {
	mode, _ := h.Store.GetMode()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mode": mode})
}
