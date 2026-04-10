package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartSession_Happy(t *testing.T) {
	h := testHandler()

	body := `{"response_url":"http://localhost/callback","location_id":"LOC-1","token":{"uid":"TOK1","type":"RFID"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/START_SESSION", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "START_SESSION"})

	h.PostCommand(w, r)

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

	sessions, _ := h.Store.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session stored, got %d", len(sessions))
	}
}

func TestStartSession_RejectMode(t *testing.T) {
	h := testHandler()
	h.Store.(*testStore).mode = "reject"

	body := `{"location_id":"LOC-1","token":{"uid":"TOK1"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/START_SESSION", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "START_SESSION"})

	h.PostCommand(w, r)

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "REJECTED" {
		t.Errorf("expected REJECTED, got %s", result["result"])
	}
}

func TestStartSession_UnknownLocation(t *testing.T) {
	h := testHandler()

	body := `{"location_id":"NONEXISTENT","token":{"uid":"TOK1"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/START_SESSION", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "START_SESSION"})

	h.PostCommand(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestStopSession_Happy(t *testing.T) {
	h := testHandler()

	// First start a session
	startBody := `{"location_id":"LOC-1","token":{"uid":"TOK1"}}`
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("POST", "/commands/START_SESSION", strings.NewReader(startBody))
	r1 = withChiParams(r1, map[string]string{"command": "START_SESSION"})
	h.PostCommand(w1, r1)

	sessions, _ := h.Store.ListSessions()
	if len(sessions) != 1 {
		t.Fatal("expected 1 session")
	}

	var sess SessionRecord
	json.Unmarshal(sessions[0], &sess)

	// Now stop it
	stopBody := `{"session_id":"` + sess.ID + `","response_url":"http://localhost/cb"}`
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("POST", "/commands/STOP_SESSION", strings.NewReader(stopBody))
	r2 = withChiParams(r2, map[string]string{"command": "STOP_SESSION"})
	h.PostCommand(w2, r2)

	var resp ocpiResp
	json.Unmarshal(w2.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "ACCEPTED" {
		t.Errorf("expected ACCEPTED, got %s", result["result"])
	}

	raw, _ := h.Store.GetSession(sess.ID)
	var updated SessionRecord
	json.Unmarshal(raw, &updated)
	if updated.Status != "STOPPING" {
		t.Errorf("expected STOPPING, got %s", updated.Status)
	}
}

func TestReserveNow_Happy(t *testing.T) {
	h := testHandler()

	body := `{"reservation_id":"RES-1","location_id":"LOC-1","expiry_date":"2030-01-01T00:00:00Z","token":{"uid":"TOK1"},"response_url":"http://localhost/cb"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/RESERVE_NOW", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "RESERVE_NOW"})

	h.PostCommand(w, r)

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

	reservations, _ := h.Store.ListReservations()
	if len(reservations) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(reservations))
	}
}

func TestReserveNow_RejectMode(t *testing.T) {
	h := testHandler()
	h.Store.(*testStore).mode = "reject"

	body := `{"reservation_id":"RES-1","location_id":"LOC-1","token":{"uid":"TOK1"}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/RESERVE_NOW", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "RESERVE_NOW"})

	h.PostCommand(w, r)

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "REJECTED" {
		t.Errorf("expected REJECTED, got %s", result["result"])
	}
}

func TestCancelReservation_Found(t *testing.T) {
	h := testHandler()

	// First reserve
	resBody := `{"reservation_id":"RES-1","location_id":"LOC-1","token":{"uid":"TOK1"}}`
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("POST", "/commands/RESERVE_NOW", strings.NewReader(resBody))
	r1 = withChiParams(r1, map[string]string{"command": "RESERVE_NOW"})
	h.PostCommand(w1, r1)

	// Then cancel
	cancelBody := `{"reservation_id":"RES-1"}`
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("POST", "/commands/CANCEL_RESERVATION", strings.NewReader(cancelBody))
	r2 = withChiParams(r2, map[string]string{"command": "CANCEL_RESERVATION"})
	h.PostCommand(w2, r2)

	var resp ocpiResp
	json.Unmarshal(w2.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "ACCEPTED" {
		t.Errorf("expected ACCEPTED, got %s", result["result"])
	}

	reservations, _ := h.Store.ListReservations()
	if len(reservations) != 0 {
		t.Errorf("expected 0 reservations after cancel, got %d", len(reservations))
	}
}

func TestCancelReservation_NotFound(t *testing.T) {
	h := testHandler()

	body := `{"reservation_id":"NONEXISTENT"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/CANCEL_RESERVATION", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "CANCEL_RESERVATION"})

	h.PostCommand(w, r)

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "REJECTED" {
		t.Errorf("expected REJECTED for non-existent reservation, got %s", result["result"])
	}
}

func TestStopSession_EmptySessionID(t *testing.T) {
	h := testHandler()

	body := `{}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/STOP_SESSION", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "STOP_SESSION"})

	h.PostCommand(w, r)

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "REJECTED" {
		t.Errorf("expected REJECTED for empty session_id, got %s", result["result"])
	}
}

func TestStopSession_NonexistentSession(t *testing.T) {
	h := testHandler()

	body := `{"session_id":"DOES-NOT-EXIST"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/STOP_SESSION", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "STOP_SESSION"})

	h.PostCommand(w, r)

	var resp ocpiResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	if result["result"] != "REJECTED" {
		t.Errorf("expected REJECTED for nonexistent session, got %s", result["result"])
	}
}

func TestStopSession_ResponseURLReset(t *testing.T) {
	h := testHandler()

	startBody := `{"location_id":"LOC-1","response_url":"http://localhost/start-cb","token":{"uid":"TOK1"}}`
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("POST", "/commands/START_SESSION", strings.NewReader(startBody))
	r1 = withChiParams(r1, map[string]string{"command": "START_SESSION"})
	h.PostCommand(w1, r1)

	sessions, _ := h.Store.ListSessions()
	var sess SessionRecord
	json.Unmarshal(sessions[0], &sess)

	stopBody := `{"session_id":"` + sess.ID + `","response_url":"http://localhost/stop-cb"}`
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("POST", "/commands/STOP_SESSION", strings.NewReader(stopBody))
	r2 = withChiParams(r2, map[string]string{"command": "STOP_SESSION"})
	h.PostCommand(w2, r2)

	raw, _ := h.Store.GetSession(sess.ID)
	var updated SessionRecord
	json.Unmarshal(raw, &updated)

	if updated.ResponseURL != "http://localhost/stop-cb" {
		t.Errorf("expected ResponseURL updated to stop-cb, got %s", updated.ResponseURL)
	}
	if updated.CallbackSent {
		t.Error("expected CallbackSent reset to false after STOP_SESSION with new ResponseURL")
	}
}

func TestUnlockConnector_Happy(t *testing.T) {
	h := testHandler()

	body := `{"location_id":"LOC-1","evse_uid":"EVSE-1","connector_id":"C1","response_url":"http://localhost/cb"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/UNLOCK_CONNECTOR", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "UNLOCK_CONNECTOR"})

	h.PostCommand(w, r)

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
}

func TestUnlockConnector_UnknownLocation(t *testing.T) {
	h := testHandler()

	body := `{"location_id":"NONEXISTENT","evse_uid":"E1","connector_id":"C1"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/commands/UNLOCK_CONNECTOR", strings.NewReader(body))
	r = withChiParams(r, map[string]string{"command": "UNLOCK_CONNECTOR"})

	h.PostCommand(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}
