package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/correctness"
)

func (h *Handler) GetCorrectnessSuites(w http.ResponseWriter, r *http.Request) {
	if h.Correctness == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, h.Correctness.ListSuites())
}

func (h *Handler) ListCorrectnessSessions(w http.ResponseWriter, r *http.Request) {
	if h.Correctness == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, h.Correctness.ListSessions())
}

func (h *Handler) GetCorrectnessSession(w http.ResponseWriter, r *http.Request) {
	if h.Correctness == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "correctness manager not configured"})
		return
	}
	sessionID := chi.URLParam(r, "sessionID")
	session, err := h.Correctness.GetSession(sessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) CreateCorrectnessSession(w http.ResponseWriter, r *http.Request) {
	if h.Correctness == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "correctness manager not configured"})
		return
	}
	var payload correctness.SessionConfig
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if payload.PeerVersionsURL == "" || payload.PeerToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "peer_versions_url and peer_token are required"})
		return
	}
	session, err := h.Correctness.StartSession(payload)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, correctness.ErrActiveSessionExists) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) RunCorrectnessAction(w http.ResponseWriter, r *http.Request) {
	if h.Correctness == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "correctness manager not configured"})
		return
	}
	sessionID := chi.URLParam(r, "sessionID")
	actionID := chi.URLParam(r, "actionID")

	if err := h.Correctness.MarkActionStarted(sessionID, actionID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	output, err := h.executeCorrectnessAction(r, sessionID, actionID)
	if err != nil {
		session, markErr := h.Correctness.FailAction(sessionID, actionID, err)
		if markErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": markErr.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, session)
		return
	}

	session, err := h.Correctness.CompleteAction(sessionID, actionID, output)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) SubmitCorrectnessCheckpoint(w http.ResponseWriter, r *http.Request) {
	if h.Correctness == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "correctness manager not configured"})
		return
	}
	sessionID := chi.URLParam(r, "sessionID")
	checkpointID := chi.URLParam(r, "checkpointID")
	var payload struct {
		Answer string `json:"answer"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	session, err := h.Correctness.SubmitCheckpoint(sessionID, checkpointID, payload.Answer, payload.Notes)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) RerunCorrectnessSession(w http.ResponseWriter, r *http.Request) {
	if h.Correctness == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "correctness manager not configured"})
		return
	}
	sessionID := chi.URLParam(r, "sessionID")
	session, err := h.Correctness.RerunSession(sessionID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, correctness.ErrActiveSessionExists) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, session)
}
