package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/correctness"
)

func TestPutTokenRoutesCorrectnessTrafficIntoOverlayOnly(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	baseStore := h.Store.(*testStore)
	baseStore.tokenB = "base-token"

	overlay := h.Correctness.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	if err := overlay.SetTokenB("correctness-token"); err != nil {
		t.Fatalf("set overlay tokenB: %v", err)
	}
	if err := h.Correctness.SetPeerState(session.ID, correctness.SessionPeerState{
		CountryCode: "NL",
		PartyID:     "EMS",
	}); err != nil {
		t.Fatalf("set peer state: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/tokens/NL/EMS/TOK-1", strings.NewReader(`{"uid":"TOK-1","country_code":"NL","party_id":"EMS"}`))
	r.Header.Set("Authorization", "Token correctness-token")
	r.Header.Set("OCPI-From-Country-Code", "NL")
	r.Header.Set("OCPI-From-Party-Id", "EMS")
	r = withChiParams(r, map[string]string{
		"countryCode": "NL",
		"partyID":     "EMS",
		"uid":         "TOK-1",
	})

	h.PutToken(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("put token status: got %d, want 200", w.Code)
	}

	if len(baseStore.tokens) != 0 {
		t.Fatalf("expected base store to remain untouched, got %d token(s)", len(baseStore.tokens))
	}
	tokens, err := overlay.ListTokens()
	if err != nil {
		t.Fatalf("list overlay tokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected overlay store to contain 1 token, got %d", len(tokens))
	}
}
