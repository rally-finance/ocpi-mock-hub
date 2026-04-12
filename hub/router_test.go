package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func testApp() *App {
	store := NewMemoryStore()
	seed := fakegen.GenerateSeed(5)
	return &App{
		Config: Config{
			TokenA:     "test-token-a",
			HubCountry: "DE",
			HubParty:   "HUB",
		},
		Store: store,
		Seed:  seed,
	}
}

func TestPutCredentials_NoHandshake_Returns401(t *testing.T) {
	app := testApp()
	router := NewRouter(app)

	body := `{"token":"emsp-token","url":"https://emsp.example.com/ocpi/versions"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/credentials", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Token some-random-token")

	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("PUT /credentials without handshake: got %d, want 401", w.Code)
	}
}

func TestPutCredentials_WithTokenB_Succeeds(t *testing.T) {
	app := testApp()
	app.Store.SetTokenB("valid-token-b")
	router := NewRouter(app)

	body := `{"token":"new-emsp-token","url":"https://emsp.example.com/ocpi/versions"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/credentials", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Token valid-token-b")

	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("PUT /credentials with valid Token B: got %d, want 200", w.Code)
	}

	newTokenB, _ := app.Store.GetTokenB()
	if newTokenB == "valid-token-b" {
		t.Error("expected Token B to be rotated after PUT")
	}
	if newTokenB == "" {
		t.Error("expected new Token B to be non-empty")
	}
}

func TestDeleteCredentials_NoHandshake_Returns401(t *testing.T) {
	app := testApp()
	router := NewRouter(app)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/ocpi/2.2.1/credentials", nil)
	r.Header.Set("Authorization", "Token some-random-token")

	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("DELETE /credentials without handshake: got %d, want 401", w.Code)
	}
}

func TestDeleteCredentials_WithTokenB_ClearsState(t *testing.T) {
	app := testApp()
	app.Store.SetTokenB("valid-token-b")
	app.Store.SetEMSPCallbackURL("https://emsp.example.com")
	app.Store.SetEMSPOwnToken("emsp-token")
	router := NewRouter(app)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/ocpi/2.2.1/credentials", nil)
	r.Header.Set("Authorization", "Token valid-token-b")

	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("DELETE /credentials with valid Token B: got %d, want 200", w.Code)
	}

	tokenB, _ := app.Store.GetTokenB()
	if tokenB != "" {
		t.Errorf("expected Token B cleared, got %q", tokenB)
	}
	callbackURL, _ := app.Store.GetEMSPCallbackURL()
	if callbackURL != "" {
		t.Errorf("expected callback URL cleared, got %q", callbackURL)
	}
}

func TestPostCredentials_StillExemptFromTokenB(t *testing.T) {
	app := testApp()
	router := NewRouter(app)

	body := `{"token":"emsp-token","url":"https://emsp.example.com/ocpi/versions"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/ocpi/2.2.1/credentials", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Token test-token-a")

	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("POST /credentials with Token A (no Token B): got %d, want 200", w.Code)
	}
}

func TestOCPIFromHeaders_SetOnOCPIResponse(t *testing.T) {
	app := testApp()
	app.Store.SetTokenB("valid-token-b")
	router := NewRouter(app)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations", nil)
	r.Header.Set("Authorization", "Token valid-token-b")

	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /sender/locations: got %d, want 200", w.Code)
	}

	fromCC := w.Header().Get("OCPI-From-Country-Code")
	fromPID := w.Header().Get("OCPI-From-Party-Id")

	if fromCC != "DE" {
		t.Errorf("OCPI-From-Country-Code: got %q, want %q", fromCC, "DE")
	}
	if fromPID != "HUB" {
		t.Errorf("OCPI-From-Party-Id: got %q, want %q", fromPID, "HUB")
	}
}

func TestOCPIFromHeaders_PresentOnAuthError(t *testing.T) {
	app := testApp()
	router := NewRouter(app)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations", nil)
	r.Header.Set("Authorization", "Token bad-token")

	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if cc := w.Header().Get("OCPI-From-Country-Code"); cc != "DE" {
		t.Errorf("OCPI-From-Country-Code on 401: got %q, want %q", cc, "DE")
	}
	if pid := w.Header().Get("OCPI-From-Party-Id"); pid != "HUB" {
		t.Errorf("OCPI-From-Party-Id on 401: got %q, want %q", pid, "HUB")
	}
}

func TestOCPIFromHeaders_NotSetOnAdminResponse(t *testing.T) {
	app := testApp()
	router := NewRouter(app)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/status", nil)

	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /admin/status: got %d, want 200", w.Code)
	}

	if fromCC := w.Header().Get("OCPI-From-Country-Code"); fromCC != "" {
		t.Errorf("expected no OCPI-From-Country-Code on admin endpoint, got %q", fromCC)
	}
}

func TestCorrectnessSessionTrafficUsesOverlayWhileRegularTrafficUsesBaseStore(t *testing.T) {
	store := NewMemoryStore()
	store.SetTokenB("base-token")
	seed := fakegen.GenerateSeed(2)
	baseName := seed.Locations[0].Name

	manager := correctness.NewManager(seed)
	session, err := manager.StartSession(correctness.SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start correctness session: %v", err)
	}
	if err := manager.ActiveOverlay().SetTokenB("correctness-token"); err != nil {
		t.Fatalf("set correctness tokenB: %v", err)
	}
	if err := manager.SetPeerState(session.ID, correctness.SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}
	if err := manager.UpdateSandbox(session.ID, func(sandbox *correctness.Sandbox) error {
		sandbox.Seed.Locations[0].Name = "Correctness Overlay Location"
		return nil
	}); err != nil {
		t.Fatalf("update correctness sandbox: %v", err)
	}

	app := &App{
		Config: Config{
			TokenA:     "test-token-a",
			HubCountry: "DE",
			HubParty:   "HUB",
		},
		Store:       store,
		BaseStore:   store,
		Seed:        seed,
		Correctness: manager,
	}
	router := NewRouter(app)

	baseReq := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations", nil)
	baseReq.Header.Set("Authorization", "Token base-token")
	baseResp := httptest.NewRecorder()
	router.ServeHTTP(baseResp, baseReq)
	if baseResp.Code != http.StatusOK {
		t.Fatalf("base request status: got %d, want 200", baseResp.Code)
	}
	if !strings.Contains(baseResp.Body.String(), baseName) {
		t.Fatalf("expected base response to include base seed location name %q", baseName)
	}
	if strings.Contains(baseResp.Body.String(), "Correctness Overlay Location") {
		t.Fatal("did not expect base request to use correctness overlay seed")
	}

	correctnessReq := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations", nil)
	correctnessReq.Header.Set("Authorization", "Token correctness-token")
	correctnessReq.Header.Set("OCPI-From-Country-Code", "NL")
	correctnessReq.Header.Set("OCPI-From-Party-Id", "EMS")
	correctnessResp := httptest.NewRecorder()
	router.ServeHTTP(correctnessResp, correctnessReq)
	if correctnessResp.Code != http.StatusOK {
		t.Fatalf("correctness request status: got %d, want 200", correctnessResp.Code)
	}
	if !strings.Contains(correctnessResp.Body.String(), "Correctness Overlay Location") {
		t.Fatal("expected correctness-scoped request to use the overlay seed")
	}
}

type ocpiResponse struct {
	Data       json.RawMessage `json:"data"`
	StatusCode int             `json:"status_code"`
}
