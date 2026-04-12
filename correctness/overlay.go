package correctness

import (
	"encoding/json"
	"strings"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

type overlayState struct {
	TokenB           string            `json:"token_b,omitempty"`
	CallbackURL      string            `json:"callback_url,omitempty"`
	Credentials      []byte            `json:"credentials,omitempty"`
	EMSPToken        string            `json:"emsp_token,omitempty"`
	VersionsURL      string            `json:"versions_url,omitempty"`
	Mode             string            `json:"mode,omitempty"`
	Parties          map[string][]byte `json:"parties,omitempty"`
	TokenBIndex      map[string]string `json:"token_b_index,omitempty"`
	Tokens           map[string][]byte `json:"tokens,omitempty"`
	Sessions         map[string][]byte `json:"sessions,omitempty"`
	CDRs             map[string][]byte `json:"cdrs,omitempty"`
	Reservations     map[string][]byte `json:"reservations,omitempty"`
	ChargingProfiles map[string][]byte `json:"charging_profiles,omitempty"`
}

type OverlayStore struct {
	stateStore StateStore
	key        string
}

func NewOverlayStore(stateStore StateStore, sessionID string) *OverlayStore {
	if stateStore == nil {
		stateStore = newMemoryStateStore()
	}
	return &OverlayStore{
		stateStore: stateStore,
		key:        overlayBlobKey(sessionID),
	}
}

func overlayBlobKey(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = "default"
	}
	return overlayStateBlobKeyPrefix + sessionID
}

func defaultOverlayState() overlayState {
	return overlayState{
		Parties:          make(map[string][]byte),
		TokenBIndex:      make(map[string]string),
		Tokens:           make(map[string][]byte),
		Sessions:         make(map[string][]byte),
		CDRs:             make(map[string][]byte),
		Reservations:     make(map[string][]byte),
		ChargingProfiles: make(map[string][]byte),
		Mode:             "happy",
	}
}

func decodeOverlayState(raw []byte) (overlayState, error) {
	state := defaultOverlayState()
	if len(raw) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return overlayState{}, err
	}
	if state.Parties == nil {
		state.Parties = make(map[string][]byte)
	}
	if state.TokenBIndex == nil {
		state.TokenBIndex = make(map[string]string)
	}
	if state.Tokens == nil {
		state.Tokens = make(map[string][]byte)
	}
	if state.Sessions == nil {
		state.Sessions = make(map[string][]byte)
	}
	if state.CDRs == nil {
		state.CDRs = make(map[string][]byte)
	}
	if state.Reservations == nil {
		state.Reservations = make(map[string][]byte)
	}
	if state.ChargingProfiles == nil {
		state.ChargingProfiles = make(map[string][]byte)
	}
	if state.Mode == "" {
		state.Mode = "happy"
	}
	return state, nil
}

func encodeOverlayState(state overlayState) ([]byte, error) {
	if state.Mode == "" {
		state.Mode = "happy"
	}
	if state.Parties == nil {
		state.Parties = make(map[string][]byte)
	}
	if state.TokenBIndex == nil {
		state.TokenBIndex = make(map[string]string)
	}
	if state.Tokens == nil {
		state.Tokens = make(map[string][]byte)
	}
	if state.Sessions == nil {
		state.Sessions = make(map[string][]byte)
	}
	if state.CDRs == nil {
		state.CDRs = make(map[string][]byte)
	}
	if state.Reservations == nil {
		state.Reservations = make(map[string][]byte)
	}
	if state.ChargingProfiles == nil {
		state.ChargingProfiles = make(map[string][]byte)
	}
	return json.Marshal(state)
}

func (o *OverlayStore) getState() (overlayState, error) {
	raw, err := o.stateStore.GetBlob(o.key)
	if err != nil {
		return overlayState{}, err
	}
	return decodeOverlayState(raw)
}

func (o *OverlayStore) updateState(fn func(*overlayState) error) error {
	return o.stateStore.UpdateBlob(o.key, func(raw []byte) ([]byte, error) {
		state, err := decodeOverlayState(raw)
		if err != nil {
			return nil, err
		}
		if err := fn(&state); err != nil {
			return nil, err
		}
		return encodeOverlayState(state)
	})
}

func copyBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	return append([]byte(nil), src...)
}

func (o *OverlayStore) GetTokenB() (string, error) {
	state, err := o.getState()
	if err != nil {
		return "", err
	}
	return state.TokenB, nil
}

func (o *OverlayStore) SetTokenB(token string) error {
	return o.updateState(func(state *overlayState) error {
		state.TokenB = token
		return nil
	})
}

func (o *OverlayStore) GetEMSPCallbackURL() (string, error) {
	state, err := o.getState()
	if err != nil {
		return "", err
	}
	return state.CallbackURL, nil
}

func (o *OverlayStore) SetEMSPCallbackURL(url string) error {
	return o.updateState(func(state *overlayState) error {
		state.CallbackURL = url
		return nil
	})
}

func (o *OverlayStore) GetEMSPCredentials() ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyBytes(state.Credentials), nil
}

func (o *OverlayStore) SetEMSPCredentials(creds []byte) error {
	return o.updateState(func(state *overlayState) error {
		state.Credentials = copyBytes(creds)
		return nil
	})
}

func (o *OverlayStore) GetEMSPOwnToken() (string, error) {
	state, err := o.getState()
	if err != nil {
		return "", err
	}
	return state.EMSPToken, nil
}

