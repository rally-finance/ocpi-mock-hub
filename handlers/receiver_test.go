package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPutReceiverSession_Happy(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	body := `{"country_code":"DE","party_id":"AAA","status":"ACTIVE","kwh":5.5,"last_updated":"2026-01-01T00:00:00Z"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/sessions/DE/AAA/SESS-EXT-1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "sessionID": "SESS-EXT-1"})

	h.PutReceiverSession(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	raw, _ := store.GetSession("SESS-EXT-1")
	if raw == nil {
		t.Fatal("session not stored")
	}

	var stored map[string]any
	json.Unmarshal(raw, &stored)
	if stored["id"] != "SESS-EXT-1" {
		t.Errorf("expected id=SESS-EXT-1, got %v", stored["id"])
	}
}

func TestPostReceiverCDR_Happy(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	body := `{"id":"CDR-EXT-1","country_code":"DE","party_id":"AAA","total_energy":20.5,"last_updated":"2026-01-01T00:00:00Z"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/ocpi/2.2.1/receiver/cdrs", strings.NewReader(body))

	h.PostReceiverCDR(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	if loc := w.Header().Get("Location"); !strings.Contains(loc, "CDR-EXT-1") {
		t.Errorf("expected Location header with CDR-EXT-1, got %s", loc)
	}

	raw, _ := store.GetCDR("CDR-EXT-1")
	if raw == nil {
		t.Fatal("CDR not stored")
	}
}

func TestPostReceiverCDR_MissingID(t *testing.T) {
	h := testHandler()

	body := `{"country_code":"DE","party_id":"AAA"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/ocpi/2.2.1/receiver/cdrs", strings.NewReader(body))

	h.PostReceiverCDR(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestGetReceiverCDR_Found(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	cdr := map[string]string{"id": "CDR-RCV-1", "country_code": "DE", "party_id": "AAA"}
	data, _ := json.Marshal(cdr)
	store.PutCDR("CDR-RCV-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/receiver/cdrs/CDR-RCV-1", nil)
	r = withChiParams(r, map[string]string{"cdrID": "CDR-RCV-1"})

	h.GetReceiverCDR(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
}

func TestGetReceiverCDR_NotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/receiver/cdrs/NOPE", nil)
	r = withChiParams(r, map[string]string{"cdrID": "NOPE"})

	h.GetReceiverCDR(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}
