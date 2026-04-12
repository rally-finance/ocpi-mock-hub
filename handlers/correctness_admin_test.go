package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/correctness"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func testCorrectnessHandler() *Handler {
	seed := &fakegen.SeedData{
		Locations: []fakegen.Location{
			{
				CountryCode: "NL",
				PartyID:     "EMS",
				ID:          "LOC-1",
				Name:        "Test Location",
				Address:     "Main Street 1",
				City:        "Amsterdam",
				PostalCode:  "1000AA",
				Country:     "NLD",
				Coordinates: fakegen.Coords{Latitude: "52.3676", Longitude: "4.9041"},
				TimeZone:    "Europe/Amsterdam",
				ParkingType: "ON_STREET",
				Publish:     true,
				EVSEs: []fakegen.EVSE{
					{
						UID:         "EVSE-1",
						EvseID:      "NL*EMS*E1",
						Status:      "AVAILABLE",
						LastUpdated: "2026-01-01T00:00:00Z",
						Connectors: []fakegen.Connector{
							{
								ID:               "C1",
								Standard:         "IEC_62196_T2",
								Format:           "SOCKET",
								PowerType:        "AC_3_PHASE",
								MaxVoltage:       400,
								MaxAmperage:      16,
								MaxElectricPower: 11000,
								LastUpdated:      "2026-01-01T00:00:00Z",
							},
						},
					},
				},
				LastUpdated: "2026-01-01T00:00:00Z",
			},
		},
		Tariffs: []fakegen.Tariff{
			{
				CountryCode: "NL",
				PartyID:     "EMS",
				ID:          "TAR-1",
				Currency:    "EUR",
				Type:        "REGULAR",
				Elements: []fakegen.TariffElement{
					{
						PriceComponents: []fakegen.PriceComponent{
							{Type: "ENERGY", Price: 0.31, StepSize: 1},
						},
					},
				},
				LastUpdated: "2026-01-01T00:00:00Z",
			},
		},
	}

	return &Handler{
		Config: HandlerConfig{
			TokenA:     "test-token-a",
			HubCountry: "NL",
			HubParty:   "HUB",
		},
		Store:       newTestStore(),
		Seed:        seed,
		ReqLog:      NewRequestLog(),
		Correctness: correctness.NewManager(seed),
	}
}

func createCorrectnessSessionForTest(t *testing.T, h *Handler) correctness.TestSession {
	t.Helper()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/test-sessions", strings.NewReader(`{
		"peer_versions_url":"https://peer.example.com/ocpi/versions",
		"peer_token":"peer-token"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CreateCorrectnessSession(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create session status: got %d, want 201, body=%s", w.Code, w.Body.String())
	}

	var session correctness.TestSession
	if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return session
}

func TestCorrectnessSessionEndpointsCreateListAndGet(t *testing.T) {
	h := testCorrectnessHandler()
	created := createCorrectnessSessionForTest(t, h)

	listW := httptest.NewRecorder()
	listR := httptest.NewRequest("GET", "/admin/test-sessions", nil)
	h.ListCorrectnessSessions(listW, listR)
	if listW.Code != http.StatusOK {
		t.Fatalf("list sessions status: got %d, want 200", listW.Code)
	}

	var snapshots []correctness.SessionSnapshot
	if err := json.Unmarshal(listW.Body.Bytes(), &snapshots); err != nil {
		t.Fatalf("decode snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if snapshots[0].ID != created.ID {
		t.Fatalf("expected snapshot ID %q, got %q", created.ID, snapshots[0].ID)
	}

	getW := httptest.NewRecorder()
	getR := withChiParams(httptest.NewRequest("GET", "/admin/test-sessions/"+created.ID, nil), map[string]string{
		"sessionID": created.ID,
	})
	h.GetCorrectnessSession(getW, getR)
	if getW.Code != http.StatusOK {
		t.Fatalf("get session status: got %d, want 200", getW.Code)
	}

	var fetched correctness.TestSession
	if err := json.Unmarshal(getW.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode fetched session: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("expected fetched ID %q, got %q", created.ID, fetched.ID)
	}
	if fetched.SuiteName != "OCPI 2.2.1 eMSP Correctness Tests" {
		t.Fatalf("unexpected suite name: %q", fetched.SuiteName)
	}
}

func TestCreateCorrectnessSessionRejectsSecondActiveSession(t *testing.T) {
	h := testCorrectnessHandler()
	createCorrectnessSessionForTest(t, h)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/admin/test-sessions", strings.NewReader(`{
		"peer_versions_url":"https://peer.example.com/ocpi/versions",
		"peer_token":"peer-token"
	}`))
	r.Header.Set("Content-Type", "application/json")

	h.CreateCorrectnessSession(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when another session is active, got %d", w.Code)
	}
}

func TestRunCorrectnessActionCompletesObserveAction(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	w := httptest.NewRecorder()
	r := withChiParams(httptest.NewRequest("POST", "/admin/test-sessions/"+session.ID+"/actions/arm_pull_locations_full", nil), map[string]string{
		"sessionID": session.ID,
		"actionID":  "arm_pull_locations_full",
	})

	h.RunCorrectnessAction(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("run action status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var updated correctness.TestSession
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated session: %v", err)
	}

	found := false
	for _, action := range updated.Actions {
		if action.ID != "arm_pull_locations_full" {
			continue
		}
		found = true
		if action.Status != "completed" {
			t.Fatalf("expected completed action status, got %q", action.Status)
		}
	}
	if !found {
		t.Fatal("did not find action arm_pull_locations_full in session response")
	}
}

func TestSubmitCorrectnessCheckpointStoresAnswer(t *testing.T) {
	h := testCorrectnessHandler()
	session := createCorrectnessSessionForTest(t, h)

	w := httptest.NewRecorder()
	r := withChiParams(httptest.NewRequest(
		"POST",
		"/admin/test-sessions/"+session.ID+"/checkpoints/confirm_connector_removed_after_full_pull",
		strings.NewReader(`{"answer":"connector absent","notes":"verified manually"}`),
	), map[string]string{
		"sessionID":    session.ID,
		"checkpointID": "confirm_connector_removed_after_full_pull",
	})
	r.Header.Set("Content-Type", "application/json")

	h.SubmitCorrectnessCheckpoint(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("submit checkpoint status: got %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var updated correctness.TestSession
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated session: %v", err)
	}

	found := false
	for _, result := range updated.Cases {
		for _, checkpoint := range result.Checkpoints {
			if checkpoint.ID != "confirm_connector_removed_after_full_pull" {
				continue
			}
			found = true
			if checkpoint.Status != "answered" {
				t.Fatalf("expected answered checkpoint, got %q", checkpoint.Status)
			}
			if checkpoint.Answer != "connector absent" {
				t.Fatalf("unexpected checkpoint answer: %q", checkpoint.Answer)
			}
		}
	}
	if !found {
		t.Fatal("checkpoint was not returned in the updated session")
	}
}
