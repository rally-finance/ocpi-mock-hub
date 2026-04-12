package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type failingTickStore struct {
	*testStore
}

func (s *failingTickStore) ListSessions() ([][]byte, error) {
	return nil, fmt.Errorf("boom")
}

func TestTickReturnsJSONErrorResponse(t *testing.T) {
	h := testHandler()
	h.Store = &failingTickStore{testStore: newTestStore()}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/tick", nil)
	h.Tick(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["error"] == "" {
		t.Fatal("expected JSON error message in tick response")
	}
}

func TestTriggerTickDoesNotFallbackToBaseCallbackForCorrectnessOverlay(t *testing.T) {
	h := testCorrectnessHandler()
	h.Config.EMSPCallbackURL = "https://base.example.com/ocpi/2.2.1"
	h.Config.CommandDelayMS = 100
	h.Config.SessionDurationS = 60

	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	h.Config.EMSPCallbackURL = ts.URL

	session := createCorrectnessSessionForTest(t, h)
	past := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339)
	record := SessionRecord{
		CountryCode:   "NL",
		PartyID:       "EMS",
		ID:            "SESS-1",
		StartDateTime: past,
		Status:        "PENDING",
		CreatedAt:     past,
		LocationID:    "LOC-1",
		EvseUID:       "EVSE-1",
		LastUpdated:   past,
	}
	raw, _ := json.Marshal(record)
	if err := h.correctnessStore(session.ID).PutSession(record.ID, raw); err != nil {
		t.Fatalf("put overlay session: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/tick", nil)
	h.TriggerTick(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("expected correctness overlay tick not to call the base callback URL, got %d request(s)", got)
	}

	updated, err := h.correctnessStore(session.ID).GetSession(record.ID)
	if err != nil {
		t.Fatalf("get overlay session: %v", err)
	}
	var sessionAfter SessionRecord
	if err := json.Unmarshal(updated, &sessionAfter); err != nil {
		t.Fatalf("decode overlay session: %v", err)
	}
	if sessionAfter.Status != "ACTIVE" {
		t.Fatalf("expected overlay session to advance to ACTIVE, got %q", sessionAfter.Status)
	}
}
