package fakegen

import (
	"fmt"
	"math/rand"
)

func generateTariffs(rng *rand.Rand, cpos []CPO) []Tariff {
	var tariffs []Tariff

	for _, cpo := range cpos {
		// AC tariff: energy + time with optional restrictions
		acPrice := 0.25 + rng.Float64()*0.20
		acTimePrice := 0.02 + rng.Float64()*0.08
		acElement := TariffElement{
			PriceComponents: []PriceComponent{
				{Type: "ENERGY", Price: roundTo(acPrice, 4), VAT: 19.0, StepSize: 1},
				{Type: "TIME", Price: roundTo(acTimePrice, 4), VAT: 19.0, StepSize: 60},
			},
		}
		if rng.Float64() < 0.4 {
			acElement.Restrictions = &TariffRestrictions{StartTime: "08:00", EndTime: "20:00"}
		}

		acTariff := Tariff{
			CountryCode:   cpo.CountryCode,
			PartyID:       cpo.PartyID,
			ID:            fmt.Sprintf("TARIFF-%s-AC", cpo.PartyID),
			Currency:      "EUR",
			Type:          "REGULAR",
			TariffAltText: []DisplayText{{Language: "en", Text: "Standard AC tariff"}},
			Elements:      []TariffElement{acElement},
			LastUpdated:   seedTime,
		}
		if rng.Float64() < 0.3 {
			acTariff.MinPrice = &Price{ExclVat: 1.00, InclVat: 1.19}
		}
		tariffs = append(tariffs, acTariff)

		// DC tariff: energy + flat session fee
		dcPrice := 0.39 + rng.Float64()*0.20
		flatFee := 0.50 + rng.Float64()*1.50
		dcElement := TariffElement{
			PriceComponents: []PriceComponent{
				{Type: "ENERGY", Price: roundTo(dcPrice, 4), VAT: 19.0, StepSize: 1},
				{Type: "FLAT", Price: roundTo(flatFee, 4), VAT: 19.0, StepSize: 1},
			},
		}

		dcTariff := Tariff{
			CountryCode:   cpo.CountryCode,
			PartyID:       cpo.PartyID,
			ID:            fmt.Sprintf("TARIFF-%s-DC", cpo.PartyID),
			Currency:      "EUR",
			Type:          "REGULAR",
			TariffAltText: []DisplayText{{Language: "en", Text: "Standard DC tariff"}},
			Elements:      []TariffElement{dcElement},
			LastUpdated:   seedTime,
		}
		if rng.Float64() < 0.3 {
			dcTariff.MaxPrice = &Price{ExclVat: 50.00, InclVat: 59.50}
		}
		tariffs = append(tariffs, dcTariff)
	}

	return tariffs
}

func roundTo(val float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int(val*factor+0.5)) / factor
}
