package hub

import (
	"encoding/json"
	"sync"
)

type MemoryStore struct {
	mu               sync.RWMutex
	blobs            map[string][]byte
	blobLocks        sync.Map
	tokenB           string
	emspCallbackURL  string
	emspCreds        []byte
	emspOwnToken     string
	emspVersionsURL  string
	parties          map[string][]byte            // key: "CC/PID" -> JSON
	tokenBIndex      map[string]map[string]bool   // tokenB -> set of party keys (multi-party connections share one token)
	tokens           map[string][]byte // key: "cc/pid/uid"
	sessions         map[string][]byte // key: session ID
	cdrs             map[string][]byte // key: CDR ID
	reservations     map[string][]byte // key: reservation ID
	chargingProfiles map[string][]byte // key: session ID
	mode             string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		blobs:            make(map[string][]byte),
		parties:          make(map[string][]byte),
		tokenBIndex:      make(map[string]map[string]bool),
		tokens:           make(map[string][]byte),
		sessions:         make(map[string][]byte),
		cdrs:             make(map[string][]byte),
		reservations:     make(map[string][]byte),
		chargingProfiles: make(map[string][]byte),
		mode:             "happy",
	}
}

func (m *MemoryStore) GetBlob(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val := m.blobs[key]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (m *MemoryStore) blobLock(key string) *sync.Mutex {
	lock, _ := m.blobLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (m *MemoryStore) UpdateBlob(key string, fn func([]byte) ([]byte, error)) error {
	lock := m.blobLock(key)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	current := append([]byte(nil), m.blobs[key]...)
	m.mu.RUnlock()
	next, err := fn(current)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if next == nil {
		delete(m.blobs, key)
		return nil
	}
	m.blobs[key] = append([]byte(nil), next...)
	return nil
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
	// Unbind the key from its previous TokenB, if any. Other roles that share
	// the same TokenB on a multi-party connection stay indexed.
	if old, ok := m.parties[key]; ok {
		var prev struct {
			TokenB string `json:"token_b"`
		}
		if json.Unmarshal(old, &prev) == nil && prev.TokenB != "" {
			m.unbindTokenBLocked(prev.TokenB, key)
		}
	}
	m.parties[key] = state
	var p struct {
		TokenB string `json:"token_b"`
	}
	if json.Unmarshal(state, &p) == nil && p.TokenB != "" {
		set, ok := m.tokenBIndex[p.TokenB]
		if !ok {
			set = make(map[string]bool)
			m.tokenBIndex[p.TokenB] = set
		}
		set[key] = true
	}
	return nil
}

func (m *MemoryStore) GetParty(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.parties[key], nil
}

// GetPartyByTokenB returns one party whose TokenB matches. In a multi-party
// setup where several parties share a TokenB, any one of them answers the
// "is this token valid" question; callers that need party-level context must
// still disambiguate via OCPI-To-Country-Code / OCPI-To-Party-Id headers or
// URL path parameters.
func (m *MemoryStore) GetPartyByTokenB(tokenB string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set, ok := m.tokenBIndex[tokenB]
	if !ok || len(set) == 0 {
		return nil, nil
	}
	for key := range set {
		if party, ok := m.parties[key]; ok {
			return party, nil
		}
	}
	return nil, nil
}

func (m *MemoryStore) DeleteParty(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.parties[key]; ok {
		var p struct {
			TokenB string `json:"token_b"`
		}
		if json.Unmarshal(old, &p) == nil && p.TokenB != "" {
			m.unbindTokenBLocked(p.TokenB, key)
		}
	}
	delete(m.parties, key)
	return nil
}

// unbindTokenBLocked removes one party key from the TokenB index. Must be
// called with m.mu held. The TokenB entry is dropped entirely once the last
// party referencing it is gone.
func (m *MemoryStore) unbindTokenBLocked(tokenB, partyKey string) {
	set, ok := m.tokenBIndex[tokenB]
	if !ok {
		return
	}
	delete(set, partyKey)
	if len(set) == 0 {
		delete(m.tokenBIndex, tokenB)
	}
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
