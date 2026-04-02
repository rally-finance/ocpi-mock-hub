package handler

import (
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
		app := hub.NewApp(cfg)
		router = hub.NewRouter(app)
	})
	router.ServeHTTP(w, r)
}
