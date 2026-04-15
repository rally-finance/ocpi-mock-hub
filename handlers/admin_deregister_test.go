package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type deregisterPeer struct {
	deleteAuth  string
	deleteCount int
}

func newDeregisterPeer(t *testing.T, versionsStatus, detailsStatus, deleteStatus int) (*httptest.Server, *deregisterPeer) {
	t.Helper()

	seen := &deregisterPeer{}

	var peer *httptest.Server
	peer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/versions":
			if versionsStatus != http.StatusOK {
				http.Error(w, "versions failed", versionsStatus)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": []map[string]string{
					{"version": "2.2.1", "url": peer.URL + "/ocpi/2.2.1"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ocpi/2.2.1":
			if detailsStatus != http.StatusOK {
				http.Error(w, "details failed", detailsStatus)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data": map[string]any{
					"version": "2.2.1",
					"endpoints": []map[string]string{
						{"identifier": "credentials", "url": peer.URL + "/ocpi/2.2.1/credentials"},
					},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/ocpi/2.2.1/credentials":
			seen.deleteCount++
			seen.deleteAuth = r.Header.Get("Authorization")
			if deleteStatus != http.StatusOK {
				http.Error(w, "delete failed", deleteStatus)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status_code": 1000,
				"data":        nil,
			})
		default:
			http.NotFound(w, r)
		}
	}))

	return peer, seen
}

func TestDeregisterConnection_HappyPath(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	peer, seen := newDeregisterPeer(t, http.StatusOK, http.StatusOK, http.StatusOK)
	defer peer.Close()

	store.tokenB = "hub-token-b"
	store.emspToken = "peer-token-c"
	store.versionsURL = peer.URL + "/ocpi/versions"
	store.callbackURL = peer.URL + "/ocpi/versions"
	store.creds = []byte(`{"token":"peer-token-c","url":"` + peer.URL + `/ocpi/versions"}`)
	if err := store.PutSession("SESSION-1", []byte(`{"id":"SESSION-1"}`)); err != nil {
		t.Fatalf("put session: %v", err)
	}
	if err := store.PutToken("DE", "AAA", "TOKEN-1", []byte(`{"uid":"TOKEN-1"}`)); err != nil {
		t.Fatalf("put token: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if seen.deleteCount != 1 {
		t.Fatalf("expected exactly 1 outbound DELETE, got %d", seen.deleteCount)
	}
	if seen.deleteAuth != "Token peer-token-c" {
		t.Fatalf("expected outbound auth header %q, got %q", "Token peer-token-c", seen.deleteAuth)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "deregistered" {
		t.Fatalf("expected status=deregistered, got %#v", body)
	}
	if body["credentials_url"] != peer.URL+"/ocpi/2.2.1/credentials" {
		t.Fatalf("expected credentials_url to match discovered endpoint, got %#v", body)
	}

	if store.tokenB != "" || store.emspToken != "" || store.versionsURL != "" || store.callbackURL != "" {
		t.Fatalf("expected local handshake fields to be cleared, got tokenB=%q emspToken=%q versionsURL=%q callbackURL=%q", store.tokenB, store.emspToken, store.versionsURL, store.callbackURL)
	}
	if len(store.creds) != 0 {
		t.Fatalf("expected stored credentials to be cleared, got %q", string(store.creds))
	}
	if got, _ := store.ListSessions(); len(got) != 1 {
		t.Fatalf("expected sessions to remain untouched, got %d", len(got))
	}
	if got, _ := store.ListTokens(); len(got) != 1 {
		t.Fatalf("expected tokens to remain untouched, got %d", len(got))
	}
}

func TestDeregisterConnection_UsesStoredCredentialsURLFallback(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	peer, seen := newDeregisterPeer(t, http.StatusOK, http.StatusOK, http.StatusOK)
	defer peer.Close()

	store.tokenB = "hub-token-b"
	store.emspToken = "peer-token-c"
	store.creds = []byte(`{"token":"peer-token-c","url":"` + peer.URL + `/ocpi/versions"}`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if seen.deleteCount != 1 {
		t.Fatalf("expected outbound DELETE using stored credentials URL fallback, got %d calls", seen.deleteCount)
	}
}

func TestDeregisterConnection_RequiresActiveConnection(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.emspToken = "peer-token-c"
	store.versionsURL = "https://peer.example.com/ocpi/versions"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No active OCPI connection") {
		t.Fatalf("expected missing connection error, got %s", w.Body.String())
	}
}

func TestDeregisterConnection_RequiresPeerToken(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.tokenB = "hub-token-b"
	store.versionsURL = "https://peer.example.com/ocpi/versions"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Missing stored peer credentials token") {
		t.Fatalf("expected missing peer token error, got %s", w.Body.String())
	}
}

func TestDeregisterConnection_RequiresPeerVersionsURL(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.tokenB = "hub-token-b"
	store.emspToken = "peer-token-c"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Missing peer versions URL") {
		t.Fatalf("expected missing peer versions URL error, got %s", w.Body.String())
	}
}

