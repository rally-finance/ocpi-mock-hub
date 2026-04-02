package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (h *Handler) GetVersions(w http.ResponseWriter, r *http.Request) {
	if !h.verifyTokenA(w, r) {
		return
	}

	scheme := resolveScheme(r)
	host := resolveHost(r)
	base := fmt.Sprintf("%s://%s", scheme, host)

	versions := []map[string]string{
		{"version": "2.2.1", "url": base + "/ocpi/2.2.1"},
	}
	ocpiutil.OK(w, r, versions)
}

func (h *Handler) GetVersionDetails(w http.ResponseWriter, r *http.Request) {
	if !h.verifyTokenA(w, r) {
		return
	}

	scheme := resolveScheme(r)
	host := resolveHost(r)
	base := fmt.Sprintf("%s://%s/ocpi/2.2.1", scheme, host)

	details := map[string]any{
		"version": "2.2.1",
		"endpoints": []map[string]string{
			{"identifier": "credentials", "role": "RECEIVER", "url": base + "/credentials"},
			{"identifier": "locations", "role": "SENDER", "url": base + "/sender/locations"},
			{"identifier": "tariffs", "role": "SENDER", "url": base + "/sender/tariffs"},
			{"identifier": "sessions", "role": "SENDER", "url": base + "/sender/sessions"},
			{"identifier": "cdrs", "role": "SENDER", "url": base + "/sender/cdrs"},
			{"identifier": "tokens", "role": "RECEIVER", "url": base + "/receiver/tokens"},
			{"identifier": "commands", "role": "RECEIVER", "url": base + "/receiver/commands"},
			{"identifier": "hubclientinfo", "role": "SENDER", "url": base + "/sender/hubclientinfo"},
		},
	}
	ocpiutil.OK(w, r, details)
}

// verifyTokenA checks the Authorization header against Token A.
// Returns true if valid, writes an error response and returns false otherwise.
// Also accepts Token B if the handshake is already done (for GET requests).
func (h *Handler) verifyTokenA(w http.ResponseWriter, r *http.Request) bool {
	provided := parseAuthToken(r.Header.Get("Authorization"))
	if provided == "" {
		ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Missing authorization token")
		return false
	}

	if provided == h.Config.TokenA {
		return true
	}

	// Also accept Token B for already-handshaked clients querying versions.
	tokenB, _ := h.Store.GetTokenB()
	if tokenB != "" && provided == tokenB {
		return true
	}

	ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Invalid authorization token")
	return false
}

func parseAuthToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "token") {
		return ""
	}
	raw := strings.TrimSpace(parts[1])
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil && len(decoded) > 0 {
		return string(decoded)
	}
	return raw
}

func resolveScheme(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		return fwd
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func resolveHost(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return h
	}
	return r.Host
}
