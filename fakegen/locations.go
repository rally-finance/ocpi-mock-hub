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
}

var europeanCities = []cityData{
	{"Berlin", "DEU", 52.520, 13.405, "Europe/Berlin", "10115"},
	{"Munich", "DEU", 48.137, 11.576, "Europe/Berlin", "80331"},
	{"Hamburg", "DEU", 53.551, 9.994, "Europe/Berlin", "20095"},
	{"Frankfurt", "DEU", 50.110, 8.682, "Europe/Berlin", "60311"},
	{"Cologne", "DEU", 50.938, 6.960, "Europe/Berlin", "50667"},
	{"Stuttgart", "DEU", 48.775, 9.183, "Europe/Berlin", "70173"},
	{"Amsterdam", "NLD", 52.367, 4.904, "Europe/Amsterdam", "1012"},
	{"Rotterdam", "NLD", 51.924, 4.478, "Europe/Amsterdam", "3011"},
	{"Utrecht", "NLD", 52.091, 5.122, "Europe/Amsterdam", "3511"},
	{"The Hague", "NLD", 52.070, 4.300, "Europe/Amsterdam", "2511"},
	{"Paris", "FRA", 48.857, 2.352, "Europe/Paris", "75001"},
	{"Lyon", "FRA", 45.764, 4.836, "Europe/Paris", "69001"},
	{"Marseille", "FRA", 43.296, 5.370, "Europe/Paris", "13001"},
	{"Nice", "FRA", 43.710, 7.262, "Europe/Paris", "06000"},
	{"Bordeaux", "FRA", 44.838, -0.579, "Europe/Paris", "33000"},
	{"Vienna", "AUT", 48.208, 16.372, "Europe/Vienna", "1010"},
	{"Graz", "AUT", 47.070, 15.439, "Europe/Vienna", "8010"},
	{"Salzburg", "AUT", 47.800, 13.045, "Europe/Vienna", "5020"},
	{"Innsbruck", "AUT", 47.263, 11.394, "Europe/Vienna", "6020"},
	{"Brussels", "BEL", 50.850, 4.352, "Europe/Brussels", "1000"},
	{"Antwerp", "BEL", 51.221, 4.402, "Europe/Brussels", "2000"},
	{"Ghent", "BEL", 51.054, 3.725, "Europe/Brussels", "9000"},
	{"Liege", "BEL", 50.633, 5.568, "Europe/Brussels", "4000"},
}

var streetNames = []string{
	"Hauptstraße", "Bahnhofstraße", "Marktplatz", "Industriestraße",
	"Kerkstraat", "Stationsweg", "Dorpsstraat", "Rue de la Gare",
	"Avenue de la République", "Boulevard Victor Hugo", "Ringstraße",
	"Leopoldstraat", "Rue Royale", "Kirchenweg", "Schillerstraße",
}

var parkingTypes = []string{"ON_STREET", "PARKING_GARAGE", "PARKING_LOT", "ALONG_MOTORWAY"}

func generateLocations(rng *rand.Rand, cpos []CPO, total int) []Location {
	perCPO := total / len(cpos)
	if perCPO < 1 {
		perCPO = 1
	}

	var locations []Location
	locIdx := 0

	for _, cpo := range cpos {
		// Pick cities that match this CPO's country bias
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

			numEVSEs := rng.Intn(3) + 2 // 2-4 EVSEs
			evses := make([]EVSE, numEVSEs)
			for e := 0; e < numEVSEs; e++ {
				evses[e] = generateEVSE(rng, cpo, locID, e+1)
			}

			locations = append(locations, Location{
				CountryCode: cpo.CountryCode,
				PartyID:     cpo.PartyID,
				ID:          locID,
				Publish:     true,
				Name:        fmt.Sprintf("%s %s", cpo.Name, city.Name),
				Address:     fmt.Sprintf("%s %d", street, streetNum),
				City:        city.Name,
				PostalCode:  city.PostalCode,
				Country:     city.Country,
				Coordinates: Coords{
					Latitude:  fmt.Sprintf("%.6f", lat),
					Longitude: fmt.Sprintf("%.6f", lng),
				},
				TimeZone:    city.TimeZone,
				ParkingType: parkingTypes[rng.Intn(len(parkingTypes))],
				Operator:    &Operator{Name: cpo.Name},
				EVSEs:       evses,
				LastUpdated: seedTime,
			})
			locIdx++
		}
	}

	return locations
}

func generateEVSE(rng *rand.Rand, cpo CPO, locID string, idx int) EVSE {
	uid := fmt.Sprintf("%s-EVSE-%03d", locID, idx)
	evseID := fmt.Sprintf("%s*%s*E%03d", cpo.CountryCode, cpo.PartyID, idx)

	isDC := rng.Float64() < 0.3 // 30% DC chargers
	connectors := make([]Connector, 1)
	if isDC {
		connectors[0] = Connector{
			ID:               "1",
			Standard:         "IEC_62196_T2_COMBO",
			Format:           "CABLE",
			PowerType:        "DC",
			MaxVoltage:       400,
			MaxAmperage:      125 + rng.Intn(250),
			MaxElectricPower: 50000 + rng.Intn(100000),
			TariffIDs:        []string{fmt.Sprintf("TARIFF-%s-DC", cpo.PartyID)},
			LastUpdated:      seedTime,
		}
	} else {
		connectors[0] = Connector{
			ID:               "1",
			Standard:         "IEC_62196_T2",
			Format:           "SOCKET",
			PowerType:        "AC_3_PHASE",
			MaxVoltage:       230,
			MaxAmperage:      32,
			MaxElectricPower: 22000,
			TariffIDs:        []string{fmt.Sprintf("TARIFF-%s-AC", cpo.PartyID)},
			LastUpdated:      seedTime,
		}
	}

	statuses := []string{"AVAILABLE", "AVAILABLE", "AVAILABLE", "CHARGING", "BLOCKED"}
	return EVSE{
		UID:          uid,
		EvseID:       evseID,
		Status:       statuses[rng.Intn(len(statuses))],
		Capabilities: []string{"RFID_READER", "REMOTE_START_STOP_CAPABLE"},
		Connectors:   connectors,
		LastUpdated:  seedTime,
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
