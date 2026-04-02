package handlers

import (
	"net/http"

	"github.com/rally-finance/ocpi-mock-hub/simulation"
)

func (h *Handler) Tick(w http.ResponseWriter, r *http.Request) {
	sim := simulation.New(
		h.Store,
		h.Seed,
		h.Config.EMSPCallbackURL,
		h.Config.CommandDelayMS,
		h.Config.SessionDurationS,
	)

	if err := sim.Tick(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
