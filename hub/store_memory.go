package hub

import (
	"encoding/json"
	"sync"
)

const defaultPartyKey = "DEFAULT"

type MemoryStore struct {
	mu                sync.RWMutex
	tokenB            string
	emspCallbackURL  string
	emspCreds        []byte
	emspOwnToken     string
	emspVersionsURL  string
	parties           map[string][]byte  // key: "CC/PID" -> JSON
	tokenBIndex       map[string]string  // tokenB -> party key
	tokens            map[string][]byte      // key: "cc/pid/uid"
	sessions          map[string][]byte      // key: session ID
	cdrs              map[string][]byte      // key: CDR ID
	reservations      map[string][]byte      // key: reservation ID
	chargingProfiles  map[string][]byte      // key: session ID
	mode              string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		parties:          make(map[string][]byte),
		tokenBIndex:      make(map[string]string),
		tokens:           make(map[string][]byte),
		sessions:         make(map[string][]byte),
		cdrs:             make(map[string][]byte),
		reservations:     make(map[string][]byte),
		chargingProfiles: make(map[string][]byte),
		mode:             "happy",
	}
}

func (m *MemoryStore) GetTokenB() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tokenB, nil
}

func (m *MemoryStore) SetTokenB(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenB = token
	return nil
}

func (m *MemoryStore) GetEMSPCallbackURL() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.emspCallbackURL, nil
}

func (m *MemoryStore) SetEMSPCallbackURL(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emspCallbackURL = url
	return nil
}

func (m *MemoryStore) GetEMSPCredentials() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.emspCreds, nil
}

func (m *MemoryStore) SetEMSPCredentials(creds []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emspCreds = creds
	return nil
}

func (m *MemoryStore) GetEMSPOwnToken() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.emspOwnToken, nil
}

func (m *MemoryStore) SetEMSPOwnToken(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emspOwnToken = token
	return nil
}

func (m *MemoryStore) GetEMSPVersionsURL() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.emspVersionsURL, nil
}

func (m *MemoryStore) SetEMSPVersionsURL(url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emspVersionsURL = url
	return nil
}

func (m *MemoryStore) PutParty(key string, state []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Remove old tokenB index entry
	if old, ok := m.parties[key]; ok {
		var prev struct{ TokenB string `json:"token_b"` }
		if json.Unmarshal(old, &prev) == nil && prev.TokenB != "" {
			delete(m.tokenBIndex, prev.TokenB)
		}
	}
	m.parties[key] = state
	var p struct{ TokenB string `json:"token_b"` }
	if json.Unmarshal(state, &p) == nil && p.TokenB != "" {
		m.tokenBIndex[p.TokenB] = key
	}
	return nil
}

func (m *MemoryStore) GetParty(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.parties[key], nil
}

func (m *MemoryStore) GetPartyByTokenB(tokenB string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key, ok := m.tokenBIndex[tokenB]
	if !ok {
		return nil, nil
	}
	return m.parties[key], nil
}

func (m *MemoryStore) DeleteParty(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.parties[key]; ok {
		var p struct{ TokenB string `json:"token_b"` }
		if json.Unmarshal(old, &p) == nil && p.TokenB != "" {
			delete(m.tokenBIndex, p.TokenB)
		}
	}
	delete(m.parties, key)
	return nil
}

func (m *MemoryStore) ListParties() ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([][]byte, 0, len(m.parties))
	for _, v := range m.parties {
		result = append(result, v)
	}
	return result, nil
}

func tokenKey(cc, pid, uid string) string {
	return cc + "/" + pid + "/" + uid
}

func (m *MemoryStore) PutToken(cc, pid, uid string, token []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[tokenKey(cc, pid, uid)] = token
	return nil
}

func (m *MemoryStore) GetToken(cc, pid, uid string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tokens[tokenKey(cc, pid, uid)], nil
}

func (m *MemoryStore) ListTokens() ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([][]byte, 0, len(m.tokens))
	for _, v := range m.tokens {
		result = append(result, v)
	}
	return result, nil
}

func (m *MemoryStore) PutSession(id string, session []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = session
	return nil
}

func (m *MemoryStore) GetSession(id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id], nil
}

func (m *MemoryStore) ListSessions() ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([][]byte, 0, len(m.sessions))
	for _, v := range m.sessions {
		result = append(result, v)
	}
	return result, nil
}

func (m *MemoryStore) DeleteSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *MemoryStore) PutCDR(id string, cdr []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cdrs[id] = cdr
	return nil
}

func (m *MemoryStore) GetCDR(id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cdrs[id], nil
}

func (m *MemoryStore) ListCDRs() ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([][]byte, 0, len(m.cdrs))
	for _, v := range m.cdrs {
		result = append(result, v)
	}
	return result, nil
}

func (m *MemoryStore) PutReservation(id string, reservation []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reservations[id] = reservation
	return nil
}

func (m *MemoryStore) GetReservation(id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reservations[id], nil
}

func (m *MemoryStore) ListReservations() ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([][]byte, 0, len(m.reservations))
	for _, v := range m.reservations {
		result = append(result, v)
	}
	return result, nil
}

func (m *MemoryStore) DeleteReservation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.reservations, id)
	return nil
}

func (m *MemoryStore) PutChargingProfile(sessionID string, profile []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chargingProfiles[sessionID] = profile
	return nil
}

func (m *MemoryStore) GetChargingProfile(sessionID string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.chargingProfiles[sessionID], nil
}

func (m *MemoryStore) DeleteChargingProfile(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.chargingProfiles, sessionID)
	return nil
}

func (m *MemoryStore) GetMode() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.mode == "" {
		return "happy", nil
	}
	return m.mode, nil
}

func (m *MemoryStore) SetMode(mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
	return nil
}
