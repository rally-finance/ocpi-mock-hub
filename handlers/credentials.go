package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

type credentialsPayload struct {
	Token           string `json:"token"`
	URL             string `json:"url,omitempty"`
	PartyID         string `json:"party_id,omitempty"`
	CountryCode     string `json:"country_code,omitempty"`
	BusinessDetails *struct {
		Name string `json:"name,omitempty"`
	} `json:"business_details,omitempty"`
	Roles []struct {
		Role            string `json:"role"`
		PartyID         string `json:"party_id"`
		CountryCode     string `json:"country_code"`
		BusinessDetails *struct {
			Name string `json:"name,omitempty"`
		} `json:"business_details,omitempty"`
	} `json:"roles,omitempty"`
}

func (h *Handler) PostCredentials(w http.ResponseWriter, r *http.Request) {
	if !h.verifyTokenA(w, r) {
		return
	}
	h.registerCredentials(w, r)
}

func (h *Handler) PutCredentials(w http.ResponseWriter, r *http.Request) {
	h.registerCredentials(w, r)
}

// registerCredentials contains the shared logic for POST and PUT /credentials:
// parse payload, store eMSP credentials, rotate Token B, return hub credentials.
func (h *Handler) registerCredentials(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read request body")
		return
	}

	var creds credentialsPayload
	if err := json.Unmarshal(body, &creds); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON payload")
		return
	}

	if creds.Token == "" {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Token is required")
		return
	}

	store.SetEMSPCredentials(body)
	store.SetEMSPOwnToken(creds.Token)
	if creds.URL != "" {
		store.SetEMSPCallbackURL(creds.URL)
	}

	tokenB := uuid.NewString()
	store.SetTokenB(tokenB)

	// Multi-party wiring (OCPI 2.2.1 §8.4.3): a single credentials exchange
	// may advertise multiple roles that share one TokenB. Persist a
	// PartyState per role so the admin UI, auth middleware, and deregister
	// flow can all see every connected party. Single-role callers still get
	// one entry, keyed by the role's country_code/party_id.
	persistCredentialRoles(store, creds, tokenB, body)

	scheme := resolveScheme(r)
	host := resolveHost(r)

	response := credentialsPayload{
		Token:       tokenB,
		URL:         scheme + "://" + host + "/ocpi/versions",
		CountryCode: h.Config.HubCountry,
		PartyID:     h.Config.HubParty,
		BusinessDetails: &struct {
			Name string `json:"name,omitempty"`
		}{Name: "OCPI Mock Hub"},
		Roles: []struct {
			Role            string `json:"role"`
			PartyID         string `json:"party_id"`
			CountryCode     string `json:"country_code"`
			BusinessDetails *struct {
				Name string `json:"name,omitempty"`
			} `json:"business_details,omitempty"`
		}{
			{
				Role:        "HUB",
				PartyID:     h.Config.HubParty,
				CountryCode: h.Config.HubCountry,
				BusinessDetails: &struct {
					Name string `json:"name,omitempty"`
				}{Name: "OCPI Mock Hub"},
			},
		},
	}

	ocpiutil.OK(w, r, response)
}

func (h *Handler) DeleteCredentials(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	if err := h.resetHandshakeState(store); err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to clear connection state")
		return
	}

	ocpiutil.OK(w, r, nil)
}

// persistCredentialRoles writes one PartyState entry per role advertised in
// the incoming credentials payload. Every entry shares the same TokenB so the
// auth middleware accepts inbound traffic from any of the party contexts on
// the connection. If the payload advertises no roles, a best-effort entry is
// still written from the top-level country_code/party_id (older callers that
// predate OCPI 2.2.1 multi-party credentials).
func persistCredentialRoles(store Store, creds credentialsPayload, tokenB string, rawBody []byte) {
	type roleEntry struct {
		CountryCode string
		PartyID     string
		Role        string
	}

	seen := map[string]bool{}
	var entries []roleEntry
	for _, role := range creds.Roles {
		cc := strings.ToUpper(strings.TrimSpace(role.CountryCode))
		pid := strings.ToUpper(strings.TrimSpace(role.PartyID))
		if cc == "" || pid == "" {
			continue
		}
		key := cc + "/" + pid
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, roleEntry{CountryCode: cc, PartyID: pid, Role: strings.ToUpper(role.Role)})
	}
	if len(entries) == 0 {
		cc := strings.ToUpper(strings.TrimSpace(creds.CountryCode))
		pid := strings.ToUpper(strings.TrimSpace(creds.PartyID))
		if cc == "" || pid == "" {
			return
		}
		entries = append(entries, roleEntry{CountryCode: cc, PartyID: pid, Role: "EMSP"})
	}

	for _, e := range entries {
		key := e.CountryCode + "/" + e.PartyID
		state := struct {
			Key         string `json:"key"`
			CountryCode string `json:"country_code"`
			PartyID     string `json:"party_id"`
			TokenB      string `json:"token_b"`
			OwnToken    string `json:"own_token"`
			CallbackURL string `json:"callback_url"`
			Credentials []byte `json:"credentials,omitempty"`
			Role        string `json:"role"`
		}{
			Key:         key,
			CountryCode: e.CountryCode,
			PartyID:     e.PartyID,
			TokenB:      tokenB,
			OwnToken:    creds.Token,
			CallbackURL: creds.URL,
			Credentials: rawBody,
			Role:        e.Role,
		}
		encoded, err := json.Marshal(state)
		if err != nil {
			continue
		}
		_ = store.PutParty(key, encoded)
	}
}

func (h *Handler) GetCredentials(w http.ResponseWriter, r *http.Request) {
	tokenB, _ := h.storeForRequest(r).GetTokenB()
	if tokenB == "" {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "No credentials exchanged yet")
		return
	}

	scheme := resolveScheme(r)
	host := resolveHost(r)

	response := credentialsPayload{
		Token:       tokenB,
		URL:         scheme + "://" + host + "/ocpi/versions",
		CountryCode: h.Config.HubCountry,
		PartyID:     h.Config.HubParty,
		BusinessDetails: &struct {
			Name string `json:"name,omitempty"`
		}{Name: "OCPI Mock Hub"},
	}

	ocpiutil.OK(w, r, response)
}
