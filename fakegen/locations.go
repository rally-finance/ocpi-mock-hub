package fakegen

import (
	"fmt"
	"math/rand"
)

type cityData struct {
	Name       string
	Country    string
	Lat, Lng   float64
	TimeZone   string
	PostalCode string
	State      string
}

var europeanCities = []cityData{
	{"Berlin", "DEU", 52.520, 13.405, "Europe/Berlin", "10115", "Berlin"},
	{"Munich", "DEU", 48.137, 11.576, "Europe/Berlin", "80331", "Bavaria"},
	{"Hamburg", "DEU", 53.551, 9.994, "Europe/Berlin", "20095", "Hamburg"},
	{"Frankfurt", "DEU", 50.110, 8.682, "Europe/Berlin", "60311", "Hesse"},
	{"Cologne", "DEU", 50.938, 6.960, "Europe/Berlin", "50667", "North Rhine-Westphalia"},
	{"Stuttgart", "DEU", 48.775, 9.183, "Europe/Berlin", "70173", "Baden-Württemberg"},
	{"Amsterdam", "NLD", 52.367, 4.904, "Europe/Amsterdam", "1012", "North Holland"},
	{"Rotterdam", "NLD", 51.924, 4.478, "Europe/Amsterdam", "3011", "South Holland"},
	{"Utrecht", "NLD", 52.091, 5.122, "Europe/Amsterdam", "3511", "Utrecht"},
	{"The Hague", "NLD", 52.070, 4.300, "Europe/Amsterdam", "2511", "South Holland"},
	{"Paris", "FRA", 48.857, 2.352, "Europe/Paris", "75001", "Île-de-France"},
	{"Lyon", "FRA", 45.764, 4.836, "Europe/Paris", "69001", "Auvergne-Rhône-Alpes"},
	{"Marseille", "FRA", 43.296, 5.370, "Europe/Paris", "13001", "Provence-Alpes-Côte d'Azur"},
	{"Nice", "FRA", 43.710, 7.262, "Europe/Paris", "06000", "Provence-Alpes-Côte d'Azur"},
	{"Bordeaux", "FRA", 44.838, -0.579, "Europe/Paris", "33000", "Nouvelle-Aquitaine"},
	{"Vienna", "AUT", 48.208, 16.372, "Europe/Vienna", "1010", "Vienna"},
	{"Graz", "AUT", 47.070, 15.439, "Europe/Vienna", "8010", "Styria"},
	{"Salzburg", "AUT", 47.800, 13.045, "Europe/Vienna", "5020", "Salzburg"},
	{"Innsbruck", "AUT", 47.263, 11.394, "Europe/Vienna", "6020", "Tyrol"},
	{"Brussels", "BEL", 50.850, 4.352, "Europe/Brussels", "1000", "Brussels-Capital"},
	{"Antwerp", "BEL", 51.221, 4.402, "Europe/Brussels", "2000", "Flanders"},
	{"Ghent", "BEL", 51.054, 3.725, "Europe/Brussels", "9000", "Flanders"},
	{"Liege", "BEL", 50.633, 5.568, "Europe/Brussels", "4000", "Wallonia"},
}

var streetNames = []string{
	"Hauptstraße", "Bahnhofstraße", "Marktplatz", "Industriestraße",
	"Kerkstraat", "Stationsweg", "Dorpsstraat", "Rue de la Gare",
	"Avenue de la République", "Boulevard Victor Hugo", "Ringstraße",
	"Leopoldstraat", "Rue Royale", "Kirchenweg", "Schillerstraße",
}

var parkingTypes = []string{"ON_STREET", "PARKING_GARAGE", "PARKING_LOT", "ALONG_MOTORWAY"}

var facilityOptions = []string{"RESTAURANT", "CAFE", "SHOPPING_MALL", "HOTEL", "SUPERMARKET"}

