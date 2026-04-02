package hub

import (
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

// App holds shared dependencies injected into all handlers.
type App struct {
	Config    Config
	Store     Store
	Seed      *fakegen.SeedData
}

func NewApp(cfg Config) *App {
	return &App{
		Config: cfg,
		Store:  NewStore(cfg),
		Seed:   fakegen.GenerateSeed(cfg.SeedLocations),
	}
}
