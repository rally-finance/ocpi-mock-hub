package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type initiateHandshakePayload struct {
	EMSPVersionsURL string `json:"emsp_versions_url"`
	EMSPOwnToken    string `json:"emsp_own_token"`
}

type initiateHandshakeResult struct {
	Success     bool   `json:"success"`
	TokenB      string `json:"token_b,omitempty"`
	CallbackURL string `json:"callback_url,omitempty"`
	Error       string `json:"error,omitempty"`
}

type ocpiVersionsResponse struct {
	Data []struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	} `json:"data"`
	StatusCode int `json:"status_code"`
}

type ocpiVersionDetailResponse struct {
	Data struct {
		Version   string `json:"version"`
		Endpoints []struct {
			Identifier string `json:"identifier"`
			Role       string `json:"role,omitempty"`
			URL        string `json:"url"`
		} `json:"endpoints"`
	} `json:"data"`
	StatusCode int `json:"status_code"`
}

type ocpiCredentialsResponse struct {
	Data       credentialsPayload `json:"data"`
	StatusCode int                `json:"status_code"`
}

func (h *Handler) InitiateHandshake(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, initiateHandshakeResult{Error: "failed to read body"})
		return
	}

	var payload initiateHandshakePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, initiateHandshakeResult{Error: "invalid JSON"})
		return
	}

	if payload.EMSPVersionsURL == "" || payload.EMSPOwnToken == "" {
		writeJSON(w, http.StatusBadRequest, initiateHandshakeResult{Error: "emsp_versions_url and emsp_own_token are required"})
		return
	}

	versionsURL := normalizeVersionsURL(payload.EMSPVersionsURL)
	h.Store.SetEMSPOwnToken(payload.EMSPOwnToken)
	h.Store.SetEMSPVersionsURL(versionsURL)

	versionDetailURL, err := h.discoverVersion(versionsURL, payload.EMSPOwnToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, initiateHandshakeResult{Error: fmt.Sprintf("version discovery failed: %v", err)})
		return
	}

	credentialsURL, err := h.discoverCredentialsEndpoint(versionDetailURL, payload.EMSPOwnToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, initiateHandshakeResult{Error: fmt.Sprintf("endpoint discovery failed: %v", err)})
		return
	}

	tokenB, _ := h.Store.GetTokenB()
	if tokenB == "" {
		tokenB = "hub-token-b-" + uuid.NewString()[:12]
		h.Store.SetTokenB(tokenB)
	}

	scheme := resolveScheme(r)
	host := resolveHost(r)

	credsBody := credentialsPayload{
		Token:       tokenB,
		URL:         h.handshakeAdvertisedVersionsURL(scheme, host),
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

	emspCreds, err := h.postCredentials(credentialsURL, payload.EMSPOwnToken, credsBody)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, initiateHandshakeResult{Error: fmt.Sprintf("credentials exchange failed: %v", err)})
		return
	}

	if emspCreds.Data.Token != "" {
		h.Store.SetEMSPOwnToken(emspCreds.Data.Token)
	}
	if emspCreds.Data.URL != "" {
		h.Store.SetEMSPCallbackURL(emspCreds.Data.URL)
	}

	rawCreds, _ := json.Marshal(emspCreds.Data)
	h.Store.SetEMSPCredentials(rawCreds)

	writeJSON(w, http.StatusOK, initiateHandshakeResult{
		Success:     true,
		TokenB:      tokenB,
		CallbackURL: emspCreds.Data.URL,
	})
}

func (h *Handler) handshakeAdvertisedVersionsURL(scheme, host string) string {
	if h.Config.InitiateHandshakeVersionsURL != "" {
		return h.Config.InitiateHandshakeVersionsURL
	}
	return scheme + "://" + host + "/ocpi/versions"
}

func (h *Handler) discoverVersion(versionsURL, token string) (string, error) {
	req, err := http.NewRequest("GET", versionsURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", versionsURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s returned %d: %s", versionsURL, resp.StatusCode, string(body))
	}

	var versions ocpiVersionsResponse
	if err := json.Unmarshal(body, &versions); err != nil {
		return "", fmt.Errorf("parse versions: %w", err)
	}

	for _, v := range versions.Data {
		if v.Version == "2.2.1" {
			return v.URL, nil
		}
	}

	return "", fmt.Errorf("version 2.2.1 not found in %d versions", len(versions.Data))
}

func (h *Handler) discoverCredentialsEndpoint(versionDetailURL, token string) (string, error) {
	req, err := http.NewRequest("GET", versionDetailURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", versionDetailURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s returned %d: %s", versionDetailURL, resp.StatusCode, string(body))
	}

	var detail ocpiVersionDetailResponse
	if err := json.Unmarshal(body, &detail); err != nil {
		return "", fmt.Errorf("parse version detail: %w", err)
	}

	for _, ep := range detail.Data.Endpoints {
		if strings.EqualFold(ep.Identifier, "credentials") {
			return ep.URL, nil
		}
	}

	return "", fmt.Errorf("credentials endpoint not found in version details")
}

func (h *Handler) postCredentials(credentialsURL, token string, creds credentialsPayload) (*ocpiCredentialsResponse, error) {
	payload, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("marshal credentials: %w", err)
	}

	req, err := http.NewRequest("POST", credentialsURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", credentialsURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST %s returned %d: %s", credentialsURL, resp.StatusCode, string(body))
	}

	var result ocpiCredentialsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse credentials response: %w", err)
	}

	return &result, nil
}

// normalizeVersionsURL strips a trailing version path (e.g. /2.2.1) so the
// user can paste either the versions list URL or the version detail URL.
func normalizeVersionsURL(u string) string {
	u = strings.TrimRight(u, "/")
	suffixes := []string{"/2.2.1", "/2.2", "/2.1.1", "/2.0"}
	for _, s := range suffixes {
		if strings.HasSuffix(u, s) {
			return strings.TrimSuffix(u, s)
		}
	}
	return u
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
