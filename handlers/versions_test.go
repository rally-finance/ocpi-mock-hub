package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func authHeader(token string) string {
	return "Token " + base64.StdEncoding.EncodeToString([]byte(token))
}

func TestResolveHostPrefersXRallyForwardedHost(t *testing.T) {
	req := httptest.NewRequest("GET", "http://inner.example/ocpi/versions", nil)
	req.Host = "inner.example"
	req.Header.Set("X-Forwarded-Host", "x-forwarded.example")
	req.Header.Set("X-Rally-Forwarded-Host", "x-rally.example")

	got := resolveHost(req)
	if got != "x-rally.example" {
		t.Fatalf("expected X-Rally-Forwarded-Host to win, got %q", got)
	}
}

func TestGetVersionsUsesXRallyForwardedHost(t *testing.T) {
	h := &Handler{
		Config: HandlerConfig{
			TokenA: "token-a",
		},
	}

	req := httptest.NewRequest("GET", "http://inner.example/ocpi/versions", nil)
	req.Host = "inner.example"
	req.Header.Set("Authorization", authHeader("token-a"))
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "x-forwarded.example")
	req.Header.Set("X-Rally-Forwarded-Host", "x-rally.example")

	rr := httptest.NewRecorder()
	h.GetVersions(rr, req)

	var body struct {
		Data []struct {
			Version string `json:"version"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected one version, got %d", len(body.Data))
	}
	if want := "https://x-rally.example/ocpi/2.2.1"; body.Data[0].URL != want {
		t.Fatalf("unexpected version URL, want %q got %q", want, body.Data[0].URL)
	}
}

func TestGetVersionsAcceptsLiteralBase64LookingTokenA(t *testing.T) {
	h := &Handler{
		Config: HandlerConfig{
			TokenA: "dG9rZW4tYS0xMjM=",
		},
	}

	req := httptest.NewRequest("GET", "http://inner.example/ocpi/versions", nil)
	req.Host = "inner.example"
	req.Header.Set("Authorization", "Token dG9rZW4tYS0xMjM=")

	rr := httptest.NewRecorder()
	h.GetVersions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
}

func TestGetVersionsAcceptsCorrectnessSessionTokenB(t *testing.T) {
	manager := correctness.NewManager(&fakegen.SeedData{})
	session, err := manager.StartSession(correctness.SessionConfig{
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
	if err := overlay.SetTokenB("correctness-token-b"); err != nil {
		t.Fatalf("set tokenB: %v", err)
	}
	if err := manager.SetPeerState(session.ID, correctness.SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}

	h := &Handler{
		Config: HandlerConfig{
			TokenA: "global-token-a",
		},
		Store:       newTestStore(),
		Correctness: manager,
	}

	req := httptest.NewRequest("GET", "http://inner.example/ocpi/versions", nil)
	req.Host = "inner.example"
	req.Header.Set("Authorization", "Token correctness-token-b")
	req.Header.Set("OCPI-From-Country-Code", "NL")
	req.Header.Set("OCPI-From-Party-Id", "EMS")

	rr := httptest.NewRecorder()
	h.GetVersions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected correctness session Token B to be accepted, got %d", rr.Code)
	}
}

func TestGetVersionsAcceptsPersistedCorrectnessPartyTokenBWithoutCorrectnessOverlay(t *testing.T) {
	store := newTestStore()
	payload, err := json.Marshal(map[string]string{
		"key":     "correctness/CTS-1234",
		"token_b": "shared-token-b",
		"role":    "EMSP",
	})
	if err != nil {
		t.Fatalf("marshal shared party payload: %v", err)
	}
	if err := store.PutParty("correctness/CTS-1234", payload); err != nil {
		t.Fatalf("put party: %v", err)
	}

	h := &Handler{
		Config: HandlerConfig{
			TokenA: "global-token-a",
		},
		Store: store,
	}

	req := httptest.NewRequest("GET", "http://inner.example/ocpi/versions", nil)
	req.Host = "inner.example"
	req.Header.Set("Authorization", "Token shared-token-b")

	rr := httptest.NewRecorder()
	h.GetVersions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected shared party Token B to be accepted without a correctness overlay, got %d", rr.Code)
	}
}

func TestGetVersionsRejectsNonCorrectnessPartyTokenBWithoutOverlay(t *testing.T) {
	store := newTestStore()
	payload, err := json.Marshal(map[string]string{
		"key":     "NL/EMS",
		"token_b": "shared-token-b",
		"role":    "EMSP",
	})
	if err != nil {
		t.Fatalf("marshal shared party payload: %v", err)
	}
	if err := store.PutParty("NL/EMS", payload); err != nil {
		t.Fatalf("put party: %v", err)
	}

	h := &Handler{
		Config: HandlerConfig{
			TokenA: "global-token-a",
		},
		Store: store,
	}

	req := httptest.NewRequest("GET", "http://inner.example/ocpi/versions", nil)
	req.Host = "inner.example"
	req.Header.Set("Authorization", "Token shared-token-b")

	rr := httptest.NewRecorder()
	h.GetVersions(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected non-correctness party Token B to be rejected without a correctness overlay, got %d", rr.Code)
	}
}
