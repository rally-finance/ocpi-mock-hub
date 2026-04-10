package simulation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

// Store matches the main package Store interface.
type Store interface {
	GetTokenB() (string, error)
	GetEMSPCallbackURL() (string, error)
	GetEMSPOwnToken() (string, error)
	PutSession(id string, session []byte) error
	GetSession(id string) ([]byte, error)
	ListSessions() ([][]byte, error)
	DeleteSession(id string) error
	PutCDR(id string, cdr []byte) error
	PutReservation(id string, reservation []byte) error
	ListReservations() ([][]byte, error)
	DeleteReservation(id string) error
	GetChargingProfile(sessionID string) ([]byte, error)
	GetMode() (string, error)
}

type Simulator struct {
	store            Store
	seed             *fakegen.SeedData
	emspCallbackURL string
	commandDelayMS   int
	sessionDurationS int
	authToken        string
}

func New(store Store, seed *fakegen.SeedData, emspCallback string, delayMS, durationS int) *Simulator {
	return &Simulator{
		store:           store,
		seed:            seed,
		emspCallbackURL: emspCallback,
		commandDelayMS:   delayMS,
		sessionDurationS: durationS,
	}
}

type sessionRecord struct {
	CountryCode   string  `json:"country_code"`
	PartyID       string  `json:"party_id"`
	ID            string  `json:"id"`
	StartDateTime string  `json:"start_date_time"`
	EndDateTime   *string `json:"end_date_time,omitempty"`
	KWH           float64 `json:"kwh"`
	CDRToken      struct {
		UID        string `json:"uid"`
		Type       string `json:"type"`
		ContractID string `json:"contract_id,omitempty"`
	} `json:"cdr_token"`
	AuthMethod             string `json:"auth_method"`
	AuthorizationReference string `json:"authorization_reference,omitempty"`
	LocationID             string `json:"location_id"`
	EvseUID                string `json:"evse_uid"`
	ConnectorID            string `json:"connector_id"`
	MeterID                string `json:"meter_id,omitempty"`
	Currency               string `json:"currency"`
	TotalCost              any    `json:"total_cost,omitempty"`
	Status                 string `json:"status"`
	ChargingPeriods        []any  `json:"charging_periods,omitempty"`
	LastUpdated            string `json:"last_updated"`

	ResponseURL  string `json:"_response_url,omitempty"`
	CreatedAt    string `json:"_created_at,omitempty"`
	ActivatedAt  string `json:"_activated_at,omitempty"`
	CallbackSent bool   `json:"_callback_sent,omitempty"`
}

// Tick processes all sessions, advancing their state machine.
func (s *Simulator) Tick() error {
	raw, err := s.store.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	emspURL, _ := s.store.GetEMSPCallbackURL()
	if emspURL == "" {
		emspURL = s.emspCallbackURL
	}
	emspURL = stripVersionsSuffix(emspURL)

	s.authToken, _ = s.store.GetEMSPOwnToken()

	now := time.Now().UTC()

	for _, b := range raw {
		var session sessionRecord
		if err := json.Unmarshal(b, &session); err != nil {
			log.Printf("[tick] failed to unmarshal session: %v", err)
			continue
		}

		created, _ := time.Parse(time.RFC3339, session.CreatedAt)
		pendingAge := now.Sub(created)

		switch session.Status {
		case "PENDING":
			if pendingAge > time.Duration(s.commandDelayMS)*time.Millisecond {
				s.advancePendingToActive(&session, emspURL, now)
			}
		case "ACTIVE":
			activated := created
			if session.ActivatedAt != "" {
				activated, _ = time.Parse(time.RFC3339, session.ActivatedAt)
			}
			activeAge := now.Sub(activated)
			if activeAge > time.Duration(s.sessionDurationS)*time.Second {
				s.completeSession(&session, emspURL, now)
			} else {
				s.updateActiveSession(&session, now, activeAge)
			}
		case "STOPPING":
			s.completeSession(&session, emspURL, now)
		case "COMPLETED":
			// Already done; keep in store for pull queries.
			continue
		}
	}

	s.processReservations(now)

	return nil
}

