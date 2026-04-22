package hub

import (
	"os"
	"strconv"
)

type Config struct {
	Port                         string
	TokenA                       string
	HubCountry                   string
	HubParty                     string
	InitiateHandshakeVersionsURL string
	SessionDurationS             int
	CommandDelayMS               int
	EMSPCallbackURL              string
	SeedLocations                int
	Mode                         string
	RedisURL                     string
}

func LoadConfig() Config {
	return Config{
		Port:                         envOr("PORT", "4000"),
		TokenA:                       envOr("MOCK_TOKEN_A", "mock-token-a-secret"),
		HubCountry:                   envOr("MOCK_HUB_COUNTRY", "DE"),
		HubParty:                     envOr("MOCK_HUB_PARTY", "HUB"),
		InitiateHandshakeVersionsURL: envOr("MOCK_INITIATE_HANDSHAKE_VERSIONS_URL", ""),
		SessionDurationS:             envInt("MOCK_SESSION_DURATION_S", 60),
		CommandDelayMS:               envInt("MOCK_COMMAND_DELAY_MS", 2000),
		EMSPCallbackURL:              envOr("EMSP_CALLBACK_URL", "http://localhost:3000/api/ocpi"),
		SeedLocations:                envInt("MOCK_SEED_LOCATIONS", 50),
		Mode:                         envOr("MOCK_MODE", "happy"),
		RedisURL:                     os.Getenv("FREE_TIER_REDIS_URL"),
	}
}

func (c Config) UseRedis() bool {
	return c.RedisURL != ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
