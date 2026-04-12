package hub

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

func parseToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "token") {
		return ""
	}
	raw := strings.TrimSpace(parts[1])
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil && len(decoded) > 0 {
		return string(decoded)
	}
	return raw
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

			provided := parseToken(r.Header.Get("Authorization"))
			if provided == "" {
				ocpiutil.Error(w, r, http.StatusUnauthorized, ocpiutil.StatusUnauthorized, "Missing authorization token")
				return
			}

			tokenB, err := app.Store.GetTokenB()
			if err != nil || tokenB == "" {
				// Try multi-party lookup
				party, _ := app.Store.GetPartyByTokenB(provided)
				if party != nil {
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

			if provided != tokenB {
				// Try multi-party lookup
				party, _ := app.Store.GetPartyByTokenB(provided)
				if party != nil {
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
