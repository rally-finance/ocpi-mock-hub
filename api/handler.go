package handler

import (
	"log"
	"net/http"
	"sync"

	"github.com/rally-finance/ocpi-mock-hub/hub"
)

var (
	router http.Handler
	once   sync.Once
)

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		cfg := hub.LoadConfig()
		app, err := hub.NewApp(cfg)
		if err != nil {
			log.Fatalf("failed to initialize app: %v", err)
		}
		router = hub.NewRouter(app)
	})
	router.ServeHTTP(w, r)
}
