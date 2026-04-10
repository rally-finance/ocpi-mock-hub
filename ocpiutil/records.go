package ocpiutil

// SessionRecord is the JSON stored in KV for a session.
// Shared between handlers and simulation packages.
type SessionRecord struct {
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

	// Internal fields for tick processing (not part of OCPI spec).
	ResponseURL string `json:"_response_url,omitempty"`
	CreatedAt   string `json:"_created_at,omitempty"`
	ActivatedAt string `json:"_activated_at,omitempty"`
	// CallbackSent tracks whether the async command-result callback has been
	// posted to ResponseURL.  It is set to true after the PENDING→ACTIVE
	// callback and reset to false by STOP_SESSION when a new ResponseURL is
	// provided, so the STOPPING→COMPLETED callback fires separately.
	CallbackSent bool `json:"_callback_sent,omitempty"`
}
