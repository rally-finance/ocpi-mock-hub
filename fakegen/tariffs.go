package fakegen

import (
	"fmt"
	"math/rand"
)

// generateTariffs seeds a rich tariff corpus per CPO. Coverage goals (so
// downstream simulation and spec-conformance tests can rely on finding
// realistic data for any scenario):
//
//   - All five OCPI 2.2.1 TariffType enum values are represented per CPO.
//   - All four PriceComponent types (ENERGY, FLAT, TIME, PARKING_TIME) appear.
//   - Every TariffRestrictions field is exercised by at least one element
//     somewhere in the corpus, including the reservation enum values.
//   - At least one tariff carries tariff_alt_url, start_date_time/end_date_time
//     and an energy_mix block, matching the fields added in Stage 4.
//
// The function is deterministic given the provided rng so fixtures stay
// stable between test runs.
func generateTariffs(rng *rand.Rand, cpos []CPO) []Tariff {
	var tariffs []Tariff
	for _, cpo := range cpos {
		tariffs = append(tariffs, regularACTariff(rng, cpo))
		tariffs = append(tariffs, regularDCTariff(rng, cpo))
		tariffs = append(tariffs, adHocTariff(cpo))
		tariffs = append(tariffs, profileCheapTariff(rng, cpo))
		tariffs = append(tariffs, profileFastTariff(rng, cpo))
		tariffs = append(tariffs, profileGreenTariff(rng, cpo))
		tariffs = append(tariffs, reservationTariff(cpo))
	}
	return tariffs
}

// regularACTariff: peak-hour weekday tariff with a low-current lane.
// Exercises start_time, end_time, day_of_week, max_current.
func regularACTariff(rng *rand.Rand, cpo CPO) Tariff {
	peak := ptrf(0.35 + rng.Float64()*0.10)
	offPeak := 0.22 + rng.Float64()*0.05
	maxCurrent := ptrf(16)

	return Tariff{
		CountryCode:   cpo.CountryCode,
		PartyID:       cpo.PartyID,
		ID:            fmt.Sprintf("TARIFF-%s-AC", cpo.PartyID),
		Currency:      "EUR",
		Type:          "REGULAR",
		TariffAltText: []DisplayText{{Language: "en", Text: "Standard AC tariff, cheaper outside peak hours"}},
		Elements: []TariffElement{
			{
				PriceComponents: []PriceComponent{
					{Type: "ENERGY", Price: roundTo(*peak, 4), VAT: 21.0, StepSize: 1},
					{Type: "TIME", Price: 0.04, VAT: 21.0, StepSize: 60},
				},
				Restrictions: &TariffRestrictions{
					StartTime:  "08:00",
					EndTime:    "20:00",
					DayOfWeek:  []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"},
					MaxCurrent: maxCurrent,
				},
			},
			{
				PriceComponents: []PriceComponent{
					{Type: "ENERGY", Price: roundTo(offPeak, 4), VAT: 21.0, StepSize: 1},
				},
			},
		},
		LastUpdated: seedTime,
	}
}

