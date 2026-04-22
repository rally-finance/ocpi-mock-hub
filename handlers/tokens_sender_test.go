package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func testHandler() *Handler {
	seed := &fakegen.SeedData{
		Locations: []fakegen.Location{
			{
				CountryCode: "DE",
				PartyID:     "AAA",
				ID:          "LOC-1",
				Name:        "Test Location",
				EVSEs: []fakegen.EVSE{
					{UID: "EVSE-1", Connectors: []fakegen.Connector{{ID: "C1"}}},
				},
				LastUpdated: "2026-01-01T00:00:00Z",
			},
		},
		Tariffs:       []fakegen.Tariff{},
		HubClientInfo: []fakegen.HubClientInfo{},
	}

	return &Handler{
		Config: HandlerConfig{
			TokenA:     "test-token-a",
			HubCountry: "HU",
			HubParty:   "BME",
		},
		Store:  newTestStore(),
		Seed:   seed,
		ReqLog: NewRequestLog(),
	}
}

type testStore struct {
	tokenB           string
	callbackURL      string
	creds            []byte
	emspToken        string
	versionsURL      string
	tokens           map[string][]byte
	sessions         map[string][]byte
	cdrs             map[string][]byte
	reservations     map[string][]byte
	chargingProfiles map[string][]byte
	parties          map[string][]byte
	// tokenBIndex maps a TokenB to the set of party keys authenticated under
	// it. Mirrors the production MemoryStore semantics so tests exercise the
	// Stage 7 invariant: a single TokenB can back several party contexts,
	// and removing one party must not invalidate the binding for siblings.
	tokenBIndex   map[string]map[string]bool
	mode          string
	putSessionErr error
}

func newTestStore() *testStore {
	return &testStore{
		tokens:           make(map[string][]byte),
		sessions:         make(map[string][]byte),
		cdrs:             make(map[string][]byte),
		reservations:     make(map[string][]byte),
		chargingProfiles: make(map[string][]byte),
		parties:          make(map[string][]byte),
		tokenBIndex:      make(map[string]map[string]bool),
		mode:             "happy",
	}
}

