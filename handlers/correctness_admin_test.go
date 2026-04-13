package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func testCorrectnessHandler() *Handler {
	seed := &fakegen.SeedData{
		Locations: []fakegen.Location{
			{
				CountryCode: "NL",
				PartyID:     "EMS",
				ID:          "LOC-1",
				Name:        "Test Location",
				Address:     "Main Street 1",
				City:        "Amsterdam",
				PostalCode:  "1000AA",
				Country:     "NLD",
				Coordinates: fakegen.Coords{Latitude: "52.3676", Longitude: "4.9041"},
				TimeZone:    "Europe/Amsterdam",
				ParkingType: "ON_STREET",
				Publish:     true,
				EVSEs: []fakegen.EVSE{
					{
						UID:         "EVSE-1",
						EvseID:      "NL*EMS*E1",
						Status:      "AVAILABLE",
						LastUpdated: "2026-01-01T00:00:00Z",
						Connectors: []fakegen.Connector{
							{
								ID:               "C1",
								Standard:         "IEC_62196_T2",
								Format:           "SOCKET",
								PowerType:        "AC_3_PHASE",
								MaxVoltage:       400,
								MaxAmperage:      16,
								MaxElectricPower: 11000,
								LastUpdated:      "2026-01-01T00:00:00Z",
							},
						},
					},
				},
				LastUpdated: "2026-01-01T00:00:00Z",
			},
		},
		Tariffs: []fakegen.Tariff{
			{
				CountryCode: "NL",
				PartyID:     "EMS",
				ID:          "TAR-1",
				Currency:    "EUR",
				Type:        "REGULAR",
				Elements: []fakegen.TariffElement{
					{
						PriceComponents: []fakegen.PriceComponent{
							{Type: "ENERGY", Price: 0.31, StepSize: 1},
						},
					},
				},
				LastUpdated: "2026-01-01T00:00:00Z",
			},
		},
	}

	return &Handler{
		Config: HandlerConfig{
			TokenA:     "test-token-a",
			HubCountry: "NL",
			HubParty:   "HUB",
		},
		Store:       newTestStore(),
		Seed:        seed,
		ReqLog:      NewRequestLog(),
		Correctness: correctness.NewManager(seed),
	}
}

func createCorrectnessSessionForTest(t *testing.T, h *Handler) correctness.TestSession {
	t.Helper()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/test-sessions", strings.NewReader(`{
		"peer_versions_url":"https://peer.example.com/ocpi/versions",
		"peer_token":"peer-token"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CreateCorrectnessSession(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create session status: got %d, want 201, body=%s", w.Code, w.Body.String())
	}

	var session correctness.TestSession
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return session
}

