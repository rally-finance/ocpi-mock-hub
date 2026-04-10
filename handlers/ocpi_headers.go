package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

// filterRawByParty filters raw JSON records by country_code and party_id.
func filterRawByParty(items [][]byte, cc, pid string) [][]byte {
	if cc == "" || pid == "" {
		return items
	}
	var party struct {
		CountryCode string `json:"country_code"`
		PartyID     string `json:"party_id"`
	}
	result := make([][]byte, 0, len(items))
	for _, b := range items {
		if json.Unmarshal(b, &party) != nil {
			continue
		}
		if strings.ToUpper(party.CountryCode) == cc && strings.ToUpper(party.PartyID) == pid {
			result = append(result, b)
		}
	}
	return result
}

// parsePaging returns OCPI paging params, forcing Limit=1 under pagination-stress mode.
func (h *Handler) parsePaging(r *http.Request, defaultLimit int) ocpiutil.Paging {
	p := ocpiutil.ParsePaging(r, defaultLimit)
	mode, _ := h.Store.GetMode()
	if mode == "pagination-stress" {
		p.Limit = 1
	}
	return p
}
