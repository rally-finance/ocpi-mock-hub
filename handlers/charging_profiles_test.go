package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPutChargingProfile_Happy(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]string{"id": "SESS-1", "status": "ACTIVE"}
	sData, _ := json.Marshal(session)
	store.PutSession("SESS-1", sData)

	body := `{"charging_profile":{"charging_rate_unit":"W","min_charging_rate":3700}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/chargingprofiles/SESS-1", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.PutChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "ACCEPTED" {
		t.Errorf("expected ACCEPTED, got %s", result["result"])
	}

	stored, _ := store.GetChargingProfile("SESS-1")
	if stored == nil {
		t.Error("charging profile not stored")
	}
}

func TestPutChargingProfile_NoSession(t *testing.T) {
	h := testHandler()

	body := `{"charging_profile":{"charging_rate_unit":"W"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/ocpi/2.2.1/receiver/chargingprofiles/NOPE", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"sessionID": "NOPE"})

	h.PutChargingProfile(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetChargingProfile_Found(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	profile := `{"charging_rate_unit":"W","min_charging_rate":3700}`
	store.PutChargingProfile("SESS-1", []byte(profile))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/receiver/chargingprofiles/SESS-1", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.GetChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]any
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "ACCEPTED" {
		t.Errorf("expected ACCEPTED, got %v", result["result"])
	}
	if result["profile"] == nil {
		t.Error("expected profile in response")
	}
}

func TestGetChargingProfile_NotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/receiver/chargingprofiles/NOPE", nil)
	r = withChiParams(r, map[string]string{"sessionID": "NOPE"})

	h.GetChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]any
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "ACCEPTED" {
		t.Errorf("expected ACCEPTED with default profile, got %v", result["result"])
	}
}

func TestDeleteChargingProfile_Happy(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	store.PutChargingProfile("SESS-1", []byte(`{"charging_rate_unit":"W"}`))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/ocpi/2.2.1/receiver/chargingprofiles/SESS-1", nil)
	r = withChiParams(r, map[string]string{"sessionID": "SESS-1"})

	h.DeleteChargingProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	stored, _ := store.GetChargingProfile("SESS-1")
	if stored != nil {
		t.Error("expected profile to be deleted")
	}
}