func (s *testStore) GetTokenB() (string, error)          { return s.tokenB, nil }
func (s *testStore) SetTokenB(t string) error            { s.tokenB = t; return nil }
func (s *testStore) GetEMSPCallbackURL() (string, error) { return s.callbackURL, nil }
func (s *testStore) SetEMSPCallbackURL(u string) error   { s.callbackURL = u; return nil }
func (s *testStore) GetEMSPCredentials() ([]byte, error) { return s.creds, nil }
func (s *testStore) SetEMSPCredentials(c []byte) error   { s.creds = c; return nil }
func (s *testStore) GetEMSPOwnToken() (string, error)    { return s.emspToken, nil }
func (s *testStore) SetEMSPOwnToken(t string) error      { s.emspToken = t; return nil }
func (s *testStore) GetEMSPVersionsURL() (string, error) { return s.versionsURL, nil }
func (s *testStore) SetEMSPVersionsURL(u string) error   { s.versionsURL = u; return nil }
func (s *testStore) PutToken(cc, pid, uid string, tok []byte) error {
	s.tokens[cc+"/"+pid+"/"+uid] = tok
	return nil
}
func (s *testStore) GetToken(cc, pid, uid string) ([]byte, error) {
	return s.tokens[cc+"/"+pid+"/"+uid], nil
}
func (s *testStore) ListTokens() ([][]byte, error) {
	r := make([][]byte, 0, len(s.tokens))
	for _, v := range s.tokens {
		r = append(r, v)
	}
	return r, nil
}
func (s *testStore) PutSession(id string, data []byte) error {
	if s.putSessionErr != nil {
		return s.putSessionErr
	}
	s.sessions[id] = data
	return nil
}
func (s *testStore) GetSession(id string) ([]byte, error)    { return s.sessions[id], nil }
func (s *testStore) ListSessions() ([][]byte, error) {
	r := make([][]byte, 0, len(s.sessions))
	for _, v := range s.sessions {
		r = append(r, v)
	}
	return r, nil
}
func (s *testStore) DeleteSession(id string) error       { delete(s.sessions, id); return nil }
func (s *testStore) PutCDR(id string, data []byte) error { s.cdrs[id] = data; return nil }
func (s *testStore) GetCDR(id string) ([]byte, error)    { return s.cdrs[id], nil }
func (s *testStore) ListCDRs() ([][]byte, error) {
	r := make([][]byte, 0, len(s.cdrs))
	for _, v := range s.cdrs {
		r = append(r, v)
	}
	return r, nil
}
func (s *testStore) PutReservation(id string, data []byte) error {
	s.reservations[id] = data
	return nil
}
func (s *testStore) GetReservation(id string) ([]byte, error) { return s.reservations[id], nil }
func (s *testStore) ListReservations() ([][]byte, error) {
	r := make([][]byte, 0, len(s.reservations))
	for _, v := range s.reservations {
		r = append(r, v)
	}
	return r, nil
}
func (s *testStore) DeleteReservation(id string) error { delete(s.reservations, id); return nil }
func (s *testStore) PutChargingProfile(sessionID string, profile []byte) error {
	s.chargingProfiles[sessionID] = profile
	return nil
}
func (s *testStore) GetChargingProfile(sessionID string) ([]byte, error) {
	return s.chargingProfiles[sessionID], nil
}
func (s *testStore) DeleteChargingProfile(sessionID string) error {
	delete(s.chargingProfiles, sessionID)
	return nil
}
func (s *testStore) PutParty(key string, state []byte) error {
	// Unbind the key from its previous TokenB (if any) before reindexing,
	// leaving siblings on the same token untouched.
	if old, ok := s.parties[key]; ok {
		var prev struct {
			TokenB string `json:"token_b"`
		}
		if json.Unmarshal(old, &prev) == nil && prev.TokenB != "" {
			s.unbindTokenB(prev.TokenB, key)
		}
	}
	s.parties[key] = state
	var p struct {
		TokenB string `json:"token_b"`
	}
	if json.Unmarshal(state, &p) == nil && p.TokenB != "" {
		set, ok := s.tokenBIndex[p.TokenB]
		if !ok {
			set = make(map[string]bool)
			s.tokenBIndex[p.TokenB] = set
		}
		set[key] = true
	}
	return nil
}
func (s *testStore) GetParty(key string) ([]byte, error) { return s.parties[key], nil }
func (s *testStore) GetPartyByTokenB(tokenB string) ([]byte, error) {
	set, ok := s.tokenBIndex[tokenB]
	if !ok || len(set) == 0 {
		return nil, nil
	}
	for key := range set {
		if party, ok := s.parties[key]; ok {
			return party, nil
		}
	}
	return nil, nil
}
func (s *testStore) DeleteParty(key string) error {
	if raw, ok := s.parties[key]; ok {
		var p struct {
			TokenB string `json:"token_b"`
		}
		if json.Unmarshal(raw, &p) == nil && p.TokenB != "" {
			s.unbindTokenB(p.TokenB, key)
		}
	}
	delete(s.parties, key)
	return nil
}
func (s *testStore) unbindTokenB(tokenB, partyKey string) {
	set, ok := s.tokenBIndex[tokenB]
	if !ok {
		return
	}
	delete(set, partyKey)
	if len(set) == 0 {
		delete(s.tokenBIndex, tokenB)
	}
}
func (s *testStore) ListParties() ([][]byte, error) {
	r := make([][]byte, 0, len(s.parties))
	for _, v := range s.parties {
		r = append(r, v)
	}
	return r, nil
}
func (s *testStore) GetMode() (string, error) { return s.mode, nil }
func (s *testStore) SetMode(m string) error   { s.mode = m; return nil }

func withChiParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

type ocpiResp struct {
	Data       json.RawMessage `json:"data"`
	StatusCode int             `json:"status_code"`
}

