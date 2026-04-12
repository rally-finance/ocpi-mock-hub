package hub

import "github.com/rally-finance/ocpi-mock-hub/correctness"

type CorrectnessStore struct {
	base        Store
	correctness *correctness.Manager
}

func NewCorrectnessStore(base Store, manager *correctness.Manager) *CorrectnessStore {
	return &CorrectnessStore{base: base, correctness: manager}
}

func (s *CorrectnessStore) overlay() *correctness.OverlayStore {
	if s == nil || s.correctness == nil {
		return nil
	}
	return s.correctness.ActiveOverlay()
}

func (s *CorrectnessStore) GetTokenB() (string, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetTokenB()
	}
	return s.base.GetTokenB()
}

func (s *CorrectnessStore) SetTokenB(token string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.SetTokenB(token)
	}
	return s.base.SetTokenB(token)
}

func (s *CorrectnessStore) GetEMSPCallbackURL() (string, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetEMSPCallbackURL()
	}
	return s.base.GetEMSPCallbackURL()
}

func (s *CorrectnessStore) SetEMSPCallbackURL(url string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.SetEMSPCallbackURL(url)
	}
	return s.base.SetEMSPCallbackURL(url)
}

func (s *CorrectnessStore) GetEMSPCredentials() ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetEMSPCredentials()
	}
	return s.base.GetEMSPCredentials()
}

func (s *CorrectnessStore) SetEMSPCredentials(creds []byte) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.SetEMSPCredentials(creds)
	}
	return s.base.SetEMSPCredentials(creds)
}

func (s *CorrectnessStore) GetEMSPOwnToken() (string, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetEMSPOwnToken()
	}
	return s.base.GetEMSPOwnToken()
}

func (s *CorrectnessStore) SetEMSPOwnToken(token string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.SetEMSPOwnToken(token)
	}
	return s.base.SetEMSPOwnToken(token)
}

func (s *CorrectnessStore) GetEMSPVersionsURL() (string, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetEMSPVersionsURL()
	}
	return s.base.GetEMSPVersionsURL()
}

func (s *CorrectnessStore) SetEMSPVersionsURL(url string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.SetEMSPVersionsURL(url)
	}
	return s.base.SetEMSPVersionsURL(url)
}

func (s *CorrectnessStore) PutParty(key string, state []byte) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.PutParty(key, state)
	}
	return s.base.PutParty(key, state)
}

func (s *CorrectnessStore) GetParty(key string) ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetParty(key)
	}
	return s.base.GetParty(key)
}

func (s *CorrectnessStore) GetPartyByTokenB(tokenB string) ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetPartyByTokenB(tokenB)
	}
	return s.base.GetPartyByTokenB(tokenB)
}

func (s *CorrectnessStore) DeleteParty(key string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.DeleteParty(key)
	}
	return s.base.DeleteParty(key)
}

func (s *CorrectnessStore) ListParties() ([][]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.ListParties()
	}
	return s.base.ListParties()
}

func (s *CorrectnessStore) PutToken(cc, pid, uid string, token []byte) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.PutToken(cc, pid, uid, token)
	}
	return s.base.PutToken(cc, pid, uid, token)
}

func (s *CorrectnessStore) GetToken(cc, pid, uid string) ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetToken(cc, pid, uid)
	}
	return s.base.GetToken(cc, pid, uid)
}

func (s *CorrectnessStore) ListTokens() ([][]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.ListTokens()
	}
	return s.base.ListTokens()
}

func (s *CorrectnessStore) PutSession(id string, session []byte) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.PutSession(id, session)
	}
	return s.base.PutSession(id, session)
}

func (s *CorrectnessStore) GetSession(id string) ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetSession(id)
	}
	return s.base.GetSession(id)
}

func (s *CorrectnessStore) ListSessions() ([][]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.ListSessions()
	}
	return s.base.ListSessions()
}

func (s *CorrectnessStore) DeleteSession(id string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.DeleteSession(id)
	}
	return s.base.DeleteSession(id)
}

func (s *CorrectnessStore) PutCDR(id string, cdr []byte) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.PutCDR(id, cdr)
	}
	return s.base.PutCDR(id, cdr)
}

func (s *CorrectnessStore) GetCDR(id string) ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetCDR(id)
	}
	return s.base.GetCDR(id)
}

func (s *CorrectnessStore) ListCDRs() ([][]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.ListCDRs()
	}
	return s.base.ListCDRs()
}

func (s *CorrectnessStore) PutReservation(id string, reservation []byte) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.PutReservation(id, reservation)
	}
	return s.base.PutReservation(id, reservation)
}

func (s *CorrectnessStore) GetReservation(id string) ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetReservation(id)
	}
	return s.base.GetReservation(id)
}

func (s *CorrectnessStore) ListReservations() ([][]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.ListReservations()
	}
	return s.base.ListReservations()
}

func (s *CorrectnessStore) DeleteReservation(id string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.DeleteReservation(id)
	}
	return s.base.DeleteReservation(id)
}

func (s *CorrectnessStore) PutChargingProfile(sessionID string, profile []byte) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.PutChargingProfile(sessionID, profile)
	}
	return s.base.PutChargingProfile(sessionID, profile)
}

func (s *CorrectnessStore) GetChargingProfile(sessionID string) ([]byte, error) {
	if overlay := s.overlay(); overlay != nil {
		return overlay.GetChargingProfile(sessionID)
	}
	return s.base.GetChargingProfile(sessionID)
}

func (s *CorrectnessStore) DeleteChargingProfile(sessionID string) error {
	if overlay := s.overlay(); overlay != nil {
		return overlay.DeleteChargingProfile(sessionID)
	}
	return s.base.DeleteChargingProfile(sessionID)
}

func (s *CorrectnessStore) GetMode() (string, error) {
	return s.base.GetMode()
}

func (s *CorrectnessStore) SetMode(mode string) error {
	return s.base.SetMode(mode)
}