// parkingRestrictionOptions mirrors OCPI 2.2.1 ParkingRestriction enum.
var parkingRestrictionOptions = []string{"EV_ONLY", "PLUGGED", "DISABLED", "CUSTOMERS", "MOTORCYCLES"}

// floorLevels are representative values exercised across the seed corpus so
// clients see both underground and above-ground parking references.
var floorLevels = []string{"-2", "-1", "0", "1", "2"}

func generateLocations(rng *rand.Rand, cpos []CPO, total int) []Location {
	perCPO := total / len(cpos)
	if perCPO < 1 {
		perCPO = 1
	}

	var locations []Location
	locIdx := 0

	for _, cpo := range cpos {
		cities := filterCities(cpo.CountryCode)
		if len(cities) == 0 {
			cities = europeanCities
		}

		for i := 0; i < perCPO && locIdx < total; i++ {
			city := cities[rng.Intn(len(cities))]
			street := streetNames[rng.Intn(len(streetNames))]
			streetNum := rng.Intn(200) + 1

			locID := fmt.Sprintf("LOC-%s-%03d", cpo.PartyID, i+1)
			lat := city.Lat + (rng.Float64()-0.5)*0.05
			lng := city.Lng + (rng.Float64()-0.5)*0.05

			numEVSEs := rng.Intn(3) + 2
			evses := make([]EVSE, numEVSEs)
			for e := 0; e < numEVSEs; e++ {
				evses[e] = generateEVSE(rng, cpo, locID, e+1)
			}

			numFacilities := rng.Intn(3)
			facilities := make([]string, 0, numFacilities)
			used := map[string]bool{}
			for f := 0; f < numFacilities; f++ {
				pick := facilityOptions[rng.Intn(len(facilityOptions))]
				if !used[pick] {
					facilities = append(facilities, pick)
					used[pick] = true
				}
			}

			var openingTimes any
			if rng.Float64() < 0.7 {
				openingTimes = map[string]any{"twentyfourseven": true}
			} else {
				openingTimes = map[string]any{
					"regular_hours": []map[string]string{
						{"weekday": "1", "period_begin": "06:00", "period_end": "22:00"},
						{"weekday": "2", "period_begin": "06:00", "period_end": "22:00"},
						{"weekday": "3", "period_begin": "06:00", "period_end": "22:00"},
						{"weekday": "4", "period_begin": "06:00", "period_end": "22:00"},
						{"weekday": "5", "period_begin": "06:00", "period_end": "22:00"},
						{"weekday": "6", "period_begin": "08:00", "period_end": "20:00"},
						{"weekday": "7", "period_begin": "08:00", "period_end": "20:00"},
					},
				}
			}

			isGreen := rng.Float64() < 0.6
			energyMix := map[string]any{
				"is_green_energy": isGreen,
				"energy_sources": []map[string]any{
					{"source": "SOLAR", "percentage": 40},
					{"source": "WIND", "percentage": 30},
					{"source": "GENERAL_FOSSIL", "percentage": 30},
				},
			}
			if isGreen {
				energyMix["energy_sources"] = []map[string]any{
					{"source": "SOLAR", "percentage": 50},
					{"source": "WIND", "percentage": 50},
				}
			}

			loc := Location{
				CountryCode: cpo.CountryCode,
				PartyID:     cpo.PartyID,
				ID:          locID,
				Publish:     true,
				Name:        fmt.Sprintf("%s %s", cpo.Name, city.Name),
				Address:     fmt.Sprintf("%s %d", street, streetNum),
				City:        city.Name,
				PostalCode:  city.PostalCode,
				State:       city.State,
				Country:     city.Country,
				Coordinates: Coords{
					Latitude:  fmt.Sprintf("%.6f", lat),
					Longitude: fmt.Sprintf("%.6f", lng),
				},
				TimeZone:           city.TimeZone,
				ParkingType:        parkingTypes[rng.Intn(len(parkingTypes))],
				Operator:           &BusinessDetails{Name: cpo.Name, Website: fmt.Sprintf("https://%s.example.com", cpo.PartyID)},
				Facilities:         facilities,
				OpeningTimes:       openingTimes,
				ChargingWhenClosed: false,
				EnergyMix:          energyMix,
				EVSEs:              evses,
				LastUpdated:        seedTime,
			}

			enrichLocation(rng, &loc, cpo, city)
			locations = append(locations, loc)
			locIdx++
		}
	}

	return locations
}