func TestDeregisterConnection_PreservesStateWhenVersionDiscoveryFails(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	peer, seen := newDeregisterPeer(t, http.StatusBadGateway, http.StatusOK, http.StatusOK)
	defer peer.Close()

	store.tokenB = "hub-token-b"
	store.emspToken = "peer-token-c"
	store.versionsURL = peer.URL + "/ocpi/versions"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d, want 502, body=%s", w.Code, w.Body.String())
	}
	if seen.deleteCount != 0 {
		t.Fatalf("expected no outbound DELETE when versions discovery fails, got %d", seen.deleteCount)
	}
	if store.tokenB != "hub-token-b" || store.emspToken != "peer-token-c" || store.versionsURL != peer.URL+"/ocpi/versions" {
		t.Fatalf("expected handshake state preserved after versions failure, got tokenB=%q emspToken=%q versionsURL=%q", store.tokenB, store.emspToken, store.versionsURL)
	}
}

func TestDeregisterConnection_PreservesStateWhenVersionDetailsFail(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	peer, seen := newDeregisterPeer(t, http.StatusOK, http.StatusBadGateway, http.StatusOK)
	defer peer.Close()

	store.tokenB = "hub-token-b"
	store.emspToken = "peer-token-c"
	store.versionsURL = peer.URL + "/ocpi/versions"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d, want 502, body=%s", w.Code, w.Body.String())
	}
	if seen.deleteCount != 0 {
		t.Fatalf("expected no outbound DELETE when version details fail, got %d", seen.deleteCount)
	}
	if store.tokenB != "hub-token-b" || store.emspToken != "peer-token-c" || store.versionsURL != peer.URL+"/ocpi/versions" {
		t.Fatalf("expected handshake state preserved after version details failure, got tokenB=%q emspToken=%q versionsURL=%q", store.tokenB, store.emspToken, store.versionsURL)
	}
}

func TestDeregisterConnection_PreservesStateWhenPeerDeleteFails(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	peer, seen := newDeregisterPeer(t, http.StatusOK, http.StatusOK, http.StatusBadGateway)
	defer peer.Close()

	store.tokenB = "hub-token-b"
	store.emspToken = "peer-token-c"
	store.versionsURL = peer.URL + "/ocpi/versions"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/admin/deregister", nil)
	h.DeregisterConnection(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d, want 502, body=%s", w.Code, w.Body.String())
	}
	if seen.deleteCount != 1 {
		t.Fatalf("expected outbound DELETE attempt, got %d", seen.deleteCount)
	}
	if store.tokenB != "hub-token-b" || store.emspToken != "peer-token-c" || store.versionsURL != peer.URL+"/ocpi/versions" {
		t.Fatalf("expected handshake state preserved after peer delete failure, got tokenB=%q emspToken=%q versionsURL=%q", store.tokenB, store.emspToken, store.versionsURL)
	}
}
