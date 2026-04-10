package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

type startSessionPayload struct {
	ResponseURL string `json:"response_url"`
	LocationID  string `json:"location_id"`
	EvseUID     string `json:"evse_uid,omitempty"`
	ConnectorID string `json:"connector_id,omitempty"`
	Token       struct {
		CountryCode string `json:"country_code,omitempty"`
		PartyID     string `json:"party_id,omitempty"`
		UID         string `json:"uid"`
		Type        string `json:"type,omitempty"`
		ContractID  string `json:"contract_id,omitempty"`
	} `json:"token"`
}

type stopSessionPayload struct {
	ResponseURL string `json:"response_url,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

// SessionRecord is the JSON stored in KV for a session.
type SessionRecord struct {
	// OCPI session fields
	CountryCode   string  `json:"country_code"`
	PartyID       string  `json:"party_id"`
	ID            string  `json:"id"`
	StartDateTime string  `json:"start_date_time"`
	EndDateTime   *string `json:"end_date_time,omitempty"`
	KWH           float64 `json:"kwh"`
	CDRToken      struct {
		UID        string `json:"uid"`
		Type       string `json:"type"`
		ContractID string `json:"contract_id,omitempty"`
	} `json:"cdr_token"`
	AuthMethod  string  `json:"auth_method"`
	LocationID  string  `json:"location_id"`
	EvseUID     string  `json:"evse_uid"`
	ConnectorID string  `json:"connector_id"`
	Currency    string  `json:"currency"`
	TotalCost   any     `json:"total_cost,omitempty"`
	Status      string  `json:"status"`
	LastUpdated string  `json:"last_updated"`

	// Internal fields for tick processing (not part of OCPI spec)
	ResponseURL  string `json:"_response_url,omitempty"`
	CreatedAt    string `json:"_created_at,omitempty"`
	ActivatedAt  string `json:"_activated_at,omitempty"`
	CallbackSent bool   `json:"_callback_sent,omitempty"`
}

func (h *Handler) PostCommand(w http.ResponseWriter, r *http.Request) {
	command := strings.ToUpper(chi.URLParam(r, "command"))

	mode, _ := h.Store.GetMode()

	switch command {
	case "START_SESSION":
		h.handleStartSession(w, r, mode)
	case "STOP_SESSION":
		h.handleStopSession(w, r, mode)
	default:
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams,
			fmt.Sprintf("Unknown command: %s", command))
	}
}

func (h *Handler) handleStartSession(w http.ResponseWriter, r *http.Request, mode string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var payload startSessionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	if mode == "reject" {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	// Validate location exists in seed.
	loc := h.Seed.LocationByID(payload.LocationID)
	if loc == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject,
			fmt.Sprintf("Location %s not found", payload.LocationID))
		return
	}

	evseUID := payload.EvseUID
	connectorID := payload.ConnectorID
	if evseUID == "" && len(loc.EVSEs) > 0 {
		evseUID = loc.EVSEs[0].UID
		if connectorID == "" && len(loc.EVSEs[0].Connectors) > 0 {
			connectorID = loc.EVSEs[0].Connectors[0].ID
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	sessionID := "MOCK-" + uuid.NewString()[:8]

	session := SessionRecord{
		CountryCode:   loc.CountryCode,
		PartyID:       loc.PartyID,
		ID:            sessionID,
		StartDateTime: now,
		KWH:           0,
		Status:        "PENDING",
		AuthMethod:    "COMMAND",
		LocationID:    loc.ID,
		EvseUID:       evseUID,
		ConnectorID:   connectorID,
		Currency:      "EUR",
		LastUpdated:   now,
		ResponseURL:   payload.ResponseURL,
		CreatedAt:     now,
	}
	session.CDRToken.UID = payload.Token.UID
	session.CDRToken.Type = payload.Token.Type
	if session.CDRToken.Type == "" {
		session.CDRToken.Type = "RFID"
	}
	session.CDRToken.ContractID = payload.Token.ContractID

	data, _ := json.Marshal(session)
	h.Store.PutSession(sessionID, data)

	ocpiutil.OK(w, r, map[string]string{
		"result": "ACCEPTED",
	})
}

func (h *Handler) handleStopSession(w http.ResponseWriter, r *http.Request, mode string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var payload stopSessionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	if mode == "reject" {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	if payload.SessionID != "" {
		raw, _ := h.Store.GetSession(payload.SessionID)
		if raw != nil {
			var session SessionRecord
			if err := json.Unmarshal(raw, &session); err == nil {
				now := time.Now().UTC().Format(time.RFC3339)
				session.Status = "STOPPING"
				session.LastUpdated = now
				if payload.ResponseURL != "" {
					session.ResponseURL = payload.ResponseURL
					session.CallbackSent = false
				}
				data, _ := json.Marshal(session)
				h.Store.PutSession(session.ID, data)
			}
		}
	}

	ocpiutil.OK(w, r, map[string]string{
		"result": "ACCEPTED",
	})
}
