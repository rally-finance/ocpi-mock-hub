package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPutCredentials_Happy(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	store.tokenB = "old-token-b"
	store.emspToken = "old-emsp-token"

	body := `{"token":"new-emsp-token","url":"https://emsp.example.com/ocpi/versions"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/credentials", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h.PutCredentials(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var creds credentialsPayload
	json.Unmarshal(resp.Data, &creds)

	if creds.Token == "" {
		t.Error("expected non-empty Token B in response")
	}
	if creds.Token == "old-token-b" {
		t.Error("expected new Token B to differ from old")
	}

	newTokenB, _ := store.GetTokenB()
	if newTokenB != creds.Token {
		t.Errorf("stored Token B %q != response Token B %q", newTokenB, creds.Token)
	}

	emspToken, _ := store.GetEMSPOwnToken()
	if emspToken != "new-emsp-token" {
		t.Errorf("expected eMSP token to be updated, got %s", emspToken)
	}

	callbackURL, _ := store.GetEMSPCallbackURL()
	if callbackURL != "https://emsp.example.com/ocpi/versions" {
		t.Errorf("expected callback URL updated, got %s", callbackURL)
	}
}

func TestPutCredentials_MissingToken(t *testing.T) {
	h := testHandler()

	body := `{"url":"https://emsp.example.com/ocpi/versions"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/credentials", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h.PutCredentials(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestDeleteCredentials_Happy(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	store.tokenB = "some-token-b"
	store.callbackURL = "https://emsp.example.com"
	store.emspToken = "emsp-own-token"
	store.versionsURL = "https://emsp.example.com/versions"
	store.creds = []byte(`{"token":"emsp-own-token"}`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/ocpi/2.2.1/credentials", nil)

	h.DeleteCredentials(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	tokenB, _ := store.GetTokenB()
	if tokenB != "" {
		t.Errorf("expected Token B cleared, got %q", tokenB)
	}
	callbackURL, _ := store.GetEMSPCallbackURL()
	if callbackURL != "" {
		t.Errorf("expected callback URL cleared, got %q", callbackURL)
	}
	emspToken, _ := store.GetEMSPOwnToken()
	if emspToken != "" {
		t.Errorf("expected eMSP token cleared, got %q", emspToken)
	}
	versionsURL, _ := store.GetEMSPVersionsURL()
	if versionsURL != "" {
		t.Errorf("expected versions URL cleared, got %q", versionsURL)
	}
}
