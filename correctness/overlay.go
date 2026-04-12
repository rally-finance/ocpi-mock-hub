package correctness

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

type OverlayStore struct {
	mu               sync.RWMutex
	tokenB           string
	callbackURL      string
	credentials      []byte
	emspToken        string
	versionsURL      string
	mode             string
	parties          map[string][]byte
	tokenBIndex      map[string]string
	tokens           map[string][]byte
	sessions         map[string][]byte
	cdrs             map[string][]byte
	reservations     map[string][]byte
	chargingProfiles map[string][]byte
}

func NewOverlayStore() *OverlayStore {
	return &OverlayStore{
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

func (o *OverlayStore) GetTokenB() (string, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.tokenB, nil
}

func (o *OverlayStore) SetTokenB(token string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.tokenB = token
	return nil
}

func (o *OverlayStore) GetEMSPCallbackURL() (string, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.callbackURL, nil
}

func (o *OverlayStore) SetEMSPCallbackURL(url string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.callbackURL = url
	return nil
}

func (o *OverlayStore) GetEMSPCredentials() ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return append([]byte(nil), o.credentials...), nil
}

func (o *OverlayStore) SetEMSPCredentials(creds []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.credentials = append([]byte(nil), creds...)
	return nil
}

func (o *OverlayStore) GetEMSPOwnToken() (string, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.emspToken, nil
}

func (o *OverlayStore) SetEMSPOwnToken(token string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.emspToken = token
	return nil
}

func (o *OverlayStore) GetEMSPVersionsURL() (string, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.versionsURL, nil
}

func (o *OverlayStore) SetEMSPVersionsURL(url string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.versionsURL = url
	return nil
}

func (o *OverlayStore) PutParty(key string, state []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if prev, ok := o.parties[key]; ok {
		var old struct {
			TokenB string `json:"token_b"`
		}
		if json.Unmarshal(prev, &old) == nil && old.TokenB != "" {
			delete(o.tokenBIndex, old.TokenB)
		}
	}
	o.parties[key] = append([]byte(nil), state...)
	var next struct {
		TokenB string `json:"token_b"`
	}
	if json.Unmarshal(state, &next) == nil && next.TokenB != "" {
		o.tokenBIndex[next.TokenB] = key
	}
	return nil
}

func (o *OverlayStore) GetParty(key string) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	val := o.parties[key]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (o *OverlayStore) GetPartyByTokenB(tokenB string) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	key, ok := o.tokenBIndex[tokenB]
	if !ok {
		return nil, nil
	}
	val := o.parties[key]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (o *OverlayStore) DeleteParty(key string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if prev, ok := o.parties[key]; ok {
		var old struct {
			TokenB string `json:"token_b"`
		}
		if json.Unmarshal(prev, &old) == nil && old.TokenB != "" {
			delete(o.tokenBIndex, old.TokenB)
		}
	}
	delete(o.parties, key)
	return nil
}

func (o *OverlayStore) ListParties() ([][]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return copyMapValues(o.parties), nil
}

func (o *OverlayStore) PutToken(cc, pid, uid string, token []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.tokens[tokenKey(cc, pid, uid)] = append([]byte(nil), token...)
	return nil
}

func (o *OverlayStore) GetToken(cc, pid, uid string) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	val := o.tokens[tokenKey(cc, pid, uid)]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (o *OverlayStore) ListTokens() ([][]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return copyMapValues(o.tokens), nil
}

func (o *OverlayStore) PutSession(id string, session []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sessions[id] = append([]byte(nil), session...)
	return nil
}

func (o *OverlayStore) GetSession(id string) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	val := o.sessions[id]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (o *OverlayStore) ListSessions() ([][]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return copyMapValues(o.sessions), nil
}

func (o *OverlayStore) DeleteSession(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.sessions, id)
	return nil
}

func (o *OverlayStore) PutCDR(id string, cdr []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.cdrs[id] = append([]byte(nil), cdr...)
	return nil
}

