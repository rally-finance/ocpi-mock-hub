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
	CountryCode string    `json:"country_code"`
	PartyID     string    `json:"party_id"`
	ID          string    `json:"id"`
	Publish     bool      `json:"publish"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	City        string    `json:"city"`
	PostalCode  string    `json:"postal_code"`
	Country     string    `json:"country"`
	Coordinates Coords    `json:"coordinates"`
	TimeZone    string    `json:"time_zone"`
	ParkingType string    `json:"parking_type"`
	Operator    *Operator `json:"operator,omitempty"`
	EVSEs       []EVSE    `json:"evses"`
	LastUpdated string    `json:"last_updated"`
}

type PriceComponent struct {
	Type     string  `json:"type"`
	Price    float64 `json:"price"`
	VAT      float64 `json:"vat,omitempty"`
	StepSize int     `json:"step_size"`
}

type TariffElement struct {
	PriceComponents []PriceComponent `json:"price_components"`
}

type Tariff struct {
	CountryCode string          `json:"country_code"`
	PartyID     string          `json:"party_id"`
	ID          string          `json:"id"`
	Currency    string          `json:"currency"`
	Type        string          `json:"type,omitempty"`
	Elements    []TariffElement `json:"elements"`
	LastUpdated string          `json:"last_updated"`
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
