package correctness

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuiltinSuiteUsesCorrectnessNaming(t *testing.T) {
	suites := BuiltinSuites()
	if len(suites) != 1 {
		t.Fatalf("expected exactly one built-in suite, got %d", len(suites))
	}

	suite := suites[0]
	if suite.ID != DefaultSuiteID {
		t.Fatalf("expected suite ID %q, got %q", DefaultSuiteID, suite.ID)
	}
	if suite.Name != "OCPI 2.2.1 eMSP Correctness Tests" {
		t.Fatalf("unexpected suite name: %q", suite.Name)
	}

	lowerProductText := strings.ToLower(strings.Join([]string{
		suite.ID,
		suite.Name,
		suite.Description,
	}, " "))
	for _, forbidden := range []string{"certification", "gireve", "gireve_id"} {
		if strings.Contains(lowerProductText, forbidden) {
			t.Fatalf("suite product text unexpectedly contained %q", forbidden)
		}
	}
}

func TestBuiltinSuiteDefinitionsAreNormativeAndVendorNeutral(t *testing.T) {
	suite := BuiltinSuites()[0]

	actionIDs := make(map[string]struct{}, len(suite.Actions))
	for _, action := range suite.Actions {
		if action.ID == "" {
			t.Fatal("found action with empty ID")
		}
		if _, exists := actionIDs[action.ID]; exists {
			t.Fatalf("duplicate action ID %q", action.ID)
		}
		actionIDs[action.ID] = struct{}{}

		productText := strings.ToLower(strings.Join([]string{
			action.ID,
			action.Group,
			action.Title,
			action.Description,
			action.Kind,
		}, " "))
		for _, forbidden := range []string{"certification", "gireve", "gireve_id"} {
			if strings.Contains(productText, forbidden) {
				t.Fatalf("action %q unexpectedly contained %q", action.ID, forbidden)
			}
		}
	}

	caseIDs := make(map[string]struct{}, len(suite.Cases))
	for _, def := range suite.Cases {
		if def.ID == "" {
			t.Fatal("found case with empty ID")
		}
		if _, exists := caseIDs[def.ID]; exists {
			t.Fatalf("duplicate case ID %q", def.ID)
		}
		caseIDs[def.ID] = struct{}{}

		if def.CompatibilityProfile != "" {
			t.Fatalf("expected empty compatibility profile for base case %q, got %q", def.ID, def.CompatibilityProfile)
		}
		if len(def.ScenarioSource) == 0 {
			t.Fatalf("case %q did not keep any scenario provenance", def.ID)
		}
		if len(def.NormativeSource) == 0 {
			t.Fatalf("case %q did not include any OCPI normative sources", def.ID)
		}

		productText := strings.ToLower(strings.Join([]string{
			def.ID,
			def.Group,
			def.Title,
			def.Description,
			def.Evaluator,
		}, " "))
		for _, forbidden := range []string{"certification", "gireve", "gireve_id"} {
			if strings.Contains(productText, forbidden) {
				t.Fatalf("case %q unexpectedly contained %q", def.ID, forbidden)
			}
		}
	}

	for _, deferred := range suite.Deferred {
		if deferred.ID == "" {
			t.Fatal("found deferred scenario with empty ID")
		}
		if deferred.Reason == "" {
			t.Fatalf("deferred scenario %q did not include a reason", deferred.ID)
		}
		if len(deferred.Normative) == 0 {
			t.Fatalf("deferred scenario %q did not include normative provenance", deferred.ID)
		}
	}
}

func TestBuiltinSuiteJSONDoesNotContainVendorKeys(t *testing.T) {
	raw, err := json.Marshal(BuiltinSuites())
	if err != nil {
		t.Fatalf("marshal built-in suites: %v", err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"gireve_id", "\"compatibility_profile\":\"gireve\"", "certification"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("built-in suite JSON unexpectedly contained %q", forbidden)
		}
	}
}

