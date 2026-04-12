package correctness

import (
	"context"
	"encoding/json"
	"net/http/httptest"
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