type reservationRecord struct {
	ID           string `json:"id"`
	ExpiryDate   string `json:"expiry_date"`
	Status       string `json:"status"`
	ResponseURL  string `json:"_response_url,omitempty"`
	CreatedAt    string `json:"_created_at,omitempty"`
	CallbackSent bool   `json:"_callback_sent,omitempty"`
}

func (s *Simulator) processReservations(now time.Time) {
	raw, err := s.store.ListReservations()
	if err != nil {
		log.Printf("[tick] list reservations: %v", err)
		return
	}

	for _, b := range raw {
		var res reservationRecord
		if err := json.Unmarshal(b, &res); err != nil {
			continue
		}

		if res.ResponseURL != "" && !res.CallbackSent {
			created, _ := time.Parse(time.RFC3339, res.CreatedAt)
			if now.Sub(created) > time.Duration(s.commandDelayMS)*time.Millisecond {
				callback := map[string]any{"result": "ACCEPTED"}
				s.pushToEMSP("POST", res.ResponseURL, callback)
				res.CallbackSent = true
				data, _ := json.Marshal(res)
				s.store.PutReservation(res.ID, data)
			}
		}

		if res.ExpiryDate != "" {
			expiry, err := time.Parse(time.RFC3339, res.ExpiryDate)
			if err == nil && now.After(expiry) {
				s.store.DeleteReservation(res.ID)
				log.Printf("[tick] reservation %s expired, deleted", res.ID)
			}
		}
	}
}

func (s *Simulator) advancePendingToActive(session *sessionRecord, emspURL string, now time.Time) {
	// Send command callback if we have a response_url.
	if session.ResponseURL != "" && !session.CallbackSent {
		callback := map[string]any{
			"result":     "ACCEPTED",
			"session_id": session.ID,
		}
		s.pushToEMSP("POST", session.ResponseURL, callback)
		session.CallbackSent = true
	}

	session.Status = "ACTIVE"
	session.KWH = 0.5
	session.ActivatedAt = now.Format(time.RFC3339)
	session.LastUpdated = now.Format(time.RFC3339)

	data, _ := json.Marshal(session)
	s.store.PutSession(session.ID, data)

	// Push session update to eMSP's receiver.
	if emspURL != "" {
		url := fmt.Sprintf("%s/receiver/sessions/%s/%s/%s",
			emspURL, session.CountryCode, session.PartyID, session.ID)
		s.pushToEMSP("PUT", url, session)
	}

	// Push full EVSE object with status CHARGING
	if emspURL != "" && s.seed != nil && session.LocationID != "" && session.EvseUID != "" {
		loc, evse := s.seed.EVSEByUID(session.LocationID, session.EvseUID)
		if loc != nil && evse != nil {
			evseURL := fmt.Sprintf("%s/receiver/locations/%s/%s/%s/%s",
				emspURL, loc.CountryCode, loc.PartyID, loc.ID, evse.UID)
			clone := *evse
			clone.Status = "CHARGING"
			clone.LastUpdated = now.Format(time.RFC3339)
			s.pushToEMSP("PUT", evseURL, clone)
		}
	}

	log.Printf("[tick] session %s: PENDING -> ACTIVE", session.ID)
}

