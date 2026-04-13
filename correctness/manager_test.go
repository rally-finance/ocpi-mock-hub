package correctness

import (
	"errors"
	"strings"
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

func loadRuntime(t *testing.T, manager *Manager, sessionID string) *sessionRuntime {
	t.Helper()

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := manager.loadStateLocked(); err != nil {
		t.Fatalf("load state: %v", err)
	}
	rt := manager.sessions[sessionID]
	if rt == nil {
		t.Fatalf("session %q not found", sessionID)
	}
	return rt
}

func mutateManagerState(t *testing.T, manager *Manager, fn func() error) {
	t.Helper()

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := manager.withStateMutationLocked(fn); err != nil {
		t.Fatalf("mutate manager state: %v", err)
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

	originalAddress := loadRuntime(t, manager, session.ID).sandbox.Seed.Locations[0].Address
	err = manager.UpdateSandbox(session.ID, func(sandbox *Sandbox) error {
		sandbox.Seed.Locations[0].Address = "Changed In Session"
		return nil
	})
	if err != nil {
		t.Fatalf("update sandbox: %v", err)
	}
	if got := loadRuntime(t, manager, session.ID).sandbox.Seed.Locations[0].Address; got != "Changed In Session" {
		t.Fatalf("expected mutated address, got %q", got)
	}

	mutateManagerState(t, manager, func() error {
		manager.activeID = ""
		return nil
	})
	rerun, err := manager.RerunSession(session.ID)
	if err != nil {
		t.Fatalf("rerun session: %v", err)
	}
	if rerun.ID == session.ID {
		t.Fatal("expected rerun to create a fresh session ID")
	}

	if got := loadRuntime(t, manager, rerun.ID).sandbox.Seed.Locations[0].Address; got != originalAddress {
		t.Fatalf("expected rerun sandbox address %q, got %q", originalAddress, got)
	}
}

func TestMarkActionStartedRejectsIdleActionOutOfSequence(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	err = manager.MarkActionStarted(session.ID, "run_unregister")
	if err == nil {
		t.Fatal("expected out-of-sequence unregister action to be rejected")
	}
	if !strings.Contains(err.Error(), "Run Handshake") {
		t.Fatalf("expected error to point back to the handshake step, got %v", err)
	}
}

func TestMarkActionStartedReactivatesCompletedActionRerunAndResetsCheckpoints(t *testing.T) {
	manager := NewManager(testSeed())
	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	mutateManagerState(t, manager, func() error {
		rt := manager.sessions[session.ID]
		if rt == nil {
			return ErrSessionNotFound
		}
		action := rt.actions["prepare_pull_locations_full_delete_connector"]
		if action == nil {
			t.Fatal("expected prepare_pull_locations_full_delete_connector action")
		}
		action.Status = "completed"
		action.Output = map[string]string{
			"location_id":  "LOC-1",
			"evse_uid":     "EVSE-1",
			"connector_id": "C1",
		}

		result := rt.cases["pull_locations_full_delete_connector"]
		if result == nil || len(result.Checkpoints) == 0 {
			t.Fatal("expected pull_locations_full_delete_connector checkpoint state")
		}
		result.Checkpoints[0].Answer = "still present"
		result.Checkpoints[0].Notes = "before retry"
		result.Checkpoints[0].Status = "answered"

		manager.activeID = ""
		rt.session.Status = "completed"
		rt.session.CurrentStep = SessionStep{
			Title:       "Session Complete",
			Description: "All currently included OCPI correctness checks reached a terminal state.",
		}
		return nil
	})

	if err := manager.MarkActionStarted(session.ID, "prepare_pull_locations_full_delete_connector"); err != nil {
		t.Fatalf("rerun action start: %v", err)
	}

	rt := loadRuntime(t, manager, session.ID)
	action := rt.actions["prepare_pull_locations_full_delete_connector"]
	result := rt.cases["pull_locations_full_delete_connector"]
	if manager.ActiveSessionID() != session.ID {
		t.Fatalf("expected rerun to reactivate session %q, got %q", session.ID, manager.ActiveSessionID())
	}
	if action.Status != "running" {
		t.Fatalf("expected action status running, got %q", action.Status)
	}
	if action.Output["connector_id"] != "C1" {
		t.Fatalf("expected previous action output to remain available during rerun, got %#v", action.Output)
	}
	if result.Checkpoints[0].Answer != "" || result.Checkpoints[0].Notes != "" {
		t.Fatalf("expected checkpoint answer and notes to reset, got %#v", result.Checkpoints[0])
	}
	if result.Checkpoints[0].Status != "pending" {
		t.Fatalf("expected checkpoint status pending after rerun, got %#v", result.Checkpoints[0])
	}
}

func TestManagerPersistsAcrossInstances(t *testing.T) {
	store := newMemoryStateStore()
	managerA := NewManager(testSeed(), store)

	session, err := managerA.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := managerA.UpdateSandbox(session.ID, func(sandbox *Sandbox) error {
		sandbox.Seed.Locations[0].Address = "Shared State Avenue 1"
		return nil
	}); err != nil {
		t.Fatalf("update sandbox: %v", err)
	}
	managerA.RecordTrafficEvent(TrafficEvent{
		Direction:      "inbound",
		Method:         "GET",
		Path:           "/ocpi/versions",
		RequestHeaders: map[string]string{"authorization": "Token peer-token"},
		StartedAt:      "2026-01-01T00:00:00Z",
	})

	managerB := NewManager(testSeed(), store)
	got, err := managerB.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session from second manager: %v", err)
	}
	if got.EventCount != 1 {
		t.Fatalf("expected second manager to see 1 event, got %d", got.EventCount)
	}
	if got.CurrentStep.ActionID != "run_handshake" {
		t.Fatalf("expected second manager to keep current step, got %#v", got.CurrentStep)
	}
	if current := managerB.CurrentSeed(testSeed()).Locations[0].Address; current != "Shared State Avenue 1" {
		t.Fatalf("expected second manager to see persisted seed mutation, got %q", current)
	}
}

func TestManagerDeleteSessionRemovesActiveSessionAndOverlayState(t *testing.T) {
	store := newMemoryStateStore()
	manager := NewManager(testSeed(), store)

	session, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	overlay := manager.ActiveOverlay()
	if overlay == nil {
		t.Fatal("expected active overlay store")
	}
	if err := overlay.SetTokenB("session-token-b"); err != nil {
		t.Fatalf("set overlay tokenB: %v", err)
	}
	if raw, err := store.GetBlob(overlayBlobKey(session.ID)); err != nil || len(raw) == 0 {
		t.Fatalf("expected overlay blob to exist before delete, got len=%d err=%v", len(raw), err)
	}

	if err := manager.DeleteSession(session.ID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := manager.GetSession(session.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected deleted session to be gone, got %v", err)
	}
	if manager.ActiveSessionID() != "" {
		t.Fatalf("expected active session ID to clear after delete, got %q", manager.ActiveSessionID())
	}
	if raw, err := store.GetBlob(overlayBlobKey(session.ID)); err != nil || len(raw) != 0 {
		t.Fatalf("expected overlay blob to be removed after delete, got len=%d err=%v", len(raw), err)
	}

	restarted, err := manager.StartSession(SessionConfig{
		PeerVersionsURL: "https://peer.example.com/ocpi/versions",
		PeerToken:       "peer-token",
	})
	if err != nil {
		t.Fatalf("start session after delete: %v", err)
	}
	if restarted.ID == session.ID {
		t.Fatalf("expected a fresh session after delete, got reused ID %q", restarted.ID)
	}
}

func TestNextStepSkipsBlockedCaseActions(t *testing.T) {
	suite := SuiteDefinition{
		Cases: []CaseDefinition{
			{
				ID:        "blocked_case",
				Title:     "Blocked Case",
				ActionIDs: []string{"blocked_action"},
			},
			{
				ID:        "ready_case",
				Title:     "Ready Case",
				ActionIDs: []string{"ready_action"},
			},
		},
	}
	session := TestSession{
		Actions: []ActionState{
			{
				ID:          "blocked_action",
				Title:       "Blocked Action",
				Description: "Should not be suggested yet.",
				Status:      "idle",
			},
			{
				ID:          "ready_action",
				Title:       "Ready Action",
				Description: "This is the actionable next step.",
				Status:      "idle",
			},
		},
		Cases: []CaseResult{
			{
				ID:       "blocked_case",
				Title:    "Blocked Case",
				Status:   "blocked",
				Messages: []string{"Waiting for a prerequisite to pass first."},
			},
			{
				ID:     "ready_case",
				Title:  "Ready Case",
				Status: "pending",
			},
		},
	}

	step := nextStep(suite, session)
	if step.ActionID != "ready_action" {
		t.Fatalf("expected ready_action to be suggested, got %#v", step)
	}
}

func TestNextStepSkipsFailedCasesAndMovesToNextAction(t *testing.T) {
	suite := SuiteDefinition{
		Cases: []CaseDefinition{
			{
				ID:        "failed_case",
				Title:     "Failed Case",
				ActionIDs: []string{"failed_action"},
			},
			{
				ID:        "ready_case",
				Title:     "Ready Case",
				ActionIDs: []string{"ready_action"},
			},
		},
	}
	session := TestSession{
		Actions: []ActionState{
			{
				ID:          "failed_action",
				Title:       "Failed Action",
				Description: "Already evaluated and failed.",
				Status:      "completed",
			},
			{
				ID:          "ready_action",
				Title:       "Ready Action",
				Description: "This should still be runnable.",
				Status:      "idle",
			},
		},
		Cases: []CaseResult{
			{
				ID:       "failed_case",
				Title:    "Failed Case",
				Status:   "failed",
				Messages: []string{"The previous check failed, but this should not block the suite."},
			},
			{
				ID:     "ready_case",
				Title:  "Ready Case",
				Status: "pending",
			},
		},
	}

	step := nextStep(suite, session)
	if step.ActionID != "ready_action" {
		t.Fatalf("expected ready_action to be suggested after a failed case, got %#v", step)
	}
}

func TestBlockingRequirementIgnoresFailedDependency(t *testing.T) {
	manager := NewManager(testSeed())
	rt := &sessionRuntime{
		suite: SuiteDefinition{
			Cases: []CaseDefinition{
				{ID: "handshake_flow", Title: "Handshake Flow"},
				{ID: "pull_locations_full", Title: "Full Locations Pull", Requires: []string{"handshake_flow"}},
			},
		},
		cases: map[string]*CaseResult{
			"handshake_flow": {
				ID:     "handshake_flow",
				Title:  "Handshake Flow",
				Status: "failed",
			},
			"pull_locations_full": {
				ID:     "pull_locations_full",
				Title:  "Full Locations Pull",
				Status: "pending",
			},
		},
	}

	blockedBy := manager.blockingRequirement(rt, rt.suite.Cases[1])
	if blockedBy != "" {
		t.Fatalf("expected failed dependency not to block follow-up cases, got %q", blockedBy)
	}
}

func TestNextStepFallsBackToBlockedMessageWhenNothingElseIsActionable(t *testing.T) {
	suite := SuiteDefinition{
		Cases: []CaseDefinition{
			{
				ID:        "blocked_case",
				Title:     "Blocked Case",
				ActionIDs: []string{"blocked_action"},
			},
		},
	}
	session := TestSession{
		Actions: []ActionState{
			{
				ID:          "blocked_action",
				Title:       "Blocked Action",
				Description: "Should remain blocked.",
				Status:      "idle",
			},
		},
		Cases: []CaseResult{
			{
				ID:       "blocked_case",
				Title:    "Blocked Case",
				Status:   "blocked",
				Messages: []string{"Waiting for a prerequisite to pass first."},
			},
		},
	}

	step := nextStep(suite, session)
	if step.ActionID != "" {
		t.Fatalf("expected no actionable step for blocked-only session, got %#v", step)
	}
	if step.CaseID != "blocked_case" {
		t.Fatalf("expected blocked case fallback, got %#v", step)
	}
	if step.Description != "Waiting for a prerequisite to pass first." {
		t.Fatalf("expected blocked message, got %#v", step)
	}
}
