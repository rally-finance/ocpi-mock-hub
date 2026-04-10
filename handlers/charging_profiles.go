package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) PutChargingProfile(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	raw, err := h.Store.GetSession(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to check session")
		return
	}
	if raw == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Session not found")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var payload struct {
		ChargingProfile json.RawMessage `json:"charging_profile"`
		ResponseURL     string          `json:"response_url"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	h.Store.PutChargingProfile(sessionID, payload.ChargingProfile)
	ocpiutil.OK(w, r, map[string]string{"result": "ACCEPTED"})

	if payload.ResponseURL != "" {
		go func() {
			callback := map[string]any{
				"result": "ACCEPTED",
				"profile": map[string]any{
					"charging_profile": json.RawMessage(payload.ChargingProfile),
				},
			}
			cbData, _ := json.Marshal(callback)
			req, err := http.NewRequest("POST", payload.ResponseURL, bytes.NewReader(cbData))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if token, _ := h.Store.GetEMSPOwnToken(); token != "" {
				req.Header.Set("Authorization", "Token "+token)
			}
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
}

func (h *Handler) GetChargingProfile(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	profile, err := h.Store.GetChargingProfile(sessionID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to get profile")
		return
	}

	result := map[string]any{"result": "ACCEPTED"}
	if profile != nil {
		result["profile"] = map[string]any{
			"charging_profile": json.RawMessage(profile),
		}
	} else {
		result["profile"] = map[string]any{
			"charging_profile": map[string]any{
				"charging_rate_unit":     "W",
				"charging_profile_period": []map[string]any{},
			},
		}
	}

	ocpiutil.OK(w, r, result)
}

func (h *Handler) DeleteChargingProfile(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.Store.DeleteChargingProfile(sessionID)
	ocpiutil.OK(w, r, map[string]string{"result": "ACCEPTED"})
}
