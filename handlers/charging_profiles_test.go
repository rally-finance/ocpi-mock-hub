package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// captureCallback stands up a fake eMSP response_url endpoint that records the
// first callback it receives. It returns the URL and a channel the test can
// read from, with a timeout.
type captured struct {
	method  string
	path    string
	headers http.Header
	body    []byte
}

func captureCallback(t *testing.T) (string, <-chan captured, *httptest.Server) {
	t.Helper()
	ch := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case ch <- captured{method: r.Method, path: r.URL.Path, headers: r.Header.Clone(), body: body}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	return srv.URL, ch, srv
}

func waitCallback(t *testing.T, ch <-chan captured) captured {
	t.Helper()
	select {
	case c := <-ch:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async callback")
		return captured{}
	}
}

func TestPutChargingProfile_SyncShape_Accepted(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.sessions["SESS-1"] = []byte(`{"id":"SESS-1","status":"ACTIVE"}`)

	cbURL, ch, srv := captureCallback(t)
	defer srv.Close()

	body := `{"charging_profile":{"charging_rate_unit":"W","min_charging_rate":3700},"response_url":"` + cbURL + `/cb-put"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/chargingprofiles/SESS-1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.PutChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad envelope: %v", err)
	}
	if resp.StatusCode != 1000 {
		t.Errorf("envelope status_code: got %d, want 1000", resp.StatusCode)
	}
	var data struct {
		Result  string `json:"result"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("bad data: %v", err)
	}
	if data.Result != "ACCEPTED" {
		t.Errorf("sync result: got %q, want ACCEPTED", data.Result)
	}
	if data.Timeout <= 0 {
		t.Errorf("sync timeout: got %d, want >0", data.Timeout)
	}

	if stored, _ := store.GetChargingProfile("SESS-1"); stored == nil {
		t.Error("profile not persisted")
	}

	cb := waitCallback(t, ch)
	if cb.method != "POST" {
		t.Errorf("callback method: got %s, want POST", cb.method)
	}
	if cb.path != "/cb-put" {
		t.Errorf("callback path: got %s, want /cb-put", cb.path)
	}
	var cbBody struct {
		Result  string `json:"result"`
		Profile any    `json:"profile"`
	}
	if err := json.Unmarshal(cb.body, &cbBody); err != nil {
		t.Fatalf("bad callback json: %v", err)
	}
	if cbBody.Result != "ACCEPTED" {
		t.Errorf("callback result: got %q, want ACCEPTED", cbBody.Result)
	}
	// ChargingProfileResult is {result} only — it should not include a profile wrapper.
	if cbBody.Profile != nil {
		t.Errorf("PUT callback should not include profile wrapper, got: %v", cbBody.Profile)
	}
}

func TestPutChargingProfile_UnknownSession(t *testing.T) {
	h := testHandler()

	body := `{"charging_profile":{"charging_rate_unit":"W"},"response_url":"http://localhost/cb"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/chargingprofiles/NOPE", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"sessionID": "NOPE"})

	h.PutChargingProfile(w, r)

	// Spec: UNKNOWN_SESSION is a domain-level result, not an OCPI/HTTP error.
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.StatusCode != 1000 {
		t.Errorf("envelope status_code: got %d, want 1000", resp.StatusCode)
	}
	var data struct {
		Result  string `json:"result"`
		Timeout int    `json:"timeout"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Result != "UNKNOWN_SESSION" {
		t.Errorf("result: got %q, want UNKNOWN_SESSION", data.Result)
	}
	if data.Timeout <= 0 {
		t.Errorf("timeout should still be advertised, got %d", data.Timeout)
	}
}

