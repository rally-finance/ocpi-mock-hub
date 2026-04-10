package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCDRByID_Found(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	cdr := map[string]string{
		"id": "CDR-1", "country_code": "DE", "party_id": "AAA",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(cdr)
	store.PutCDR("CDR-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/cdrs/DE/AAA/CDR-1", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "cdrID": "CDR-1"})

	h.GetCDRByID(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["id"] != "CDR-1" {
		t.Errorf("expected CDR id CDR-1, got %s", result["id"])
	}
}

func TestGetCDRByID_NotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/cdrs/DE/AAA/NOPE", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "cdrID": "NOPE"})

	h.GetCDRByID(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetCDRByID_WrongParty(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	cdr := map[string]string{
		"id": "CDR-1", "country_code": "DE", "party_id": "AAA",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(cdr)
	store.PutCDR("CDR-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/cdrs/NL/BBB/CDR-1", nil)
	r = withChiParams(r, map[string]string{"countryCode": "NL", "partyID": "BBB", "cdrID": "CDR-1"})

	h.GetCDRByID(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}
