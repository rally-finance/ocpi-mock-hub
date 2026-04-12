package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
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
