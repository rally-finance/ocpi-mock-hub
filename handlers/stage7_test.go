package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// -- Credit CDR tests -------------------------------------------------------

func TestIssueCreditCDR_StoresAndNegatesTotals(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	original := map[string]any{
		"id":           "CDR-ORIG-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"total_cost":   map[string]any{"excl_vat": 10.0, "incl_vat": 12.0},
		"total_energy": 42.5,
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(original)
	store.PutCDR("CDR-ORIG-1", data)

	body := `{"cdr_id":"CDR-ORIG-1","push":false}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/credit-cdr", strings.NewReader(body))

	h.IssueCreditCDR(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Status            string `json:"status"`
		CreditCDRID       string `json:"credit_cdr_id"`
		CreditReferenceID string `json:"credit_reference_id"`
		Pushed            bool   `json:"pushed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "issued" {
		t.Errorf("status: got %q, want issued", resp.Status)
	}
	if resp.CreditReferenceID != "CDR-ORIG-1" {
		t.Errorf("credit_reference_id: got %q, want CDR-ORIG-1", resp.CreditReferenceID)
	}
	if resp.Pushed {
		t.Errorf("pushed: got true, want false (push=false in request)")
	}
	if !strings.HasPrefix(resp.CreditCDRID, "CDR-CREDIT-") {
		t.Errorf("credit_cdr_id prefix: got %q", resp.CreditCDRID)
	}

	raw, _ := store.GetCDR(resp.CreditCDRID)
	if raw == nil {
		t.Fatal("credit CDR not stored")
	}
	var credit map[string]any
	json.Unmarshal(raw, &credit)
	if credit["credit"] != true {
		t.Errorf("credit: got %v, want true", credit["credit"])
	}
	if credit["credit_reference_id"] != "CDR-ORIG-1" {
		t.Errorf("credit_reference_id in stored CDR: got %v", credit["credit_reference_id"])
	}
	price, _ := credit["total_cost"].(map[string]any)
	if price["excl_vat"].(float64) != -10.0 || price["incl_vat"].(float64) != -12.0 {
		t.Errorf("total_cost not negated: %v", price)
	}
	if credit["total_energy"].(float64) != -42.5 {
		t.Errorf("total_energy: got %v, want -42.5", credit["total_energy"])
	}
	if credit["id"] == "CDR-ORIG-1" {
		t.Errorf("credit CDR reused original id")
	}
}

func TestIssueCreditCDR_MissingCDR(t *testing.T) {
	h := testHandler()

	body := `{"cdr_id":"CDR-NOPE"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/credit-cdr", strings.NewReader(body))

	h.IssueCreditCDR(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestIssueCreditCDR_PushesToEMSP(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/receiver/cdrs") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Token own-emsp-token" {
			t.Errorf("auth header: got %q", got)
		}
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store.tokenB = "token-b"
	store.emspToken = "own-emsp-token"
	store.callbackURL = srv.URL

	original := map[string]any{
		"id":           "CDR-PSH-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"total_cost":   map[string]any{"excl_vat": 5.0, "incl_vat": 6.0},
		"total_energy": 7.0,
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(original)
	store.PutCDR("CDR-PSH-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/credit-cdr",
		strings.NewReader(`{"cdr_id":"CDR-PSH-1"}`))
	r = r.WithContext(context.Background())

	h.IssueCreditCDR(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d; body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["pushed"] != true {
		t.Errorf("pushed: got %v, want true; body=%s", resp["pushed"], w.Body.String())
	}
	if len(receivedBody) == 0 {
		t.Fatal("eMSP did not receive credit CDR push")
	}
	var pushed map[string]any
	json.Unmarshal(receivedBody, &pushed)
	if pushed["credit"] != true || pushed["credit_reference_id"] != "CDR-PSH-1" {
		t.Errorf("pushed payload not a credit CDR: %v", pushed)
	}
}

func TestIssueCreditCDR_RepeatDoesNotOverwrite(t *testing.T) {
	// Bugbot regression: the original buildCreditCDR derived the credit id
	// deterministically from the source id, so two credits against the same
	// CDR clobbered each other in storage. Each credit must now persist
	// under a distinct id.
	h := testHandler()
	store := h.Store.(*testStore)

	original := map[string]any{
		"id":           "CDR-DUP-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"total_cost":   map[string]any{"excl_vat": 1.0, "incl_vat": 1.21},
		"total_energy": 2.0,
		"last_updated": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(original)
	store.PutCDR("CDR-DUP-1", data)

	issue := func() string {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/admin/credit-cdr",
			strings.NewReader(`{"cdr_id":"CDR-DUP-1","push":false}`))
		h.IssueCreditCDR(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d; body=%s", w.Code, w.Body.String())
		}
		var resp struct {
			CreditCDRID string `json:"credit_cdr_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.CreditCDRID
	}

	first := issue()
	second := issue()
	if first == "" || second == "" {
		t.Fatalf("empty credit id: first=%q second=%q", first, second)
	}
	if first == second {
		t.Fatalf("repeat credits produced identical id %q — second would overwrite the first", first)
	}
	if got, _ := store.GetCDR(first); got == nil {
		t.Error("first credit CDR was overwritten or lost")
	}
	if got, _ := store.GetCDR(second); got == nil {
		t.Error("second credit CDR not stored")
	}
}

