package fakegen

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

// TestGenerateTariffs_CoversFullSpecEnum asserts that the seed generator
// produces at least one tariff element hitting every OCPI 2.2.1 enum value
// that Stage 4 cares about, so tariff-driven simulations and tests never
// have to "hope" a given restriction or PriceComponent type is in the seed.
func TestGenerateTariffs_CoversFullSpecEnum(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	cpos := []CPO{{CountryCode: "NL", PartyID: "TST", Name: "Test Co"}}
	tariffs := generateTariffs(rng, cpos)

	if len(tariffs) == 0 {
		t.Fatal("expected at least one tariff per CPO")
	}

	tariffTypes := map[string]bool{}
	priceTypes := map[string]bool{}
	restrictionSeen := map[string]bool{}
	var sawTariffAltURL, sawTariffWindow, sawEnergyMix, sawMultilangText bool

	for _, tr := range tariffs {
		tariffTypes[tr.Type] = true
		if tr.TariffAltURL != "" {
			sawTariffAltURL = true
		}
		if tr.StartDateTime != "" && tr.EndDateTime != "" {
			sawTariffWindow = true
		}
		if tr.EnergyMix != nil {
			sawEnergyMix = true
		}
		if len(tr.TariffAltText) > 1 {
			sawMultilangText = true
		}

		for _, el := range tr.Elements {
			for _, pc := range el.PriceComponents {
				priceTypes[pc.Type] = true
			}
			r := el.Restrictions
			if r == nil {
				continue
			}
			if r.StartTime != "" {
				restrictionSeen["start_time"] = true
			}
			if r.EndTime != "" {
				restrictionSeen["end_time"] = true
			}
			if r.StartDate != "" {
				restrictionSeen["start_date"] = true
			}
			if r.EndDate != "" {
				restrictionSeen["end_date"] = true
			}
			if r.MinKwh != nil {
				restrictionSeen["min_kwh"] = true
			}
			if r.MaxKwh != nil {
				restrictionSeen["max_kwh"] = true
			}
			if r.MinCurrent != nil {
				restrictionSeen["min_current"] = true
			}
			if r.MaxCurrent != nil {
				restrictionSeen["max_current"] = true
			}
			if r.MinPower != nil {
				restrictionSeen["min_power"] = true
			}
			if r.MaxPower != nil {
				restrictionSeen["max_power"] = true
			}
			if r.MinDuration != nil {
				restrictionSeen["min_duration"] = true
			}
			if r.MaxDuration != nil {
				restrictionSeen["max_duration"] = true
			}
			if len(r.DayOfWeek) > 0 {
				restrictionSeen["day_of_week"] = true
			}
			if r.Reservation != "" {
				restrictionSeen["reservation"] = true
			}
		}
	}

	for _, tt := range []string{"REGULAR", "AD_HOC_PAYMENT", "PROFILE_CHEAP", "PROFILE_FAST", "PROFILE_GREEN"} {
		if !tariffTypes[tt] {
			t.Errorf("expected TariffType %s in seed corpus, got %v", tt, keys(tariffTypes))
		}
	}
	for _, pc := range []string{"ENERGY", "FLAT", "TIME", "PARKING_TIME"} {
		if !priceTypes[pc] {
			t.Errorf("expected PriceComponent type %s in seed corpus, got %v", pc, keys(priceTypes))
		}
	}
	for _, f := range []string{
		"start_time", "end_time", "start_date", "end_date",
		"min_kwh", "max_kwh", "min_current", "max_current",
		"min_power", "max_power", "min_duration", "max_duration",
		"day_of_week", "reservation",
	} {
		if !restrictionSeen[f] {
			t.Errorf("expected TariffRestrictions field %s to be exercised in seed corpus", f)
		}
	}
	if !sawTariffAltURL {
		t.Error("expected at least one tariff to carry tariff_alt_url")
	}
	if !sawTariffWindow {
		t.Error("expected at least one tariff to carry start_date_time + end_date_time")
	}
	if !sawEnergyMix {
		t.Error("expected at least one tariff to carry energy_mix")
	}
	if !sawMultilangText {
		t.Error("expected at least one tariff to carry multi-language tariff_alt_text")
	}

	// Both ReservationRestrictionType enum values should appear, not just
	// one — otherwise simulations of expired reservations have no rate.
	reservations := map[string]bool{}
	for _, tr := range tariffs {
		for _, el := range tr.Elements {
			if el.Restrictions != nil && el.Restrictions.Reservation != "" {
				reservations[el.Restrictions.Reservation] = true
			}
		}
	}
	for _, v := range []string{"RESERVATION", "RESERVATION_EXPIRES"} {
		if !reservations[v] {
			t.Errorf("expected Reservation=%s somewhere in the seed corpus", v)
		}
	}
}

