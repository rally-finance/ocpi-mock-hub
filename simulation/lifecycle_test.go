package simulation

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
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
func (s *mockStore) DeleteReservation(id string) error                   { delete(s.reservations, id); return nil }
func (s *mockStore) GetChargingProfile(sessionID string) ([]byte, error) { return nil, nil }
func (s *mockStore) GetMode() (string, error)                            { return s.mode, nil }

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

func TestTick_EVSEStatusChangeOnActivation(t *testing.T) {
	var evseStatusPushed string
	var evseUIDPushed string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/receiver/locations/") {
			body, _ := io.ReadAll(r.Body)
			var update map[string]any
			json.Unmarshal(body, &update)
			if s, ok := update["status"].(string); ok {
				evseStatusPushed = s
			}
			if u, ok := update["uid"].(string); ok {
				evseUIDPushed = u
			}
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	seed := &fakegen.SeedData{
		Locations: []fakegen.Location{
			{
				CountryCode: "DE",
				PartyID:     "AAA",
				ID:          "LOC-1",
				EVSEs: []fakegen.EVSE{{
					UID:    "EVSE-1",
					Status: "AVAILABLE",
					Connectors: []fakegen.Connector{
						{ID: "C1", Standard: "IEC_62196_T2"},
					},
				}},
			},
		},
	}

	store := newMockStore()
	store.callbackURL = ts.URL
	store.emspToken = "test-token"

	past := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-EVSE",
		StartDateTime: past,
		Status:        "PENDING",
		CreatedAt:     past,
		LocationID:    "LOC-1",
		EvseUID:       "EVSE-1",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-EVSE", data)

	sim := New(store, seed, ts.URL, 100, 60)
	sim.Tick()

	if evseStatusPushed != "CHARGING" {
		t.Errorf("expected EVSE status push CHARGING, got %q", evseStatusPushed)
	}
	if evseUIDPushed != "EVSE-1" {
		t.Errorf("expected full EVSE object with uid EVSE-1, got %q", evseUIDPushed)
	}
}

