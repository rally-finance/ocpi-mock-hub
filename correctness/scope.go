package correctness

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func (m *Manager) MatchesInboundRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	rt := m.sessions[m.activeID]
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil {
		return false
	}

	provided := parseAuthToken(r.Header.Get("Authorization"))
	if provided == "" {
		return false
	}

	return inboundRequestMatchesSession(rt, provided, r.Header.Get("OCPI-From-Country-Code"), r.Header.Get("OCPI-From-Party-Id"))
}

func (m *Manager) ShouldCaptureInboundRequest(r *http.Request) bool {
	return m.MatchesInboundRequest(r)
}

func (m *Manager) ShouldCaptureOutboundRequest(req *http.Request) bool {
	if req == nil {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	rt := m.sessions[m.activeID]
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil {
		return false
	}

	if outboundMetaFromContext(req.Context()).ActionID != "" {
		return true
	}

	target := strings.TrimRight(req.URL.String(), "/")
	if target == "" {
		return false
	}

	return isSessionCallbackURL(rt, target) && outboundRequestMatchesSessionToken(rt, req)
}

func inboundRequestMatchesSession(rt *sessionRuntime, providedToken, fromCountry, fromParty string) bool {
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil {
		return false
	}

	tokenB, _ := rt.sandbox.Store.GetTokenB()
	if tokenB == "" || providedToken != tokenB {
		return false
	}

	peerCountry := strings.ToUpper(strings.TrimSpace(rt.session.Peer.CountryCode))
	peerParty := strings.ToUpper(strings.TrimSpace(rt.session.Peer.PartyID))
	if peerCountry == "" || peerParty == "" {
		return true
	}

	fromCountry = strings.ToUpper(strings.TrimSpace(fromCountry))
	fromParty = strings.ToUpper(strings.TrimSpace(fromParty))
	if fromCountry == "" && fromParty == "" {
		return true
	}

	return fromCountry == peerCountry && fromParty == peerParty
}

func isSessionCallbackURL(rt *sessionRuntime, target string) bool {
	for _, raw := range callbackURLs(rt) {
		if strings.TrimRight(raw, "/") == target {
			return true
		}
	}
	return false
}

func outboundRequestMatchesSessionToken(rt *sessionRuntime, req *http.Request) bool {
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil || req == nil {
		return false
	}

	expected, _ := rt.sandbox.Store.GetEMSPOwnToken()
	if expected == "" {
		expected = rt.session.Config.PeerToken
	}
	if expected == "" {
		return true
	}

	return parseAuthToken(req.Header.Get("Authorization")) == expected
}

func callbackURLs(rt *sessionRuntime) []string {
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil {
		return nil
	}

	seen := map[string]struct{}{}
	var urls []string

	appendURL := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if _, err := url.Parse(raw); err != nil {
			return
		}
		if _, exists := seen[raw]; exists {
			return
		}
		seen[raw] = struct{}{}
		urls = append(urls, raw)
	}

	rawSessions, _ := rt.sandbox.Store.ListSessions()
	for _, entry := range rawSessions {
		var payload struct {
			ResponseURL string `json:"_response_url"`
		}
		if json.Unmarshal(entry, &payload) == nil {
			appendURL(payload.ResponseURL)
		}
	}

	rawReservations, _ := rt.sandbox.Store.ListReservations()
	for _, entry := range rawReservations {
		var payload struct {
			ResponseURL string `json:"_response_url"`
		}
		if json.Unmarshal(entry, &payload) == nil {
			appendURL(payload.ResponseURL)
		}
	}

	return urls
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
