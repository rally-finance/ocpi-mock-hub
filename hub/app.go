package hub

import (
	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

// App holds shared dependencies injected into all handlers.
type App struct {
	Config      Config
	Store       Store
	BaseStore   Store
	Seed        *fakegen.SeedData
	Correctness *correctness.Manager
}

func NewApp(cfg Config) (*App, error) {
	store, err := NewStore(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Mode != "" {
		store.SetMode(cfg.Mode)
	}
	seed := fakegen.GenerateSeed(cfg.SeedLocations)
	manager := correctness.NewManager(seed, store)
	return &App{
		Config:      cfg,
		Store:       store,
		BaseStore:   store,
		Seed:        seed,
		Correctness: manager,
	}, nil
}

func (a *App) CurrentSeed() *fakegen.SeedData {
	if a == nil || a.Correctness == nil {
		return a.Seed
	}
	return a.Correctness.CurrentSeed(a.Seed)
}
