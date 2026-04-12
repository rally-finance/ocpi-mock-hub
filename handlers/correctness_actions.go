package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func (h *Handler) executeCorrectnessAction(r *http.Request, sessionID, actionID string) (map[string]string, error) {
	session, err := h.Correctness.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	switch actionID {
	case "run_handshake":
		return h.correctnessRunHandshake(r, session)
	case "run_unregister":
		return h.correctnessRunUnregister(session)
	case "arm_pull_locations_full", "arm_pull_tariffs_full", "arm_push_token_create", "arm_remote_start", "arm_remote_stop":
		return map[string]string{}, nil
	case "prepare_pull_locations_delta_update":
		return h.prepareLocationDeltaUpdate(sessionID)
	case "prepare_pull_locations_full_delete_connector":
		return h.prepareLocationFullDeleteConnector(sessionID)
	case "prepare_pull_locations_delta_delete_evse":
		return h.prepareLocationDeltaDeleteEVSE(sessionID)
	case "prepare_pull_locations_delta_delete_location":
		return h.prepareLocationDeltaDeleteLocation(sessionID)
	case "prepare_pull_tariffs_delta_update":
		return h.prepareTariffDeltaUpdate(sessionID)
	case "arm_push_token_update":
		return h.armKnownToken()
	case "arm_push_token_invalidate":
		return h.armKnownToken()
	case "run_rta_invalid":
		return h.runRealtimeAuthorization(session, false)
	case "run_rta_valid":
		return h.runRealtimeAuthorization(session, true)
	case "run_evse_status_unknown":
		return h.runEVSEStatusPush(session, false)
	case "run_evse_status_known":
		return h.runEVSEStatusPush(session, true)
	case "run_session_push_pending":
		return h.runSessionPush(session, "PENDING")
	case "run_session_push_active":
		return h.runSessionPush(session, "ACTIVE")
	case "run_session_push_completed":
		return h.runSessionPush(session, "COMPLETED")
	case "run_cdr_push":
		return h.runCDRPush(session)
	default:
		return nil, fmt.Errorf("unsupported correctness action %s", actionID)
	}
}

func (h *Handler) correctnessRunHandshake(r *http.Request, session *correctness.TestSession) (map[string]string, error) {
	ctx := correctness.WithOutboundMeta(context.Background(), correctness.OutboundMeta{ActionID: "run_handshake"})
	versionsURL := normalizeVersionsURL(session.Config.PeerVersionsURL)
	store := h.correctnessStore(session.ID)

	if err := store.SetEMSPOwnToken(session.Config.PeerToken); err != nil {
		return nil, err
	}
	if err := store.SetEMSPVersionsURL(versionsURL); err != nil {
		return nil, err
	}

	versionDetailURL, err := h.discoverVersion(ctx, versionsURL, session.Config.PeerToken)
	if err != nil {
		return nil, err
	}
	detail, err := h.fetchVersionDetails(ctx, versionDetailURL, session.Config.PeerToken)
	if err != nil {
		return nil, err
	}

	credentialsURL := ""
	peerEndpoints := make([]correctness.SessionPeerEndpoint, 0, len(detail.Data.Endpoints))
	for _, endpoint := range detail.Data.Endpoints {
		peerEndpoints = append(peerEndpoints, correctness.SessionPeerEndpoint{
			Identifier: endpoint.Identifier,
			Role:       strings.ToUpper(endpoint.Role),
			URL:        endpoint.URL,
		})
		if strings.EqualFold(endpoint.Identifier, "credentials") {
			credentialsURL = endpoint.URL
		}
	}
	if credentialsURL == "" {
		return nil, fmt.Errorf("credentials endpoint not found in version details")
	}

	tokenB, _ := store.GetTokenB()
	if tokenB == "" {
		tokenB = "hub-token-b-" + uuid.NewString()[:12]
		if err := store.SetTokenB(tokenB); err != nil {
			return nil, err
		}
	}

	credsBody := credentialsPayload{
		Token:       tokenB,
		URL:         h.correctnessAdvertisedVersionsURL(r),
		CountryCode: h.Config.HubCountry,
		PartyID:     h.Config.HubParty,
		BusinessDetails: &struct {
			Name string `json:"name,omitempty"`
		}{Name: "OCPI Mock Hub"},
		Roles: []struct {
			Role            string `json:"role"`
			PartyID         string `json:"party_id"`
			CountryCode     string `json:"country_code"`
			BusinessDetails *struct {
				Name string `json:"name,omitempty"`
			} `json:"business_details,omitempty"`
		}{
			{
				Role:        "HUB",
				PartyID:     h.Config.HubParty,
				CountryCode: h.Config.HubCountry,
				BusinessDetails: &struct {
					Name string `json:"name,omitempty"`
				}{Name: "OCPI Mock Hub"},
			},
		},
	}

	peerCreds, err := h.postCredentials(ctx, credentialsURL, session.Config.PeerToken, credsBody)
	if err != nil {
		return nil, err
	}

	if peerCreds.Data.Token != "" {
		if err := store.SetEMSPOwnToken(peerCreds.Data.Token); err != nil {
			return nil, err
		}
	}
	if peerCreds.Data.URL != "" {
		if err := store.SetEMSPCallbackURL(peerCreds.Data.URL); err != nil {
			return nil, err
		}
	}
	rawCreds, _ := json.Marshal(peerCreds.Data)
	if err := store.SetEMSPCredentials(rawCreds); err != nil {
		return nil, err
	}

	if err := h.Correctness.SetPeerState(session.ID, correctness.SessionPeerState{
		VersionDetailURL: versionDetailURL,
		CredentialsURL:   credentialsURL,
		CountryCode:      peerCreds.Data.CountryCode,
		PartyID:          peerCreds.Data.PartyID,
		Endpoints:        peerEndpoints,
	}); err != nil {
		return nil, err
	}

	return map[string]string{
		"version_detail_url": versionDetailURL,
		"credentials_url":    credentialsURL,
		"token_b":            tokenB,
	}, nil
}