func TestPutChargingProfile_MissingResponseURL(t *testing.T) {
	h := testHandler()

	body := `{"charging_profile":{"charging_rate_unit":"W"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/chargingprofiles/SESS-1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.PutChargingProfile(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestGetChargingProfile_AsyncDelivery(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.sessions["SESS-1"] = []byte(`{"id":"SESS-1","status":"ACTIVE"}`)
	store.PutChargingProfile("SESS-1", []byte(`{"charging_rate_unit":"W","min_charging_rate":3700}`))

	cbURL, ch, srv := captureCallback(t)
	defer srv.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET",
		"/ocpi/2.2.1/receiver/chargingprofiles/SESS-1?duration=600&response_url="+cbURL+"/cb-get", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.GetChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var sync struct {
		Result  string `json:"result"`
		Timeout int    `json:"timeout"`
	}
	json.Unmarshal(resp.Data, &sync)
	if sync.Result != "ACCEPTED" {
		t.Errorf("sync result: got %q, want ACCEPTED", sync.Result)
	}
	// The sync body must NOT carry the profile — delivery is async per spec.
	if strings.Contains(string(resp.Data), "charging_rate_unit") {
		t.Errorf("sync body leaked the ActiveChargingProfile, want async-only delivery: %s", string(resp.Data))
	}

	cb := waitCallback(t, ch)
	if cb.method != "POST" || cb.path != "/cb-get" {
		t.Errorf("unexpected callback: %s %s", cb.method, cb.path)
	}
	var cbBody struct {
		Result  string `json:"result"`
		Profile *struct {
			StartDateTime   string          `json:"start_date_time"`
			ChargingProfile json.RawMessage `json:"charging_profile"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(cb.body, &cbBody); err != nil {
		t.Fatalf("bad callback json: %v", err)
	}
	if cbBody.Result != "ACCEPTED" {
		t.Errorf("callback result: got %q, want ACCEPTED", cbBody.Result)
	}
	if cbBody.Profile == nil {
		t.Fatal("callback should include ActiveChargingProfile")
	}
	if cbBody.Profile.StartDateTime == "" {
		t.Error("ActiveChargingProfile.start_date_time must be present")
	}
	if !strings.Contains(string(cbBody.Profile.ChargingProfile), "charging_rate_unit") {
		t.Errorf("ActiveChargingProfile.charging_profile missing stored content: %s",
			string(cbBody.Profile.ChargingProfile))
	}
}

func TestGetChargingProfile_UnknownSession(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET",
		"/ocpi/2.2.1/receiver/chargingprofiles/NOPE?duration=600&response_url=http://localhost/cb", nil)
	r = withChiParams(r, map[string]string{"sessionID": "NOPE"})

	h.GetChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data struct {
		Result string `json:"result"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Result != "UNKNOWN_SESSION" {
		t.Errorf("result: got %q, want UNKNOWN_SESSION", data.Result)
	}
}

func TestGetChargingProfile_MissingResponseURL(t *testing.T) {
	h := testHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/receiver/chargingprofiles/SESS-1?duration=600", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.GetChargingProfile(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestGetChargingProfile_NoStoredProfileYieldsSynthetic(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.sessions["SESS-1"] = []byte(`{"id":"SESS-1","status":"ACTIVE"}`)

	cbURL, ch, srv := captureCallback(t)
	defer srv.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET",
		"/ocpi/2.2.1/receiver/chargingprofiles/SESS-1?duration=600&response_url="+cbURL+"/cb", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.GetChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	cb := waitCallback(t, ch)
	if !strings.Contains(string(cb.body), "\"charging_rate_unit\"") {
		t.Errorf("synthesized profile missing charging_rate_unit: %s", string(cb.body))
	}
}

func TestDeleteChargingProfile_AsyncClearProfileResult(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.sessions["SESS-1"] = []byte(`{"id":"SESS-1","status":"ACTIVE"}`)
	store.PutChargingProfile("SESS-1", []byte(`{"charging_rate_unit":"W"}`))

	cbURL, ch, srv := captureCallback(t)
	defer srv.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE",
		"/ocpi/2.2.1/receiver/chargingprofiles/SESS-1?response_url="+cbURL+"/cb-del", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.DeleteChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var sync struct {
		Result  string `json:"result"`
		Timeout int    `json:"timeout"`
	}
	json.Unmarshal(resp.Data, &sync)
	if sync.Result != "ACCEPTED" {
		t.Errorf("sync result: got %q, want ACCEPTED", sync.Result)
	}
	if sync.Timeout <= 0 {
		t.Errorf("sync timeout must be >0, got %d", sync.Timeout)
	}

	if stored, _ := store.GetChargingProfile("SESS-1"); stored != nil {
		t.Error("profile should be deleted")
	}

	cb := waitCallback(t, ch)
	if cb.method != "POST" || cb.path != "/cb-del" {
		t.Errorf("unexpected callback: %s %s", cb.method, cb.path)
	}
	var cbBody struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(cb.body, &cbBody); err != nil {
		t.Fatalf("bad callback json: %v", err)
	}
	if cbBody.Result != "ACCEPTED" {
		t.Errorf("DELETE callback result: got %q, want ACCEPTED", cbBody.Result)
	}
}

func TestDeleteChargingProfile_NothingToClearReportsUnknown(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)
	store.sessions["SESS-1"] = []byte(`{"id":"SESS-1","status":"ACTIVE"}`)

	cbURL, ch, srv := captureCallback(t)
	defer srv.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE",
		"/ocpi/2.2.1/receiver/chargingprofiles/SESS-1?response_url="+cbURL+"/cb-del", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.DeleteChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	// Sync response is still ACCEPTED (the CPO accepted the request);
	// only the async result enum distinguishes "nothing to clear".
	cb := waitCallback(t, ch)
	var cbBody struct {
		Result string `json:"result"`
	}
	json.Unmarshal(cb.body, &cbBody)
	if cbBody.Result != "UNKNOWN" {
		t.Errorf("callback result: got %q, want UNKNOWN (nothing to clear)", cbBody.Result)
	}
}

func TestDeleteChargingProfile_UnknownSession(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE",
		"/ocpi/2.2.1/receiver/chargingprofiles/NOPE?response_url=http://localhost/cb", nil)
	r = withChiParams(r, map[string]string{"sessionID": "NOPE"})

	h.DeleteChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var data struct {
		Result string `json:"result"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.Result != "UNKNOWN_SESSION" {
		t.Errorf("result: got %q, want UNKNOWN_SESSION", data.Result)
	}
}

