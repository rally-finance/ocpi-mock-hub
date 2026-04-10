package simulation

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type mockStore struct {
	sessions     map[string][]byte
	cdrs         map[string][]byte
	reservations map[string][]byte
	mode         string
	callbackURL  string
	emspToken    string
	tokenB       string
}

func newMockStore() *mockStore {
	return &mockStore{
		sessions:     make(map[string][]byte),
		cdrs:         make(map[string][]byte),
		reservations: make(map[string][]byte),
		mode:         "happy",
	}
}

func (s *mockStore) GetTokenB() (string, error)          { return s.tokenB, nil }
func (s *mockStore) GetEMSPCallbackURL() (string, error) { return s.callbackURL, nil }
func (s *mockStore) GetEMSPOwnToken() (string, error)    { return s.emspToken, nil }
func (s *mockStore) PutSession(id string, data []byte) error {
	s.sessions[id] = data
	return nil
}
func (s *mockStore) GetSession(id string) ([]byte, error) { return s.sessions[id], nil }
func (s *mockStore) ListSessions() ([][]byte, error) {
	r := make([][]byte, 0, len(s.sessions))
	for _, v := range s.sessions {
		r = append(r, v)
	}
	return r, nil
}
func (s *mockStore) DeleteSession(id string) error { delete(s.sessions, id); return nil }
func (s *mockStore) PutCDR(id string, data []byte) error {
	s.cdrs[id] = data
	return nil
}
func (s *mockStore) PutReservation(id string, data []byte) error {
	s.reservations[id] = data
	return nil
}
func (s *mockStore) ListReservations() ([][]byte, error) {
	r := make([][]byte, 0, len(s.reservations))
	for _, v := range s.reservations {
		r = append(r, v)
	}
	return r, nil
}
func (s *mockStore) DeleteReservation(id string) error { delete(s.reservations, id); return nil }
func (s *mockStore) GetMode() (string, error)          { return s.mode, nil }

func TestTick_PendingToActive(t *testing.T) {
	store := newMockStore()

	past := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-1",
		StartDateTime: past,
		Status:        "PENDING",
		CreatedAt:     past,
		ResponseURL:   "",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	sim := New(store, nil, "", 100, 60)

	if err := sim.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	raw, _ := store.GetSession("SESS-1")
	if raw == nil {
		t.Fatal("session not found after tick")
	}

	var updated sessionRecord
	json.Unmarshal(raw, &updated)
	if updated.Status != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %s", updated.Status)
	}
	if updated.ActivatedAt == "" {
		t.Error("expected ActivatedAt to be set")
	}
}

func TestTick_ActiveToCompleted(t *testing.T) {
	store := newMockStore()

	past := time.Now().UTC().Add(-120 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-2",
		StartDateTime: past,
		Status:        "ACTIVE",
		CreatedAt:     past,
		ActivatedAt:   past,
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-2", data)

	sim := New(store, nil, "", 100, 5)

	if err := sim.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	raw, _ := store.GetSession("SESS-2")
	var updated sessionRecord
	json.Unmarshal(raw, &updated)
	if updated.Status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", updated.Status)
	}

	cdrs := store.cdrs
	if len(cdrs) != 1 {
		t.Errorf("expected 1 CDR, got %d", len(cdrs))
	}
}

func TestTick_StoppingToCompleted(t *testing.T) {
	store := newMockStore()

	past := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-3",
		StartDateTime: past,
		Status:        "STOPPING",
		CreatedAt:     past,
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-3", data)

	sim := New(store, nil, "", 100, 60)

	if err := sim.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	raw, _ := store.GetSession("SESS-3")
	var updated sessionRecord
	json.Unmarshal(raw, &updated)
	if updated.Status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", updated.Status)
	}

	if len(store.cdrs) != 1 {
		t.Errorf("expected 1 CDR, got %d", len(store.cdrs))
	}
}

func TestTick_ReservationExpiry(t *testing.T) {
	store := newMockStore()

	expired := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	res := reservationRecord{
		ID:           "RES-1",
		ExpiryDate:   expired,
		Status:       "RESERVED",
		CreatedAt:    expired,
		CallbackSent: true,
	}
	data, _ := json.Marshal(res)
	store.PutReservation("RES-1", data)

	sim := New(store, nil, "", 100, 60)
	if err := sim.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(store.reservations) != 0 {
		t.Errorf("expected reservation to be deleted after expiry, got %d", len(store.reservations))
	}
}

func TestTick_ReservationCallback(t *testing.T) {
	var callbackCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callbackCount, 1)
		body, _ := io.ReadAll(r.Body)
		var cb map[string]any
		json.Unmarshal(body, &cb)
		if cb["result"] != "ACCEPTED" {
			t.Errorf("callback result: got %v, want ACCEPTED", cb["result"])
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	store := newMockStore()
	past := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339)
	res := reservationRecord{
		ID:           "RES-2",
		ExpiryDate:   time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
		Status:       "RESERVED",
		ResponseURL:  ts.URL + "/callback",
		CreatedAt:    past,
		CallbackSent: false,
	}
	data, _ := json.Marshal(res)
	store.PutReservation("RES-2", data)

	sim := New(store, nil, "", 100, 60)
	sim.Tick()

	if atomic.LoadInt32(&callbackCount) != 1 {
		t.Errorf("expected 1 callback, got %d", callbackCount)
	}
}

func TestTick_PushesToEMSP(t *testing.T) {
	var pushCount int32
	var lastMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&pushCount, 1)
		lastMethod = r.Method
		w.WriteHeader(200)
	}))
	defer ts.Close()

	store := newMockStore()
	store.callbackURL = ts.URL
	store.emspToken = "test-emsp-token"

	past := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-PUSH",
		StartDateTime: past,
		Status:        "PENDING",
		CreatedAt:     past,
		ResponseURL:   ts.URL + "/callback",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-PUSH", data)

	sim := New(store, nil, ts.URL, 100, 60)
	sim.Tick()

	if atomic.LoadInt32(&pushCount) < 2 {
		t.Errorf("expected at least 2 pushes (callback + session PUT), got %d", pushCount)
	}
	_ = lastMethod
}
