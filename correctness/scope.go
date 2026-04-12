package correctness

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func (m *Manager) MatchesInboundRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return false
	}

	rt := m.sessions[m.activeID]
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil {
		return false
	}

	if len(ocpiutil.AuthTokenCandidates(r.Header.Get("Authorization"))) == 0 {
		return false
	}

	return inboundRequestMatchesSession(rt, r.Header.Get("Authorization"), r.Header.Get("OCPI-From-Country-Code"), r.Header.Get("OCPI-From-Party-Id"))
}

func (m *Manager) ShouldCaptureInboundRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if m.MatchesInboundRequest(r) {
		return true
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return false
	}

	rt := m.sessions[m.activeID]
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil {
		return false
	}

	return inboundHandshakeDiscoveryCandidate(rt, r)
}

func (m *Manager) ShouldCaptureOutboundRequest(req *http.Request) bool {
	if req == nil {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return false
	}

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

func inboundRequestMatchesSession(rt *sessionRuntime, authHeader, fromCountry, fromParty string) bool {
	if rt == nil || rt.sandbox == nil || rt.sandbox.Store == nil {
		return false
	}

	tokenB, _ := rt.sandbox.Store.GetTokenB()
	if tokenB == "" || !ocpiutil.AuthHeaderMatchesToken(authHeader, tokenB) {
		return false
	}

	return peerHeadersMatchSession(rt, fromCountry, fromParty, true)
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

	return ocpiutil.AuthHeaderMatchesToken(req.Header.Get("Authorization"), expected)
}

func inboundHandshakeDiscoveryCandidate(rt *sessionRuntime, r *http.Request) bool {
	if rt == nil || r == nil || !isHandshakeDiscoveryPath(r.URL.Path) {
		return false
	}

	authHeader := r.Header.Get("Authorization")
	if len(ocpiutil.AuthTokenCandidates(authHeader)) == 0 {
		return false
	}

	// Capture peer discovery attempts that still use the session's configured
	// outbound peer token so the evaluator can surface the resulting 401.
	expectedPeerToken := strings.TrimSpace(rt.session.Config.PeerToken)
	if expectedPeerToken == "" || !ocpiutil.AuthHeaderMatchesToken(authHeader, expectedPeerToken) {
		return false
	}

	return peerHeadersMatchSession(rt, r.Header.Get("OCPI-From-Country-Code"), r.Header.Get("OCPI-From-Party-Id"), true)
}

func peerHeadersMatchSession(rt *sessionRuntime, fromCountry, fromParty string, allowEmpty bool) bool {
	peerCountry := strings.ToUpper(strings.TrimSpace(rt.session.Peer.CountryCode))
	peerParty := strings.ToUpper(strings.TrimSpace(rt.session.Peer.PartyID))
	if peerCountry == "" || peerParty == "" {
		return true
	}

	fromCountry = strings.ToUpper(strings.TrimSpace(fromCountry))
	fromParty = strings.ToUpper(strings.TrimSpace(fromParty))
	if allowEmpty && fromCountry == "" && fromParty == "" {
		return true
	}

	return fromCountry == peerCountry && fromParty == peerParty
}

func isHandshakeDiscoveryPath(path string) bool {
	switch strings.TrimSpace(path) {
	case "/ocpi/versions", "/ocpi/2.2.1":
		return true
	default:
		return false
	}
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
