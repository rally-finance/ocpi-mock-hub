package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSessionByID_Found(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]string{
		"id": "SESS-1", "country_code": "DE", "party_id": "AAA",
		"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions/DE/AAA/SESS-1", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "SESS-1"})

	h.GetSessionByID(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["id"] != "SESS-1" {
		t.Errorf("expected session id SESS-1, got %s", result["id"])
	}
}

func TestGetSessionByID_NotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions/DE/AAA/NOPE", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "NOPE"})

	h.GetSessionByID(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetSessionByID_WrongParty(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]string{
		"id": "SESS-1", "country_code": "DE", "party_id": "AAA",
		"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions/NL/BBB/SESS-1", nil)
	r = withChiParams(r, map[string]string{"countryCode": "NL", "partyID": "BBB", "sessionID": "SESS-1"})

	h.GetSessionByID(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestPatchReceiverSession_MergesFields(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]any{
		"id":           "SESS-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"status":       "PENDING",
		"kwh":          1.0,
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	body := `{"status":"ACTIVE","kwh":2.5,"last_updated":"2026-01-01T00:05:00Z"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/sessions/DE/AAA/SESS-1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "SESS-1"})

	h.PatchReceiverSession(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	raw, _ := store.GetSession("SESS-1")
	var merged map[string]any
	json.Unmarshal(raw, &merged)
	if merged["status"] != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %v", merged["status"])
	}
	if merged["kwh"].(float64) != 2.5 {
		t.Errorf("expected kwh 2.5, got %v", merged["kwh"])
	}
	if merged["id"] != "SESS-1" {
		t.Errorf("expected id preserved, got %v", merged["id"])
	}
}

func TestPatchReceiverSession_AppendsChargingPeriods(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]any{
		"id":           "SESS-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"status":       "ACTIVE",
		"charging_periods": []map[string]any{
			{"start_date_time": "2026-01-01T00:00:00Z", "dimensions": []any{}},
		},
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	body := `{"charging_periods":[{"start_date_time":"2026-01-01T00:05:00Z","dimensions":[]}],"last_updated":"2026-01-01T00:05:00Z"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/sessions/DE/AAA/SESS-1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "SESS-1"})

	h.PatchReceiverSession(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d", w.Code)
	}
	raw, _ := store.GetSession("SESS-1")
	var merged map[string]any
	json.Unmarshal(raw, &merged)
	periods, ok := merged["charging_periods"].([]any)
	if !ok || len(periods) != 2 {
		t.Fatalf("expected charging_periods to grow to 2, got %v", merged["charging_periods"])
	}
}

func TestPatchReceiverSession_URLWinsOverBodyIdentity(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]any{
		"id":           "SESS-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"status":       "ACTIVE",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	// PATCH body attempts to rename and transfer ownership.
	body := `{"id":"HIJACK","country_code":"NL","party_id":"BBB","status":"COMPLETED","last_updated":"2026-01-01T00:05:00Z"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/sessions/DE/AAA/SESS-1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "SESS-1"})

	h.PatchReceiverSession(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	raw, _ := store.GetSession("SESS-1")
	var merged map[string]any
	json.Unmarshal(raw, &merged)
	if merged["id"] != "SESS-1" {
		t.Errorf("expected id SESS-1, got %v", merged["id"])
	}
	if merged["country_code"] != "DE" {
		t.Errorf("expected country_code DE, got %v", merged["country_code"])
	}
	if merged["party_id"] != "AAA" {
		t.Errorf("expected party_id AAA, got %v", merged["party_id"])
	}
	if merged["status"] != "COMPLETED" {
		t.Errorf("expected status update to apply, got %v", merged["status"])
	}
	// No stray session should have been minted under the hijacked id.
	if hijack, _ := store.GetSession("HIJACK"); hijack != nil {
		t.Error("expected no session at hijacked id")
	}
}

func TestPatchReceiverSession_RequiresLastUpdated(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	data, _ := json.Marshal(map[string]any{
		"id": "SESS-1", "country_code": "DE", "party_id": "AAA",
		"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
	})
	store.PutSession("SESS-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/sessions/DE/AAA/SESS-1", strings.NewReader(`{"status":"COMPLETED"}`))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "SESS-1"})

	h.PatchReceiverSession(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
}

func TestPatchReceiverSession_NotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/sessions/DE/AAA/UNKNOWN", strings.NewReader(`{"last_updated":"2026-01-01T00:00:00Z"}`))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "UNKNOWN"})

	h.PatchReceiverSession(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
}

func TestGetSessions_WithOCPIToHeaders(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	s1 := map[string]string{
		"id": "S1", "country_code": "DE", "party_id": "AAA",
		"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
	}
	s2 := map[string]string{
		"id": "S2", "country_code": "NL", "party_id": "BBB",
		"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
	}
	d1, _ := json.Marshal(s1)
	d2, _ := json.Marshal(s2)
	store.PutSession("S1", d1)
	store.PutSession("S2", d2)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions", nil)
	r.Header.Set("OCPI-To-Country-Code", "DE")
	r.Header.Set("OCPI-To-Party-Id", "AAA")

	h.GetSessions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data []json.RawMessage
	json.Unmarshal(resp.Data, &data)
	if len(data) != 1 {
		t.Errorf("expected 1 session after filtering, got %d", len(data))
	}
}
