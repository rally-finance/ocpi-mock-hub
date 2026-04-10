package handlers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPaginationStressMode_ForcesLimit1(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "pagination-stress"

	for i := 0; i < 5; i++ {
		s := map[string]any{
			"id": "S" + string(rune('A'+i)), "country_code": "DE", "party_id": "AAA",
			"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
		}
		data, _ := json.Marshal(s)
		store.PutSession(s["id"].(string), data)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions", nil)
	h.GetSessions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	if w.Header().Get("X-Total-Count") != "5" {
		t.Errorf("X-Total-Count: got %s, want 5", w.Header().Get("X-Total-Count"))
	}
	if w.Header().Get("X-Limit") != "1" {
		t.Errorf("X-Limit: got %s, want 1", w.Header().Get("X-Limit"))
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data []json.RawMessage
	json.Unmarshal(resp.Data, &data)
	if len(data) != 1 {
		t.Errorf("expected 1 item in page, got %d", len(data))
	}
}

func TestRateLimitMode_Returns429(t *testing.T) {
	rand.Seed(42)
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "rate-limit"

	got429 := false
	got200 := false
	for i := 0; i < 50; i++ {
		pw := wrapPartialWriter(httptest.NewRecorder())
		w := httptest.NewRecorder()
		_ = pw

		if h.applyFaultMode(w, httptest.NewRequest("GET", "/", nil)) {
			got200 = true
		} else {
			got429 = true
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("expected 429, got %d", w.Code)
			}
			if w.Header().Get("Retry-After") != "2" {
				t.Error("expected Retry-After: 2 header")
			}
		}
	}

	if !got429 {
		t.Error("expected at least one 429 in 50 requests")
	}
	if !got200 {
		t.Error("expected at least one pass-through in 50 requests")
	}
}

func TestRandom500Mode_ReturnsErrors(t *testing.T) {
	rand.Seed(42)
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "random-500"

	got500 := false
	gotOK := false
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		if h.applyFaultMode(w, httptest.NewRequest("GET", "/", nil)) {
			gotOK = true
		} else {
			got500 = true
			if w.Code != http.StatusInternalServerError {
				t.Errorf("expected 500, got %d", w.Code)
			}
		}
	}

	if !got500 {
		t.Error("expected at least one 500 in 100 requests")
	}
	if !gotOK {
		t.Error("expected at least one pass-through in 100 requests")
	}
}

func TestAuthFailMode_RejectsAuthorize(t *testing.T) {
	rand.Seed(42)
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "auth-fail"

	tok := map[string]string{"uid": "TOK1"}
	data, _ := json.Marshal(tok)
	store.PutToken("DE", "AAA", "TOK1", data)

	responses := map[string]bool{}
	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/authorize", nil)
		r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})
		h.PostTokenAuthorize(w, r)

		var resp ocpiResp
		json.Unmarshal(w.Body.Bytes(), &resp)
		var result map[string]any
		json.Unmarshal(resp.Data, &result)
		if allowed, ok := result["allowed"].(string); ok {
			responses[allowed] = true
		}
	}

	if responses["ALLOWED"] {
		t.Error("auth-fail mode should never return ALLOWED")
	}

	validRejections := []string{"NOT_ALLOWED", "EXPIRED", "BLOCKED"}
	hasAny := false
	for _, v := range validRejections {
		if responses[v] {
			hasAny = true
		}
	}
	if !hasAny {
		t.Error("expected at least one rejection response")
	}
}

func TestSlowMode_AddsDelay(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "slow"

	start := time.Now()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	proceed := h.applyFaultMode(w, r)
	elapsed := time.Since(start)

	if !proceed {
		t.Error("slow mode should still proceed after delay")
	}
	if elapsed < 3*time.Second {
		t.Errorf("expected at least 3s delay, got %v", elapsed)
	}
}

func TestPartialMode_TruncatesJSON(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "pagination-stress" // use to populate data

	for i := 0; i < 3; i++ {
		s := map[string]any{
			"id": "S" + string(rune('A'+i)), "country_code": "DE", "party_id": "AAA",
			"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
		}
		data, _ := json.Marshal(s)
		store.PutSession(s["id"].(string), data)
	}

	// Switch to partial mode for the actual test
	store.mode = "partial"

	pw := wrapPartialWriter(httptest.NewRecorder())
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions", nil)
	h.GetSessions(pw, r)
	pw.Flush()

	body := pw.buf
	truncated := pw.ResponseWriter.(*httptest.ResponseRecorder).Body.Bytes()

	// The truncated output should be shorter than the full body
	if len(truncated) >= len(body) {
		t.Errorf("expected truncated body (%d bytes) to be shorter than full body (%d bytes)", len(truncated), len(body))
	}

	// The truncated output should NOT be valid JSON
	var dummy any
	if json.Unmarshal(truncated, &dummy) == nil {
		t.Error("expected truncated JSON to be invalid, but it parsed successfully")
	}
}

func TestSlowMode_Proceeds(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "happy"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	if !h.applyFaultMode(w, r) {
		t.Error("happy mode should proceed")
	}
}

func TestSetMode_NewModes(t *testing.T) {
	h := testHandler()

	for _, mode := range []string{"rate-limit", "random-500", "auth-fail"} {
		body := `{"mode":"` + mode + `"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/admin/mode", strings.NewReader(body))
		h.SetMode(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("mode %s: got status %d, want 200", mode, w.Code)
		}
	}
}