func TestGetTokens_Empty(t *testing.T) {
	h := testHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/tokens", nil)

	h.GetTokens(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if w.Header().Get("X-Total-Count") != "0" {
		t.Errorf("X-Total-Count: got %s, want 0", w.Header().Get("X-Total-Count"))
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data []json.RawMessage
	json.Unmarshal(resp.Data, &data)
	if len(data) != 0 {
		t.Errorf("expected empty list, got %d", len(data))
	}
}

func TestGetTokens_Paginated(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	for i := 0; i < 5; i++ {
		uid := "TOK" + string(rune('A'+i))
		tok := map[string]any{"uid": uid, "country_code": "DE", "party_id": "AAA", "type": "RFID", "last_updated": "2026-01-01T00:00:00Z"}
		data, _ := json.Marshal(tok)
		store.PutToken("DE", "AAA", uid, data)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/tokens?offset=0&limit=2", nil)
	h.GetTokens(w, r)

	if w.Header().Get("X-Total-Count") != "5" {
		t.Errorf("X-Total-Count: got %s, want 5", w.Header().Get("X-Total-Count"))
	}
	if w.Header().Get("X-Limit") != "2" {
		t.Errorf("X-Limit: got %s, want 2", w.Header().Get("X-Limit"))
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data []json.RawMessage
	json.Unmarshal(resp.Data, &data)
	if len(data) != 2 {
		t.Errorf("expected 2 in page, got %d", len(data))
	}
}

func TestGetTokenByID_Found(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	tok := map[string]string{"uid": "TOK1", "country_code": "DE", "party_id": "AAA", "type": "RFID"}
	data, _ := json.Marshal(tok)
	store.PutToken("DE", "AAA", "TOK1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/tokens/DE/AAA/TOK1", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.GetTokenByID(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
}

func TestGetTokenByID_NotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/tokens/XX/YY/ZZ", nil)
	r = withChiParams(r, map[string]string{"countryCode": "XX", "partyID": "YY", "uid": "ZZ"})

	h.GetTokenByID(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
}

func TestPostTokenAuthorize_Allowed(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	tok := map[string]string{"uid": "TOK1", "country_code": "DE", "party_id": "AAA", "type": "RFID"}
	data, _ := json.Marshal(tok)
	store.PutToken("DE", "AAA", "TOK1", data)

	body := `{"location_id":"LOC-1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/authorize", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PostTokenAuthorize(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]any
	json.Unmarshal(resp.Data, &result)
	if result["allowed"] != "ALLOWED" {
		t.Errorf("expected ALLOWED, got %v", result["allowed"])
	}
	if result["location"] == nil {
		t.Error("expected location in response when location_id provided")
	}
}

func TestPostTokenAuthorize_NotAllowed(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/authorize", nil)
	r = withChiParams(r, map[string]string{"countryCode": "XX", "partyID": "YY", "uid": "ZZ"})

	h.PostTokenAuthorize(w, r)

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]any
	json.Unmarshal(resp.Data, &result)
	if result["allowed"] != "NOT_ALLOWED" {
		t.Errorf("expected NOT_ALLOWED, got %v", result["allowed"])
	}
}

func TestPutToken_InjectsIdentifiersAndType(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	body := `{"last_updated":"2026-01-01T00:00:00Z","whitelist":"ALWAYS"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PutToken(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	raw, _ := store.GetToken("DE", "AAA", "TOK1")
	var stored map[string]any
	json.Unmarshal(raw, &stored)
	if stored["country_code"] != "DE" || stored["party_id"] != "AAA" || stored["uid"] != "TOK1" || stored["type"] != "RFID" {
		t.Errorf("expected identifiers and RFID type injected, got %v", stored)
	}
}

// TestPutToken_PreservesOCPI221OptionalFields ensures the receiver PUT flow
// round-trips the OCPI 2.2.1 optional Token fields (visual_number, issuer,
// group_id, language, default_profile_type, energy_contract) untouched.
// These fields are not required by the mock's own logic, but clients must be
// able to push them through without loss.
func TestPutToken_PreservesOCPI221OptionalFields(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	body := `{
		"last_updated":"2026-01-01T00:00:00Z",
		"whitelist":"ALLOWED",
		"valid":true,
		"contract_id":"NL-TNM-C012345678-X",
		"issuer":"TheNewMotion",
		"visual_number":"DF000-2001-8999-1",
		"group_id":"DF000-2001-8999",
		"language":"en",
		"default_profile_type":"GREEN",
		"energy_contract":{"supplier_name":"GreenEnergy","contract_id":"GE-2026-0001"}
	}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK-OPT", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK-OPT"})

	h.PutToken(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	raw, _ := store.GetToken("DE", "AAA", "TOK-OPT")
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("unmarshal stored token: %v", err)
	}

	for _, key := range []string{
		"visual_number", "issuer", "group_id", "language",
		"default_profile_type", "energy_contract",
	} {
		if _, ok := stored[key]; !ok {
			t.Errorf("expected optional field %q to round-trip, got %v", key, stored)
		}
	}
	if stored["visual_number"] != "DF000-2001-8999-1" {
		t.Errorf("visual_number mismatch: %v", stored["visual_number"])
	}
	ec, ok := stored["energy_contract"].(map[string]any)
	if !ok {
		t.Fatalf("energy_contract not an object: %v", stored["energy_contract"])
	}
	if ec["supplier_name"] != "GreenEnergy" || ec["contract_id"] != "GE-2026-0001" {
		t.Errorf("energy_contract contents altered: %v", ec)
	}
}

func TestPutToken_SeparateStorageForAppUserType(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	rfidBody := `{"last_updated":"2026-01-01T00:00:00Z","type":"RFID"}`
	appBody := `{"last_updated":"2026-01-01T00:00:00Z","type":"APP_USER"}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK1", strings.NewReader(rfidBody))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})
	h.PutToken(w, r)

	w = httptest.NewRecorder()
	r = httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK1?type=APP_USER", strings.NewReader(appBody))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})
	h.PutToken(w, r)

	rfid, _ := store.GetToken("DE", "AAA", "TOK1")
	if rfid == nil {
		t.Fatal("expected RFID token to remain")
	}
	app, _ := store.GetToken("DE", "AAA", "TOK1|APP_USER")
	if app == nil {
		t.Fatal("expected APP_USER token stored under composite key")
	}
}

func TestPutToken_URLWinsOverBodyType(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	body := `{"last_updated":"2026-01-01T00:00:00Z","type":"APP_USER","country_code":"XX","party_id":"YY","uid":"OTHER"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PutToken(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	// URL routed as RFID, so the token must land under the bare UID key with
	// type=RFID regardless of what the body claimed.
	raw, _ := store.GetToken("DE", "AAA", "TOK1")
	if raw == nil {
		t.Fatal("expected token under RFID key from URL")
	}
	var stored map[string]any
	json.Unmarshal(raw, &stored)
	if stored["type"] != "RFID" {
		t.Errorf("expected URL type RFID to win over body APP_USER, got %v", stored["type"])
	}
	if stored["country_code"] != "DE" || stored["party_id"] != "AAA" || stored["uid"] != "TOK1" {
		t.Errorf("expected URL identifiers to win, got %v", stored)
	}
	// No stray APP_USER record should be written.
	if app, _ := store.GetToken("DE", "AAA", "TOK1|APP_USER"); app != nil {
		t.Error("expected no APP_USER record when URL type is RFID")
	}
}

func TestPatchToken_URLWinsOverBodyType(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	existing := map[string]any{
		"uid":          "TOK1",
		"country_code": "DE",
		"party_id":     "AAA",
		"type":         "RFID",
		"whitelist":    "ALLOWED",
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(existing)
	store.PutToken("DE", "AAA", "TOK1", data)

	body := `{"type":"APP_USER","country_code":"XX","party_id":"YY","uid":"OTHER","last_updated":"2026-01-01T00:05:00Z"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PatchToken(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	raw, _ := store.GetToken("DE", "AAA", "TOK1")
	var stored map[string]any
	json.Unmarshal(raw, &stored)
	if stored["type"] != "RFID" {
		t.Errorf("expected URL type RFID to win over body APP_USER, got %v", stored["type"])
	}
	if stored["uid"] != "TOK1" || stored["country_code"] != "DE" || stored["party_id"] != "AAA" {
		t.Errorf("expected URL identifiers to win, got %v", stored)
	}
}

func TestPatchToken_MergesIntoExisting(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	existing := map[string]any{
		"uid":          "TOK1",
		"country_code": "DE",
		"party_id":     "AAA",
		"type":         "RFID",
		"whitelist":    "ALLOWED",
		"valid":        true,
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(existing)
	store.PutToken("DE", "AAA", "TOK1", data)

	body := `{"valid":false,"last_updated":"2026-01-01T00:05:00Z"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PatchToken(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	raw, _ := store.GetToken("DE", "AAA", "TOK1")
	var merged map[string]any
	json.Unmarshal(raw, &merged)
	if merged["valid"] != false {
		t.Errorf("expected valid=false, got %v", merged["valid"])
	}
	if merged["whitelist"] != "ALLOWED" {
		t.Errorf("expected whitelist preserved, got %v", merged["whitelist"])
	}
}

func TestPatchToken_UnknownReturns404(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/ocpi/2.2.1/receiver/tokens/DE/AAA/MISSING", strings.NewReader(`{"last_updated":"2026-01-01T00:00:00Z"}`))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "MISSING"})

	h.PatchToken(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
}

func TestGetReceiverToken_Roundtrip(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	data, _ := json.Marshal(map[string]any{
		"uid": "TOK1", "country_code": "DE", "party_id": "AAA", "type": "RFID",
	})
	store.PutToken("DE", "AAA", "TOK1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/receiver/tokens/DE/AAA/TOK1", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.GetReceiverToken(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
}

func TestPostTokenAuthorize_TypeQueryRoutesToAppUserToken(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	data, _ := json.Marshal(map[string]any{"uid": "TOK1", "type": "APP_USER"})
	store.PutToken("DE", "AAA", "TOK1|APP_USER", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/authorize?type=APP_USER", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PostTokenAuthorize(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]any
	json.Unmarshal(resp.Data, &result)
	if result["allowed"] != "ALLOWED" {
		t.Errorf("expected ALLOWED, got %v", result["allowed"])
	}
}

func TestPostTokenAuthorize_WhitelistNeverRequiresLocationReferences(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	tok := map[string]any{"uid": "TOK1", "whitelist": "NEVER"}
	data, _ := json.Marshal(tok)
	store.PutToken("DE", "AAA", "TOK1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/authorize", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PostTokenAuthorize(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != 2002 {
		t.Fatalf("expected OCPI status 2002, got %d", resp.StatusCode)
	}
}

func TestPostTokenAuthorize_AcceptsLocationReferencesObject(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	tok := map[string]any{"uid": "TOK1", "whitelist": "NEVER"}
	data, _ := json.Marshal(tok)
	store.PutToken("DE", "AAA", "TOK1", data)

	body := `{"location_references":{"location_id":"LOC-1"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/authorize", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PostTokenAuthorize(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]any
	json.Unmarshal(resp.Data, &result)
	if result["allowed"] != "ALLOWED" {
		t.Errorf("expected ALLOWED, got %v", result["allowed"])
	}
	if result["location"] == nil {
		t.Error("expected location in response when location_references.location_id matches seed")
	}
}

func TestPostTokenAuthorize_RejectMode(t *testing.T) {
	h := testHandler()
	h.Store.(*testStore).mode = "reject"

	tok := map[string]string{"uid": "TOK1"}
	data, _ := json.Marshal(tok)
	h.Store.(*testStore).PutToken("DE", "AAA", "TOK1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/authorize", nil)
	r = withChiParams(r, map[string]string{"countryCode": "DE", "partyID": "AAA", "uid": "TOK1"})

	h.PostTokenAuthorize(w, r)

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]any
	json.Unmarshal(resp.Data, &result)
	if result["allowed"] != "NOT_ALLOWED" {
		t.Errorf("expected NOT_ALLOWED in reject mode, got %v", result["allowed"])
	}
}