// regularDCTariff: tiered by max_power, with a time-window activation and a
// min_price floor. Exercises max_power, start_date_time, end_date_time, min_price.
func regularDCTariff(rng *rand.Rand, cpo CPO) Tariff {
	lowTier := 0.45 + rng.Float64()*0.05
	midTier := 0.55 + rng.Float64()*0.05
	highTier := 0.65 + rng.Float64()*0.05

	return Tariff{
		CountryCode:   cpo.CountryCode,
		PartyID:       cpo.PartyID,
		ID:            fmt.Sprintf("TARIFF-%s-DC", cpo.PartyID),
		Currency:      "EUR",
		Type:          "REGULAR",
		TariffAltText: []DisplayText{{Language: "en", Text: "DC fast-charge tariff, price by power tier"}},
		MinPrice:      &Price{ExclVat: 2.00, InclVat: 2.42},
		MaxPrice:      &Price{ExclVat: 80.00, InclVat: 96.80},
		StartDateTime: "2026-01-01T00:00:00Z",
		EndDateTime:   "2027-01-01T00:00:00Z",
		Elements: []TariffElement{
			{
				PriceComponents: []PriceComponent{
					{Type: "ENERGY", Price: roundTo(lowTier, 4), VAT: 21.0, StepSize: 1},
				},
				Restrictions: &TariffRestrictions{MaxPower: ptrf(50)},
			},
			{
				PriceComponents: []PriceComponent{
					{Type: "ENERGY", Price: roundTo(midTier, 4), VAT: 21.0, StepSize: 1},
				},
				Restrictions: &TariffRestrictions{MinPower: ptrf(50), MaxPower: ptrf(150)},
			},
			{
				PriceComponents: []PriceComponent{
					{Type: "ENERGY", Price: roundTo(highTier, 4), VAT: 21.0, StepSize: 1},
				},
				Restrictions: &TariffRestrictions{MinPower: ptrf(150)},
			},
		},
		LastUpdated: seedTime,
	}
}

// adHocTariff: walk-up / debit-card price plan. Exercises FLAT + TIME
// dimensions, multi-language tariff_alt_text, and tariff_alt_url.
func adHocTariff(cpo CPO) Tariff {
	return Tariff{
		CountryCode: cpo.CountryCode,
		PartyID:     cpo.PartyID,
		ID:          fmt.Sprintf("TARIFF-%s-ADHOC", cpo.PartyID),
		Currency:    "EUR",
		Type:        "AD_HOC_PAYMENT",
		TariffAltText: []DisplayText{
			{Language: "en", Text: "Ad-hoc payment: 1.00 EUR start fee + 0.50 EUR per kWh"},
			{Language: "nl", Text: "Betalen aan de paal: 1,00 EUR startgeld + 0,50 EUR per kWh"},
			{Language: "de", Text: "Direktzahlung: 1,00 EUR Startgebühr + 0,50 EUR pro kWh"},
		},
		TariffAltURL: fmt.Sprintf("https://%s.example/tariffs/adhoc", cpo.PartyID),
		Elements: []TariffElement{{
			PriceComponents: []PriceComponent{
				{Type: "FLAT", Price: 1.00, VAT: 21.0, StepSize: 1},
				{Type: "ENERGY", Price: 0.50, VAT: 21.0, StepSize: 1},
				{Type: "TIME", Price: 0.10, VAT: 21.0, StepSize: 60},
			},
		}},
		LastUpdated: seedTime,
	}
}

// profileCheapTariff: activates when Charging Preferences profile_type=CHEAP
// and the session has cleared certain thresholds. Exercises min_kwh and
// min_duration.
func profileCheapTariff(rng *rand.Rand, cpo CPO) Tariff {
	price := 0.18 + rng.Float64()*0.04
	return Tariff{
		CountryCode:   cpo.CountryCode,
		PartyID:       cpo.PartyID,
		ID:            fmt.Sprintf("TARIFF-%s-CHEAP", cpo.PartyID),
		Currency:      "EUR",
		Type:          "PROFILE_CHEAP",
		TariffAltText: []DisplayText{{Language: "en", Text: "Cheap profile: discounted rate once 5 kWh and 30 min are reached"}},
		Elements: []TariffElement{{
			PriceComponents: []PriceComponent{
				{Type: "ENERGY", Price: roundTo(price, 4), VAT: 21.0, StepSize: 1},
			},
			Restrictions: &TariffRestrictions{
				MinKwh:      ptrf(5),
				MaxKwh:      ptrf(80),
				MinDuration: ptri(1800),
			},
		}},
		LastUpdated: seedTime,
	}
}