func (o *OverlayStore) SetEMSPOwnToken(token string) error {
	return o.updateState(func(state *overlayState) error {
		state.EMSPToken = token
		return nil
	})
}

func (o *OverlayStore) GetEMSPVersionsURL() (string, error) {
	state, err := o.getState()
	if err != nil {
		return "", err
	}
	return state.VersionsURL, nil
}

func (o *OverlayStore) SetEMSPVersionsURL(url string) error {
	return o.updateState(func(state *overlayState) error {
		state.VersionsURL = url
		return nil
	})
}

func (o *OverlayStore) PutParty(key string, payload []byte) error {
	return o.updateState(func(state *overlayState) error {
		if prev, ok := state.Parties[key]; ok {
			var old struct {
				TokenB string `json:"token_b"`
			}
			if json.Unmarshal(prev, &old) == nil && old.TokenB != "" {
				delete(state.TokenBIndex, old.TokenB)
			}
		}
		state.Parties[key] = copyBytes(payload)
		var next struct {
			TokenB string `json:"token_b"`
		}
		if json.Unmarshal(payload, &next) == nil && next.TokenB != "" {
			state.TokenBIndex[next.TokenB] = key
		}
		return nil
	})
}

func (o *OverlayStore) GetParty(key string) ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyBytes(state.Parties[key]), nil
}

func (o *OverlayStore) GetPartyByTokenB(tokenB string) ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	key, ok := state.TokenBIndex[tokenB]
	if !ok {
		return nil, nil
	}
	return copyBytes(state.Parties[key]), nil
}

func (o *OverlayStore) DeleteParty(key string) error {
	return o.updateState(func(state *overlayState) error {
		if prev, ok := state.Parties[key]; ok {
			var old struct {
				TokenB string `json:"token_b"`
			}
			if json.Unmarshal(prev, &old) == nil && old.TokenB != "" {
				delete(state.TokenBIndex, old.TokenB)
			}
		}
		delete(state.Parties, key)
		return nil
	})
}

func (o *OverlayStore) ListParties() ([][]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyMapValues(state.Parties), nil
}

func (o *OverlayStore) PutToken(cc, pid, uid string, token []byte) error {
	return o.updateState(func(state *overlayState) error {
		state.Tokens[tokenKey(cc, pid, uid)] = copyBytes(token)
		return nil
	})
}

func (o *OverlayStore) GetToken(cc, pid, uid string) ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyBytes(state.Tokens[tokenKey(cc, pid, uid)]), nil
}

func (o *OverlayStore) ListTokens() ([][]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyMapValues(state.Tokens), nil
}

func (o *OverlayStore) PutSession(id string, session []byte) error {
	return o.updateState(func(state *overlayState) error {
		state.Sessions[id] = copyBytes(session)
		return nil
	})
}

func (o *OverlayStore) GetSession(id string) ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyBytes(state.Sessions[id]), nil
}

func (o *OverlayStore) ListSessions() ([][]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyMapValues(state.Sessions), nil
}

func (o *OverlayStore) DeleteSession(id string) error {
	return o.updateState(func(state *overlayState) error {
		delete(state.Sessions, id)
		return nil
	})
}

func (o *OverlayStore) PutCDR(id string, cdr []byte) error {
	return o.updateState(func(state *overlayState) error {
		state.CDRs[id] = copyBytes(cdr)
		return nil
	})
}

func (o *OverlayStore) GetCDR(id string) ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyBytes(state.CDRs[id]), nil
}

func (o *OverlayStore) ListCDRs() ([][]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyMapValues(state.CDRs), nil
}

func (o *OverlayStore) PutReservation(id string, reservation []byte) error {
	return o.updateState(func(state *overlayState) error {
		state.Reservations[id] = copyBytes(reservation)
		return nil
	})
}

func (o *OverlayStore) GetReservation(id string) ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyBytes(state.Reservations[id]), nil
}

func (o *OverlayStore) ListReservations() ([][]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyMapValues(state.Reservations), nil
}

func (o *OverlayStore) DeleteReservation(id string) error {
	return o.updateState(func(state *overlayState) error {
		delete(state.Reservations, id)
		return nil
	})
}

func (o *OverlayStore) PutChargingProfile(sessionID string, profile []byte) error {
	return o.updateState(func(state *overlayState) error {
		state.ChargingProfiles[sessionID] = copyBytes(profile)
		return nil
	})
}

func (o *OverlayStore) GetChargingProfile(sessionID string) ([]byte, error) {
	state, err := o.getState()
	if err != nil {
		return nil, err
	}
	return copyBytes(state.ChargingProfiles[sessionID]), nil
}

func (o *OverlayStore) DeleteChargingProfile(sessionID string) error {
	return o.updateState(func(state *overlayState) error {
		delete(state.ChargingProfiles, sessionID)
		return nil
	})
}

func (o *OverlayStore) GetMode() (string, error) {
	state, err := o.getState()
	if err != nil {
		return "", err
	}
	return state.Mode, nil
}

func (o *OverlayStore) SetMode(mode string) error {
	return o.updateState(func(state *overlayState) error {
		state.Mode = mode
		return nil
	})
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
	return newSessionSandbox(base, nil, "")
}

func newSessionSandbox(base *fakegen.SeedData, stateStore StateStore, sessionID string) *Sandbox {
	return &Sandbox{
		Seed:  CloneSeed(base),
		Store: NewOverlayStore(stateStore, sessionID),
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
