package hub

// Store abstracts state persistence with two backends:
// - MemoryStore for standalone/local dev
// - RedisStore for deployed environments (requires REDIS_URL)
type Store interface {
	// Handshake state
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
