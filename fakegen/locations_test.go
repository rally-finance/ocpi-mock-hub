package fakegen

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGenerateLocations_CoversOptionalFields asserts that the seed corpus
// exercises the OCPI 2.2.1 optional Location/EVSE/Connector fields added in
// Stage 6. Field presence needs only to be demonstrated by at least one
// object — every location does not need to carry every field.
func TestGenerateLocations_CoversOptionalFields(t *testing.T) {
	seed := GenerateSeed(50)
	if len(seed.Locations) == 0 {
		t.Fatal("expected at least one seeded location")
	}

	var (
		sawState               bool
		sawRelatedLocations    bool
		sawLocDirections       bool
		sawSubOperator         bool
		sawOwner               bool
		sawOperatorWebsite     bool
		sawImages              bool
		sawOwnerLogo           bool
		sawEVSEStatusSchedule  bool
		sawFloorLevel          bool
		sawPhysicalReference   bool
		sawEVSEDirections      bool
		sawParkingRestrictions bool
		sawEVSEImages          bool
		sawConnectorTerms      bool
	)

	for _, loc := range seed.Locations {
		if loc.State != "" {
			sawState = true
		}
		if len(loc.RelatedLocations) > 0 {
			sawRelatedLocations = true
		}
		if len(loc.Directions) > 0 {
			sawLocDirections = true
		}
		if loc.SubOperator != nil && loc.SubOperator.Name != "" {
			sawSubOperator = true
		}
		if loc.Owner != nil && loc.Owner.Name != "" {
			sawOwner = true
			if loc.Owner.Logo != nil && loc.Owner.Logo.URL != "" {
				sawOwnerLogo = true
			}
		}
		if loc.Operator != nil && loc.Operator.Website != "" {
			sawOperatorWebsite = true
		}
		if len(loc.Images) > 0 {
			sawImages = true
		}
		for _, evse := range loc.EVSEs {
			if len(evse.StatusSchedule) > 0 {
				sawEVSEStatusSchedule = true
			}
			if evse.FloorLevel != "" {
				sawFloorLevel = true
			}
			if evse.PhysicalReference != "" {
				sawPhysicalReference = true
			}
			if len(evse.Directions) > 0 {
				sawEVSEDirections = true
			}
			if len(evse.ParkingRestrictions) > 0 {
				sawParkingRestrictions = true
			}
			if len(evse.Images) > 0 {
				sawEVSEImages = true
			}
			for _, c := range evse.Connectors {
				if c.TermsAndConditions != "" {
					sawConnectorTerms = true
				}
			}
		}
	}

	cases := []struct {
		name string
		ok   bool
	}{
		{"Location.state", sawState},
		{"Location.related_locations", sawRelatedLocations},
		{"Location.directions", sawLocDirections},
		{"Location.suboperator", sawSubOperator},
		{"Location.owner", sawOwner},
		{"Location.owner.logo", sawOwnerLogo},
		{"Location.operator.website", sawOperatorWebsite},
		{"Location.images", sawImages},
		{"EVSE.status_schedule", sawEVSEStatusSchedule},
		{"EVSE.floor_level", sawFloorLevel},
		{"EVSE.physical_reference", sawPhysicalReference},
		{"EVSE.directions", sawEVSEDirections},
		{"EVSE.parking_restrictions", sawParkingRestrictions},
		{"EVSE.images", sawEVSEImages},
		{"Connector.terms_and_conditions", sawConnectorTerms},
	}
	for _, c := range cases {
		if !c.ok {
			t.Errorf("seed corpus never exercised %s", c.name)
		}
	}
}

func TestLocation_JSONRoundTripsNewFields(t *testing.T) {
	loc := Location{
		CountryCode: "DE",
		PartyID:     "AAA",
		ID:          "LOC-1",
		Publish:     true,
		Name:        "Test",
		Address:     "Hauptstraße 1",
		City:        "Berlin",
		PostalCode:  "10115",
		State:       "Berlin",
		Country:     "DEU",
		Coordinates: Coords{Latitude: "52.520", Longitude: "13.405"},
		RelatedLocations: []AdditionalGeoLocation{
			{Latitude: "52.5", Longitude: "13.4", Name: []DisplayText{{Language: "en", Text: "Entrance B"}}},
		},
		Directions: []DisplayText{{Language: "en", Text: "Go left"}},
		Operator:   &BusinessDetails{Name: "Operator", Website: "https://op.example"},
		SubOperator: &BusinessDetails{Name: "SubOp"},
		Owner: &BusinessDetails{
			Name: "Owner",
			Logo: &Image{URL: "https://cdn.example/logo.png", Category: "OWNER", Type: "png"},
		},
		Images: []Image{{URL: "https://cdn.example/loc.jpg", Category: "LOCATION", Type: "jpg"}},
		PublishAllowedTo: []PublishTokenType{
			{UID: "TOK1", Type: "RFID", Issuer: "FleetCo"},
		},
		TimeZone:    "Europe/Berlin",
		ParkingType: "ON_STREET",
		LastUpdated: "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)

	mustContain := []string{
		`"state":"Berlin"`,
		`"related_locations":`,
		`"directions":`,
		`"suboperator":`,
		`"owner":`,
		`"images":`,
		`"publish_allowed_to":`,
		`"website":"https://op.example"`,
		`"logo":`,
	}
	for _, needle := range mustContain {
		if !strings.Contains(s, needle) {
			t.Errorf("marshalled Location missing %q:\n%s", needle, s)
		}
	}
}

func TestEVSE_JSONOmitsUnsetOptionalFields(t *testing.T) {
	evse := EVSE{
		UID:          "EVSE-1",
		EvseID:       "DE*AAA*E001",
		Status:       "AVAILABLE",
		Capabilities: []string{"RFID_READER"},
		Connectors:   []Connector{{ID: "1", LastUpdated: "2026-01-01T00:00:00Z"}},
		LastUpdated:  "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(evse)
	for _, forbidden := range []string{
		`"status_schedule"`,
		`"floor_level"`,
		`"physical_reference"`,
		`"directions"`,
		`"parking_restrictions"`,
		`"images"`,
	} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("unset EVSE optional %s should be omitted: %s", forbidden, string(data))
		}
	}
}

func TestConnector_OmitsUnsetTermsAndConditions(t *testing.T) {
	c := Connector{ID: "1", Standard: "IEC_62196_T2", LastUpdated: "2026-01-01T00:00:00Z"}
	data, _ := json.Marshal(c)
	if strings.Contains(string(data), "terms_and_conditions") {
		t.Errorf("unset terms_and_conditions should be omitted: %s", string(data))
	}
}

func TestOperatorAliasBusinessDetails(t *testing.T) {
	// Operator is an alias of BusinessDetails — both names must marshal the same.
	op := &Operator{Name: "Alias"}
	bd := &BusinessDetails{Name: "Alias"}
	a, _ := json.Marshal(op)
	b, _ := json.Marshal(bd)
	if string(a) != string(b) {
		t.Errorf("Operator/BusinessDetails alias mismatch: %s vs %s", string(a), string(b))
	}
}
