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
	tokenB       string
	callbackURL  string
	creds        []byte
	emspToken    string
	versionsURL  string
	tokens       map[string][]byte
	sessions     map[string][]byte
	cdrs         map[string][]byte
	reservations map[string][]byte
	mode         string
}

func newTestStore() *testStore {
	return &testStore{
		tokens:       make(map[string][]byte),
		sessions:     make(map[string][]byte),
		cdrs:         make(map[string][]byte),
		reservations: make(map[string][]byte),
		mode:         "happy",
	}
}

func (s *testStore) GetTokenB() (string, error)                           { return s.tokenB, nil }
func (s *testStore) SetTokenB(t string) error                             { s.tokenB = t; return nil }
func (s *testStore) GetEMSPCallbackURL() (string, error)                  { return s.callbackURL, nil }
func (s *testStore) SetEMSPCallbackURL(u string) error                    { s.callbackURL = u; return nil }
func (s *testStore) GetEMSPCredentials() ([]byte, error)                  { return s.creds, nil }
func (s *testStore) SetEMSPCredentials(c []byte) error                    { s.creds = c; return nil }
func (s *testStore) GetEMSPOwnToken() (string, error)                     { return s.emspToken, nil }
func (s *testStore) SetEMSPOwnToken(t string) error                       { s.emspToken = t; return nil }
func (s *testStore) GetEMSPVersionsURL() (string, error)                  { return s.versionsURL, nil }
func (s *testStore) SetEMSPVersionsURL(u string) error                    { s.versionsURL = u; return nil }
func (s *testStore) PutToken(cc, pid, uid string, tok []byte) error       { s.tokens[cc+"/"+pid+"/"+uid] = tok; return nil }
func (s *testStore) GetToken(cc, pid, uid string) ([]byte, error)         { return s.tokens[cc+"/"+pid+"/"+uid], nil }
func (s *testStore) ListTokens() ([][]byte, error) {
	r := make([][]byte, 0, len(s.tokens))
	for _, v := range s.tokens {
		r = append(r, v)
	}
	return r, nil
}
func (s *testStore) PutSession(id string, data []byte) error { s.sessions[id] = data; return nil }
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
func (s *testStore) PutReservation(id string, data []byte) error { s.reservations[id] = data; return nil }
func (s *testStore) GetReservation(id string) ([]byte, error)    { return s.reservations[id], nil }
func (s *testStore) ListReservations() ([][]byte, error) {
	r := make([][]byte, 0, len(s.reservations))
	for _, v := range s.reservations {
		r = append(r, v)
	}
	return r, nil
}
func (s *testStore) DeleteReservation(id string) error { delete(s.reservations, id); return nil }
func (s *testStore) GetMode() (string, error)          { return s.mode, nil }
func (s *testStore) SetMode(m string) error            { s.mode = m; return nil }

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