// enrichLocation populates the OCPI 2.2.1 optional fields — related_locations,
// directions, suboperator, owner, images — deterministically on a subset of
// locations so the generated corpus exercises the full spec surface without
// making every object repeat the same filler data.
func enrichLocation(rng *rand.Rand, loc *Location, cpo CPO, city cityData) {
	// Roughly a third of locations advertise an accessible-entrance AddtlGeoLoc.
	if rng.Float64() < 0.35 {
		loc.RelatedLocations = []AdditionalGeoLocation{
			{
				Latitude:  fmt.Sprintf("%.6f", city.Lat+0.0005),
				Longitude: fmt.Sprintf("%.6f", city.Lng+0.0005),
				Name: []DisplayText{
					{Language: "en", Text: "Accessible entrance"},
				},
			},
		}
	}
	// Half get turn-by-turn directions.
	if rng.Float64() < 0.5 {
		loc.Directions = []DisplayText{
			{Language: "en", Text: "Enter from " + city.Name + " main street; chargers are at the rear of the lot."},
		}
	}
	// A quarter carry a suboperator (e.g. a facility operator).
	if rng.Float64() < 0.25 {
		loc.SubOperator = &BusinessDetails{
			Name:    cpo.Name + " Local Ops",
			Website: fmt.Sprintf("https://ops.%s.example.com", cpo.PartyID),
		}
	}
	// A sixth carry an owner distinct from the operator.
	if rng.Float64() < 0.2 {
		loc.Owner = &BusinessDetails{
			Name:    city.Name + " Municipal Authority",
			Website: fmt.Sprintf("https://city.%s.example.gov", city.Country),
			Logo: &Image{
				URL:      fmt.Sprintf("https://cdn.example.com/owners/%s.png", city.Name),
				Category: "OWNER",
				Type:     "png",
				Width:    256,
				Height:   256,
			},
		}
	}
	// ~40% carry a hero image.
	if rng.Float64() < 0.4 {
		loc.Images = []Image{
			{
				URL:       fmt.Sprintf("https://cdn.example.com/locations/%s.jpg", loc.ID),
				Thumbnail: fmt.Sprintf("https://cdn.example.com/locations/%s_thumb.jpg", loc.ID),
				Category:  "LOCATION",
				Type:      "jpg",
				Width:     1280,
				Height:    720,
			},
		}
	}
}