// TestTariff_JSONRoundTripsNewFields makes sure the new Tariff and
// TariffRestrictions fields serialize under their spec-correct JSON names.
// A silent field-name typo would otherwise pass the build but ship wrong
// keys to clients.
func TestTariff_JSONRoundTripsNewFields(t *testing.T) {
	tr := Tariff{
		CountryCode:   "NL",
		PartyID:       "TST",
		ID:            "T1",
		Currency:      "EUR",
		Type:          "PROFILE_GREEN",
		TariffAltURL:  "https://example.com/t1",
		StartDateTime: "2026-01-01T00:00:00Z",
		EndDateTime:   "2026-12-31T23:59:59Z",
		EnergyMix:     map[string]any{"is_green_energy": true},
		Elements: []TariffElement{{
			PriceComponents: []PriceComponent{{Type: "ENERGY", Price: 0.3, StepSize: 1}},
			Restrictions: &TariffRestrictions{
				StartDate:   "2026-04-01",
				EndDate:     "2026-10-31",
				MinKwh:      ptrf(5),
				MaxKwh:      ptrf(80),
				MinCurrent:  ptrf(16),
				MaxCurrent:  ptrf(32),
				MinPower:    ptrf(11),
				MaxPower:    ptrf(50),
				MinDuration: ptri(600),
				MaxDuration: ptri(10800),
				DayOfWeek:   []string{"SATURDAY", "SUNDAY"},
				Reservation: "RESERVATION",
			},
		}},
		LastUpdated: "2026-01-01T00:00:00Z",
	}

	raw, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)
	required := []string{
		`"tariff_alt_url":"https://example.com/t1"`,
		`"start_date_time":"2026-01-01T00:00:00Z"`,
		`"end_date_time":"2026-12-31T23:59:59Z"`,
		`"energy_mix":{"is_green_energy":true}`,
		`"start_date":"2026-04-01"`,
		`"end_date":"2026-10-31"`,
		`"min_kwh":5`,
		`"max_kwh":80`,
		`"min_current":16`,
		`"max_current":32`,
		`"min_power":11`,
		`"max_power":50`,
		`"min_duration":600`,
		`"max_duration":10800`,
		`"day_of_week":["SATURDAY","SUNDAY"]`,
		`"reservation":"RESERVATION"`,
	}
	for _, needle := range required {
		if !strings.Contains(got, needle) {
			t.Errorf("expected %s in tariff JSON; got %s", needle, got)
		}
	}
}

// TestTariffRestrictions_OmitEmptyDropsUnsetFields ensures clients don't
// see a pile of null fields for tariffs that use only a subset of
// restrictions — omitempty on both the value fields and the pointer
// numerics must hold.
func TestTariffRestrictions_OmitEmptyDropsUnsetFields(t *testing.T) {
	tr := TariffRestrictions{StartTime: "08:00", EndTime: "20:00"}
	raw, _ := json.Marshal(tr)
	got := string(raw)
	forbidden := []string{
		"start_date", "end_date",
		"min_kwh", "max_kwh",
		"min_current", "max_current",
		"min_power", "max_power",
		"min_duration", "max_duration",
		"day_of_week", "reservation",
	}
	for _, f := range forbidden {
		if strings.Contains(got, f) {
			t.Errorf("expected unset field %s to be omitted, got %s", f, got)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
