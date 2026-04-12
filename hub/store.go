package hub

// PartyState holds handshake state for a single connected party (eMSP or CPO).
type PartyState struct {
	Key         string `json:"key"`
	CountryCode string `json:"country_code"`
	PartyID     string `json:"party_id"`
	TokenB      string `json:"token_b"`
	OwnToken    string `json:"own_token"`
	CallbackURL string `json:"callback_url"`
	VersionsURL string `json:"versions_url"`
	Credentials []byte `json:"credentials,omitempty"`
	Role        string `json:"role"`
}

// Store abstracts state persistence with two backends:
// - MemoryStore for standalone/local dev
// - RedisStore for deployed environments (requires REDIS_URL)
type Store interface {
	// Generic blobs for shared multi-instance state.
	GetBlob(key string) ([]byte, error)
	UpdateBlob(key string, fn func([]byte) ([]byte, error)) error

	// Handshake state (single-party shims — delegate to default party)
	GetTokenB() (string, error)
	SetTokenB(token string) error
	GetEMSPCallbackURL() (string, error)
	SetEMSPCallbackURL(url string) error
	GetEMSPCredentials() ([]byte, error)
	SetEMSPCredentials(creds []byte) error
	GetEMSPOwnToken() (string, error)
	SetEMSPOwnToken(token string) error
	GetEMSPVersionsURL() (string, error)
	SetEMSPVersionsURL(url string) error

	// Multi-party handshake state (raw JSON)
	PutParty(key string, state []byte) error
	GetParty(key string) ([]byte, error)
	GetPartyByTokenB(tokenB string) ([]byte, error)
	DeleteParty(key string) error
	ListParties() ([][]byte, error)

	// Tokens (eMSP pushes to us)
	PutToken(cc, pid, uid string, token []byte) error
	GetToken(cc, pid, uid string) ([]byte, error)
	ListTokens() ([][]byte, error)

	// Sessions (created by commands, advanced by tick)
	PutSession(id string, session []byte) error
	GetSession(id string) ([]byte, error)
	ListSessions() ([][]byte, error)
	DeleteSession(id string) error

	// CDRs (generated on session completion)
	PutCDR(id string, cdr []byte) error
	GetCDR(id string) ([]byte, error)
	ListCDRs() ([][]byte, error)

	// Reservations (created by RESERVE_NOW, expired by tick)
	PutReservation(id string, reservation []byte) error
	GetReservation(id string) ([]byte, error)
	ListReservations() ([][]byte, error)
	DeleteReservation(id string) error

	// Charging profiles
	PutChargingProfile(sessionID string, profile []byte) error
	GetChargingProfile(sessionID string) ([]byte, error)
	DeleteChargingProfile(sessionID string) error

	// Simulation mode
	GetMode() (string, error)
	SetMode(mode string) error
}

func NewStore(cfg Config) (Store, error) {
	if cfg.UseRedis() {
		return NewRedisStore(cfg.RedisURL)
	}
	return NewMemoryStore(), nil
}