func TestTick_EVSEStatusChangeOnCompletion(t *testing.T) {
	var evseStatusPushed string
	var evseUIDPushed string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/receiver/locations/") {
			body, _ := io.ReadAll(r.Body)
			var update map[string]any
			json.Unmarshal(body, &update)
			if s, ok := update["status"].(string); ok {
				evseStatusPushed = s
			}
			if u, ok := update["uid"].(string); ok {
				evseUIDPushed = u
			}
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	seed := &fakegen.SeedData{
		Locations: []fakegen.Location{
			{
				CountryCode: "DE",
				PartyID:     "AAA",
				ID:          "LOC-1",
				EVSEs: []fakegen.EVSE{{
					UID:    "EVSE-1",
					Status: "AVAILABLE",
					Connectors: []fakegen.Connector{
						{ID: "C1", Standard: "IEC_62196_T2"},
					},
				}},
			},
		},
	}

	store := newMockStore()
	store.callbackURL = ts.URL
	store.emspToken = "test-token"

	past := time.Now().UTC().Add(-120 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-EVSE2",
		StartDateTime: past,
		Status:        "ACTIVE",
		CreatedAt:     past,
		ActivatedAt:   past,
		LocationID:    "LOC-1",
		EvseUID:       "EVSE-1",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-EVSE2", data)

	sim := New(store, seed, ts.URL, 100, 5)
	sim.Tick()

	if evseStatusPushed != "AVAILABLE" {
		t.Errorf("expected EVSE status push AVAILABLE, got %q", evseStatusPushed)
	}
	if evseUIDPushed != "EVSE-1" {
		t.Errorf("expected full EVSE object with uid EVSE-1, got %q", evseUIDPushed)
	}
}

func TestTick_EVSENotSetAvailableWhenOtherSessionActive(t *testing.T) {
	var evseStatusPushed string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/receiver/locations/") {
			body, _ := io.ReadAll(r.Body)
			var update map[string]any
			json.Unmarshal(body, &update)
			if s, ok := update["status"].(string); ok {
				evseStatusPushed = s
			}
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	seed := &fakegen.SeedData{
		Locations: []fakegen.Location{
			{
				CountryCode: "DE",
				PartyID:     "AAA",
				ID:          "LOC-1",
				EVSEs:       []fakegen.EVSE{{UID: "EVSE-1", Status: "AVAILABLE"}},
			},
		},
	}

	store := newMockStore()
	store.callbackURL = ts.URL
	store.emspToken = "test-token"

	past := time.Now().UTC().Add(-120 * time.Second).Format(time.RFC3339)

	// Session being completed
	s1 := sessionRecord{
		CountryCode: "DE", PartyID: "AAA", ID: "SESS-A",
		StartDateTime: past, Status: "ACTIVE", CreatedAt: past,
		ActivatedAt: past, LocationID: "LOC-1", EvseUID: "EVSE-1",
	}
	d1, _ := json.Marshal(s1)
	store.PutSession("SESS-A", d1)

	// Another active session on the same EVSE
	s2 := sessionRecord{
		CountryCode: "DE", PartyID: "AAA", ID: "SESS-B",
		StartDateTime: past, Status: "ACTIVE", CreatedAt: past,
		ActivatedAt: time.Now().UTC().Format(time.RFC3339),
		LocationID:  "LOC-1", EvseUID: "EVSE-1",
	}
	d2, _ := json.Marshal(s2)
	store.PutSession("SESS-B", d2)

	sim := New(store, seed, ts.URL, 100, 5)
	sim.Tick()

	// SESS-A completes but SESS-B is still active, so EVSE should NOT be pushed as AVAILABLE
	if evseStatusPushed == "AVAILABLE" {
		t.Error("should not push AVAILABLE when another session is still active on the same EVSE")
	}
}

func TestTick_StoppingCallbackFires(t *testing.T) {
	var callbackCount int32
	var lastBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stop-cb") {
			atomic.AddInt32(&callbackCount, 1)
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &lastBody)
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	store := newMockStore()
	past := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-STOP-CB",
		StartDateTime: past,
		Status:        "STOPPING",
		CreatedAt:     past,
		ResponseURL:   ts.URL + "/stop-cb",
		CallbackSent:  false,
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-STOP-CB", data)

	sim := New(store, nil, "", 100, 60)
	sim.Tick()

	if atomic.LoadInt32(&callbackCount) != 1 {
		t.Errorf("expected 1 stop callback, got %d", callbackCount)
	}
	if lastBody["result"] != "ACCEPTED" {
		t.Errorf("expected callback result ACCEPTED, got %v", lastBody["result"])
	}
	if lastBody["session_id"] != "SESS-STOP-CB" {
		t.Errorf("expected session_id in callback, got %v", lastBody["session_id"])
	}
}

