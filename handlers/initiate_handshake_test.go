package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type handshakeTestStore struct {
	tokenB          string
	emspCallback    string
	emspCreds       []byte
	emspOwnToken    string
	emspVersionsURL string
	mode            string
	tokens          map[string][]byte
	sessions        map[string][]byte
	cdrs            map[string][]byte
}

func newHandshakeTestStore() *handshakeTestStore {
	return &handshakeTestStore{
		tokens:   map[string][]byte{},
		sessions: map[string][]byte{},
		cdrs:     map[string][]byte{},
	}
}

func (s *handshakeTestStore) GetTokenB() (string, error)            { return s.tokenB, nil }
func (s *handshakeTestStore) SetTokenB(token string) error          { s.tokenB = token; return nil }
func (s *handshakeTestStore) GetEMSPCallbackURL() (string, error)   { return s.emspCallback, nil }
func (s *handshakeTestStore) SetEMSPCallbackURL(url string) error   { s.emspCallback = url; return nil }
func (s *handshakeTestStore) GetEMSPCredentials() ([]byte, error)   { return s.emspCreds, nil }
func (s *handshakeTestStore) SetEMSPCredentials(creds []byte) error { s.emspCreds = creds; return nil }
func (s *handshakeTestStore) GetEMSPOwnToken() (string, error)      { return s.emspOwnToken, nil }
func (s *handshakeTestStore) SetEMSPOwnToken(token string) error    { s.emspOwnToken = token; return nil }
func (s *handshakeTestStore) GetEMSPVersionsURL() (string, error)   { return s.emspVersionsURL, nil }
func (s *handshakeTestStore) SetEMSPVersionsURL(url string) error {
	s.emspVersionsURL = url
	return nil
}
func (s *handshakeTestStore) PutToken(cc, pid, uid string, token []byte) error {
	s.tokens[cc+"|"+pid+"|"+uid] = token
	return nil
}
func (s *handshakeTestStore) GetToken(cc, pid, uid string) ([]byte, error) {
	return s.tokens[cc+"|"+pid+"|"+uid], nil
}
func (s *handshakeTestStore) ListTokens() ([][]byte, error) {
	var out [][]byte
	for _, v := range s.tokens {
		out = append(out, v)
	}
	return out, nil
}
func (s *handshakeTestStore) PutSession(id string, session []byte) error {
	s.sessions[id] = session
	return nil
}
func (s *handshakeTestStore) GetSession(id string) ([]byte, error) { return s.sessions[id], nil }
func (s *handshakeTestStore) ListSessions() ([][]byte, error) {
	var out [][]byte
	for _, v := range s.sessions {
		out = append(out, v)
	}
	return out, nil
}
func (s *handshakeTestStore) DeleteSession(id string) error      { delete(s.sessions, id); return nil }
func (s *handshakeTestStore) PutCDR(id string, cdr []byte) error { s.cdrs[id] = cdr; return nil }
func (s *handshakeTestStore) GetCDR(id string) ([]byte, error)  { return s.cdrs[id], nil }
func (s *handshakeTestStore) ListCDRs() ([][]byte, error) {
	var out [][]byte
	for _, v := range s.cdrs {
		out = append(out, v)
	}
	return out, nil
}
func (s *handshakeTestStore) PutReservation(id string, data []byte) error { return nil }
func (s *handshakeTestStore) GetReservation(id string) ([]byte, error)    { return nil, nil }
func (s *handshakeTestStore) ListReservations() ([][]byte, error)         { return nil, nil }
func (s *handshakeTestStore) DeleteReservation(id string) error           { return nil }
func (s *handshakeTestStore) GetMode() (string, error)                    { return s.mode, nil }
func (s *handshakeTestStore) SetMode(mode string) error                   { s.mode = mode; return nil }

func TestInitiateHandshakePostsConfiguredVersionsURLWhenSet(t *testing.T) {
	const expectedVersionsURL = "https://ocpi-mock.ingress.getrally.com/ocpi/versions"

	var postedCredentialsURL string
	var emsp *httptest.Server
	emsp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/versions":
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": []map[string]string{
					{"version": "2.2.1", "url": emsp.URL + "/ocpi/2.2.1"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/2.2.1":
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": map[string]any{
					"version": "2.2.1",
					"endpoints": []map[string]string{
						{"identifier": "credentials", "url": emsp.URL + "/ocpi/2.2.1/credentials"},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/ocpi/2.2.1/credentials":
			var payload credentialsPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode credentials request payload: %v", err)
			}
			postedCredentialsURL = payload.URL
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": map[string]string{
					"token": "emsp-token-b",
					"url":   emsp.URL + "/ocpi/versions",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer emsp.Close()

	h := &Handler{
		Config: HandlerConfig{
			HubCountry:                   "DE",
			HubParty:                     "HUB",
			InitiateHandshakeVersionsURL: expectedVersionsURL,
		},
		Store: newHandshakeTestStore(),
	}

	req := httptest.NewRequest(http.MethodPost, "http://admin.local/admin/initiate-handshake", strings.NewReader(`{"emsp_versions_url":"`+emsp.URL+`/ocpi/versions","emsp_own_token":"token-a"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Rally-Forwarded-Host", "ocpi-mock-hub-rally-europe.vercel.app")
	rr := httptest.NewRecorder()

	h.InitiateHandshake(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if postedCredentialsURL != expectedVersionsURL {
		t.Fatalf("expected posted credentials url %q, got %q", expectedVersionsURL, postedCredentialsURL)
	}
}

func TestInitiateHandshakeFallsBackToForwardedHostURL(t *testing.T) {
	const expectedVersionsURL = "https://ocpi-mock-hub-rally-europe.vercel.app/ocpi/versions"

	var postedCredentialsURL string
	var emsp *httptest.Server
	emsp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/versions":
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": []map[string]string{
					{"version": "2.2.1", "url": emsp.URL + "/ocpi/2.2.1"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/2.2.1":
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": map[string]any{
					"version": "2.2.1",
					"endpoints": []map[string]string{
						{"identifier": "credentials", "url": emsp.URL + "/ocpi/2.2.1/credentials"},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/ocpi/2.2.1/credentials":
			var payload credentialsPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode credentials request payload: %v", err)
			}
			postedCredentialsURL = payload.URL
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": map[string]string{
					"token": "emsp-token-b",
					"url":   emsp.URL + "/ocpi/versions",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer emsp.Close()

	h := &Handler{
		Config: HandlerConfig{
			HubCountry: "DE",
			HubParty:   "HUB",
		},
		Store: newHandshakeTestStore(),
	}

	req := httptest.NewRequest(http.MethodPost, "http://admin.local/admin/initiate-handshake", strings.NewReader(`{"emsp_versions_url":"`+emsp.URL+`/ocpi/versions","emsp_own_token":"token-a"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Rally-Forwarded-Host", "ocpi-mock-hub-rally-europe.vercel.app")
	rr := httptest.NewRecorder()

	h.InitiateHandshake(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if postedCredentialsURL != expectedVersionsURL {
		t.Fatalf("expected posted credentials url %q, got %q", expectedVersionsURL, postedCredentialsURL)
	}
}