func TestBuiltinSuiteOrdersHandshakeBeforeUnregister(t *testing.T) {
	suite := BuiltinSuites()[0]

	actionIndex := make(map[string]int, len(suite.Actions))
	for i, action := range suite.Actions {
		actionIndex[action.ID] = i
	}
	for _, actionID := range []string{"run_handshake", "run_unregister"} {
		if _, ok := actionIndex[actionID]; !ok {
			t.Fatalf("expected action %q to exist", actionID)
		}
	}
	if actionIndex["run_handshake"] >= actionIndex["run_unregister"] {
		t.Fatalf("expected run_handshake before run_unregister, got %#v", actionIndex)
	}

	caseIndex := make(map[string]int, len(suite.Cases))
	for i, def := range suite.Cases {
		caseIndex[def.ID] = i
	}
	for _, caseID := range []string{"handshake_flow", "peer_fetches_hub_versions", "unregister_flow"} {
		if _, ok := caseIndex[caseID]; !ok {
			t.Fatalf("expected case %q to exist", caseID)
		}
	}
	if caseIndex["handshake_flow"] >= caseIndex["peer_fetches_hub_versions"] {
		t.Fatalf("expected handshake_flow before peer_fetches_hub_versions, got %#v", caseIndex)
	}
	if caseIndex["peer_fetches_hub_versions"] >= caseIndex["unregister_flow"] {
		t.Fatalf("expected unregister_flow to remain at the end of the flow, got %#v", caseIndex)
	}
}

func TestBuiltinSuiteOrdersValidAuthorizationBeforeInvalidationAndRemoteCommandsBeforeLocationRemoval(t *testing.T) {
	suite := BuiltinSuites()[0]

	actionIndex := make(map[string]int, len(suite.Actions))
	for i, action := range suite.Actions {
		actionIndex[action.ID] = i
	}
	if actionIndex["run_rta_valid"] >= actionIndex["arm_push_token_invalidate"] {
		t.Fatalf("expected run_rta_valid before arm_push_token_invalidate, got %#v", actionIndex)
	}
	if actionIndex["arm_push_token_invalidate"] >= actionIndex["run_rta_invalid"] {
		t.Fatalf("expected run_rta_invalid after arm_push_token_invalidate, got %#v", actionIndex)
	}
	if actionIndex["run_rta_invalid"] >= actionIndex["prepare_pull_locations_full_delete_connector"] {
		t.Fatalf("expected run_rta_invalid before destructive location deletion actions, got %#v", actionIndex)
	}

	caseIndex := make(map[string]int, len(suite.Cases))
	for i, def := range suite.Cases {
		caseIndex[def.ID] = i
	}

	if caseIndex["rta_valid"] >= caseIndex["token_push_invalidate"] {
		t.Fatalf("expected rta_valid before token_push_invalidate, got %#v", caseIndex)
	}
	if caseIndex["token_push_invalidate"] >= caseIndex["rta_invalid"] {
		t.Fatalf("expected rta_invalid after token_push_invalidate, got %#v", caseIndex)
	}
	if caseIndex["rta_invalid"] >= caseIndex["pull_locations_full_delete_connector"] {
		t.Fatalf("expected rta_invalid before full connector removal checks, got %#v", caseIndex)
	}
	if caseIndex["rta_invalid"] >= caseIndex["pull_locations_delta_delete_evse"] {
		t.Fatalf("expected rta_invalid before EVSE removal checks, got %#v", caseIndex)
	}
	if caseIndex["rta_invalid"] >= caseIndex["pull_locations_delta_delete_location"] {
		t.Fatalf("expected rta_invalid before location removal checks, got %#v", caseIndex)
	}
	if caseIndex["remote_start"] >= caseIndex["pull_locations_full_delete_connector"] {
		t.Fatalf("expected remote_start before destructive location deletion cases, got %#v", caseIndex)
	}
	if caseIndex["remote_stop"] >= caseIndex["pull_locations_delta_delete_evse"] {
		t.Fatalf("expected remote_stop before EVSE removal checks, got %#v", caseIndex)
	}
	if caseIndex["remote_stop"] >= caseIndex["pull_locations_delta_delete_location"] {
		t.Fatalf("expected remote_stop before location removal checks, got %#v", caseIndex)
	}
}
