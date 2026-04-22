package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

// chargingPrefsHandler builds a handler with an EVSE that supports charging
// preferences and an already-active session wired to it.
func chargingPrefsHandler(t *testing.T, options ...func(*fakegen.EVSE, *map[string]any)) *Handler {
	t.Helper()
	evse := fakegen.EVSE{
		UID:          "EVSE-1",
		Connectors:   []fakegen.Connector{{ID: "C1"}},
		Capabilities: []string{"RFID_READER", capabilityChargingPreferences},
	}
	session := map[string]any{
		"id":           "SESS-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"status":       "ACTIVE",
		"location_id":  "LOC-1",
		"evse_uid":     "EVSE-1",
		"last_updated": "2026-01-01T00:00:00Z",
	}

	for _, opt := range options {
		opt(&evse, &session)
	}

	seed := &fakegen.SeedData{
		Locations: []fakegen.Location{{
			CountryCode: "DE", PartyID: "AAA", ID: "LOC-1",
			EVSEs:       []fakegen.EVSE{evse},
			LastUpdated: "2026-01-01T00:00:00Z",
		}},
	}
	h := &Handler{
		Config: HandlerConfig{TokenA: "test-token-a"},
		Store:  newTestStore(),
		Seed:   seed,
		ReqLog: NewRequestLog(),
	}

	data, _ := json.Marshal(session)
	if err := h.Store.PutSession("SESS-1", data); err != nil {
		t.Fatalf("put session: %v", err)
	}
	return h
}

func putChargingPreferences(h *Handler, sessionID, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/sender/sessions/"+sessionID+"/charging_preferences", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"sessionID": sessionID})
	h.PutChargingPreferences(w, r)
	return w
}

func decodePreferencesResult(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp ocpiResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var result string
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		t.Fatalf("decode preferences enum: %v", err)
	}
	return result
}

func TestPutChargingPreferences_AcceptedRegular(t *testing.T) {
	h := chargingPrefsHandler(t)

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"REGULAR"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if got := decodePreferencesResult(t, w); got != chargingPreferencesAccepted {
		t.Fatalf("expected ACCEPTED, got %s", got)
	}

	raw, _ := h.Store.GetSession("SESS-1")
	var session map[string]any
	json.Unmarshal(raw, &session)
	prefs, ok := session["charging_preferences"].(map[string]any)
	if !ok {
		t.Fatalf("expected charging_preferences persisted on session, got %v", session["charging_preferences"])
	}
	if prefs["profile_type"] != "REGULAR" {
		t.Errorf("expected profile_type REGULAR persisted, got %v", prefs["profile_type"])
	}
}

func TestPutChargingPreferences_CheapRequiresDepartureThenEnergy(t *testing.T) {
	h := chargingPrefsHandler(t)

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"CHEAP"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesDepartureReqd {
		t.Fatalf("expected DEPARTURE_REQUIRED, got %s", got)
	}

	w = putChargingPreferences(h, "SESS-1", `{"profile_type":"CHEAP","departure_time":"2026-01-01T10:00:00Z"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesEnergyNeedReqd {
		t.Fatalf("expected ENERGY_NEED_REQUIRED, got %s", got)
	}

	w = putChargingPreferences(h, "SESS-1", `{"profile_type":"CHEAP","departure_time":"2026-01-01T10:00:00Z","energy_need":20.5}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesAccepted {
		t.Fatalf("expected ACCEPTED, got %s", got)
	}
}

func TestPutChargingPreferences_GreenRequiresDepartureButNotEnergy(t *testing.T) {
	h := chargingPrefsHandler(t)

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"GREEN"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesDepartureReqd {
		t.Fatalf("expected DEPARTURE_REQUIRED, got %s", got)
	}

	w = putChargingPreferences(h, "SESS-1", `{"profile_type":"GREEN","departure_time":"2026-01-01T10:00:00Z"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesAccepted {
		t.Fatalf("expected ACCEPTED, got %s", got)
	}
}

func TestPutChargingPreferences_FastRequiresDepartureAndEnergy(t *testing.T) {
	h := chargingPrefsHandler(t)

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"FAST"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesDepartureReqd {
		t.Fatalf("expected DEPARTURE_REQUIRED, got %s", got)
	}

	w = putChargingPreferences(h, "SESS-1", `{"profile_type":"FAST","departure_time":"2026-01-01T10:00:00Z"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesEnergyNeedReqd {
		t.Fatalf("expected ENERGY_NEED_REQUIRED, got %s", got)
	}

	w = putChargingPreferences(h, "SESS-1", `{"profile_type":"FAST","departure_time":"2026-01-01T10:00:00Z","energy_need":30}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesAccepted {
		t.Fatalf("expected ACCEPTED, got %s", got)
	}
}

func TestPutChargingPreferences_UnsupportedProfileType(t *testing.T) {
	h := chargingPrefsHandler(t)

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"TURBO"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesProfileNotSup {
		t.Fatalf("expected PROFILE_TYPE_NOT_SUPPORTED, got %s", got)
	}

	// COMFORT used to be in our enum by mistake; spec doesn't define it.
	w = putChargingPreferences(h, "SESS-1", `{"profile_type":"COMFORT"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesProfileNotSup {
		t.Fatalf("COMFORT is not a spec ProfileType; expected PROFILE_TYPE_NOT_SUPPORTED, got %s", got)
	}
}

func TestPutChargingPreferences_NotPossibleWhenEVSENotCapable(t *testing.T) {
	h := chargingPrefsHandler(t, func(e *fakegen.EVSE, _ *map[string]any) {
		e.Capabilities = []string{"RFID_READER"}
	})

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"REGULAR"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesNotPossible {
		t.Fatalf("expected NOT_POSSIBLE, got %s", got)
	}
}

func TestPutChargingPreferences_NotPossibleWhenSessionNotActive(t *testing.T) {
	h := chargingPrefsHandler(t, func(_ *fakegen.EVSE, s *map[string]any) {
		(*s)["status"] = "PENDING"
	})

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"REGULAR"}`)
	if got := decodePreferencesResult(t, w); got != chargingPreferencesNotPossible {
		t.Fatalf("expected NOT_POSSIBLE, got %s", got)
	}
}

func TestPutChargingPreferences_UnknownSession(t *testing.T) {
	h := chargingPrefsHandler(t)

	w := putChargingPreferences(h, "MISSING", `{"profile_type":"REGULAR"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPutChargingPreferences_RejectsEmptyProfileType(t *testing.T) {
	h := chargingPrefsHandler(t)

	w := putChargingPreferences(h, "SESS-1", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPutChargingPreferences_FailsLoudlyWhenStoreErrors(t *testing.T) {
	h := chargingPrefsHandler(t)
	store := h.Store.(*testStore)
	store.putSessionErr = errors.New("boom")

	w := putChargingPreferences(h, "SESS-1", `{"profile_type":"REGULAR"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when store fails, got %d (body=%s)", w.Code, w.Body.String())
	}
	// Must not lie by replying ACCEPTED.
	if strings.Contains(w.Body.String(), chargingPreferencesAccepted) {
		t.Errorf("expected no ACCEPTED reply on persistence failure, got body %s", w.Body.String())
	}
}
