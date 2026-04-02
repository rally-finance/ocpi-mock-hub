package hub

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rally-finance/ocpi-mock-hub/admin"
	"github.com/rally-finance/ocpi-mock-hub/handlers"
)

func NewRouter(app *App) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	reqLog := handlers.NewRequestLog()
	r.Use(handlers.RequestLogMiddleware(reqLog))
	r.Use(TokenAuthMiddleware(app))

	h := handlers.New(handlers.HandlerConfig{
		TokenA:           app.Config.TokenA,
		HubCountry:       app.Config.HubCountry,
		HubParty:         app.Config.HubParty,
		EMSPCallbackURL: app.Config.EMSPCallbackURL,
		EncodeBase64:     app.Config.EncodeBase64,
		CommandDelayMS:   app.Config.CommandDelayMS,
		SessionDurationS: app.Config.SessionDurationS,
	}, app.Store, app.Seed, reqLog)

	// OCPI version discovery
	r.Get("/ocpi/versions", h.GetVersions)
	r.Get("/ocpi/2.2.1", h.GetVersionDetails)

	// OCPI credentials
	r.Post("/ocpi/2.2.1/credentials", h.PostCredentials)
	r.Get("/ocpi/2.2.1/credentials", h.GetCredentials)

	// OCPI sender modules (hub pushes data to eMSP)
	r.Get("/ocpi/2.2.1/sender/locations", h.GetLocations)
	r.Get("/ocpi/2.2.1/sender/locations/{locationID}", h.GetLocation)
	r.Get("/ocpi/2.2.1/sender/tariffs", h.GetTariffs)
	r.Get("/ocpi/2.2.1/sender/tariffs/{countryCode}/{partyID}/{tariffID}", h.GetTariff)
	r.Get("/ocpi/2.2.1/sender/sessions", h.GetSessions)
	r.Get("/ocpi/2.2.1/sender/cdrs", h.GetCDRs)
	r.Get("/ocpi/2.2.1/sender/hubclientinfo", h.GetHubClientInfo)

	// OCPI receiver modules (eMSP pushes data to hub)
	r.Put("/ocpi/2.2.1/receiver/tokens/{countryCode}/{partyID}/{uid}", h.PutToken)
	r.Post("/ocpi/2.2.1/receiver/commands/{command}", h.PostCommand)

	// Simulation tick (for Vercel cron)
	r.Post("/api/tick", h.Tick)
	r.Get("/api/tick", h.Tick)

	// Admin JSON API
	r.Get("/admin/status", h.GetStatus)
	r.Get("/admin/sessions", h.GetAdminSessions)
	r.Get("/admin/cdrs", h.GetAdminCDRs)
	r.Get("/admin/locations", h.GetAdminLocations)
	r.Get("/admin/log", h.GetRequestLog)
	r.Post("/admin/mode", h.SetMode)
	r.Get("/admin/mode", h.GetMode)
	r.Post("/admin/initiate-handshake", h.InitiateHandshake)
	r.Post("/admin/reset", h.ResetConnection)
	r.Post("/admin/trigger-tick", h.TriggerTick)
	r.Post("/admin/push-locations", h.PushLocations)
	r.Post("/admin/push-tariffs", h.PushTariffs)

	// Admin HTML UI (embedded)
	r.Get("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently).ServeHTTP)
	r.Get("/admin/", serveAdminHTML)

	// Health check
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"ocpi-mock-hub","admin":"/admin/"}`))
	})

	return r
}

func serveAdminHTML(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(admin.FS, "admin.html")
	if err != nil {
		http.Error(w, "admin UI not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
