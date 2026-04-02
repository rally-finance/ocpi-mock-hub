package hub

import "sync"

type MemoryStore struct {
	mu               sync.RWMutex
	tokenB           string
	emspCallbackURL string
	emspCreds       []byte
	emspOwnToken    string
	emspVersionsURL string
	tokens           map[string][]byte // key: "cc/pid/uid"
	sessions         map[string][]byte // key: session ID
	cdrs             map[string][]byte // key: CDR ID
	mode             string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tokens:   make(map[string][]byte),
		sessions: make(map[string][]byte),
		cdrs:     make(map[string][]byte),
		mode:     "happy",
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

func (m *MemoryStore) ListCDRs() ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([][]byte, 0, len(m.cdrs))
	for _, v := range m.cdrs {
		result = append(result, v)
	}
	return result, nil
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
