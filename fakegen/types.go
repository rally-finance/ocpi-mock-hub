package fakegen

// Image mirrors OCPI 2.2.1 §16.15 — used on Location, EVSE, and BusinessDetails.logo.
type Image struct {
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Category  string `json:"category"`
	Type      string `json:"type"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

// BusinessDetails mirrors OCPI 2.2.1 §8.4.3 — used for operator, suboperator,
// owner, and HubClientInfo. Operator is kept as an alias so existing callers
// that reference &fakegen.Operator{...} continue to compile.
type BusinessDetails struct {
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
	Logo    *Image `json:"logo,omitempty"`
}

// Operator is the historical name for BusinessDetails. Preserved as a type
// alias so callers outside this package don't need updates.
type Operator = BusinessDetails

// StatusSchedule mirrors OCPI 2.2.1 §8.4.24 — a planned EVSE status change.
type StatusSchedule struct {
	PeriodBegin string `json:"period_begin"`
	PeriodEnd   string `json:"period_end,omitempty"`
	Status      string `json:"status"`
}

// AdditionalGeoLocation mirrors OCPI 2.2.1 §8.4.1. Used on Location.related_locations
// to advertise supplementary coordinates (alternate entrance, accessible route, etc.).
type AdditionalGeoLocation struct {
	Latitude  string        `json:"latitude"`
	Longitude string        `json:"longitude"`
	Name      []DisplayText `json:"name,omitempty"`
}

// PublishTokenType mirrors OCPI 2.2.1 §8.4.20. Used on Location.publish_allowed_to
// when publish is false to restrict which tokens may see the location.
type PublishTokenType struct {
	UID          string `json:"uid,omitempty"`
	Type         string `json:"type,omitempty"`
	VisualNumber string `json:"visual_number,omitempty"`
	Issuer       string `json:"issuer,omitempty"`
	GroupID      string `json:"group_id,omitempty"`
}

type Connector struct {
	ID                 string   `json:"id"`
	Standard           string   `json:"standard"`
	Format             string   `json:"format"`
	PowerType          string   `json:"power_type"`
	MaxVoltage         int      `json:"max_voltage"`
	MaxAmperage        int      `json:"max_amperage"`
	MaxElectricPower   int      `json:"max_electric_power"`
	TariffIDs          []string `json:"tariff_ids,omitempty"`
	TermsAndConditions string   `json:"terms_and_conditions,omitempty"`
	LastUpdated        string   `json:"last_updated"`
}

type EVSE struct {
	UID                 string           `json:"uid"`
	EvseID              string           `json:"evse_id"`
	Status              string           `json:"status"`
	StatusSchedule      []StatusSchedule `json:"status_schedule,omitempty"`
	Capabilities        []string         `json:"capabilities"`
	Connectors          []Connector      `json:"connectors"`
	FloorLevel          string           `json:"floor_level,omitempty"`
	Coordinates         *Coords          `json:"coordinates,omitempty"`
	PhysicalReference   string           `json:"physical_reference,omitempty"`
	Directions          []DisplayText    `json:"directions,omitempty"`
	ParkingRestrictions []string         `json:"parking_restrictions,omitempty"`
	Images              []Image          `json:"images,omitempty"`
	LastUpdated         string           `json:"last_updated"`
}

type Coords struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

type Location struct {
	CountryCode        string                  `json:"country_code"`
	PartyID            string                  `json:"party_id"`
	ID                 string                  `json:"id"`
	Publish            bool                    `json:"publish"`
	PublishAllowedTo   []PublishTokenType      `json:"publish_allowed_to,omitempty"`
	Name               string                  `json:"name"`
	Address            string                  `json:"address"`
	City               string                  `json:"city"`
	PostalCode         string                  `json:"postal_code"`
	State              string                  `json:"state,omitempty"`
	Country            string                  `json:"country"`
	Coordinates        Coords                  `json:"coordinates"`
	RelatedLocations   []AdditionalGeoLocation `json:"related_locations,omitempty"`
	ParkingType        string                  `json:"parking_type"`
	EVSEs              []EVSE                  `json:"evses"`
	Directions         []DisplayText           `json:"directions,omitempty"`
	Operator           *BusinessDetails        `json:"operator,omitempty"`
	SubOperator        *BusinessDetails        `json:"suboperator,omitempty"`
	Owner              *BusinessDetails        `json:"owner,omitempty"`
	Facilities         []string                `json:"facilities,omitempty"`
	TimeZone           string                  `json:"time_zone"`
	OpeningTimes       any                     `json:"opening_times,omitempty"`
	ChargingWhenClosed bool                    `json:"charging_when_closed"`
	Images             []Image                 `json:"images,omitempty"`
	EnergyMix          any                     `json:"energy_mix,omitempty"`
	LastUpdated        string                  `json:"last_updated"`
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
	CountryCode     string          `json:"country_code"`
	PartyID         string          `json:"party_id"`
	Role            string          `json:"role"`
	Status          string          `json:"status"`
	BusinessDetails BusinessDetails `json:"business_details"`
	LastUpdated     string          `json:"last_updated"`
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
