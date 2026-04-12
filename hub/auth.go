package hub

import (
	"net/http"
	"strings"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func matchesAnyPartyToken(app *App, header string) bool {
	for _, candidate := range ocpiutil.AuthTokenCandidates(header) {
		party, _ := app.Store.GetPartyByTokenB(candidate)
		if party != nil {
			return true
		}
	}
	return false
}

func TokenAuthMiddleware(app *App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if path == "/ocpi/versions" ||
				path == "/ocpi/2.2.1" ||
				(path == "/ocpi/2.2.1/credentials" && r.Method == "POST") ||
				path == "/admin" ||
				strings.HasPrefix(path, "/admin/") ||
				strings.HasPrefix(path, "/api/tick") ||
				path == "/" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if len(ocpiutil.AuthTokenCandidates(authHeader)) == 0 {
				ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Missing authorization token")
				return
			}

			tokenB, err := app.Store.GetTokenB()
			if err != nil || tokenB == "" {
				// Try multi-party lookup
				if matchesAnyPartyToken(app, authHeader) {
					next.ServeHTTP(w, r)
					return
				}
				if app.Correctness != nil && app.Correctness.MatchesInboundRequest(r) {
					next.ServeHTTP(w, r)
					return
				}
				ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Handshake not completed")
				return
			}

			if !ocpiutil.AuthHeaderMatchesToken(authHeader, tokenB) {
				// Try multi-party lookup
				if matchesAnyPartyToken(app, authHeader) {
					next.ServeHTTP(w, r)
					return
				}
				if app.Correctness != nil && app.Correctness.MatchesInboundRequest(r) {
					next.ServeHTTP(w, r)
					return
				}
				ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Invalid authorization token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