func TestCorrectnessSessionEndpointsCreateListAndGet(t *testing.T) {
	h := testCorrectnessHandler()
	created := createCorrectnessSessionForTest(t, h)

	listW := httptest.NewRecorder()
	listR := httptest.NewRequest("GET", "/admin/test-sessions", nil)
	h.ListCorrectnessSessions(listW, listR)
	if listW.Code != http.StatusOK {
		t.Fatalf("list sessions status: got %d, want 200", listW.Code)
	}

	var snapshots []correctness.SessionSnapshot
	if err := json.Unmarshal(listW.Body.Bytes(), &snapshots); err != nil {
		t.Fatalf("decode snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if snapshots[0].ID != created.ID {
		t.Fatalf("expected snapshot ID %q, got %q", created.ID, snapshots[0].ID)
	}

	getW := httptest.NewRecorder()
	getR := withChiParams(httptest.NewRequest("GET", "/admin/test-sessions/"+created.ID, nil), map[string]string{
		"sessionID": created.ID,
	})
	h.GetCorrectnessSession(getW, getR)
	if getW.Code != http.StatusOK {
		t.Fatalf("get session status: got %d, want 200", getW.Code)
	}

	var fetched correctness.TestSession
	if err := json.Unmarshal(getW.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode fetched session: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("expected fetched ID %q, got %q", created.ID, fetched.ID)
	}
	if fetched.SuiteName != "OCPI 2.2.1 eMSP Correctness Tests" {
		t.Fatalf("unexpected suite name: %q", fetched.SuiteName)
	}
}

func TestCreateCorrectnessSessionRejectsSecondActiveSession(t *testing.T) {
	h := testCorrectnessHandler()
	createCorrectnessSessionForTest(t, h)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/test-sessions", strings.NewReader(`{
		"peer_versions_url":"https://peer.example.com/ocpi/versions",
		"peer_token":"peer-token"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CreateCorrectnessSession(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when another session is active, got %d", w.Code)
	}
}

func TestDeleteCorrectnessSessionRemovesActiveSessionAndSharedPartyState(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	overlay := h.Correctness.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	if err := overlay.SetTokenB("session-token-b"); err != nil {
		t.Fatalf("set overlay tokenB: %v", err)
	}
	if err := h.registerCorrectnessPeerToken(&session, "session-token-b", "https://hub.example.com/ocpi/versions", "", correctness.SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}, "peer-token"); err != nil {
		t.Fatalf("register correctness peer token: %v", err)
	}
	if party, err := h.Store.GetPartyByTokenB("session-token-b"); err != nil || party == nil {
		t.Fatalf("expected shared party state before delete, got party=%v err=%v", party != nil, err)
	}

	w := httptest.NewRecorder()
	r := withChiParams(httptest.NewRequest("DELETE", "/admin/test-sessions/"+session.ID, nil), map[string]string{
		"sessionID": session.ID,
	})
	h.DeleteCorrectnessSession(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("delete session status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}

	if _, err := h.Correctness.GetSession(session.ID); err == nil {
		t.Fatal("expected deleted correctness session to be removed")
	}
	if active := h.Correctness.ActiveSessionID(); active != "" {
		t.Fatalf("expected active correctness session to clear, got %q", active)
	}
	if party, err := h.Store.GetPartyByTokenB("session-token-b"); err != nil || party != nil {
		t.Fatalf("expected shared party state to be removed, got party=%v err=%v", party != nil, err)
	}
}

func TestRunCorrectnessActionCompletesObserveAction(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	if err := h.Correctness.MarkActionStarted(session.ID, "run_handshake"); err != nil {
		t.Fatalf("start handshake action: %v", err)
	}

	overlay := h.Correctness.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	if err := overlay.SetTokenB("session-token-b"); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}
	if err := h.Correctness.SetPeerState(session.ID, correctness.SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}

	for _, event := range []correctness.TrafficEvent{
		{
			Direction:      "outbound",
			Method:         "GET",
			Path:           "/ocpi/versions",
			RequestHeaders: map[string]string{"authorization": "Token peer-token"},
			ResponseStatus: 200,
			ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":[{"version":"2.2.1","url":"https://peer.example.com/ocpi/2.2.1"}]}`,
			StartedAt:      "2026-01-01T00:00:00Z",
		},
		{
			Direction:      "outbound",
			Method:         "GET",
			Path:           "/ocpi/2.2.1",
			RequestHeaders: map[string]string{"authorization": "Token peer-token"},
			ResponseStatus: 200,
			ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:01Z","data":{"version":"2.2.1","endpoints":[{"identifier":"credentials","role":"RECEIVER","url":"https://peer.example.com/ocpi/2.2.1/credentials"}]}}`,
			StartedAt:      "2026-01-01T00:00:01Z",
		},
		{
			Direction: "outbound",
			Method:    "POST",
			Path:      "/ocpi/2.2.1/credentials",
			RequestHeaders: map[string]string{
				"authorization": "Token peer-token",
				"content-type":  "application/json",
			},
			ResponseStatus: 200,
			ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:02Z","data":{"token":"peer-token-b","url":"https://peer.example.com/ocpi/versions","country_code":"NL","party_id":"EMS"}}`,
			StartedAt:      "2026-01-01T00:00:02Z",
		},
		{
			Direction: "inbound",
			Method:    "GET",
			Path:      "/ocpi/versions",
			RequestHeaders: map[string]string{
				"authorization":          "Token session-token-b",
				"x-request-id":           "req-1",
				"x-correlation-id":       "corr-1",
				"ocpi-from-country-code": "NL",
				"ocpi-from-party-id":     "EMS",
			},
			ResponseStatus: 200,
			ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:03Z","data":[{"version":"2.2.1","url":"https://hub.example.com/ocpi/2.2.1"}]}`,
			StartedAt:      "2026-01-01T00:00:03Z",
		},
		{
			Direction: "inbound",
			Method:    "GET",
			Path:      "/ocpi/2.2.1",
			RequestHeaders: map[string]string{
				"authorization":          "Token session-token-b",
				"x-request-id":           "req-2",
				"x-correlation-id":       "corr-2",
				"ocpi-from-country-code": "NL",
				"ocpi-from-party-id":     "EMS",
			},
			ResponseStatus: 200,
			ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:04Z","data":{"version":"2.2.1","endpoints":[]}}`,
			StartedAt:      "2026-01-01T00:00:04Z",
		},
	} {
		h.Correctness.RecordTrafficEvent(event)
	}
	if _, err := h.Correctness.CompleteAction(session.ID, "run_handshake", map[string]string{"token_b": "session-token-b"}); err != nil {
		t.Fatalf("complete handshake action: %v", err)
	}

	w := httptest.NewRecorder()
	r := withChiParams(httptest.NewRequest("POST", "/admin/test-sessions/"+session.ID+"/actions/arm_pull_locations_full", nil), map[string]string{
		"sessionID": session.ID,
		"actionID":  "arm_pull_locations_full",
	})

	h.RunCorrectnessAction(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("run action status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var updated correctness.TestSession
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated session: %v", err)
	}

	found := false
	for _, action := range updated.Actions {
		if action.ID != "arm_pull_locations_full" {
			continue
		}
		found = true
		if action.Status != "completed" {
			t.Fatalf("expected completed action status, got %q", action.Status)
		}
	}
	if !found {
		t.Fatal("did not find action arm_pull_locations_full in session response")
	}
}

func TestRunCorrectnessActionRejectsOutOfSequenceIdleAction(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	w := httptest.NewRecorder()
	r := withChiParams(httptest.NewRequest("POST", "/admin/test-sessions/"+session.ID+"/actions/run_unregister", nil), map[string]string{
		"sessionID": session.ID,
		"actionID":  "run_unregister",
	})

	h.RunCorrectnessAction(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-sequence action, got %d with body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Run Handshake") {
		t.Fatalf("expected handshake guidance in error body, got %s", w.Body.String())
	}
}

func TestRunCorrectnessHandshakeUsesSessionScopedTokens(t *testing.T) {
	h := testCorrectnessHandler()
	h.Config.TokenA = "global-token-a"

	var versionsAuth string
	var detailsAuth string
	var credentialsAuth string
	var postedCredentials credentialsPayload

	var peer *httptest.Server
	peer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/versions":
			versionsAuth = r.Header.Get("Authorization")
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"timestamp":   "2026-01-01T00:00:00Z",
				"data": []map[string]string{
					{"version": "2.2.1", "url": peer.URL + "/ocpi/2.2.1"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/2.2.1":
			detailsAuth = r.Header.Get("Authorization")
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"timestamp":   "2026-01-01T00:00:01Z",
				"data": map[string]any{
					"version": "2.2.1",
					"endpoints": []map[string]string{
						{"identifier": "credentials", "role": "RECEIVER", "url": peer.URL + "/ocpi/2.2.1/credentials"},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/ocpi/2.2.1/credentials":
			credentialsAuth = r.Header.Get("Authorization")
			if err := json.NewDecoder(r.Body).Decode(&postedCredentials); err != nil {
				t.Fatalf("decode posted credentials: %v", err)
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"timestamp":   "2026-01-01T00:00:02Z",
				"data": map[string]string{
					"token":        "peer-token-b",
					"url":          peer.URL + "/ocpi/versions",
					"country_code": "NL",
					"party_id":     "EMS",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer peer.Close()

	session, err := h.Correctness.StartSession(correctness.SessionConfig{
		PeerVersionsURL: peer.URL + "/ocpi/versions",
		PeerToken:       "session-peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	w := httptest.NewRecorder()
	r := withChiParams(httptest.NewRequest("POST", "/admin/test-sessions/"+session.ID+"/actions/run_handshake", nil), map[string]string{
		"sessionID": session.ID,
		"actionID":  "run_handshake",
	})
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Rally-Forwarded-Host", "hub.example.com")

	h.RunCorrectnessAction(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("run handshake status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}

	for name, value := range map[string]string{
		"versions":    versionsAuth,
		"details":     detailsAuth,
		"credentials": credentialsAuth,
	} {
		if value != "Token session-peer-token" {
			t.Fatalf("expected %s request to use the session peer token, got %q", name, value)
		}
	}

	if postedCredentials.Token == "" {
		t.Fatal("expected POST /credentials payload to include a generated session token")
	}
	if postedCredentials.Token == h.Config.TokenA {
		t.Fatalf("expected POST /credentials payload token to differ from global Token A %q", h.Config.TokenA)
	}

	overlay := h.Correctness.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	tokenB, err := overlay.GetTokenB()
	if err != nil {
		t.Fatalf("get overlay tokenB: %v", err)
	}
	if tokenB != postedCredentials.Token {
		t.Fatalf("expected posted credentials token %q to match session tokenB %q", postedCredentials.Token, tokenB)
	}
	if party, err := h.Store.GetPartyByTokenB(postedCredentials.Token); err != nil || party == nil {
		t.Fatalf("expected posted credentials token %q to be registered in shared store, got party=%v err=%v", postedCredentials.Token, party != nil, err)
	}
}

func TestRunCorrectnessHandshakeRegistersTokenBeforePeerImmediatelyFetchesVersions(t *testing.T) {
	baseStore := newTestStore()
	seed := &fakegen.SeedData{}

	h := &Handler{
		Config: HandlerConfig{
			TokenA:     "global-token-a",
			HubCountry: "NL",
			HubParty:   "HUB",
		},
		Store:       baseStore,
		Seed:        seed,
		ReqLog:      NewRequestLog(),
		Correctness: correctness.NewManager(seed),
	}
	otherInstance := &Handler{
		Config: HandlerConfig{
			TokenA: "global-token-a",
		},
		Store: baseStore,
	}

	var followUpStatus int
	var followUpBody string

	var peer *httptest.Server
	peer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/versions":
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"timestamp":   "2026-01-01T00:00:00Z",
				"data": []map[string]string{
					{"version": "2.2.1", "url": peer.URL + "/ocpi/2.2.1"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/2.2.1":
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"timestamp":   "2026-01-01T00:00:01Z",
				"data": map[string]any{
					"version": "2.2.1",
					"endpoints": []map[string]string{
						{"identifier": "credentials", "role": "RECEIVER", "url": peer.URL + "/ocpi/2.2.1/credentials"},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/ocpi/2.2.1/credentials":
			var payload credentialsPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode posted credentials: %v", err)
			}

			followUpReq := httptest.NewRequest(http.MethodGet, "http://hub.example.com/ocpi/versions", nil)
			followUpReq.Host = "hub.example.com"
			followUpReq.Header.Set("Authorization", "Token "+payload.Token)
			followUpRes := httptest.NewRecorder()
			otherInstance.GetVersions(followUpRes, followUpReq)
			followUpStatus = followUpRes.Code
			followUpBody = followUpRes.Body.String()

			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"timestamp":   "2026-01-01T00:00:02Z",
				"data": map[string]string{
					"token":        "peer-token-b",
					"url":          peer.URL + "/ocpi/versions",
					"country_code": "NL",
					"party_id":     "EMS",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer peer.Close()

	session, err := h.Correctness.StartSession(correctness.SessionConfig{
		PeerVersionsURL: peer.URL + "/ocpi/versions",
		PeerToken:       "session-peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://hub.example.com/admin/test-sessions/"+session.ID+"/actions/run_handshake", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Rally-Forwarded-Host", "hub.example.com")

	if _, err := h.correctnessRunHandshake(req, session); err != nil {
		t.Fatalf("run handshake: %v", err)
	}

	if followUpStatus != http.StatusOK {
		t.Fatalf("expected immediate follow-up GET /ocpi/versions on another instance to succeed, got %d with body %s", followUpStatus, followUpBody)
	}
}

func TestPrepareLocationFullDeleteConnectorCanBeRunAgain(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	firstSession, err := h.Correctness.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	first, err := h.prepareLocationFullDeleteConnector(firstSession)
	if err != nil {
		t.Fatalf("first prepare run: %v", err)
	}
	if _, err := h.Correctness.CompleteAction(session.ID, "prepare_pull_locations_full_delete_connector", first); err != nil {
		t.Fatalf("complete action: %v", err)
	}

	secondSession, err := h.Correctness.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session after first run: %v", err)
	}
	second, err := h.prepareLocationFullDeleteConnector(secondSession)
	if err != nil {
		t.Fatalf("second prepare run: %v", err)
	}

	if first["connector_id"] == "" {
		t.Fatalf("expected connector output from first run, got %#v", first)
	}
	if second["connector_id"] != first["connector_id"] || second["evse_uid"] != first["evse_uid"] || second["location_id"] != first["location_id"] {
		t.Fatalf("expected second prepare run to reuse the same removed connector target, got first=%#v second=%#v", first, second)
	}
}

func TestArmPushTokenCreateReturnsExplicitProfile(t *testing.T) {
	h := testCorrectnessHandler()
	createCorrectnessSessionForTest(t, h)

	output, err := h.armTokenCreate()
	if err != nil {
		t.Fatalf("arm token create: %v", err)
	}
	if output["uid"] == "" || output["contract_id"] == "" {
		t.Fatalf("expected generated token identifiers, got %#v", output)
	}
	if output["type"] != "RFID" {
		t.Fatalf("expected RFID token type, got %#v", output)
	}
	if output["whitelist"] != "ALLOWED" || output["valid"] != "true" {
		t.Fatalf("expected explicit happy-path token policy, got %#v", output)
	}
}

func TestRunRealtimeAuthorizationValidUsesLatestValidToken(t *testing.T) {
	h := testCorrectnessHandler()

	var requestPath string
	var requestAuth string
	var requestBody map[string]string
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		requestAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status_code": 1000,
			"timestamp":   "2026-01-01T00:00:00Z",
			"data": map[string]any{
				"allowed":                 "ALLOWED",
				"authorization_reference": "AUTH-1",
				"token": map[string]any{
					"uid": "TOK-VALID",
				},
			},
		})
	}))
	defer peer.Close()

	session, err := h.Correctness.StartSession(correctness.SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := h.Correctness.SetPeerState(session.ID, correctness.SessionPeerState{
		Endpoints: []correctness.SessionPeerEndpoint{
			{Identifier: "tokens", Role: "SENDER", URL: peer.URL + "/ocpi/2.2.1/sender/tokens"},
		},
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}
	session, err = h.Correctness.GetSession(session.ID)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}

	overlay := h.Correctness.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}

	validRaw, _ := json.Marshal(map[string]any{
		"country_code": "NL",
		"party_id":     "EMS",
		"uid":          "TOK-VALID",
		"type":         "RFID",
		"contract_id":  "CON-1",
		"issuer":       "Issuer",
		"whitelist":    "ALLOWED",
		"valid":        true,
		"last_updated": "2026-01-01T00:00:00Z",
	})
	invalidRaw, _ := json.Marshal(map[string]any{
		"country_code": "NL",
		"party_id":     "EMS",
		"uid":          "TOK-INVALID",
		"type":         "RFID",
		"contract_id":  "CON-2",
		"issuer":       "Issuer",
		"whitelist":    "ALLOWED",
		"valid":        false,
		"last_updated": "2026-01-02T00:00:00Z",
	})
	if err := overlay.PutToken("NL", "EMS", "TOK-VALID", validRaw); err != nil {
		t.Fatalf("put valid token: %v", err)
	}
	if err := overlay.PutToken("NL", "EMS", "TOK-INVALID", invalidRaw); err != nil {
		t.Fatalf("put invalid token: %v", err)
	}

	output, err := h.runRealtimeAuthorization(session, true)
	if err != nil {
		t.Fatalf("run realtime authorization: %v", err)
	}

	if !strings.Contains(requestPath, "/TOK-VALID/authorize") {
		t.Fatalf("expected valid authorization to target TOK-VALID, got path %q", requestPath)
	}
	if requestAuth != "Token peer-token" {
		t.Fatalf("expected peer token auth header, got %q", requestAuth)
	}
	if requestBody["location_id"] != "LOC-1" {
		t.Fatalf("expected location LOC-1 in authorization request, got %#v", requestBody)
	}
	if output["uid"] != "TOK-VALID" {
		t.Fatalf("expected output to record TOK-VALID, got %#v", output)
	}
}

func TestRunRealtimeAuthorizationInvalidRestoresHappyPathTarget(t *testing.T) {
	h := testCorrectnessHandler()

	var requestPath string
	var requestBody map[string]string
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status_code": 1000,
			"timestamp":   "2026-01-01T00:00:00Z",
			"data": map[string]any{
				"allowed": "BLOCKED",
				"token": map[string]any{
					"uid": "TOK-INVALID",
				},
			},
		})
	}))
	defer peer.Close()

	session := createCorrectnessSessionForTest(t, h)
	if err := h.Correctness.SetPeerState(session.ID, correctness.SessionPeerState{
		Endpoints: []correctness.SessionPeerEndpoint{
			{Identifier: "tokens", Role: "SENDER", URL: peer.URL + "/ocpi/2.2.1/sender/tokens"},
		},
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}
	reloaded, err := h.Correctness.GetSession(session.ID)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}

	overlay := h.Correctness.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	invalidRaw, _ := json.Marshal(map[string]any{
		"country_code": "NL",
		"party_id":     "EMS",
		"uid":          "TOK-INVALID",
		"type":         "RFID",
		"contract_id":  "CON-2",
		"issuer":       "Issuer",
		"whitelist":    "ALLOWED",
		"valid":        false,
		"last_updated": "2026-01-02T00:00:00Z",
	})
	if err := overlay.PutToken("NL", "EMS", "TOK-INVALID", invalidRaw); err != nil {
		t.Fatalf("put invalid token: %v", err)
	}
	if err := h.Correctness.UpdateSandbox(session.ID, func(sandbox *correctness.Sandbox) error {
		sandbox.Seed.Locations[0].EVSEs[0].Status = "REMOVED"
		sandbox.Seed.Locations[0].EVSEs[0].Connectors = nil
		return nil
	}); err != nil {
		t.Fatalf("remove happy-path target: %v", err)
	}

	output, err := h.runRealtimeAuthorization(reloaded, false)
	if err != nil {
		t.Fatalf("run realtime authorization: %v", err)
	}

	if !strings.Contains(requestPath, "/TOK-INVALID/authorize") {
		t.Fatalf("expected invalid authorization to target TOK-INVALID, got path %q", requestPath)
	}
	if requestBody["location_id"] != "LOC-1" {
		t.Fatalf("expected restored location LOC-1 in authorization request, got %#v", requestBody)
	}
	if output["location_id"] != "LOC-1" || output["valid"] != "false" {
		t.Fatalf("expected output to record restored location and invalid token, got %#v", output)
	}

	loc := h.currentSeed().LocationByID("LOC-1")
	if loc == nil {
		t.Fatal("expected restored location in active seed")
	}
	if len(loc.EVSEs) == 0 || strings.EqualFold(loc.EVSEs[0].Status, "REMOVED") {
		t.Fatalf("expected restored non-removed EVSE, got %#v", loc.EVSEs)
	}
}

func TestArmRemoteStartRestoresHappyPathTarget(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	overlay := h.Correctness.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	tokenRaw, _ := json.Marshal(map[string]any{
		"country_code": "NL",
		"party_id":     "EMS",
		"uid":          "TOK-1",
		"type":         "RFID",
		"contract_id":  "CON-1",
		"issuer":       "Issuer",
		"whitelist":    "ALLOWED",
		"valid":        true,
		"last_updated": "2026-01-01T00:00:00Z",
	})
	if err := overlay.PutToken("NL", "EMS", "TOK-1", tokenRaw); err != nil {
		t.Fatalf("put token: %v", err)
	}
	if err := h.Correctness.UpdateSandbox(session.ID, func(sandbox *correctness.Sandbox) error {
		sandbox.Seed.Locations[0].EVSEs[0].Status = "REMOVED"
		sandbox.Seed.Locations[0].EVSEs[0].Connectors = nil
		return nil
	}); err != nil {
		t.Fatalf("remove happy-path target: %v", err)
	}

	output, err := h.armRemoteStart(session.ID)
	if err != nil {
		t.Fatalf("arm remote start: %v", err)
	}
	if output["location_id"] != "LOC-1" || output["evse_uid"] != "EVSE-1" || output["connector_id"] != "C1" {
		t.Fatalf("expected restored happy-path target, got %#v", output)
	}
	if output["token_uid"] != "TOK-1" || output["token_type"] != "RFID" {
		t.Fatalf("expected valid token guidance, got %#v", output)
	}

	loc := h.currentSeed().LocationByID("LOC-1")
	if loc == nil {
		t.Fatal("expected restored location in active seed")
	}
	if len(loc.EVSEs) == 0 || strings.EqualFold(loc.EVSEs[0].Status, "REMOVED") {
		t.Fatalf("expected restored non-removed EVSE, got %#v", loc.EVSEs)
	}
	if len(loc.EVSEs[0].Connectors) == 0 {
		t.Fatalf("expected restored connector, got %#v", loc.EVSEs[0])
	}
}

func TestSubmitCorrectnessCheckpointStoresAnswer(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	w := httptest.NewRecorder()
	r := withChiParams(httptest.NewRequest(
		"POST",
		"/admin/test-sessions/"+session.ID+"/checkpoints/confirm_connector_removed_after_full_pull",
		strings.NewReader(`{"answer":"connector absent","notes":"verified manually"}`),
	), map[string]string{
		"sessionID":    session.ID,
		"checkpointID": "confirm_connector_removed_after_full_pull",
	})
	r.Header.Set("Content-Type", "application/json")

	h.SubmitCorrectnessCheckpoint(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("submit checkpoint status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var updated correctness.TestSession
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated session: %v", err)
	}

	found := false
	for _, result := range updated.Cases {
		for _, checkpoint := range result.Checkpoints {
			if checkpoint.ID != "confirm_connector_removed_after_full_pull" {
				continue
			}
			found = true
			if checkpoint.Status != "answered" {
				t.Fatalf("expected answered checkpoint, got %q", checkpoint.Status)
			}
			if checkpoint.Answer != "connector absent" {
				t.Fatalf("unexpected checkpoint answer: %q", checkpoint.Answer)
			}
		}
	}
	if !found {
		t.Fatal("checkpoint was not returned in the updated session")
	}
}
