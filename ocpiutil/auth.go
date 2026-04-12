package ocpiutil

import (
	"encoding/base64"
	"strings"
)

// AuthTokenCandidates returns the literal token from the Authorization header
// plus its decoded form when the peer sent a base64-encoded token value.
func AuthTokenCandidates(header string) []string {
	if header == "" {
		return nil
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "token") {
		return nil
	}

	raw := strings.TrimSpace(parts[1])
	if raw == "" {
		return nil
	}

	candidates := []string{raw}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil && len(decoded) > 0 {
		decodedToken := string(decoded)
		if decodedToken != raw {
			candidates = append(candidates, decodedToken)
		}
	}

	return candidates
}

func AuthHeaderMatchesToken(header, expected string) bool {
	if expected == "" {
		return false
	}

	for _, candidate := range AuthTokenCandidates(header) {
		if candidate == expected {
			return true
		}
	}

	return false
}
