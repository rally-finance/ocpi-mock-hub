package correctness

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOCPIDateTimeRequiresExplicitUTC(t *testing.T) {
	valid := []string{
		"2026-01-01T10:00:00Z",
		"2026-01-01T10:00:00.123Z",
		"2026-01-01T10:00:00+00:00",
	}
	for _, value := range valid {
		if _, err := parseOCPIDateTime(value); err != nil {
			t.Fatalf("expected %q to parse, got %v", value, err)
		}
	}

	invalid := []string{
		"",
		"2026-01-01T10:00:00",
		"2026-01-01 10:00:00Z",
		"2026-01-01T10:00:00+02:00",
	}
	for _, value := range invalid {
		if _, err := parseOCPIDateTime(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestValidatePaginatedRequestsRejectsInvalidDeltaWindow(t *testing.T) {
	event := TrafficEvent{
		Direction: "inbound",
		Method:    "GET",
		Path:      "/ocpi/2.2.1/sender/locations",
		RawQuery:  "offset=0&limit=50&date_from=2026-01-02T00:00:00Z&date_to=2026-01-01T00:00:00Z",
		RequestHeaders: map[string]string{
			"authorization":          "Token peer-token",
			"x-request-id":           "req-1",
			"x-correlation-id":       "corr-1",
			"ocpi-from-country-code": "NL",
			"ocpi-from-party-id":     "EMS",
		},
		ResponseStatus: 200,
		ResponseHeaders: map[string]string{
			"x-total-count": "1",
			"x-limit":       "50",
		},
		ResponseBody: `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":[]}`,
	}

	issues := validatePaginatedRequests([]TrafficEvent{event}, true)
	if !containsIssue(issues, "date_to must be later than date_from.") {
		t.Fatalf("expected invalid delta window issue, got %#v", issues)
	}
}

func TestValidatePaginatedRequestsIgnoresHubResponseMetadata(t *testing.T) {
	event := TrafficEvent{
		Direction: "inbound",
		Method:    "GET",
		Path:      "/ocpi/2.2.1/sender/locations",
		RawQuery:  "offset=0&limit=50",
		RequestHeaders: map[string]string{
			"authorization":          "Token peer-token",
			"x-request-id":           "req-1",
			"x-correlation-id":       "corr-1",
			"ocpi-from-country-code": "NL",
			"ocpi-from-party-id":     "EMS",
		},
	}

	issues := validatePaginatedRequests([]TrafficEvent{event}, false)
	if len(issues) != 0 {
		t.Fatalf("expected request-only pagination validation, got %#v", issues)
	}
}

func TestValidateTokenPayloadRejectsPathMismatchAndInvalidationErrors(t *testing.T) {
	event := TrafficEvent{
		Path:     "/ocpi/2.2.1/receiver/tokens/NL/EMS/TOK-1",
		RawQuery: "type=RFID",
		RequestBody: `{
			"country_code":"NL",
			"party_id":"EMS",
			"uid":"TOK-2",
			"type":"RFID",
			"contract_id":"CON-1",
			"issuer":"Issuer",
			"whitelist":"ALLOWED",
			"valid":true,
			"last_updated":"2026-01-01T00:00:00"
		}`,
	}

	issues := validateTokenPayload(event, true)
	if !containsIssue(issues, "The Token path uid did not match the body.") {
		t.Fatalf("expected UID mismatch issue, got %#v", issues)
	}
	if !containsIssue(issues, "Expected the invalidation Token push to set valid=false.") {
		t.Fatalf("expected valid=false issue, got %#v", issues)
	}
	if !containsIssue(issues, "The Token last_updated field was not a valid OCPI DateTime.") {
		t.Fatalf("expected invalid last_updated issue, got %#v", issues)
	}
}

func TestEvalRemoteStopIgnoresActionDrivenOutboundPostWhenMatchingCallback(t *testing.T) {
	rt := &sessionRuntime{
		sandbox: NewSandbox(testSeed()),
		actions: map[string]*ActionState{
			"arm_remote_stop": {
				ID:          "arm_remote_stop",
				Status:      "completed",
				EventAnchor: 0,
			},
		},
		events: []TrafficEvent{
			{
				Direction: "inbound",
				Method:    "POST",
				Path:      "/ocpi/2.2.1/receiver/commands/STOP_SESSION",
				RequestHeaders: map[string]string{
					"authorization":          "Token correctness-token",
					"x-request-id":           "req-1",
					"x-correlation-id":       "corr-1",
					"ocpi-from-country-code": "NL",
					"ocpi-from-party-id":     "EMS",
				},
				RequestBody:    `{"session_id":"SESS-1","response_url":"https://peer.example.com/callback"}`,
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":{"result":"ACCEPTED"}}`,
			},
			{
				ActionID:       "run_session_push_active",
				Direction:      "outbound",
				Method:         "POST",
				URL:            "https://peer.example.com/callback",
				Path:           "/ocpi/2.2.1/receiver/sessions/NL/EMS/SESS-1",
				RequestBody:    `{"result":"ACCEPTED","session_id":"SESS-1"}`,
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:01Z","data":{"status":"ok"}}`,
			},
		},
	}

	eval := evalRemoteStop(rt, CaseDefinition{})
	if eval.Status != "pending" {
		t.Fatalf("expected pending when only action-tagged outbound POST exists, got %q with messages %#v", eval.Status, eval.Messages)
	}
	if !containsIssue(eval.Messages, "Waiting for the asynchronous STOP_SESSION callback exchange.") {
		t.Fatalf("expected pending callback message, got %#v", eval.Messages)
	}
}

func TestEvalRemoteStartMatchesCallbackByResponseURL(t *testing.T) {
	rt := &sessionRuntime{
		sandbox: NewSandbox(testSeed()),
		actions: map[string]*ActionState{
			"arm_remote_start": {
				ID:          "arm_remote_start",
				Status:      "completed",
				EventAnchor: 0,
			},
		},
		events: []TrafficEvent{
			{
				Direction: "inbound",
				Method:    "POST",
				Path:      "/ocpi/2.2.1/receiver/commands/START_SESSION",
				RequestHeaders: map[string]string{
					"authorization":          "Token correctness-token",
					"x-request-id":           "req-1",
					"x-correlation-id":       "corr-1",
					"ocpi-from-country-code": "NL",
					"ocpi-from-party-id":     "EMS",
				},
				RequestBody:    `{"location_id":"LOC-1","response_url":"https://peer.example.com/callback","token":{"uid":"TOK-1","type":"RFID"}}`,
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":{"result":"ACCEPTED"}}`,
			},
			{
				Direction:      "outbound",
				Method:         "POST",
				URL:            "https://peer.example.com/other-callback",
				Path:           "/other-callback",
				RequestBody:    `{"result":"ACCEPTED","session_id":"MOCK-1"}`,
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:01Z","data":{"status":"ok"}}`,
			},
		},
	}

	eval := evalRemoteStart(rt, CaseDefinition{})
	if eval.Status != "pending" {
		t.Fatalf("expected pending when only a different callback URL exists, got %q with messages %#v", eval.Status, eval.Messages)
	}

	rt.events = append(rt.events, TrafficEvent{
		Direction:      "outbound",
		Method:         "POST",
		URL:            "https://peer.example.com/callback",
		Path:           "/callback",
		RequestBody:    `{"result":"ACCEPTED","session_id":"MOCK-1"}`,
		ResponseStatus: 200,
		ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:02Z","data":{"status":"ok"}}`,
	})

	eval = evalRemoteStart(rt, CaseDefinition{})
	if eval.Status != "passed" {
		t.Fatalf("expected passed after the correct callback URL is observed, got %q with messages %#v", eval.Status, eval.Messages)
	}
}

func TestEvalRemoteStopFallsBackToStoredSessionCallbackURL(t *testing.T) {
	sandbox := NewSandbox(testSeed())
	rawSession, err := json.Marshal(map[string]any{
		"id":            "SESS-1",
		"_response_url": "https://peer.example.com/callback",
	})
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	sandbox.Store.PutSession("SESS-1", rawSession)

	rt := &sessionRuntime{
		sandbox: sandbox,
		actions: map[string]*ActionState{
			"arm_remote_stop": {
				ID:          "arm_remote_stop",
				Status:      "completed",
				EventAnchor: 0,
			},
		},
		events: []TrafficEvent{
			{
				Direction: "inbound",
				Method:    "POST",
				Path:      "/ocpi/2.2.1/receiver/commands/STOP_SESSION",
				RequestHeaders: map[string]string{
					"authorization":          "Token correctness-token",
					"x-request-id":           "req-1",
					"x-correlation-id":       "corr-1",
					"ocpi-from-country-code": "NL",
					"ocpi-from-party-id":     "EMS",
				},
				RequestBody:    `{"session_id":"SESS-1"}`,
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":{"result":"ACCEPTED"}}`,
			},
			{
				Direction:      "outbound",
				Method:         "POST",
				URL:            "https://peer.example.com/callback",
				Path:           "/callback",
				RequestBody:    `{"result":"ACCEPTED","session_id":"SESS-1"}`,
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:01Z","data":{"status":"ok"}}`,
			},
		},
	}

	eval := evalRemoteStop(rt, CaseDefinition{})
	if eval.Status != "passed" {
		t.Fatalf("expected passed when STOP_SESSION reuses the stored callback URL, got %q with messages %#v", eval.Status, eval.Messages)
	}
}

func TestEvalHandshakeFlowRequiresConfiguredPeerToken(t *testing.T) {
	rt := &sessionRuntime{
		session: TestSession{
			Config: SessionConfig{
				PeerToken: "session-peer-token",
			},
		},
		actions: map[string]*ActionState{
			"run_handshake": {
				ID:          "run_handshake",
				Status:      "completed",
				EventAnchor: 0,
			},
		},
		events: []TrafficEvent{
			{
				Direction:      "outbound",
				Method:         "GET",
				Path:           "/ocpi/versions",
				RequestHeaders: map[string]string{"authorization": "Token wrong-token"},
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":[{"version":"2.2.1","url":"https://peer.example.com/ocpi/2.2.1"}]}`,
			},
			{
				Direction:      "outbound",
				Method:         "GET",
				Path:           "/ocpi/2.2.1",
				RequestHeaders: map[string]string{"authorization": "Token wrong-token"},
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:01Z","data":{"version":"2.2.1","endpoints":[{"identifier":"credentials","role":"RECEIVER","url":"https://peer.example.com/ocpi/2.2.1/credentials"}]}}`,
			},
			{
				Direction:      "outbound",
				Method:         "POST",
				Path:           "/ocpi/2.2.1/credentials",
				RequestHeaders: map[string]string{"authorization": "Token wrong-token", "content-type": "application/json"},
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:02Z","data":{"token":"peer-token-b","url":"https://peer.example.com/ocpi/versions","country_code":"NL","party_id":"EMS"}}`,
			},
		},
	}

	eval := evalHandshakeFlow(rt, CaseDefinition{})
	if eval.Status != "failed" {
		t.Fatalf("expected handshake flow to fail on the wrong peer token, got %q with messages %#v", eval.Status, eval.Messages)
	}
	if !containsIssue(eval.Messages, "configured peer token for this session") {
		t.Fatalf("expected configured peer token validation issue, got %#v", eval.Messages)
	}
}

func TestEvalPeerFetchesHubVersionsFailsWhenPeerUsesWrongToken(t *testing.T) {
	sandbox := NewSandbox(testSeed())
	if err := sandbox.Store.SetTokenB("hub-token-b"); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}

	rt := &sessionRuntime{
		sandbox: sandbox,
		session: TestSession{
			Config: SessionConfig{PeerToken: "peer-token"},
			Peer: SessionPeerState{
				CountryCode: "NL",
				PartyID:     "EMS",
			},
		},
		actions: map[string]*ActionState{
			"run_handshake": {
				ID:          "run_handshake",
				Status:      "completed",
				EventAnchor: 0,
			},
		},
		events: []TrafficEvent{
			{
				Direction: "inbound",
				Method:    "GET",
				Path:      "/ocpi/versions",
				RequestHeaders: map[string]string{
					"authorization":          "Token peer-token",
					"x-request-id":           "req-1",
					"x-correlation-id":       "corr-1",
					"ocpi-from-country-code": "NL",
					"ocpi-from-party-id":     "EMS",
				},
				ResponseStatus: 401,
				ResponseBody:   `{"status_code":2001,"timestamp":"2026-01-01T00:00:00Z","status_message":"Invalid authorization token"}`,
				StartedAt:      "2026-01-01T00:00:00Z",
			},
		},
	}

	eval := evalPeerFetchesHubVersions(rt, CaseDefinition{})
	if eval.Status != "failed" {
		t.Fatalf("expected peer hub versions follow-up to fail when using the wrong token, got %q with messages %#v", eval.Status, eval.Messages)
	}
	if !containsIssue(eval.Messages, "session Token B returned by the hub") {
		t.Fatalf("expected session Token B issue, got %#v", eval.Messages)
	}
}

func TestEvalPeerFetchesHubVersionsPassesWithSessionTokenB(t *testing.T) {
	sandbox := NewSandbox(testSeed())
	if err := sandbox.Store.SetTokenB("hub-token-b"); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}

	rt := &sessionRuntime{
		sandbox: sandbox,
		session: TestSession{
			Config: SessionConfig{PeerToken: "peer-token"},
			Peer: SessionPeerState{
				CountryCode: "NL",
				PartyID:     "EMS",
			},
		},
		actions: map[string]*ActionState{
			"run_handshake": {
				ID:          "run_handshake",
				Status:      "completed",
				EventAnchor: 0,
			},
		},
		events: []TrafficEvent{
			{
				ID:        "evt-versions",
				Direction: "inbound",
				Method:    "GET",
				Path:      "/ocpi/versions",
				RequestHeaders: map[string]string{
					"authorization":          "Token hub-token-b",
					"x-request-id":           "req-1",
					"x-correlation-id":       "corr-1",
					"ocpi-from-country-code": "NL",
					"ocpi-from-party-id":     "EMS",
				},
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":[{"version":"2.2.1","url":"https://hub.example.com/ocpi/2.2.1"}]}`,
				StartedAt:      "2026-01-01T00:00:00Z",
			},
			{
				ID:        "evt-details",
				Direction: "inbound",
				Method:    "GET",
				Path:      "/ocpi/2.2.1",
				RequestHeaders: map[string]string{
					"authorization":          "Token hub-token-b",
					"x-request-id":           "req-2",
					"x-correlation-id":       "corr-2",
					"ocpi-from-country-code": "NL",
					"ocpi-from-party-id":     "EMS",
				},
				ResponseStatus: 200,
				ResponseBody:   `{"status_code":1000,"timestamp":"2026-01-01T00:00:01Z","data":{"version":"2.2.1","endpoints":[]}}`,
				StartedAt:      "2026-01-01T00:00:01Z",
			},
		},
	}

	eval := evalPeerFetchesHubVersions(rt, CaseDefinition{})
	if eval.Status != "passed" {
		t.Fatalf("expected peer hub versions follow-up to pass, got %q with messages %#v", eval.Status, eval.Messages)
	}
}

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, want) {
			return true
		}
	}
	return false
}