func TestTick_AuthHeaderOnPush(t *testing.T) {
	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capturedAuth == "" {
			capturedAuth = r.Header.Get("Authorization")
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	store := newMockStore()
	store.callbackURL = ts.URL
	store.emspToken = "secret-emsp-token"

	past := time.Now().UTC().Add(-2 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-AUTH",
		StartDateTime: past,
		Status:        "PENDING",
		CreatedAt:     past,
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-AUTH", data)

	sim := New(store, nil, ts.URL, 100, 60)
	sim.Tick()

	if capturedAuth != "Token secret-emsp-token" {
		t.Errorf("expected Authorization header 'Token secret-emsp-token', got %q", capturedAuth)
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

func TestCDR_HasEnrichedFields(t *testing.T) {
	store := newMockStore()

	past := time.Now().UTC().Add(-120 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-ENRICH",
		StartDateTime: past,
		Status:        "ACTIVE",
		CreatedAt:     past,
		ActivatedAt:   past,
		Currency:      "EUR",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-ENRICH", data)

	sim := New(store, nil, "", 100, 5)
	sim.Tick()

	if len(store.cdrs) != 1 {
		t.Fatalf("expected 1 CDR, got %d", len(store.cdrs))
	}

	for _, cdrData := range store.cdrs {
		var cdr map[string]any
		json.Unmarshal(cdrData, &cdr)

		if cdr["remark"] != "Mock-generated CDR" {
			t.Errorf("expected remark='Mock-generated CDR', got %v", cdr["remark"])
		}

		if cdr["total_energy_cost"] == nil {
			t.Error("expected total_energy_cost in CDR")
		}
		if cdr["total_time_cost"] == nil {
			t.Error("expected total_time_cost in CDR")
		}
		if cdr["total_fixed_cost"] == nil {
			t.Error("expected total_fixed_cost in CDR")
		}
		if cdr["total_parking_cost"] == nil {
			t.Error("expected total_parking_cost in CDR")
		}
		if cdr["total_parking_time"] == nil {
			t.Error("expected total_parking_time in CDR")
		}

		cp, ok := cdr["charging_periods"].([]any)
		if !ok || len(cp) == 0 {
			t.Error("expected non-empty charging_periods in CDR")
		}

		// Stage 6 — OCPI 2.2.1 optional CDR fields. invoice_reference_id,
		// total_reservation_cost, home_charging_compensation, and signed_data
		// are always emitted by the mock; meter_id / authorization_reference
		// only when the originating session carried them, which this test
		// does not set, so we only assert the always-on fields.
		if cdr["invoice_reference_id"] == nil {
			t.Error("expected invoice_reference_id in CDR")
		}
		if cdr["total_reservation_cost"] == nil {
			t.Error("expected total_reservation_cost in CDR")
		}
		if cdr["home_charging_compensation"] == nil {
			t.Error("expected home_charging_compensation in CDR")
		}
		if cdr["signed_data"] == nil {
			t.Error("expected signed_data in CDR")
		}
	}
}

func TestCDR_CarriesSessionMeterAndAuthRef(t *testing.T) {
	store := newMockStore()

	past := time.Now().UTC().Add(-120 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:            "DE",
		PartyID:                "AAA",
		ID:                     "SESS-METER",
		StartDateTime:          past,
		Status:                 "ACTIVE",
		CreatedAt:              past,
		ActivatedAt:            past,
		Currency:               "EUR",
		MeterID:                "METER-XYZ",
		AuthorizationReference: "AUTH-42",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-METER", data)

	sim := New(store, nil, "", 100, 5)
	sim.Tick()

	if len(store.cdrs) != 1 {
		t.Fatalf("expected 1 CDR, got %d", len(store.cdrs))
	}
	for _, cdrData := range store.cdrs {
		var cdr map[string]any
		json.Unmarshal(cdrData, &cdr)
		if cdr["meter_id"] != "METER-XYZ" {
			t.Errorf("expected meter_id propagated to CDR, got %v", cdr["meter_id"])
		}
		if cdr["authorization_reference"] != "AUTH-42" {
			t.Errorf("expected authorization_reference propagated to CDR, got %v",
				cdr["authorization_reference"])
		}
	}
}

func TestSession_HasChargingPeriods(t *testing.T) {
	store := newMockStore()

	past := time.Now().UTC().Add(-5 * time.Second).Format(time.RFC3339)
	session := sessionRecord{
		CountryCode:   "DE",
		PartyID:       "AAA",
		ID:            "SESS-CP",
		StartDateTime: past,
		Status:        "ACTIVE",
		CreatedAt:     past,
		ActivatedAt:   past,
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-CP", data)

	sim := New(store, nil, "", 100, 60)
	sim.Tick()

	raw, _ := store.GetSession("SESS-CP")
	var updated map[string]any
	json.Unmarshal(raw, &updated)

	cp, ok := updated["charging_periods"].([]any)
	if !ok || len(cp) == 0 {
		t.Error("expected non-empty charging_periods on active session after tick")
	}

	period, ok := cp[0].(map[string]any)
	if !ok {
		t.Fatal("first charging period is not a map")
	}
	dims, ok := period["dimensions"].([]any)
	if !ok || len(dims) < 2 {
		t.Error("expected at least 2 dimensions (ENERGY and TIME)")
	}
}
