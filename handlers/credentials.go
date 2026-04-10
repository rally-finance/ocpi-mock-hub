package handlers

import (
	"encoding/json"
	"io"
	"net/http"

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

	h.Store.SetEMSPCredentials(body)
	h.Store.SetEMSPOwnToken(creds.Token)
	if creds.URL != "" {
		h.Store.SetEMSPCallbackURL(creds.URL)
	}

	tokenB := uuid.NewString()
	h.Store.SetTokenB(tokenB)

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
	h.Store.SetTokenB("")
	h.Store.SetEMSPCallbackURL("")
	h.Store.SetEMSPCredentials(nil)
	h.Store.SetEMSPOwnToken("")
	h.Store.SetEMSPVersionsURL("")

	ocpiutil.OK(w, r, nil)
}

func (h *Handler) GetCredentials(w http.ResponseWriter, r *http.Request) {
	tokenB, _ := h.Store.GetTokenB()
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
