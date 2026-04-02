package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/rally-finance/ocpi-mock-hub/hub"
	"github.com/rally-finance/ocpi-mock-hub/simulation"
)

func main() {
	cfg := hub.LoadConfig()
	app := hub.NewApp(cfg)
	router := hub.NewRouter(app)

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		sim := simulation.New(app.Store, app.Seed, cfg.EMSPCallbackURL, cfg.CommandDelayMS, cfg.SessionDurationS)
		for range ticker.C {
			if err := sim.Tick(); err != nil {
				log.Printf("[tick] error: %v", err)
			}
		}
	}()

	addr := ":" + cfg.Port
	log.Printf("ocpi-mock-hub listening on %s", addr)
	log.Printf("  Token A: %s", cfg.TokenA)
	log.Printf("  Hub identity: %s*%s", cfg.HubCountry, cfg.HubParty)
	log.Printf("  eMSP callback: %s", cfg.EMSPCallbackURL)
	log.Printf("  Seed locations: %d", cfg.SeedLocations)
	log.Printf("  Store backend: %s", func() string {
		if cfg.UseKV() {
			return "Vercel KV"
		}
		return "in-memory"
	}())
	fmt.Printf("\n  → OCPI versions URL: http://localhost%s/ocpi/versions\n\n", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
