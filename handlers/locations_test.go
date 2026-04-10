package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func TestGetEVSE_Found(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations/LOC-1/EVSE-1", nil)
	r = withChiParams(r, map[string]string{"locationID": "LOC-1", "evseUID": "EVSE-1"})

	h.GetEVSE(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var evse fakegen.EVSE
	json.Unmarshal(resp.Data, &evse)
	if evse.UID != "EVSE-1" {
		t.Errorf("expected UID EVSE-1, got %s", evse.UID)
	}
}

func TestGetEVSE_LocationNotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations/NOPE/EVSE-1", nil)
	r = withChiParams(r, map[string]string{"locationID": "NOPE", "evseUID": "EVSE-1"})

	h.GetEVSE(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetEVSE_EVSENotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations/LOC-1/NOPE", nil)
	r = withChiParams(r, map[string]string{"locationID": "LOC-1", "evseUID": "NOPE"})

	h.GetEVSE(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetConnector_Found(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations/LOC-1/EVSE-1/C1", nil)
	r = withChiParams(r, map[string]string{"locationID": "LOC-1", "evseUID": "EVSE-1", "connectorID": "C1"})

	h.GetConnector(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var conn fakegen.Connector
	json.Unmarshal(resp.Data, &conn)
	if conn.ID != "C1" {
		t.Errorf("expected connector ID C1, got %s", conn.ID)
	}
}

func TestGetConnector_NotFound(t *testing.T) {
	h := testHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations/LOC-1/EVSE-1/NOPE", nil)
	r = withChiParams(r, map[string]string{"locationID": "LOC-1", "evseUID": "EVSE-1", "connectorID": "NOPE"})

	h.GetConnector(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestGetEVSE_ChargingOverlay(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]string{
		"id": "SESS-1", "evse_uid": "EVSE-1", "status": "ACTIVE",
		"country_code": "DE", "party_id": "AAA",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations/LOC-1/EVSE-1", nil)
	r = withChiParams(r, map[string]string{"locationID": "LOC-1", "evseUID": "EVSE-1"})

	h.GetEVSE(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var evse fakegen.EVSE
	json.Unmarshal(resp.Data, &evse)
	if evse.Status != "CHARGING" {
		t.Errorf("expected EVSE status CHARGING, got %s", evse.Status)
	}
}

func TestGetLocation_ChargingOverlay(t *testing.T) {
	h := testHandler()
	store := h.Store.(*testStore)

	session := map[string]string{
		"id": "SESS-1", "evse_uid": "EVSE-1", "status": "ACTIVE",
		"country_code": "DE", "party_id": "AAA",
	}
	data, _ := json.Marshal(session)
	store.PutSession("SESS-1", data)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ocpi/2.2.1/sender/locations/LOC-1", nil)
	r = withChiParams(r, map[string]string{"locationID": "LOC-1"})

	h.GetLocation(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var loc fakegen.Location
	json.Unmarshal(resp.Data, &loc)
	if len(loc.EVSEs) == 0 {
		t.Fatal("expected EVSEs in response")
	}
	if loc.EVSEs[0].Status != "CHARGING" {
		t.Errorf("expected EVSE status CHARGING, got %s", loc.EVSEs[0].Status)
	}
}
