package correctness

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestMatchesInboundRequestScopesByTokenAndParty(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	overlay := manager.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	if err := overlay.SetTokenB("correctness-token"); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}
	if err := manager.SetPeerState(session.ID, SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}

	req := httptest.NewRequest("GET", "http://hub.example.com/ocpi/2.2.1/sender/locations", nil)
	req.Header.Set("Authorization", "Token correctness-token")
	req.Header.Set("OCPI-From-Country-Code", "NL")
	req.Header.Set("OCPI-From-Party-Id", "EMS")
	if !manager.MatchesInboundRequest(req) {
		t.Fatal("expected matching correctness request to be accepted")
	}

	wrongToken := httptest.NewRequest("GET", "http://hub.example.com/ocpi/2.2.1/sender/locations", nil)
	wrongToken.Header.Set("Authorization", "Token other-token")
	wrongToken.Header.Set("OCPI-From-Country-Code", "NL")
	wrongToken.Header.Set("OCPI-From-Party-Id", "EMS")
	if manager.MatchesInboundRequest(wrongToken) {
		t.Fatal("expected request with wrong token to be rejected")
	}

	wrongParty := httptest.NewRequest("GET", "http://hub.example.com/ocpi/2.2.1/sender/locations", nil)
	wrongParty.Header.Set("Authorization", "Token correctness-token")
	wrongParty.Header.Set("OCPI-From-Country-Code", "DE")
	wrongParty.Header.Set("OCPI-From-Party-Id", "OTH")
	if manager.MatchesInboundRequest(wrongParty) {
		t.Fatal("expected request with wrong OCPI party headers to be rejected")
	}
}

func TestShouldCaptureOutboundRequestOnlyForCorrectnessActionOrCallback(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := manager.UpdateSandbox(session.ID, func(sandbox *Sandbox) error {
		payload, _ := json.Marshal(map[string]any{
			"id":            "SESS-1",
			"_response_url": "https://peer.example.com/callback",
		})
		return sandbox.Store.PutSession("SESS-1", payload)
	}); err != nil {
		t.Fatalf("update sandbox: %v", err)
	}

	actionReq := httptest.NewRequest("POST", "https://peer.example.com/ocpi/2.2.1/credentials", nil).
		WithContext(WithOutboundMeta(context.Background(), OutboundMeta{ActionID: "run_handshake"}))
	if !manager.ShouldCaptureOutboundRequest(actionReq) {
		t.Fatal("expected outbound correctness action to be captured")
	}

	callbackReq := httptest.NewRequest("POST", "https://peer.example.com/callback", nil)
	callbackReq.Header.Set("Authorization", "Token peer-token")
	if !manager.ShouldCaptureOutboundRequest(callbackReq) {
		t.Fatal("expected outbound callback URL to be captured")
	}

	noiseReq := httptest.NewRequest("PUT", "https://peer.example.com/ocpi/2.2.1/receiver/sessions/NL/EMS/SESS-1", nil)
	if manager.ShouldCaptureOutboundRequest(noiseReq) {
		t.Fatal("expected unrelated outbound traffic without action metadata to be ignored")
	}
}

func TestMatchesInboundRequestAcceptsLiteralBase64LookingToken(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	overlay := manager.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	const token = "dG9rZW4tYi0xMjM="
	if err := overlay.SetTokenB(token); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}
	if err := manager.SetPeerState(session.ID, SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}

	req := httptest.NewRequest("GET", "http://hub.example.com/ocpi/2.2.1/sender/locations", nil)
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("OCPI-From-Country-Code", "NL")
	req.Header.Set("OCPI-From-Party-Id", "EMS")
	if !manager.MatchesInboundRequest(req) {
		t.Fatal("expected literal base64-looking token to match the active correctness session")
	}
}

func TestShouldCaptureInboundRequestIncludesHandshakeDiscoveryCandidateUsingPeerToken(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	overlay := manager.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	if err := overlay.SetTokenB("correctness-token"); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}
	if err := manager.SetPeerState(session.ID, SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}

	req := httptest.NewRequest("GET", "http://hub.example.com/ocpi/versions", nil)
	req.Header.Set("Authorization", "Token peer-token")
	req.Header.Set("OCPI-From-Country-Code", "NL")
	req.Header.Set("OCPI-From-Party-Id", "EMS")

	if manager.MatchesInboundRequest(req) {
		t.Fatal("expected peer-token versions request to remain outside the strict overlay matcher")
	}
	if !manager.ShouldCaptureInboundRequest(req) {
		t.Fatal("expected handshake discovery candidate using the peer token to be captured for evaluation")
	}
}

func TestShouldCaptureInboundRequestLoadsManagerStateOnce(t *testing.T) {
	store := newCountingStateStore()
	manager := NewManager(testSeed(), store)
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	overlay := manager.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	if err := overlay.SetTokenB("correctness-token"); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}
	if err := manager.SetPeerState(session.ID, SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}

	store.reset()

	req := httptest.NewRequest("GET", "http://hub.example.com/ocpi/2.2.1/sender/locations", nil)
	req.Header.Set("Authorization", "Token some-other-token")
	req.Header.Set("OCPI-From-Country-Code", "NL")
	req.Header.Set("OCPI-From-Party-Id", "EMS")

	if manager.ShouldCaptureInboundRequest(req) {
		t.Fatal("expected unrelated inbound request not to be captured")
	}
	if got := store.getCount(managerStateBlobKey); got != 1 {
		t.Fatalf("expected one manager-state load, got %d", got)
	}
}

type countingStateStore struct {
	base StateStore

	mu   sync.Mutex
	gets map[string]int
}

func newCountingStateStore() *countingStateStore {
	return &countingStateStore{
		base: newMemoryStateStore(),
		gets: make(map[string]int),
	}
}

func (s *countingStateStore) GetBlob(key string) ([]byte, error) {
	s.mu.Lock()
	s.gets[key]++
	s.mu.Unlock()
	return s.base.GetBlob(key)
}

func (s *countingStateStore) UpdateBlob(key string, fn func([]byte) ([]byte, error)) error {
	return s.base.UpdateBlob(key, fn)
}

func (s *countingStateStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets = make(map[string]int)
}

func (s *countingStateStore) getCount(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gets[key]
}
