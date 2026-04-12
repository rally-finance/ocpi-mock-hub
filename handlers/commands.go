package handlers

import (
	"bytes"
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

type reserveNowPayload struct {
	ResponseURL   string `json:"response_url"`
	ReservationID string `json:"reservation_id"`
	LocationID    string `json:"location_id"`
	EvseUID       string `json:"evse_uid,omitempty"`
	ExpiryDate    string `json:"expiry_date"`
	Token         struct {
		CountryCode string `json:"country_code,omitempty"`
		PartyID     string `json:"party_id,omitempty"`
		UID         string `json:"uid"`
		Type        string `json:"type,omitempty"`
		ContractID  string `json:"contract_id,omitempty"`
	} `json:"token"`
}

type cancelReservationPayload struct {
	ResponseURL   string `json:"response_url,omitempty"`
	ReservationID string `json:"reservation_id"`
}

type unlockConnectorPayload struct {
	ResponseURL string `json:"response_url"`
	LocationID  string `json:"location_id"`
	EvseUID     string `json:"evse_uid"`
	ConnectorID string `json:"connector_id"`
}

// ReservationRecord is the JSON stored in KV for a reservation.
type ReservationRecord struct {
	ID           string `json:"id"`
	LocationID   string `json:"location_id"`
	EvseUID      string `json:"evse_uid"`
	ExpiryDate   string `json:"expiry_date"`
	Status       string `json:"status"`
	TokenUID     string `json:"token_uid"`
	LastUpdated  string `json:"last_updated"`
	ResponseURL  string `json:"_response_url,omitempty"`
	CreatedAt    string `json:"_created_at,omitempty"`
	CallbackSent bool   `json:"_callback_sent,omitempty"`
}

// SessionRecord is an alias for the shared type.
type SessionRecord = ocpiutil.SessionRecord

func (h *Handler) PostCommand(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	command := strings.ToUpper(chi.URLParam(r, "command"))

	mode, _ := store.GetMode()

	switch command {
	case "START_SESSION":
		h.handleStartSession(w, r, store, mode)
	case "STOP_SESSION":
		h.handleStopSession(w, r, store, mode)
	case "RESERVE_NOW":
		h.handleReserveNow(w, r, store, mode)
	case "CANCEL_RESERVATION":
		h.handleCancelReservation(w, r, store, mode)
	case "UNLOCK_CONNECTOR":
		h.handleUnlockConnector(w, r, store, mode)
	default:
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams,
			fmt.Sprintf("Unknown command: %s", command))
	}
}

func (h *Handler) handleStartSession(w http.ResponseWriter, r *http.Request, store Store, mode string) {
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
	loc := h.seedForRequest(r).LocationByID(payload.LocationID)
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
		MeterID:       "METER-" + evseUID,
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
	store.PutSession(sessionID, data)

	ocpiutil.OK(w, r, map[string]string{
		"result": "ACCEPTED",
	})
}

func (h *Handler) handleStopSession(w http.ResponseWriter, r *http.Request, store Store, mode string) {
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

	if payload.SessionID == "" {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	raw, _ := store.GetSession(payload.SessionID)
	if raw == nil {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	var session SessionRecord
	if err := json.Unmarshal(raw, &session); err == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		session.Status = "STOPPING"
		session.LastUpdated = now
		// A STOP_SESSION may carry its own response_url for the async
		// command result callback.  Reset CallbackSent so the tick loop
		// sends the stop-result callback (distinct from the earlier
		// start-result callback that was tracked by the same flag).
		if payload.ResponseURL != "" {
			session.ResponseURL = payload.ResponseURL
			session.CallbackSent = false
		}
		data, _ := json.Marshal(session)
		store.PutSession(session.ID, data)
	}

	ocpiutil.OK(w, r, map[string]string{"result": "ACCEPTED"})
}

func (h *Handler) handleReserveNow(w http.ResponseWriter, r *http.Request, store Store, mode string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var payload reserveNowPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	if mode == "reject" {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	loc := h.seedForRequest(r).LocationByID(payload.LocationID)
	if loc == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject,
			fmt.Sprintf("Location %s not found", payload.LocationID))
		return
	}

	evseUID := payload.EvseUID
	if evseUID == "" && len(loc.EVSEs) > 0 {
		evseUID = loc.EVSEs[0].UID
	}

	now := time.Now().UTC().Format(time.RFC3339)
	resID := payload.ReservationID
	if resID == "" {
		resID = "RES-" + uuid.NewString()[:8]
	}

	reservation := ReservationRecord{
		ID:          resID,
		LocationID:  loc.ID,
		EvseUID:     evseUID,
		ExpiryDate:  payload.ExpiryDate,
		Status:      "RESERVED",
		TokenUID:    payload.Token.UID,
		LastUpdated: now,
		ResponseURL: payload.ResponseURL,
		CreatedAt:   now,
	}

	data, _ := json.Marshal(reservation)
	store.PutReservation(resID, data)

	ocpiutil.OK(w, r, map[string]string{"result": "ACCEPTED"})
}

func (h *Handler) handleCancelReservation(w http.ResponseWriter, r *http.Request, store Store, mode string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var payload cancelReservationPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	if mode == "reject" {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	raw, _ := store.GetReservation(payload.ReservationID)
	if raw == nil {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	store.DeleteReservation(payload.ReservationID)
	ocpiutil.OK(w, r, map[string]string{"result": "ACCEPTED"})
}

func (h *Handler) handleUnlockConnector(w http.ResponseWriter, r *http.Request, store Store, mode string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var payload unlockConnectorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}

	if mode == "reject" {
		ocpiutil.OK(w, r, map[string]string{"result": "REJECTED"})
		return
	}

	loc := h.seedForRequest(r).LocationByID(payload.LocationID)
	if loc == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject,
			fmt.Sprintf("Location %s not found", payload.LocationID))
		return
	}

	if payload.ResponseURL != "" {
		go func() {
			callback := map[string]any{"result": "ACCEPTED"}
			cbData, _ := json.Marshal(callback)
			req, err := http.NewRequestWithContext(h.outboundContext("unlock_connector_callback"), "POST", payload.ResponseURL, bytes.NewReader(cbData))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if token, _ := store.GetEMSPOwnToken(); token != "" {
				req.Header.Set("Authorization", "Token "+token)
			}
			resp, err := h.outboundClient().Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}

	ocpiutil.OK(w, r, map[string]string{"result": "ACCEPTED"})
}
