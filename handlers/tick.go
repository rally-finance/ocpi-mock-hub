package handlers

import (
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
	"github.com/rally-finance/ocpi-mock-hub/simulation"
)

func (h *Handler) Tick(w http.ResponseWriter, r *http.Request) {
	if err := h.tickAllStores(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) tickAllStores() error {
	if err := h.tickStore(h.Store, h.Seed, h.Config.EMSPCallbackURL); err != nil {
		return err
	}

	if overlay := h.correctnessStore(""); overlay != h.Store {
		if err := h.tickStore(overlay, h.currentSeed(), ""); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) tickStore(store Store, seed *fakegen.SeedData, defaultCallbackURL string) error {
	callbackURL := defaultCallbackURL
	if store != nil {
		if storedURL, err := store.GetEMSPCallbackURL(); err == nil && storedURL != "" {
			callbackURL = storedURL
		}
	}

	sim := simulation.New(
		store,
		seed,
		callbackURL,
		h.Config.CommandDelayMS,
		h.Config.SessionDurationS,
	)

	return sim.Tick()
}
