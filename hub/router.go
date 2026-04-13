package hub

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rally-finance/ocpi-mock-hub/admin"
	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/handlers"
	"github.com/rally-finance/ocpi-mock-hub/simulation"
)

func NewRouter(app *App) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	reqLog := handlers.NewRequestLog(app.Store)
	r.Use(handlers.RequestLogMiddleware(reqLog))
	r.Use(correctness.Middleware(app.Correctness))
	r.Use(ocpiFromHeadersMiddleware(app.Config.HubCountry, app.Config.HubParty))
	r.Use(TokenAuthMiddleware(app))

	httpClient := correctness.NewHTTPClient(app.Correctness, http.DefaultClient)
	simulation.SetHTTPClient(httpClient)

	h := handlers.New(handlers.HandlerConfig{
		TokenA:                       app.Config.TokenA,
		HubCountry:                   app.Config.HubCountry,
		HubParty:                     app.Config.HubParty,
		InitiateHandshakeVersionsURL: app.Config.InitiateHandshakeVersionsURL,
		EMSPCallbackURL:              app.Config.EMSPCallbackURL,
		EncodeBase64:                 app.Config.EncodeBase64,
		CommandDelayMS:               app.Config.CommandDelayMS,
		SessionDurationS:             app.Config.SessionDurationS,
	}, app.Store, app.Seed, reqLog, app.Correctness, httpClient)

	r.Use(handlers.FaultModeMiddleware(h))

	// OCPI version discovery
	r.Get("/ocpi/versions", h.GetVersions)
	r.Get("/ocpi/2.2.1", h.GetVersionDetails)

	// OCPI credentials
	r.Post("/ocpi/2.2.1/credentials", h.PostCredentials)
	r.Get("/ocpi/2.2.1/credentials", h.GetCredentials)
	r.Put("/ocpi/2.2.1/credentials", h.PutCredentials)
	r.Delete("/ocpi/2.2.1/credentials", h.DeleteCredentials)

	// OCPI sender modules (hub pushes data to eMSP)
	r.Get("/ocpi/2.2.1/sender/locations", h.GetLocations)
	r.Get("/ocpi/2.2.1/sender/locations/{locationID}", h.GetLocation)
	r.Get("/ocpi/2.2.1/sender/locations/{locationID}/{evseUID}", h.GetEVSE)
	r.Get("/ocpi/2.2.1/sender/locations/{locationID}/{evseUID}/{connectorID}", h.GetConnector)
	r.Get("/ocpi/2.2.1/sender/tariffs", h.GetTariffs)
	r.Get("/ocpi/2.2.1/sender/tariffs/{countryCode}/{partyID}/{tariffID}", h.GetTariff)
	r.Get("/ocpi/2.2.1/sender/sessions", h.GetSessions)
	r.Get("/ocpi/2.2.1/sender/sessions/{countryCode}/{partyID}/{sessionID}", h.GetSessionByID)
	r.Get("/ocpi/2.2.1/sender/cdrs", h.GetCDRs)
	r.Get("/ocpi/2.2.1/sender/cdrs/{countryCode}/{partyID}/{cdrID}", h.GetCDRByID)
	r.Get("/ocpi/2.2.1/sender/tokens", h.GetTokens)
	r.Get("/ocpi/2.2.1/sender/tokens/{countryCode}/{partyID}/{uid}", h.GetTokenByID)
	r.Post("/ocpi/2.2.1/sender/tokens/{countryCode}/{partyID}/{uid}/authorize", h.PostTokenAuthorize)
	r.Get("/ocpi/2.2.1/sender/hubclientinfo", h.GetHubClientInfo)

	// OCPI receiver modules (eMSP pushes data to hub)
	r.Put("/ocpi/2.2.1/receiver/tokens/{countryCode}/{partyID}/{uid}", h.PutToken)
	r.Post("/ocpi/2.2.1/receiver/commands/{command}", h.PostCommand)
	r.Put("/ocpi/2.2.1/receiver/sessions/{countryCode}/{partyID}/{sessionID}", h.PutReceiverSession)
	r.Post("/ocpi/2.2.1/receiver/cdrs", h.PostReceiverCDR)
	r.Get("/ocpi/2.2.1/receiver/cdrs/{cdrID}", h.GetReceiverCDR)
	r.Put("/ocpi/2.2.1/receiver/chargingprofiles/{sessionID}", h.PutChargingProfile)
	r.Get("/ocpi/2.2.1/receiver/chargingprofiles/{sessionID}", h.GetChargingProfile)
	r.Delete("/ocpi/2.2.1/receiver/chargingprofiles/{sessionID}", h.DeleteChargingProfile)

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
	r.Get("/admin/tokens", h.GetAdminTokens)
	r.Get("/admin/reservations", h.GetAdminReservations)
	r.Post("/admin/authorize", h.AdminAuthorize)
	r.Get("/admin/test-suites", h.GetCorrectnessSuites)
	r.Get("/admin/test-sessions", h.ListCorrectnessSessions)
	r.Post("/admin/test-sessions", h.CreateCorrectnessSession)
	r.Get("/admin/test-sessions/{sessionID}", h.GetCorrectnessSession)
	r.Post("/admin/test-sessions/{sessionID}/actions/{actionID}", h.RunCorrectnessAction)
	r.Post("/admin/test-sessions/{sessionID}/checkpoints/{checkpointID}", h.SubmitCorrectnessCheckpoint)
	r.Post("/admin/test-sessions/{sessionID}/rerun", h.RerunCorrectnessSession)

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

func ocpiFromHeadersMiddleware(hubCountry, hubParty string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/ocpi/") {
				w.Header().Set("OCPI-From-Country-Code", hubCountry)
				w.Header().Set("OCPI-From-Party-Id", hubParty)
			}
			next.ServeHTTP(w, r)
		})
	}
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
