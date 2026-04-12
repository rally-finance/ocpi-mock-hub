package handlers

import (
	"context"
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
	if sessionID == "" || h.Correctness.ActiveSessionID() == sessionID {
		if overlay := h.Correctness.ActiveOverlay(); overlay != nil {
			return overlay
		}
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