// profileFastTariff: activates when profile_type=FAST, charging above the
// min_current / min_power thresholds. Exercises min_current and min_power.
func profileFastTariff(rng *rand.Rand, cpo CPO) Tariff {
	price := 0.70 + rng.Float64()*0.08
	return Tariff{
		CountryCode:   cpo.CountryCode,
		PartyID:       cpo.PartyID,
		ID:            fmt.Sprintf("TARIFF-%s-FAST", cpo.PartyID),
		Currency:      "EUR",
		Type:          "PROFILE_FAST",
		TariffAltText: []DisplayText{{Language: "en", Text: "Fast profile: premium rate for high-power charging"}},
		Elements: []TariffElement{{
			PriceComponents: []PriceComponent{
				{Type: "ENERGY", Price: roundTo(price, 4), VAT: 21.0, StepSize: 1},
			},
			Restrictions: &TariffRestrictions{
				MinCurrent: ptrf(32),
				MinPower:   ptrf(50),
			},
		}},
		LastUpdated: seedTime,
	}
}

// profileGreenTariff: activates when profile_type=GREEN, within a date window,
// with parking-time billing and a maximum session duration. Exercises
// PARKING_TIME, start_date, end_date, max_duration, and energy_mix.
func profileGreenTariff(rng *rand.Rand, cpo CPO) Tariff {
	price := 0.28 + rng.Float64()*0.04
	return Tariff{
		CountryCode:   cpo.CountryCode,
		PartyID:       cpo.PartyID,
		ID:            fmt.Sprintf("TARIFF-%s-GREEN", cpo.PartyID),
		Currency:      "EUR",
		Type:          "PROFILE_GREEN",
		TariffAltText: []DisplayText{{Language: "en", Text: "Green profile: 100% renewable, parking included up to 3h"}},
		Elements: []TariffElement{{
			PriceComponents: []PriceComponent{
				{Type: "ENERGY", Price: roundTo(price, 4), VAT: 21.0, StepSize: 1},
				{Type: "PARKING_TIME", Price: 0.00, VAT: 21.0, StepSize: 900},
			},
			Restrictions: &TariffRestrictions{
				StartDate:   "2026-04-01",
				EndDate:     "2026-10-31",
				MaxDuration: ptri(10800),
			},
		}},
		EnergyMix: map[string]any{
			"is_green_energy":     true,
			"supplier_name":       fmt.Sprintf("%s Green Power", cpo.Name),
			"energy_product_name": "100% Renewable",
			"energy_sources": []map[string]any{
				{"source": "WIND", "percentage": 60},
				{"source": "SOLAR", "percentage": 30},
				{"source": "WATER", "percentage": 10},
			},
		},
		LastUpdated: seedTime,
	}
}

// reservationTariff: two-element tariff covering the OCPI ReservationRestrictionType
// enum — one for active reservations, one for expired ones (higher rate).
// Exercises the Reservation restriction field and the TIME+RESERVATION combo.
func reservationTariff(cpo CPO) Tariff {
	return Tariff{
		CountryCode:   cpo.CountryCode,
		PartyID:       cpo.PartyID,
		ID:            fmt.Sprintf("TARIFF-%s-RESERVATION", cpo.PartyID),
		Currency:      "EUR",
		Type:          "REGULAR",
		TariffAltText: []DisplayText{{Language: "en", Text: "Reservation billing: active and expired"}},
		Elements: []TariffElement{
			{
				PriceComponents: []PriceComponent{
					{Type: "TIME", Price: 4.00, VAT: 21.0, StepSize: 600},
				},
				Restrictions: &TariffRestrictions{Reservation: "RESERVATION"},
			},
			{
				PriceComponents: []PriceComponent{
					{Type: "TIME", Price: 6.00, VAT: 21.0, StepSize: 600},
				},
				Restrictions: &TariffRestrictions{Reservation: "RESERVATION_EXPIRES"},
			},
		},
		LastUpdated: seedTime,
	}
}

func ptrf(v float64) *float64 { return &v }
func ptri(v int) *int         { return &v }

func roundTo(val float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int(val*factor+0.5)) / factor
}
