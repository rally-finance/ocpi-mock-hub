package simulation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

type PushResult struct {
	URL        string `json:"url"`
	Method     string `json:"method"`
	StatusCode int    `json:"status_code"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type PushConfig struct {
	Pattern    string `json:"pattern"`
	Count      int    `json:"count"`
	MinDelayMS int    `json:"min_delay_ms"`
	MaxDelayMS int    `json:"max_delay_ms"`
	Mutate     bool   `json:"mutate"`
	EVSEOnly   bool   `json:"evse_only"`
}

type PushSummary struct {
	Total    int           `json:"total"`
	OK       int           `json:"ok"`
	Failed   int           `json:"failed"`
	Duration int64         `json:"duration_ms"`
	Results  []PushResult  `json:"results"`
}

var evseStatuses = []string{"AVAILABLE", "CHARGING", "BLOCKED", "OUTOFORDER", "AVAILABLE", "AVAILABLE"}

func PushLocations(cfg PushConfig, seed *fakegen.SeedData, emspURL, tokenB string) PushSummary {
	start := time.Now()
	locs := selectLocations(seed, cfg.Count)
	results := make([]PushResult, 0, len(locs))

	for i, loc := range locs {
		if cfg.Mutate {
			loc = mutateLocation(loc)
		}

		var result PushResult
		if cfg.EVSEOnly {
			for _, evse := range loc.EVSEs {
				url := fmt.Sprintf("%s/receiver/locations/%s/%s/%s/%s",
					strings.TrimRight(emspURL, "/"), loc.CountryCode, loc.PartyID, loc.ID, evse.UID)
				result = doPush("PUT", url, tokenB, evse)
				results = append(results, result)
			}
		} else {
			url := fmt.Sprintf("%s/receiver/locations/%s/%s/%s",
				strings.TrimRight(emspURL, "/"), loc.CountryCode, loc.PartyID, loc.ID)
			result = doPush("PUT", url, tokenB, loc)
			results = append(results, result)
		}

		if i < len(locs)-1 {
			sleepForPattern(cfg)
		}
	}

	elapsed := time.Since(start).Milliseconds()
	ok, failed := countResults(results)
	return PushSummary{Total: len(results), OK: ok, Failed: failed, Duration: elapsed, Results: results}
}

func PushTariffs(cfg PushConfig, seed *fakegen.SeedData, emspURL, tokenB string) PushSummary {
	start := time.Now()
	tariffs := selectTariffs(seed, cfg.Count)
	results := make([]PushResult, 0, len(tariffs))

	for i, t := range tariffs {
		if cfg.Mutate {
			t = mutateTariff(t)
		}

		url := fmt.Sprintf("%s/receiver/tariffs/%s/%s/%s",
			strings.TrimRight(emspURL, "/"), t.CountryCode, t.PartyID, t.ID)
		result := doPush("PUT", url, tokenB, t)
		results = append(results, result)

		if i < len(tariffs)-1 {
			sleepForPattern(cfg)
		}
	}

	elapsed := time.Since(start).Milliseconds()
	ok, failed := countResults(results)
	return PushSummary{Total: len(results), OK: ok, Failed: failed, Duration: elapsed, Results: results}
}

func doPush(method, url, tokenB string, payload any) PushResult {
	body, err := json.Marshal(payload)
	if err != nil {
		return PushResult{URL: url, Method: method, Error: fmt.Sprintf("marshal: %v", err)}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return PushResult{URL: url, Method: method, Error: fmt.Sprintf("request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	if tokenB != "" {
		req.Header.Set("Authorization", "Token "+tokenB)
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	dur := time.Since(start).Milliseconds()

	if err != nil {
		log.Printf("[push] %s %s -> error: %v", method, url, err)
		return PushResult{URL: url, Method: method, DurationMS: dur, Error: fmt.Sprintf("http: %v", err)}
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	log.Printf("[push] %s %s -> %d (%dms)", method, url, resp.StatusCode, dur)
	return PushResult{URL: url, Method: method, StatusCode: resp.StatusCode, DurationMS: dur}
}

func selectLocations(seed *fakegen.SeedData, count int) []fakegen.Location {
	if count <= 0 || count >= len(seed.Locations) {
		out := make([]fakegen.Location, len(seed.Locations))
		copy(out, seed.Locations)
		return out
	}
	perm := rand.Perm(len(seed.Locations))
	out := make([]fakegen.Location, count)
	for i := 0; i < count; i++ {
		out[i] = seed.Locations[perm[i]]
	}
	return out
}

func selectTariffs(seed *fakegen.SeedData, count int) []fakegen.Tariff {
	if count <= 0 || count >= len(seed.Tariffs) {
		out := make([]fakegen.Tariff, len(seed.Tariffs))
		copy(out, seed.Tariffs)
		return out
	}
	perm := rand.Perm(len(seed.Tariffs))
	out := make([]fakegen.Tariff, count)
	for i := 0; i < count; i++ {
		out[i] = seed.Tariffs[perm[i]]
	}
	return out
}

func mutateLocation(loc fakegen.Location) fakegen.Location {
	now := time.Now().UTC().Format(time.RFC3339)
	loc.LastUpdated = now
	evses := make([]fakegen.EVSE, len(loc.EVSEs))
	copy(evses, loc.EVSEs)
	for i := range evses {
		evses[i].Status = evseStatuses[rand.Intn(len(evseStatuses))]
		evses[i].LastUpdated = now
		conns := make([]fakegen.Connector, len(evses[i].Connectors))
		copy(conns, evses[i].Connectors)
		for j := range conns {
			conns[j].LastUpdated = now
		}
		evses[i].Connectors = conns
	}
	loc.EVSEs = evses
	return loc
}

func mutateTariff(t fakegen.Tariff) fakegen.Tariff {
	now := time.Now().UTC().Format(time.RFC3339)
	t.LastUpdated = now
	elems := make([]fakegen.TariffElement, len(t.Elements))
	copy(elems, t.Elements)
	for i := range elems {
		pcs := make([]fakegen.PriceComponent, len(elems[i].PriceComponents))
		copy(pcs, elems[i].PriceComponents)
		for j := range pcs {
			factor := 0.85 + rand.Float64()*0.30 // +/- 15%
			pcs[j].Price = roundTo(pcs[j].Price*factor, 4)
		}
		elems[i].PriceComponents = pcs
	}
	t.Elements = elems
	return t
}

func sleepForPattern(cfg PushConfig) {
	switch cfg.Pattern {
	case "burst":
		return
	case "staggered":
		minD := cfg.MinDelayMS
		maxD := cfg.MaxDelayMS
		if minD <= 0 {
			minD = 50
		}
		if maxD <= minD {
			maxD = minD + 450
		}
		d := minD + rand.Intn(maxD-minD)
		time.Sleep(time.Duration(d) * time.Millisecond)
	case "realistic":
		// Small batch pauses: every 2-5 items, pause 1-3s
		d := 1000 + rand.Intn(2000)
		time.Sleep(time.Duration(d) * time.Millisecond)
	default:
		return
	}
}

func countResults(results []PushResult) (ok, failed int) {
	for _, r := range results {
		if r.Error == "" && r.StatusCode >= 200 && r.StatusCode < 300 {
			ok++
		} else {
			failed++
		}
	}
	return
}
