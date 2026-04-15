package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type deregisterState struct {
	PeerToken       string
	PeerVersionsURL string
}

func maskToken(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	if len(t) <= 8 {
		return "****"
	}
	return t[:4] + "..." + t[len(t)-4:]
}

func (h *Handler) resetHandshakeState(store Store) error {
	if store == nil {
		return nil
	}
	return errors.Join(
		store.SetTokenB(""),
		store.SetEMSPCallbackURL(""),
		store.SetEMSPCredentials(nil),
		store.SetEMSPOwnToken(""),
		store.SetEMSPVersionsURL(""),
	)
}

func (h *Handler) currentDeregisterState(store Store) (deregisterState, string) {
	if store == nil {
		return deregisterState{}, "Connection state is unavailable"
	}

	tokenB, _ := store.GetTokenB()
	if strings.TrimSpace(tokenB) == "" {
		return deregisterState{}, "No active OCPI connection to deregister"
	}

	peerToken, _ := store.GetEMSPOwnToken()
	peerToken = strings.TrimSpace(peerToken)
	if peerToken == "" {
		return deregisterState{}, "Missing stored peer credentials token"
	}

	peerVersionsURL, err := h.resolvePeerVersionsURL(store)
	if err != nil {
		return deregisterState{}, err.Error()
	}

	return deregisterState{
		PeerToken:       peerToken,
		PeerVersionsURL: peerVersionsURL,
	}, ""
}

func (h *Handler) resolvePeerVersionsURL(store Store) (string, error) {
	if store == nil {
		return "", errors.New("Missing peer versions URL")
	}

	if versionsURL, _ := store.GetEMSPVersionsURL(); strings.TrimSpace(versionsURL) != "" {
		return strings.TrimSpace(versionsURL), nil
	}
	if callbackURL, _ := store.GetEMSPCallbackURL(); strings.TrimSpace(callbackURL) != "" {
		return strings.TrimSpace(callbackURL), nil
	}

	rawCreds, _ := store.GetEMSPCredentials()
	if len(rawCreds) == 0 {
		return "", errors.New("Missing peer versions URL")
	}

	var creds credentialsPayload
	if err := json.Unmarshal(rawCreds, &creds); err != nil {
		return "", fmt.Errorf("Stored peer credentials are invalid: %w", err)
	}
	if strings.TrimSpace(creds.URL) == "" {
		return "", errors.New("Missing peer versions URL")
	}
	return strings.TrimSpace(creds.URL), nil
}

func (h *Handler) discoverPeerCredentialsURL(ctx context.Context, versionsURL, token string) (string, error) {
	versionDetailURL, err := h.discoverVersion(ctx, versionsURL, token)
	if err != nil {
		return "", fmt.Errorf("version discovery failed: %w", err)
	}

	credentialsURL, err := h.discoverCredentialsEndpoint(ctx, versionDetailURL, token)
	if err != nil {
		return "", fmt.Errorf("endpoint discovery failed: %w", err)
	}

	return credentialsURL, nil
}

func (h *Handler) deleteCredentials(ctx context.Context, credentialsURL, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, credentialsURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+token)

	resp, err := h.outboundClient().Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", credentialsURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DELETE %s returned %d: %s", credentialsURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func (h *Handler) DeregisterConnection(w http.ResponseWriter, r *http.Request) {
	store := h.Store
	state, reason := h.currentDeregisterState(store)
	if reason != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": reason})
		return
	}

	credentialsURL, err := h.discoverPeerCredentialsURL(r.Context(), state.PeerVersionsURL, state.PeerToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	if err := h.deleteCredentials(r.Context(), credentialsURL, state.PeerToken); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	if err := h.resetHandshakeState(store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear local handshake state: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":          "deregistered",
		"credentials_url": credentialsURL,
	})
}
