package fakegen

import "math/rand"

const seedTime = "2026-01-01T00:00:00Z"

var defaultCPOs = []CPO{
	{CountryCode: "DE", PartyID: "AAA", Name: "FastCharge GmbH"},
	{CountryCode: "NL", PartyID: "BBB", Name: "GreenPlug BV"},
	{CountryCode: "FR", PartyID: "CCC", Name: "ChargeRapide SA"},
	{CountryCode: "AT", PartyID: "DDD", Name: "AlpenStrom"},
	{CountryCode: "BE", PartyID: "EEE", Name: "PowerBelgium"},
}

func GenerateSeed(totalLocations int) *SeedData {
	rng := rand.New(rand.NewSource(42)) // deterministic

	cpos := defaultCPOs
	locations := generateLocations(rng, cpos, totalLocations)
	tariffs := generateTariffs(rng, cpos)

	hubInfo := make([]HubClientInfo, len(cpos))
	for i, cpo := range cpos {
		hubInfo[i] = HubClientInfo{
			CountryCode: cpo.CountryCode,
			PartyID:     cpo.PartyID,
			Role:        "CPO",
			Status:      "CONNECTED",
			LastUpdated: seedTime,
		}
		hubInfo[i].BusinessDetails.Name = cpo.Name
	}

	return &SeedData{
		CPOs:          cpos,
		Locations:     locations,
		Tariffs:       tariffs,
		HubClientInfo: hubInfo,
	}
}

// LocationByID returns a location from the seed by its ID.
func (s *SeedData) LocationByID(id string) *Location {
	for i := range s.Locations {
		if s.Locations[i].ID == id {
			return &s.Locations[i]
		}
	}
	return nil
}

// LocationsByParty filters seed locations by country_code and party_id.
func (s *SeedData) LocationsByParty(cc, pid string) []Location {
	var result []Location
	for _, loc := range s.Locations {
		if loc.CountryCode == cc && loc.PartyID == pid {
			result = append(result, loc)
		}
	}
	return result
}

// TariffsByParty filters seed tariffs by country_code and party_id.
func (s *SeedData) TariffsByParty(cc, pid string) []Tariff {
	var result []Tariff
	for _, t := range s.Tariffs {
		if t.CountryCode == cc && t.PartyID == pid {
			result = append(result, t)
		}
	}
	return result
}

// TariffByID returns a tariff from the seed by composite key.
func (s *SeedData) TariffByID(cc, pid, id string) *Tariff {
	for i := range s.Tariffs {
		t := &s.Tariffs[i]
		if t.CountryCode == cc && t.PartyID == pid && t.ID == id {
			return t
		}
	}
	return nil
}