func (h *Handler) correctnessRunUnregister(session *correctness.TestSession) (map[string]string, error) {
	if session.Peer.CredentialsURL == "" {
		return nil, fmt.Errorf("the peer credentials endpoint is unknown; run the handshake action first")
	}
	token, _ := h.correctnessStore(session.ID).GetEMSPOwnToken()
	if token == "" {
		token = session.Config.PeerToken
	}
	req, err := http.NewRequestWithContext(
		correctness.WithOutboundMeta(context.Background(), correctness.OutboundMeta{ActionID: "run_unregister"}),
		http.MethodDelete,
		session.Peer.CredentialsURL,
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+token)
	resp, err := h.outboundClient().Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return map[string]string{"credentials_url": session.Peer.CredentialsURL}, nil
}

func (h *Handler) prepareLocationDeltaUpdate(sessionID string) (map[string]string, error) {
	var output map[string]string
	err := h.Correctness.UpdateSandbox(sessionID, func(sandbox *correctness.Sandbox) error {
		if len(sandbox.Seed.Locations) == 0 {
			return fmt.Errorf("no locations available in the sandbox")
		}
		loc := &sandbox.Seed.Locations[0]
		loc.Address = loc.Address + " Suite 221"
		loc.LastUpdated = time.Now().UTC().Format(time.RFC3339)
		output = map[string]string{"location_id": loc.ID}
		return nil
	})
	return output, err
}

func (h *Handler) prepareLocationFullDeleteConnector(sessionID string) (map[string]string, error) {
	var output map[string]string
	err := h.Correctness.UpdateSandbox(sessionID, func(sandbox *correctness.Sandbox) error {
		for i := range sandbox.Seed.Locations {
			for j := range sandbox.Seed.Locations[i].EVSEs {
				evse := &sandbox.Seed.Locations[i].EVSEs[j]
				if len(evse.Connectors) == 0 {
					continue
				}
				connectorID := evse.Connectors[0].ID
				evse.Connectors = append([]fakegen.Connector(nil), evse.Connectors[1:]...)
				now := time.Now().UTC().Format(time.RFC3339)
				evse.LastUpdated = now
				sandbox.Seed.Locations[i].LastUpdated = now
				output = map[string]string{
					"location_id":  sandbox.Seed.Locations[i].ID,
					"evse_uid":     evse.UID,
					"connector_id": connectorID,
				}
				return nil
			}
		}
		return fmt.Errorf("no connector available to remove")
	})
	return output, err
}

