package fakegen

type Connector struct {
	ID              string   `json:"id"`
	Standard        string   `json:"standard"`
	Format          string   `json:"format"`
	PowerType       string   `json:"power_type"`
	MaxVoltage      int      `json:"max_voltage"`
	MaxAmperage     int      `json:"max_amperage"`
	MaxElectricPower int     `json:"max_electric_power"`
	TariffIDs       []string `json:"tariff_ids,omitempty"`
	LastUpdated     string   `json:"last_updated"`
}

type EVSE struct {
	UID          string      `json:"uid"`
	EvseID       string      `json:"evse_id"`
	Status       string      `json:"status"`
	Capabilities []string    `json:"capabilities"`
	Connectors   []Connector `json:"connectors"`
	Coordinates  *Coords     `json:"coordinates,omitempty"`
	LastUpdated  string      `json:"last_updated"`
}

type Coords struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

type Operator struct {
	Name string `json:"name"`
}

type Location struct {
	CountryCode        string    `json:"country_code"`
	PartyID            string    `json:"party_id"`
	ID                 string    `json:"id"`
	Publish            bool      `json:"publish"`
	Name               string    `json:"name"`
	Address            string    `json:"address"`
	City               string    `json:"city"`
	PostalCode         string    `json:"postal_code"`
	Country            string    `json:"country"`
	Coordinates        Coords    `json:"coordinates"`
	TimeZone           string    `json:"time_zone"`
	ParkingType        string    `json:"parking_type"`
	Operator           *Operator `json:"operator,omitempty"`
	Facilities         []string  `json:"facilities,omitempty"`
	OpeningTimes       any       `json:"opening_times,omitempty"`
	ChargingWhenClosed bool      `json:"charging_when_closed"`
	EnergyMix          any       `json:"energy_mix,omitempty"`
	EVSEs              []EVSE    `json:"evses"`
	LastUpdated        string    `json:"last_updated"`
}

type PriceComponent struct {
	Type     string  `json:"type"`
	Price    float64 `json:"price"`
	VAT      float64 `json:"vat,omitempty"`
	StepSize int     `json:"step_size"`
}

// TariffRestrictions mirrors OCPI 2.2.1 §11.4.3.9. Numeric thresholds use
// pointers so handlers and tests can distinguish "unset" from a legitimate
// zero — omitempty alone would erase a meaningful zero.
type TariffRestrictions struct {
	StartTime   string   `json:"start_time,omitempty"`
	EndTime     string   `json:"end_time,omitempty"`
	StartDate   string   `json:"start_date,omitempty"`
	EndDate     string   `json:"end_date,omitempty"`
	MinKwh      *float64 `json:"min_kwh,omitempty"`
	MaxKwh      *float64 `json:"max_kwh,omitempty"`
	MinCurrent  *float64 `json:"min_current,omitempty"`
	MaxCurrent  *float64 `json:"max_current,omitempty"`
	MinPower    *float64 `json:"min_power,omitempty"`
	MaxPower    *float64 `json:"max_power,omitempty"`
	MinDuration *int     `json:"min_duration,omitempty"`
	MaxDuration *int     `json:"max_duration,omitempty"`
	DayOfWeek   []string `json:"day_of_week,omitempty"`
	Reservation string   `json:"reservation,omitempty"`
}

type TariffElement struct {
	PriceComponents []PriceComponent    `json:"price_components"`
	Restrictions    *TariffRestrictions `json:"restrictions,omitempty"`
}

type DisplayText struct {
	Language string `json:"language"`
	Text     string `json:"text"`
}

type Price struct {
	ExclVat float64 `json:"excl_vat"`
	InclVat float64 `json:"incl_vat"`
}

// Tariff mirrors OCPI 2.2.1 §11.4.3.7.
type Tariff struct {
	CountryCode   string          `json:"country_code"`
	PartyID       string          `json:"party_id"`
	ID            string          `json:"id"`
	Currency      string          `json:"currency"`
	Type          string          `json:"type,omitempty"`
	TariffAltText []DisplayText   `json:"tariff_alt_text,omitempty"`
	TariffAltURL  string          `json:"tariff_alt_url,omitempty"`
	MinPrice      *Price          `json:"min_price,omitempty"`
	MaxPrice      *Price          `json:"max_price,omitempty"`
	Elements      []TariffElement `json:"elements"`
	StartDateTime string          `json:"start_date_time,omitempty"`
	EndDateTime   string          `json:"end_date_time,omitempty"`
	EnergyMix     any             `json:"energy_mix,omitempty"`
	LastUpdated   string          `json:"last_updated"`
}

type HubClientInfo struct {
	CountryCode     string `json:"country_code"`
	PartyID         string `json:"party_id"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	BusinessDetails struct {
		Name string `json:"name"`
	} `json:"business_details"`
	LastUpdated string `json:"last_updated"`
}

type CPO struct {
	CountryCode string
	PartyID     string
	Name        string
}

type SeedData struct {
	CPOs          []CPO
	Locations     []Location
	Tariffs       []Tariff
	HubClientInfo []HubClientInfo
}