func generateEVSE(rng *rand.Rand, cpo CPO, locID string, idx int) EVSE {
	uid := fmt.Sprintf("%s-EVSE-%03d", locID, idx)
	evseID := fmt.Sprintf("%s*%s*E%03d", cpo.CountryCode, cpo.PartyID, idx)

	isDC := rng.Float64() < 0.3
	connectors := make([]Connector, 1)
	if isDC {
		connectors[0] = Connector{
			ID:                 "1",
			Standard:           "IEC_62196_T2_COMBO",
			Format:             "CABLE",
			PowerType:          "DC",
			MaxVoltage:         400,
			MaxAmperage:        125 + rng.Intn(250),
			MaxElectricPower:   50000 + rng.Intn(100000),
			TariffIDs:          []string{fmt.Sprintf("TARIFF-%s-DC", cpo.PartyID)},
			TermsAndConditions: fmt.Sprintf("https://%s.example.com/terms/dc", cpo.PartyID),
			LastUpdated:        seedTime,
		}
	} else {
		connectors[0] = Connector{
			ID:                 "1",
			Standard:           "IEC_62196_T2",
			Format:             "SOCKET",
			PowerType:          "AC_3_PHASE",
			MaxVoltage:         230,
			MaxAmperage:        32,
			MaxElectricPower:   22000,
			TariffIDs:          []string{fmt.Sprintf("TARIFF-%s-AC", cpo.PartyID)},
			TermsAndConditions: fmt.Sprintf("https://%s.example.com/terms/ac", cpo.PartyID),
			LastUpdated:        seedTime,
		}
	}

	statuses := []string{"AVAILABLE", "AVAILABLE", "AVAILABLE", "CHARGING", "BLOCKED"}
	capabilities := []string{"RFID_READER", "REMOTE_START_STOP_CAPABLE"}
	if rng.Float64() < 0.5 {
		capabilities = append(capabilities, "CHARGING_PREFERENCES_CAPABLE")
	}

	evse := EVSE{
		UID:          uid,
		EvseID:       evseID,
		Status:       statuses[rng.Intn(len(statuses))],
		Capabilities: capabilities,
		Connectors:   connectors,
		LastUpdated:  seedTime,
	}

	enrichEVSE(rng, &evse)
	return evse
}

// enrichEVSE layers OCPI 2.2.1 optional fields onto a subset of EVSEs so the
// generated corpus has at least one example of each: planned status changes,
// explicit floor level, physical reference labels, turn-by-turn directions,
// parking restrictions, and EVSE-level images.
func enrichEVSE(rng *rand.Rand, evse *EVSE) {
	// Give ~25% of EVSEs a status schedule entry (maintenance window).
	if rng.Float64() < 0.25 {
		evse.StatusSchedule = []StatusSchedule{
			{
				PeriodBegin: "2026-06-01T02:00:00Z",
				PeriodEnd:   "2026-06-01T04:00:00Z",
				Status:      "OUTOFORDER",
			},
		}
	}
	// Floor level — most EVSEs have one; omit for a minority to exercise omitempty.
	if rng.Float64() < 0.75 {
		evse.FloorLevel = floorLevels[rng.Intn(len(floorLevels))]
	}
	// ~60% carry a physical reference (e.g. bay number).
	if rng.Float64() < 0.6 {
		evse.PhysicalReference = fmt.Sprintf("Bay %c%d", 'A'+rng.Intn(4), rng.Intn(12)+1)
	}
	// ~20% ship turn-by-turn directions.
	if rng.Float64() < 0.2 {
		evse.Directions = []DisplayText{
			{Language: "en", Text: "Charger is immediately to the left of the lift."},
		}
	}
	// ~40% advertise one or two parking restrictions.
	if rng.Float64() < 0.4 {
		count := rng.Intn(2) + 1
		used := map[string]bool{}
		for k := 0; k < count; k++ {
			pick := parkingRestrictionOptions[rng.Intn(len(parkingRestrictionOptions))]
			if !used[pick] {
				evse.ParkingRestrictions = append(evse.ParkingRestrictions, pick)
				used[pick] = true
			}
		}
	}
	// ~15% attach an image.
	if rng.Float64() < 0.15 {
		evse.Images = []Image{
			{
				URL:      fmt.Sprintf("https://cdn.example.com/evses/%s.jpg", evse.UID),
				Category: "CHARGER",
				Type:     "jpg",
				Width:    800,
				Height:   600,
			},
		}
	}
}

func filterCities(countryCode string) []cityData {
	countryMap := map[string]string{
		"DE": "DEU", "NL": "NLD", "FR": "FRA", "AT": "AUT", "BE": "BEL",
	}
	iso3 := countryMap[countryCode]
	if iso3 == "" {
		return nil
	}
	var result []cityData
	for _, c := range europeanCities {
		if c.Country == iso3 {
			result = append(result, c)
		}
	}
	return result
}
