package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCDRs_WithOCPIToHeaders(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	c1 := map[string]string{
		"id": "CDR-1", "country_code": "DE", "party_id": "AAA",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	c2 := map[string]string{
		"id": "CDR-2", "country_code": "NL", "party_id": "BBB",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	d1, _ := json.Marshal(c1)
	d2, _ := json.Marshal(c2)
	store.PutCDR("CDR-1", d1)
	store.PutCDR("CDR-2", d2)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/cdrs", nil)
	r.Header.Set("OCPI-To-Country-Code", "NL")
	r.Header.Set("OCPI-To-Party-Id", "BBB")

	h.GetCDRs(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data []json.RawMessage
	json.Unmarshal(resp.Data, &data)
	if len(data) != 1 {
		t.Errorf("expected 1 CDR after filtering, got %d", len(data))
	}
}

func TestGetTokens_WithOCPIToHeaders(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	t1 := map[string]string{
		"uid": "TOK1", "country_code": "DE", "party_id": "AAA",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	t2 := map[string]string{
		"uid": "TOK2", "country_code": "NL", "party_id": "BBB",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	d1, _ := json.Marshal(t1)
	d2, _ := json.Marshal(t2)
	store.PutToken("DE", "AAA", "TOK1", d1)
	store.PutToken("NL", "BBB", "TOK2", d2)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/tokens", nil)
	r.Header.Set("OCPI-To-Country-Code", "DE")
	r.Header.Set("OCPI-To-Party-Id", "AAA")

	h.GetTokens(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data []json.RawMessage
	json.Unmarshal(resp.Data, &data)
	if len(data) != 1 {
		t.Errorf("expected 1 token after filtering, got %d", len(data))
	}
}
