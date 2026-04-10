package hub

import (
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

// App holds shared dependencies injected into all handlers.
type App struct {
	Config Config
	Store  Store
	Seed   *fakegen.SeedData
}

func NewApp(cfg Config) (*App, error) {
	store, err := NewStore(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Mode != "" {
		store.SetMode(cfg.Mode)
	}
	return &App{
		Config: cfg,
		Store:  store,
		Seed:   fakegen.GenerateSeed(cfg.SeedLocations),
	}, nil
}