func (o *OverlayStore) GetCDR(id string) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	val := o.cdrs[id]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (o *OverlayStore) ListCDRs() ([][]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return copyMapValues(o.cdrs), nil
}

func (o *OverlayStore) PutReservation(id string, reservation []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reservations[id] = append([]byte(nil), reservation...)
	return nil
}

func (o *OverlayStore) GetReservation(id string) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	val := o.reservations[id]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (o *OverlayStore) ListReservations() ([][]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return copyMapValues(o.reservations), nil
}

func (o *OverlayStore) DeleteReservation(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.reservations, id)
	return nil
}

func (o *OverlayStore) PutChargingProfile(sessionID string, profile []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.chargingProfiles[sessionID] = append([]byte(nil), profile...)
	return nil
}

func (o *OverlayStore) GetChargingProfile(sessionID string) ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	val := o.chargingProfiles[sessionID]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (o *OverlayStore) DeleteChargingProfile(sessionID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.chargingProfiles, sessionID)
	return nil
}

func (o *OverlayStore) GetMode() (string, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.mode, nil
}

func (o *OverlayStore) SetMode(mode string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.mode = mode
	return nil
}

func copyMapValues(src map[string][]byte) [][]byte {
	out := make([][]byte, 0, len(src))
	for _, value := range src {
		out = append(out, append([]byte(nil), value...))
	}
	return out
}

func tokenKey(cc, pid, uid string) string {
	return strings.ToUpper(cc) + "/" + strings.ToUpper(pid) + "/" + uid
}

type Sandbox struct {
	Seed  *fakegen.SeedData
	Store *OverlayStore
}

func NewSandbox(base *fakegen.SeedData) *Sandbox {
	return &Sandbox{
		Seed:  CloneSeed(base),
		Store: NewOverlayStore(),
	}
}

func CloneSeed(base *fakegen.SeedData) *fakegen.SeedData {
	if base == nil {
		return &fakegen.SeedData{}
	}
	out := &fakegen.SeedData{
		CPOs:          append([]fakegen.CPO(nil), base.CPOs...),
		Locations:     make([]fakegen.Location, len(base.Locations)),
		Tariffs:       make([]fakegen.Tariff, len(base.Tariffs)),
		HubClientInfo: make([]fakegen.HubClientInfo, len(base.HubClientInfo)),
	}

	copy(out.HubClientInfo, base.HubClientInfo)

	for i, loc := range base.Locations {
		cloned := loc
		if loc.Operator != nil {
			operator := *loc.Operator
			cloned.Operator = &operator
		}
		cloned.Facilities = append([]string(nil), loc.Facilities...)
		cloned.EVSEs = make([]fakegen.EVSE, len(loc.EVSEs))
		for j, evse := range loc.EVSEs {
			evseClone := evse
			evseClone.Capabilities = append([]string(nil), evse.Capabilities...)
			evseClone.Connectors = append([]fakegen.Connector(nil), evse.Connectors...)
			if evse.Coordinates != nil {
				coords := *evse.Coordinates
				evseClone.Coordinates = &coords
			}
			cloned.EVSEs[j] = evseClone
		}
		out.Locations[i] = cloned
	}

	for i, tariff := range base.Tariffs {
		cloned := tariff
		cloned.TariffAltText = append([]fakegen.DisplayText(nil), tariff.TariffAltText...)
		if tariff.MinPrice != nil {
			price := *tariff.MinPrice
			cloned.MinPrice = &price
		}
		if tariff.MaxPrice != nil {
			price := *tariff.MaxPrice
			cloned.MaxPrice = &price
		}
		cloned.Elements = make([]fakegen.TariffElement, len(tariff.Elements))
		for j, elem := range tariff.Elements {
			elemClone := elem
			elemClone.PriceComponents = append([]fakegen.PriceComponent(nil), elem.PriceComponents...)
			if elem.Restrictions != nil {
				restrictions := *elem.Restrictions
				elemClone.Restrictions = &restrictions
			}
			cloned.Elements[j] = elemClone
		}
		out.Tariffs[i] = cloned
	}

	return out
}
