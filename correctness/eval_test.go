package correctness

import (
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

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, want) {
			return true
		}
	}
	return false
}
