package correctness

import (
	"errors"
	"testing"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

func testSeed() *fakegen.SeedData {
	return &fakegen.SeedData{
		Locations: []fakegen.Location{
			{
				CountryCode: "NL",
				PartyID:     "TST",
				ID:          "LOC-1",
				Name:        "Correctness Test Site",
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
						EvseID:      "NL*TST*E1",
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
				PartyID:     "TST",
				ID:          "TAR-1",
				Currency:    "EUR",
				Type:        "REGULAR",
				Elements: []fakegen.TariffElement{
					{
						PriceComponents: []fakegen.PriceComponent{
							{Type: "ENERGY", Price: 0.35, StepSize: 1},
						},
					},
				},
				LastUpdated: "2026-01-01T00:00:00Z",
			},
		},
	}
}

func TestManagerStartSessionDefaultsToCorrectnessSuite(t *testing.T) {
	manager := NewManager(testSeed())

	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if session.SuiteID != DefaultSuiteID {
		t.Fatalf("expected suite ID %q, got %q", DefaultSuiteID, session.SuiteID)
	}
	if session.Status != "running" {
		t.Fatalf("expected session status running, got %q", session.Status)
	}
	if session.CurrentStep.ActionID != "run_handshake" {
		t.Fatalf("expected first action to be run_handshake, got %q", session.CurrentStep.ActionID)
	}

	_, err = manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if !errors.Is(err, ErrActiveSessionExists) {
		t.Fatalf("expected ErrActiveSessionExists, got %v", err)
	}
}

func TestManagerRecordTrafficEventAssociatesWithActiveSession(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	manager.RecordTrafficEvent(TrafficEvent{
		Direction:      "inbound",
		Method:         "GET",
		URL:            "https://hub.example.com/ocpi/2.2.1/sender/locations?offset=0&limit=50",
		Path:           "/ocpi/2.2.1/sender/locations",
		RawQuery:       "offset=0&limit=50",
		RequestHeaders: map[string]string{"authorization": "Token peer-token"},
		ResponseStatus: 200,
		ResponseHeaders: map[string]string{
			"x-total-count": "1",
			"x-limit":       "50",
		},
		ResponseBody: `{"status_code":1000,"timestamp":"2026-01-01T00:00:00Z","data":[]}`,
		StartedAt:    "2026-01-01T00:00:00Z",
	})

	got, err := manager.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.EventCount != 1 {
		t.Fatalf("expected 1 recorded event, got %d", got.EventCount)
	}
	if len(got.RecentEvents) != 1 {
		t.Fatalf("expected 1 recent event, got %d", len(got.RecentEvents))
	}
	if got.RecentEvents[0].SessionID != session.ID {
		t.Fatalf("expected recorded event to be linked to session %q, got %q", session.ID, got.RecentEvents[0].SessionID)
	}
}

func TestManagerSubmitCheckpointStoresAnswer(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	updated, err := manager.SubmitCheckpoint(session.ID, "confirm_connector_removed_after_full_pull", "connector absent", "verified in peer UI")
	if err != nil {
		t.Fatalf("submit checkpoint: %v", err)
	}

	found := false
	for _, result := range updated.Cases {
		for _, checkpoint := range result.Checkpoints {
			if checkpoint.ID != "confirm_connector_removed_after_full_pull" {
				continue
			}
			found = true
			if checkpoint.Answer != "connector absent" {
				t.Fatalf("unexpected checkpoint answer: %q", checkpoint.Answer)
			}
			if checkpoint.Notes != "verified in peer UI" {
				t.Fatalf("unexpected checkpoint notes: %q", checkpoint.Notes)
			}
			if checkpoint.Status != "answered" {
				t.Fatalf("expected answered checkpoint status, got %q", checkpoint.Status)
			}
		}
	}
	if !found {
		t.Fatal("checkpoint was not present in the updated session")
	}
}

func TestManagerRerunCreatesFreshSandbox(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	originalAddress := manager.sessions[session.ID].sandbox.Seed.Locations[0].Address
	err = manager.UpdateSandbox(session.ID, func(sandbox *Sandbox) error {
		sandbox.Seed.Locations[0].Address = "Changed In Session"
		return nil
	})
	if err != nil {
		t.Fatalf("update sandbox: %v", err)
	}
	if got := manager.sessions[session.ID].sandbox.Seed.Locations[0].Address; got != "Changed In Session" {
		t.Fatalf("expected mutated address, got %q", got)
	}

	manager.activeID = ""
	rerun, err := manager.RerunSession(session.ID)
	if err != nil {
		t.Fatalf("rerun session: %v", err)
	}
	if rerun.ID == session.ID {
		t.Fatal("expected rerun to create a fresh session ID")
	}

	if got := manager.sessions[rerun.ID].sandbox.Seed.Locations[0].Address; got != originalAddress {
		t.Fatalf("expected rerun sandbox address %q, got %q", originalAddress, got)
	}
}