func (h *Handler) prepareLocationDeltaDeleteEVSE(sessionID string) (map[string]string, error) {
	var output map[string]string
	err := h.Correctness.UpdateSandbox(sessionID, func(sandbox *correctness.Sandbox) error {
		for i := range sandbox.Seed.Locations {
			if len(sandbox.Seed.Locations[i].EVSEs) == 0 {
				continue
			}
			now := time.Now().UTC().Format(time.RFC3339)
			evse := &sandbox.Seed.Locations[i].EVSEs[0]
			evse.Status = "REMOVED"
			evse.LastUpdated = now
			sandbox.Seed.Locations[i].LastUpdated = now
			output = map[string]string{
				"location_id": sandbox.Seed.Locations[i].ID,
				"evse_uid":    evse.UID,
			}
			return nil
		}
		return fmt.Errorf("no EVSE available to remove")
	})
	return output, err
}

func (h *Handler) prepareLocationDeltaDeleteLocation(sessionID string) (map[string]string, error) {
	var output map[string]string
	err := h.Correctness.UpdateSandbox(sessionID, func(sandbox *correctness.Sandbox) error {
		for i := range sandbox.Seed.Locations {
			if len(sandbox.Seed.Locations[i].EVSEs) == 0 {
				continue
			}
			now := time.Now().UTC().Format(time.RFC3339)
			for j := range sandbox.Seed.Locations[i].EVSEs {
				sandbox.Seed.Locations[i].EVSEs[j].Status = "REMOVED"
				sandbox.Seed.Locations[i].EVSEs[j].LastUpdated = now
			}
			sandbox.Seed.Locations[i].LastUpdated = now
			output = map[string]string{"location_id": sandbox.Seed.Locations[i].ID}
			return nil
		}
		return fmt.Errorf("no location available to remove")
	})
	return output, err
}

func (h *Handler) prepareTariffDeltaUpdate(sessionID string) (map[string]string, error) {
	var output map[string]string
	err := h.Correctness.UpdateSandbox(sessionID, func(sandbox *correctness.Sandbox) error {
		if len(sandbox.Seed.Tariffs) == 0 {
			return fmt.Errorf("no tariffs available in the sandbox")
		}
		now := time.Now().UTC().Format(time.RFC3339)
		tariff := &sandbox.Seed.Tariffs[0]
		tariff.LastUpdated = now
		if len(tariff.Elements) > 0 && len(tariff.Elements[0].PriceComponents) > 0 {
			tariff.Elements[0].PriceComponents[0].Price += 0.07
		}
		output = map[string]string{"tariff_id": tariff.ID}
		return nil
	})
	return output, err
}

func (h *Handler) armKnownToken() (map[string]string, error) {
	token, err := h.latestSandboxToken()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"uid":          token.UID,
		"country_code": token.CountryCode,
		"party_id":     token.PartyID,
	}, nil
}

func (h *Handler) runRealtimeAuthorization(session *correctness.TestSession, valid bool) (map[string]string, error) {
	store := h.correctnessStore(session.ID)
	endpoint, err := peerEndpoint(session, "tokens", "SENDER")
	if err != nil {
		return nil, err
	}
	token, err := h.latestSandboxToken()
	if err != nil {
		return nil, err
	}
	if !valid {
		token.UID = "INVALID-" + uuid.NewString()[:8]
	}
	locationID := firstLocationID(h.currentSeed())
	url := strings.TrimRight(endpoint, "/") + "/" + token.CountryCode + "/" + token.PartyID + "/" + token.UID + "/authorize"
	body, _ := json.Marshal(map[string]string{"location_id": locationID})
	req, err := http.NewRequestWithContext(
		correctness.WithOutboundMeta(context.Background(), correctness.OutboundMeta{ActionID: map[bool]string{true: "run_rta_valid", false: "run_rta_invalid"}[valid]}),
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+currentPeerToken(store, session.Config.PeerToken))
	resp, err := h.outboundClient().Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return map[string]string{
		"uid":          token.UID,
		"country_code": token.CountryCode,
		"party_id":     token.PartyID,
	}, nil
}