func TestDeleteChargingProfile_MissingResponseURL(t *testing.T) {
	h := testHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/ocpi/2.2.1/receiver/chargingprofiles/SESS-1", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.DeleteChargingProfile(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestPushActiveChargingProfile_SendsPUT(t *testing.T) {
	h := testHandler()

	ch := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ch <- captured{method: r.Method, path: r.URL.Path, headers: r.Header.Clone(), body: body}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := h.PushActiveChargingProfile(context.Background(), h.Store, srv.URL+"/chargingprofiles/SESS-42", nil)
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}

	cb := waitCallback(t, ch)
	if cb.method != "PUT" {
		t.Errorf("push method: got %s, want PUT", cb.method)
	}
	if cb.path != "/chargingprofiles/SESS-42" {
		t.Errorf("push path: got %s", cb.path)
	}
	var body struct {
		StartDateTime   string          `json:"start_date_time"`
		ChargingProfile json.RawMessage `json:"charging_profile"`
	}
	if err := json.Unmarshal(cb.body, &body); err != nil {
		t.Fatalf("bad push body: %v", err)
	}
	if body.StartDateTime == "" {
		t.Error("push body missing start_date_time")
	}
	if len(body.ChargingProfile) == 0 {
		t.Error("push body missing charging_profile")
	}
}

func TestPushActiveChargingProfile_RejectsBadURL(t *testing.T) {
	h := testHandler()
	if err := h.PushActiveChargingProfile(context.Background(), h.Store, "not-a-url", nil); err == nil {
		t.Error("expected error for malformed target_url")
	}
}

func TestPushActiveProfile_AdminHandler(t *testing.T) {
	h := testHandler()

	ch := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ch <- captured{method: r.Method, path: r.URL.Path, body: body}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body := `{"session_id":"SESS-1","target_url":"` + srv.URL + `/chargingprofiles/SESS-1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/push-active-profile", strings.NewReader(body))

	h.PushActiveProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200: %s", w.Code, w.Body.String())
	}

	cb := waitCallback(t, ch)
	if cb.method != "PUT" {
		t.Errorf("admin-triggered push method: got %s, want PUT", cb.method)
	}
}