func (s *Simulator) updateActiveSession(session *sessionRecord, now time.Time, age time.Duration) {
	progress := age.Seconds() / float64(s.sessionDurationS)
	if progress > 1 {
		progress = 1
	}

	maxKWH := 20.0 + rand.Float64()*40.0

	// If a charging profile exists, cap the simulated power rate
	if profile, _ := s.store.GetChargingProfile(session.ID); profile != nil {
		var cp struct {
			ChargingRateUnit    string `json:"charging_rate_unit"`
			MinChargingRate     float64 `json:"min_charging_rate"`
		}
		if json.Unmarshal(profile, &cp) == nil && cp.MinChargingRate > 0 {
			rateKW := cp.MinChargingRate / 1000.0
			if cp.ChargingRateUnit == "A" {
				rateKW = cp.MinChargingRate * 230.0 / 1000.0
			}
			profileMaxKWH := rateKW * (float64(s.sessionDurationS) / 3600.0)
			if profileMaxKWH < maxKWH {
				maxKWH = profileMaxKWH
			}
		}
	}

	session.KWH = roundTo(maxKWH*progress, 2)
	session.LastUpdated = now.Format(time.RFC3339)

	durationHours := age.Hours()
	session.ChargingPeriods = []any{
		map[string]any{
			"start_date_time": session.StartDateTime,
			"dimensions": []map[string]any{
				{"type": "ENERGY", "volume": session.KWH},
				{"type": "TIME", "volume": roundTo(durationHours, 4)},
			},
		},
	}

	data, _ := json.Marshal(session)
	s.store.PutSession(session.ID, data)
}

func (s *Simulator) completeSession(session *sessionRecord, emspURL string, now time.Time) {
	prevStatus := session.Status
	nowStr := now.Format(time.RFC3339)
	session.Status = "COMPLETED"
	session.EndDateTime = &nowStr
	session.LastUpdated = nowStr

	if session.ResponseURL != "" && !session.CallbackSent {
		callback := map[string]any{
			"result":     "ACCEPTED",
			"session_id": session.ID,
		}
		s.pushToEMSP("POST", session.ResponseURL, callback)
		session.CallbackSent = true
	}

	totalKWH := 15.0 + rand.Float64()*45.0
	session.KWH = roundTo(totalKWH, 2)

	start, _ := time.Parse(time.RFC3339, session.StartDateTime)
	durationHours := now.Sub(start).Hours()
	pricePerKWH := 0.30 + rand.Float64()*0.15
	energyCost := roundTo(totalKWH*pricePerKWH, 2)
	timeCostRate := 0.05
	timeCost := roundTo(durationHours*timeCostRate, 2)
	fixedCost := 0.50
	totalCost := roundTo(energyCost+timeCost+fixedCost, 2)
	session.TotalCost = map[string]float64{
		"excl_vat": roundTo(totalCost/1.19, 2),
		"incl_vat": totalCost,
	}

	session.ChargingPeriods = []any{
		map[string]any{
			"start_date_time": session.StartDateTime,
			"dimensions": []map[string]any{
				{"type": "ENERGY", "volume": session.KWH},
				{"type": "TIME", "volume": roundTo(durationHours, 4)},
			},
		},
	}

	data, _ := json.Marshal(session)
	s.store.PutSession(session.ID, data)

	// Generate CDR.
	cdrID := "CDR-" + uuid.NewString()[:8]
	cdr := map[string]any{
		"country_code":    session.CountryCode,
		"party_id":        session.PartyID,
		"id":              cdrID,
		"start_date_time": session.StartDateTime,
		"end_date_time":   nowStr,
		"session_id":      session.ID,
		"cdr_token":       session.CDRToken,
		"auth_method":     session.AuthMethod,
		"cdr_location": map[string]any{
			"id":                   session.LocationID,
			"evse_uid":             session.EvseUID,
			"evse_id":              session.EvseUID,
			"connector_id":         session.ConnectorID,
			"connector_standard":   "IEC_62196_T2",
			"connector_format":     "SOCKET",
			"connector_power_type": "AC_3_PHASE",
		},
		"currency":     session.Currency,
		"total_cost":   session.TotalCost,
		"total_energy": session.KWH,
		"total_time":   roundTo(durationHours, 2),
		"total_fixed_cost":   map[string]float64{"excl_vat": roundTo(fixedCost/1.19, 2), "incl_vat": fixedCost},
		"total_energy_cost":  map[string]float64{"excl_vat": roundTo(energyCost/1.19, 2), "incl_vat": energyCost},
		"total_time_cost":    map[string]float64{"excl_vat": roundTo(timeCost/1.19, 2), "incl_vat": timeCost},
		"total_parking_cost": map[string]float64{"excl_vat": 0, "incl_vat": 0},
		"total_parking_time": 0,
		"remark":             "Mock-generated CDR",
		"charging_periods": []map[string]any{
			{
				"start_date_time": session.StartDateTime,
				"dimensions": []map[string]any{
					{"type": "ENERGY", "volume": session.KWH},
					{"type": "TIME", "volume": roundTo(durationHours, 4)},
				},
			},
		},
		"last_updated": nowStr,
	}

	cdrData, _ := json.Marshal(cdr)
	s.store.PutCDR(cdrID, cdrData)

	// Push session final update and CDR to eMSP.
	if emspURL != "" {
		sessionURL := fmt.Sprintf("%s/receiver/sessions/%s/%s/%s",
			emspURL, session.CountryCode, session.PartyID, session.ID)
		s.pushToEMSP("PUT", sessionURL, session)

		cdrURL := fmt.Sprintf("%s/receiver/cdrs/%s", emspURL, cdrID)
		s.pushToEMSP("POST", cdrURL, cdr)
	}

	// Push full EVSE object back to AVAILABLE only if no other active sessions use this EVSE
	if emspURL != "" && s.seed != nil && session.LocationID != "" && session.EvseUID != "" {
		if !s.hasOtherActiveSession(session.EvseUID, session.ID) {
			loc, evse := s.seed.EVSEByUID(session.LocationID, session.EvseUID)
			if loc != nil && evse != nil {
				evseURL := fmt.Sprintf("%s/receiver/locations/%s/%s/%s/%s",
					emspURL, loc.CountryCode, loc.PartyID, loc.ID, evse.UID)
				clone := *evse
				clone.Status = "AVAILABLE"
				clone.LastUpdated = nowStr
				s.pushToEMSP("PUT", evseURL, clone)
			}
		}
	}

	log.Printf("[tick] session %s: %s -> COMPLETED (CDR %s, %.1f kWh)", session.ID, prevStatus, cdrID, session.KWH)
}