func TestBuildCreditCDR_DeepCopiesNestedObjects(t *testing.T) {
	// Bugbot regression: nested OCPI objects used to be shared references
	// between original and credit after a shallow map copy. Mutating the
	// credit's nested fields must never leak back into the source CDR.
	originalToken := map[string]any{"uid": "TOK-ORIG", "type": "RFID"}
	periods := []any{
		map[string]any{"start_date_time": "2026-01-01T00:00:00Z", "dimensions": []any{}},
	}
	original := map[string]any{
		"id":               "CDR-DEEP-1",
		"country_code":     "DE",
		"party_id":         "AAA",
		"cdr_token":        originalToken,
		"charging_periods": periods,
	}

	credit := buildCreditCDR(original, "CDR-DEEP-1")

	creditToken, _ := credit["cdr_token"].(map[string]any)
	if creditToken == nil {
		t.Fatal("cdr_token missing from credit")
	}
	creditToken["uid"] = "TOK-MUTATED"
	if originalToken["uid"] != "TOK-ORIG" {
		t.Errorf("mutation of credit.cdr_token leaked into original: %v", originalToken["uid"])
	}

	creditPeriods, _ := credit["charging_periods"].([]any)
	if len(creditPeriods) > 0 {
		if m, ok := creditPeriods[0].(map[string]any); ok {
			m["start_date_time"] = "2099-12-31T23:59:59Z"
		}
	}
	origFirst := periods[0].(map[string]any)
	if origFirst["start_date_time"] != "2026-01-01T00:00:00Z" {
		t.Errorf("mutation of credit.charging_periods leaked into original: %v", origFirst["start_date_time"])
	}
}

func TestBuildCreditCDR_PreservesIdentity(t *testing.T) {
	original := map[string]any{
		"id":           "CDR-1",
		"country_code": "DE",
		"party_id":     "AAA",
		"cdr_token":    map[string]any{"uid": "TOK-1", "type": "RFID"},
		"total_cost":   map[string]any{"excl_vat": 3.0, "incl_vat": 3.63},
	}
	credit := buildCreditCDR(original, "CDR-1")

	if credit["country_code"] != "DE" || credit["party_id"] != "AAA" {
		t.Error("party identity not preserved")
	}
	tok, _ := credit["cdr_token"].(map[string]any)
	if tok["uid"] != "TOK-1" {
		t.Error("cdr_token not copied over")
	}
	if credit["credit"] != true {
		t.Error("credit flag not set")
	}
	if credit["credit_reference_id"] != "CDR-1" {
		t.Error("credit_reference_id not set")
	}
	if credit["last_updated"] == nil || credit["last_updated"] == "" {
		t.Error("last_updated not refreshed")
	}
}

// -- Multi-party credentials tests -----------------------------------------

func TestRegisterCredentials_PersistsOneRolePerParty(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	body := `{
		"token": "remote-token-c",
		"url": "https://emsp.example.com/ocpi/versions",
		"country_code": "NL",
		"party_id": "MSP",
		"roles": [
			{"role": "EMSP", "country_code": "NL", "party_id": "MSP"},
			{"role": "EMSP", "country_code": "DE", "party_id": "MSP"}
		]
	}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/credentials", strings.NewReader(body))

	h.PutCredentials(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d; body=%s", w.Code, w.Body.String())
	}

	tokenB := store.tokenB
	if tokenB == "" {
		t.Fatal("TokenB not issued")
	}

	// Both roles should be stored and share the same TokenB.
	nl, _ := store.GetParty("NL/MSP")
	de, _ := store.GetParty("DE/MSP")
	if nl == nil || de == nil {
		t.Fatalf("party state not persisted: NL=%v DE=%v", nl, de)
	}

	var pNL, pDE struct {
		CountryCode string `json:"country_code"`
		PartyID     string `json:"party_id"`
		TokenB      string `json:"token_b"`
		Role        string `json:"role"`
	}
	json.Unmarshal(nl, &pNL)
	json.Unmarshal(de, &pDE)
	if pNL.TokenB != tokenB || pDE.TokenB != tokenB {
		t.Errorf("roles should share TokenB: nl=%s de=%s issued=%s", pNL.TokenB, pDE.TokenB, tokenB)
	}
	if pNL.Role != "EMSP" || pDE.Role != "EMSP" {
		t.Errorf("role not stored: nl=%q de=%q", pNL.Role, pDE.Role)
	}
}

func TestRegisterCredentials_SingleRoleFallback(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	// Pre-2.2.1 style: no roles[], just top-level country_code/party_id.
	body := `{
		"token": "remote-token-c",
		"url": "https://emsp.example.com/ocpi/versions",
		"country_code": "NL",
		"party_id": "MSP"
	}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/credentials", strings.NewReader(body))

	h.PutCredentials(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d; body=%s", w.Code, w.Body.String())
	}
	if got, _ := store.GetParty("NL/MSP"); got == nil {
		t.Error("fallback single-role party not persisted")
	}
}

// The admin credit-CDR endpoint should ignore the eMSP push target when push=false.
func TestIssueCreditCDR_NoPushWhenDisabled(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	var called int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store.tokenB = "tb"
	store.emspToken = "own"
	store.callbackURL = srv.URL
	data, _ := json.Marshal(map[string]any{"id": "CDR-X", "country_code": "DE", "party_id": "AAA"})
	store.PutCDR("CDR-X", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/credit-cdr",
		bytes.NewReader([]byte(`{"cdr_id":"CDR-X","push":false}`)))

	h.IssueCreditCDR(w, r)

	if called != 0 {
		t.Errorf("expected no HTTP push when push=false, got %d calls", called)
	}
}
