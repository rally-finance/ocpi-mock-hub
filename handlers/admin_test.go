package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAdminTokens(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	tok := map[string]string{"uid": "TOK1", "country_code": "DE", "party_id": "AAA"}
	data, _ := json.Marshal(tok)
	store.PutToken("DE", "AAA", "TOK1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/tokens", nil)
	h.GetAdminTokens(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var tokens []json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &tokens)
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
}

func TestGetAdminReservations(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	res := map[string]string{"id": "RES-1", "status": "RESERVED"}
	data, _ := json.Marshal(res)
	store.PutReservation("RES-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/reservations", nil)
	h.GetAdminReservations(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var reservations []json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &reservations)
	if len(reservations) != 1 {
		t.Errorf("expected 1 reservation, got %d", len(reservations))
	}
}

func TestGetStatus_IncludesReservationCount(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	res := map[string]string{"id": "RES-1"}
	data, _ := json.Marshal(res)
	store.PutReservation("RES-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/status", nil)
	h.GetStatus(w, r)

	var status connectionStatus
	json.Unmarshal(w.Body.Bytes(), &status)
	if status.ReservationCount != 1 {
		t.Errorf("expected reservation_count=1, got %d", status.ReservationCount)
	}
}

func TestGetStatus_ReportsDeregisterReadiness(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	store.tokenB = "peer-token-b"
	store.emspToken = "peer-token-c"
	store.versionsURL = "https://peer.example.com/ocpi/versions"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/status", nil)
	h.GetStatus(w, r)

	var status connectionStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.CanDeregister {
		t.Fatalf("expected can_deregister=true, got false with reason %q", status.DeregisterReason)
	}
	if status.DeregisterReason != "" {
		t.Fatalf("expected empty deregister_reason, got %q", status.DeregisterReason)
	}
}

func TestGetStatus_ReportsIncompleteDeregisterState(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	store.tokenB = "peer-token-b"
	store.versionsURL = "https://peer.example.com/ocpi/versions"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/status", nil)
	h.GetStatus(w, r)

	var status connectionStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.CanDeregister {
		t.Fatal("expected can_deregister=false when peer token is missing")
	}
	if status.DeregisterReason != "Missing stored peer credentials token" {
		t.Fatalf("unexpected deregister_reason %q", status.DeregisterReason)
	}
}

func TestGetStatus_EmptyMaskedTokensStayEmpty(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/status", nil)
	h.GetStatus(w, r)

	var status connectionStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.TokenBMasked != "" {
		t.Fatalf("expected empty token_b_masked, got %q", status.TokenBMasked)
	}
	if status.EMSPOwnToken != "" {
		t.Fatalf("expected empty emsp_own_token_masked, got %q", status.EMSPOwnToken)
	}
}

func TestAdminAuthorize_Allowed(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	tok := map[string]string{"uid": "TOK1"}
	data, _ := json.Marshal(tok)
	store.PutToken("DE", "AAA", "TOK1", data)

	body := `{"country_code":"DE","party_id":"AAA","uid":"TOK1","location_id":"LOC-1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/authorize", strings.NewReader(body))
	h.AdminAuthorize(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["allowed"] != "ALLOWED" {
		t.Errorf("expected ALLOWED, got %v", result["allowed"])
	}
}

func TestAdminAuthorize_NotAllowed(t *testing.T) {
	h := testHandler()

	body := `{"country_code":"XX","party_id":"YY","uid":"NOPE"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/authorize", strings.NewReader(body))
	h.AdminAuthorize(w, r)

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["allowed"] != "NOT_ALLOWED" {
		t.Errorf("expected NOT_ALLOWED, got %v", result["allowed"])
	}
}
