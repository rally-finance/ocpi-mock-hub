package handlers

import (
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

// Handler holds dependencies for all OCPI route handlers.
type Handler struct {
	Config HandlerConfig
	Store  Store
	Seed   *fakegen.SeedData
	ReqLog *RequestLog
}

type HandlerConfig struct {
	TokenA                       string
	HubCountry                   string
	HubParty                     string
	InitiateHandshakeVersionsURL string
	EMSPCallbackURL              string
	EncodeBase64                 bool
	CommandDelayMS               int
	SessionDurationS             int
}

// Store is the subset of state handlers need.
type Store interface {
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
	PutToken(cc, pid, uid string, token []byte) error
	GetToken(cc, pid, uid string) ([]byte, error)
	ListTokens() ([][]byte, error)
	PutSession(id string, session []byte) error
	GetSession(id string) ([]byte, error)
	ListSessions() ([][]byte, error)
	DeleteSession(id string) error
	PutCDR(id string, cdr []byte) error
	ListCDRs() ([][]byte, error)
	PutReservation(id string, reservation []byte) error
	GetReservation(id string) ([]byte, error)
	ListReservations() ([][]byte, error)
	DeleteReservation(id string) error
	GetMode() (string, error)
	SetMode(mode string) error
}

func New(cfg HandlerConfig, s Store, seed *fakegen.SeedData, reqLog *RequestLog) *Handler {
	return &Handler{Config: cfg, Store: s, Seed: seed, ReqLog: reqLog}
}
