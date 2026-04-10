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

// faultMiddlewareHandler wraps a dummy inner handler with FaultModeMiddleware.
// The inner handler writes 200 + JSON so we can distinguish pass-through from short-circuit.
func faultMiddlewareHandler(h *Handler) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status_code":1000,"data":"ok"}`))
	})
	return FaultModeMiddleware(h)(inner)
}

const faultTestOCPIPath = "/ocpi/2.2.1/sender/sessions"

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

	mw := faultMiddlewareHandler(h)

	got429 := false
	got200 := false
	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, httptest.NewRequest("GET", faultTestOCPIPath, nil))

		if w.Code == http.StatusOK {
			got200 = true
		} else if w.Code == http.StatusTooManyRequests {
			got429 = true
			if w.Header().Get("Retry-After") != "2" {
				t.Error("expected Retry-After: 2 header")
			}
		} else {
			t.Errorf("unexpected status %d", w.Code)
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

	mw := faultMiddlewareHandler(h)

	got500 := false
	gotOK := false
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, httptest.NewRequest("GET", faultTestOCPIPath, nil))

		if w.Code == http.StatusOK {
			gotOK = true
		} else if w.Code == http.StatusInternalServerError {
			got500 = true
		} else {
			t.Errorf("unexpected status %d", w.Code)
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

	mw := faultMiddlewareHandler(h)

	start := time.Now()
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest("GET", faultTestOCPIPath, nil))
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Errorf("slow mode should still return 200 after delay, got %d", w.Code)
	}
	if elapsed < 3*time.Second {
		t.Errorf("expected at least 3s delay, got %v", elapsed)
	}
}

func TestPartialMode_TruncatesJSON(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	for i := 0; i < 3; i++ {
		s := map[string]any{
			"id": "S" + string(rune('A'+i)), "country_code": "DE", "party_id": "AAA",
			"status": "ACTIVE", "last_updated": "2026-01-01T00:00:00Z",
		}
		data, _ := json.Marshal(s)
		store.PutSession(s["id"].(string), data)
	}

	store.mode = "partial"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.GetSessions(w, r)
	})
	mw := FaultModeMiddleware(h)(inner)

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions", nil))

	body := w.Body.Bytes()

	// Get the full body for comparison by running without partial mode
	store.mode = "happy"
	wFull := httptest.NewRecorder()
	h.GetSessions(wFull, httptest.NewRequest("GET", "/ocpi/2.2.1/sender/sessions", nil))
	fullBody := wFull.Body.Bytes()

	if len(body) >= len(fullBody) {
		t.Errorf("expected truncated body (%d bytes) to be shorter than full body (%d bytes)", len(body), len(fullBody))
	}

	var dummy any
	if json.Unmarshal(body, &dummy) == nil {
		t.Error("expected truncated JSON to be invalid, but it parsed successfully")
	}
}

func TestHappyMode_PassesThrough(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.mode = "happy"

	mw := faultMiddlewareHandler(h)

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest("GET", faultTestOCPIPath, nil))

	if w.Code != http.StatusOK {
		t.Errorf("happy mode should pass through, got %d", w.Code)
	}
}

func TestFaultMiddleware_SkipsNonOCPIPaths(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	mw := faultMiddlewareHandler(h)

	nonOCPIPaths := []string{"/admin/mode", "/admin/status", "/api/tick", "/", "/ocpi/versions", "/ocpi/2.2.1/credentials"}

	for _, mode := range []string{"slow", "partial", "rate-limit", "random-500"} {
		store.mode = mode
		for _, path := range nonOCPIPaths {
			w := httptest.NewRecorder()
			start := time.Now()
			mw.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			elapsed := time.Since(start)

			if w.Code != http.StatusOK {
				t.Errorf("mode=%s path=%s: expected 200, got %d", mode, path, w.Code)
			}
			if elapsed > 1*time.Second {
				t.Errorf("mode=%s path=%s: took %v, expected no delay for non-OCPI path", mode, path, elapsed)
			}
		}
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
