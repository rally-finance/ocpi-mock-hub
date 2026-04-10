package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