// hasOtherActiveSession checks whether any session other than excludeID
// references the given EVSE UID and is still ACTIVE or PENDING.
func (s *Simulator) hasOtherActiveSession(evseUID, excludeID string) bool {
	raw, err := s.store.ListSessions()
	if err != nil {
		return false
	}
	for _, b := range raw {
		var sess struct {
			ID      string `json:"id"`
			EvseUID string `json:"evse_uid"`
			Status  string `json:"status"`
		}
		if json.Unmarshal(b, &sess) == nil &&
			sess.ID != excludeID &&
			sess.EvseUID == evseUID &&
			(sess.Status == "ACTIVE" || sess.Status == "PENDING") {
			return true
		}
	}
	return false
}

func (s *Simulator) pushToEMSP(method, url string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[push] marshal error: %v", err)
		return
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[push] request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if s.authToken != "" {
		req.Header.Set("Authorization", "Token "+s.authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[push] %s -> error: %v", url, err)
		return
	}
	resp.Body.Close()
	log.Printf("[push] %s -> %d", url, resp.StatusCode)
}

// stripVersionsSuffix removes a trailing "/versions" or "/ocpi/versions" path
// from a URL so it can be used as a base for receiver endpoint construction.
// The OCPI credentials URL field points at the versions endpoint, but pushes
// target /receiver/locations/... etc. relative to the OCPI base path.
func stripVersionsSuffix(u string) string {
	u = strings.TrimRight(u, "/")
	if strings.HasSuffix(u, "/versions") {
		return strings.TrimSuffix(u, "/versions")
	}
	return u
}

func roundTo(val float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int(val*factor+0.5)) / factor
}
