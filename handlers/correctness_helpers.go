package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func (h *Handler) currentSeed() *fakegen.SeedData {
	if h == nil || h.Correctness == nil {
		return h.Seed
	}
	return h.Correctness.CurrentSeed(h.Seed)
}

func (h *Handler) seedForRequest(r *http.Request) *fakegen.SeedData {
	if h == nil || h.Correctness == nil || r == nil {
		return h.Seed
	}
	if h.Correctness.MatchesInboundRequest(r) {
		return h.currentSeed()
	}
	return h.Seed
}

func (h *Handler) storeForRequest(r *http.Request) Store {
	if h == nil || h.Correctness == nil || r == nil {
		return h.Store
	}
	if h.Correctness.MatchesInboundRequest(r) {
		if overlay := h.Correctness.ActiveOverlay(); overlay != nil {
			return overlay
		}
	}
	return h.Store
}

func (h *Handler) correctnessStore(sessionID string) Store {
	if h == nil || h.Correctness == nil {
		return h.Store
	}
	if sessionID != "" {
		if overlay := h.Correctness.OverlayForSession(sessionID); overlay != nil {
			return overlay
		}
		return h.Store
	}
	if overlay := h.Correctness.ActiveOverlay(); overlay != nil {
		return overlay
	}
	return h.Store
}

func (h *Handler) outboundClient() *http.Client {
	if h == nil || h.HTTPClient == nil {
		return http.DefaultClient
	}
	return h.HTTPClient
}

func (h *Handler) outboundContext(actionID string) context.Context {
	if h == nil || h.Correctness == nil {
		return context.Background()
	}
	return correctness.WithOutboundMeta(context.Background(), correctness.OutboundMeta{ActionID: actionID})
}

func correctnessPartyKey(sessionID string) string {
	return "correctness/" + sessionID
}

func (h *Handler) registerCorrectnessPeerToken(session *correctness.TestSession, tokenB, advertisedVersionsURL, callbackURL string, peer correctness.SessionPeerState, ownToken string) error {
	if h == nil || h.Store == nil || session == nil || tokenB == "" {
		return nil
	}

	payload, err := json.Marshal(map[string]string{
		"key":          correctnessPartyKey(session.ID),
		"country_code": peer.CountryCode,
		"party_id":     peer.PartyID,
		"token_b":      tokenB,
		"own_token":    ownToken,
		"callback_url": callbackURL,
		"versions_url": advertisedVersionsURL,
		"role":         "EMSP",
	})
	if err != nil {
		return err
	}
	return h.Store.PutParty(correctnessPartyKey(session.ID), payload)
}

func (h *Handler) unregisterCorrectnessPeerToken(sessionID string) {
	if h == nil || h.Store == nil || sessionID == "" {
		return
	}
	_ = h.Store.DeleteParty(correctnessPartyKey(sessionID))
}