func (h *Handler) runEVSEStatusPush(session *correctness.TestSession, known bool) (map[string]string, error) {
	store := h.correctnessStore(session.ID)
	endpoint, err := peerEndpoint(session, "locations", "RECEIVER")
	if err != nil {
		return nil, err
	}
	seed := h.currentSeed()
	if len(seed.Locations) == 0 || len(seed.Locations[0].EVSEs) == 0 {
		return nil, fmt.Errorf("no location and EVSE available in the sandbox")
	}
	location := seed.Locations[0]
	evse := location.EVSEs[0]
	evseUID := evse.UID
	if !known {
		evseUID = "UNKNOWN-EVSE"
	}
	payload := map[string]any{
		"status":       "CHARGING",
		"last_updated": time.Now().UTC().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(payload)
	url := strings.TrimRight(endpoint, "/") + "/" + location.CountryCode + "/" + location.PartyID + "/" + location.ID + "/" + evseUID
	actionID := "run_evse_status_known"
	if !known {
		actionID = "run_evse_status_unknown"
	}
	req, err := http.NewRequestWithContext(
		correctness.WithOutboundMeta(context.Background(), correctness.OutboundMeta{ActionID: actionID}),
		http.MethodPatch,
		url,
		bytes.NewReader(raw),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+currentPeerToken(store, session.Config.PeerToken))
	resp, err := h.outboundClient().Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return map[string]string{
		"location_id": location.ID,
		"evse_uid":    evseUID,
	}, nil
}

func (h *Handler) runSessionPush(session *correctness.TestSession, status string) (map[string]string, error) {
	store := h.correctnessStore(session.ID)
	endpoint, err := peerEndpoint(session, "sessions", "RECEIVER")
	if err != nil {
		return nil, err
	}
	sessionID := ""
	if status == "PENDING" {
		sessionID = "SESS-" + uuid.NewString()[:8]
	} else {
		action, err := h.Correctness.ActionState(session.ID, map[string]string{
			"ACTIVE":    "run_session_push_pending",
			"COMPLETED": "run_session_push_active",
		}[status])
		if err != nil {
			return nil, fmt.Errorf("run the previous session push action first: %w", err)
		}
		sessionID = action.Output["session_id"]
		if sessionID == "" {
			return nil, fmt.Errorf("run the previous session push action first")
		}
	}
	loc := h.currentSeed().Locations[0]
	now := time.Now().UTC()
	payload := map[string]any{
		"country_code":    loc.CountryCode,
		"party_id":        loc.PartyID,
		"id":              sessionID,
		"start_date_time": now.Add(-15 * time.Minute).Format(time.RFC3339),
		"kwh":             4.2,
		"auth_method":     "COMMAND",
		"location_id":     loc.ID,
		"evse_uid":        loc.EVSEs[0].UID,
		"connector_id":    loc.EVSEs[0].Connectors[0].ID,
		"currency":        "EUR",
		"status":          status,
		"last_updated":    now.Format(time.RFC3339),
	}
	if status == "ACTIVE" {
		payload["kwh"] = 9.7
	}
	if status == "COMPLETED" {
		payload["kwh"] = 16.3
		payload["end_date_time"] = now.Format(time.RFC3339)
	}
	raw, _ := json.Marshal(payload)
	url := strings.TrimRight(endpoint, "/") + "/" + loc.CountryCode + "/" + loc.PartyID + "/" + sessionID
	actionID := map[string]string{
		"PENDING":   "run_session_push_pending",
		"ACTIVE":    "run_session_push_active",
		"COMPLETED": "run_session_push_completed",
	}[status]
	req, err := http.NewRequestWithContext(
		correctness.WithOutboundMeta(context.Background(), correctness.OutboundMeta{ActionID: actionID}),
		http.MethodPut,
		url,
		bytes.NewReader(raw),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+currentPeerToken(store, session.Config.PeerToken))
	resp, err := h.outboundClient().Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return map[string]string{"session_id": sessionID}, nil
}

func (h *Handler) runCDRPush(session *correctness.TestSession) (map[string]string, error) {
	store := h.correctnessStore(session.ID)
	endpoint, err := peerEndpoint(session, "cdrs", "RECEIVER")
	if err != nil {
		return nil, err
	}
	action, err := h.Correctness.ActionState(session.ID, "run_session_push_completed")
	if err != nil || action.Output["session_id"] == "" {
		return nil, fmt.Errorf("run the completed session push action first")
	}
	loc := h.currentSeed().Locations[0]
	now := time.Now().UTC()
	payload := map[string]any{
		"country_code":    loc.CountryCode,
		"party_id":        loc.PartyID,
		"id":              "CDR-" + uuid.NewString()[:8],
		"start_date_time": now.Add(-30 * time.Minute).Format(time.RFC3339),
		"end_date_time":   now.Format(time.RFC3339),
		"session_id":      action.Output["session_id"],
		"cdr_token": map[string]any{
			"uid":         "TEST-UID",
			"type":        "RFID",
			"contract_id": "TEST-CONTRACT",
		},
		"auth_method": "COMMAND",
		"cdr_location": map[string]any{
			"id":                   loc.ID,
			"evse_uid":             loc.EVSEs[0].UID,
			"evse_id":              loc.EVSEs[0].EvseID,
			"connector_id":         loc.EVSEs[0].Connectors[0].ID,
			"connector_standard":   loc.EVSEs[0].Connectors[0].Standard,
			"connector_format":     loc.EVSEs[0].Connectors[0].Format,
			"connector_power_type": loc.EVSEs[0].Connectors[0].PowerType,
		},
		"currency": "EUR",
		"total_cost": map[string]any{
			"excl_vat": 9.2,
			"incl_vat": 11.1,
		},
		"total_energy": 12.8,
		"total_time":   0.5,
		"charging_periods": []map[string]any{
			{
				"start_date_time": now.Add(-30 * time.Minute).Format(time.RFC3339),
				"dimensions": []map[string]any{
					{"type": "TIME", "volume": 0.5},
					{"type": "ENERGY", "volume": 12.8},
				},
			},
		},
		"last_updated": now.Format(time.RFC3339),
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(
		correctness.WithOutboundMeta(context.Background(), correctness.OutboundMeta{ActionID: "run_cdr_push"}),
		http.MethodPost,
		strings.TrimRight(endpoint, "/"),
		bytes.NewReader(raw),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+currentPeerToken(store, session.Config.PeerToken))
	resp, err := h.outboundClient().Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return map[string]string{"cdr_id": payload["id"].(string)}, nil
}

func (h *Handler) correctnessAdvertisedVersionsURL(r *http.Request) string {
	if h.Config.InitiateHandshakeVersionsURL != "" {
		return h.Config.InitiateHandshakeVersionsURL
	}
	if r != nil {
		return resolveScheme(r) + "://" + resolveHost(r) + "/ocpi/versions"
	}
	return "http://localhost:4000/ocpi/versions"
}

func peerEndpoint(session *correctness.TestSession, identifier, role string) (string, error) {
	for _, endpoint := range session.Peer.Endpoints {
		if strings.EqualFold(endpoint.Identifier, identifier) && strings.EqualFold(endpoint.Role, role) {
			return endpoint.URL, nil
		}
	}
	if identifier == "credentials" && session.Peer.CredentialsURL != "" {
		return session.Peer.CredentialsURL, nil
	}
	return "", fmt.Errorf("peer did not advertise %s %s endpoint", identifier, role)
}

type storedToken struct {
	CountryCode string `json:"country_code"`
	PartyID     string `json:"party_id"`
	UID         string `json:"uid"`
	LastUpdated string `json:"last_updated"`
	Valid       bool   `json:"valid"`
}

func (h *Handler) latestSandboxToken() (*storedToken, error) {
	raw, err := h.correctnessStore("").ListTokens()
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("no token has been pushed in this correctness session yet")
	}
	tokens := make([]storedToken, 0, len(raw))
	for _, item := range raw {
		var token storedToken
		if json.Unmarshal(item, &token) == nil {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no readable token has been pushed in this correctness session yet")
	}
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].LastUpdated > tokens[j].LastUpdated
	})
	return &tokens[0], nil
}

func currentPeerToken(store Store, fallback string) string {
	if token, _ := store.GetEMSPOwnToken(); token != "" {
		return token
	}
	return fallback
}

func firstLocationID(seed *fakegen.SeedData) string {
	if seed == nil || len(seed.Locations) == 0 {
		return ""
	}
	return seed.Locations[0].ID
}
