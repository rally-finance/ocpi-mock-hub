package correctness

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

type evaluatorFunc func(rt *sessionRuntime, def CaseDefinition) CaseEvaluation

func builtinEvaluators() map[string]evaluatorFunc {
	return map[string]evaluatorFunc{
		"handshake_flow":                       evalHandshakeFlow,
		"unregister_flow":                      evalUnregisterFlow,
		"pull_locations_full":                  evalPullLocationsFull,
		"pull_locations_delta_update":          evalPullLocationsDeltaUpdate,
		"pull_locations_full_delete_connector": evalPullLocationsFullDeleteConnector,
		"pull_locations_delta_delete_evse":     evalPullLocationsDeltaDeleteEVSE,
		"pull_locations_delta_delete_location": evalPullLocationsDeltaDeleteLocation,
		"pull_tariffs_full":                    evalPullTariffsFull,
		"pull_tariffs_delta_update":            evalPullTariffsDeltaUpdate,
		"token_push_create":                    evalTokenPushCreate,
		"token_push_update":                    evalTokenPushUpdate,
		"token_push_invalidate":                evalTokenPushInvalidate,
		"rta_invalid":                          evalRTAInvalid,
		"rta_valid":                            evalRTAValid,
		"remote_start":                         evalRemoteStart,
		"remote_stop":                          evalRemoteStop,
		"evse_status_unknown":                  evalEVSEStatusUnknown,
		"evse_status_known":                    evalEVSEStatusKnown,
		"session_push_pending":                 evalSessionPushPending,
		"session_push_active":                  evalSessionPushActive,
		"session_push_completed":               evalSessionPushCompleted,
		"cdr_push":                             evalCDRPush,
	}
}

func evalHandshakeFlow(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "run_handshake")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}

	events := windowEvents(rt, action.ID)
	events = filterDirection(events, "outbound")
	if len(events) < 3 {
		return pendingEval("Waiting for the outbound versions, version details, and credentials exchange traffic.")
	}

	getVersions := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Method == "GET"
	})
	getDetails := nthMatchingEvent(events, 2, func(event TrafficEvent) bool {
		return event.Method == "GET"
	})
	postCredentials := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Method == "POST"
	})
	if getVersions == nil || getDetails == nil || postCredentials == nil {
		return pendingEval("Handshake traffic has not completed yet.")
	}

	var issues []string
	issues = append(issues, validateRequestAuth(*getVersions)...)
	issues = append(issues, validateResponseTime(*getVersions, 2000)...)
	issues = append(issues, validateVersionsEnvelope(*getVersions)...)

	issues = append(issues, validateRequestAuth(*getDetails)...)
	issues = append(issues, validateResponseTime(*getDetails, 2000)...)
	issues = append(issues, validateVersionDetailsEnvelope(*getDetails)...)

	issues = append(issues, validateRequestAuth(*postCredentials)...)
	issues = append(issues, validateJSONContentType(*postCredentials)...)
	issues = append(issues, validateResponseTime(*postCredentials, 2000)...)
	issues = append(issues, validateCredentialsEnvelope(*postCredentials)...)

	if len(issues) > 0 {
		return failedEval([]string{
			"Handshake checks failed.",
		}, issues, *getVersions, *getDetails, *postCredentials)
	}

	return passedEval("The peer completed the OCPI versions, version details, and credentials exchange with valid response envelopes and timing.", *getVersions, *getDetails, *postCredentials)
}

func evalUnregisterFlow(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "run_unregister")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := filterDirection(windowEvents(rt, action.ID), "outbound")
	event := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Method == "DELETE"
	})
	if event == nil {
		return pendingEval("Waiting for the outbound DELETE Credentials request.")
	}

	issues := append(validateRequestAuth(*event), validateResponseTime(*event, 2000)...)
	issues = append(issues, validateDeleteCredentialsEnvelope(*event)...)
	if len(issues) > 0 {
		return failedEval([]string{"The peer did not acknowledge DELETE Credentials as expected."}, issues, *event)
	}
	return passedEval("The peer acknowledged DELETE Credentials with a valid OCPI response in time.", *event)
}

func evalPullLocationsFull(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalInboundPaginatedPull(rt, "arm_pull_locations_full", "/ocpi/2.2.1/sender/locations", false, "", "")
}

func evalPullLocationsDeltaUpdate(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "prepare_pull_locations_delta_update")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	expectedID := action.Output["location_id"]
	return evalInboundPaginatedPull(rt, action.ID, "/ocpi/2.2.1/sender/locations", true, "location", expectedID)
}

func evalPullLocationsFullDeleteConnector(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "prepare_pull_locations_full_delete_connector")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := matchingInboundListEvents(rt, action.ID, "/ocpi/2.2.1/sender/locations")
	if len(events) == 0 {
		return pendingEval("Waiting for the eMSP to perform the full locations pull after the connector was removed.")
	}
	var issues []string
	issues = append(issues, validatePaginatedRequests(events, false)...)
	connectorID := action.Output["connector_id"]
	if connectorID == "" {
		issues = append(issues, "The session did not store the expected removed connector identifier.")
	} else if payloadContainsValue(events, connectorID) {
		issues = append(issues, fmt.Sprintf("The removed connector %s still appeared in the full pull payload.", connectorID))
	}

	checkpoint := checkpointState(rt, def.ID, "confirm_connector_removed_after_full_pull")
	if len(issues) > 0 {
		return failedEval([]string{"The full pull after connector removal did not match the expected OCPI behavior."}, issues, events...)
	}
	return manualOrPassed(checkpoint, "The connector disappeared from the full pull payload. Confirm what the eMSP showed after sync.", events...)
}

func evalPullLocationsDeltaDeleteEVSE(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "prepare_pull_locations_delta_delete_evse")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := matchingInboundListEvents(rt, action.ID, "/ocpi/2.2.1/sender/locations")
	if len(events) == 0 {
		return pendingEval("Waiting for the eMSP to perform the delta locations pull after the EVSE was removed.")
	}
	var issues []string
	issues = append(issues, validatePaginatedRequests(events, true)...)
	evseID := action.Output["evse_uid"]
	if evseID == "" {
		issues = append(issues, "The session did not store the expected removed EVSE identifier.")
	} else if !payloadHasEVSEStatus(events, evseID, "REMOVED") {
		issues = append(issues, fmt.Sprintf("The EVSE %s did not appear with status REMOVED in the delta payload.", evseID))
	}
	checkpoint := checkpointState(rt, def.ID, "confirm_evse_removed_after_delta_pull")
	if len(issues) > 0 {
		return failedEval([]string{"The delta pull after EVSE removal did not match the expected OCPI behavior."}, issues, events...)
	}
	return manualOrPassed(checkpoint, "The EVSE appeared as REMOVED in the delta payload. Confirm what the eMSP showed after sync.", events...)
}

func evalPullLocationsDeltaDeleteLocation(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "prepare_pull_locations_delta_delete_location")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := matchingInboundListEvents(rt, action.ID, "/ocpi/2.2.1/sender/locations")
	if len(events) == 0 {
		return pendingEval("Waiting for the eMSP to perform the delta locations pull after the location was removed.")
	}
	var issues []string
	issues = append(issues, validatePaginatedRequests(events, true)...)
	locationID := action.Output["location_id"]
	if locationID == "" {
		issues = append(issues, "The session did not store the expected removed location identifier.")
	} else if !payloadHasAllLocationEVSEsRemoved(events, locationID) {
		issues = append(issues, fmt.Sprintf("The location %s did not return with all EVSEs marked REMOVED.", locationID))
	}
	checkpoint := checkpointState(rt, def.ID, "confirm_location_removed_after_delta_pull")
	if len(issues) > 0 {
		return failedEval([]string{"The delta pull after location removal did not match the expected OCPI behavior."}, issues, events...)
	}
	return manualOrPassed(checkpoint, "All EVSEs on the removed location appeared as REMOVED in the delta payload. Confirm what the eMSP showed after sync.", events...)
}

func evalPullTariffsFull(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalInboundPaginatedPull(rt, "arm_pull_tariffs_full", "/ocpi/2.2.1/sender/tariffs", false, "", "")
}

func evalPullTariffsDeltaUpdate(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "prepare_pull_tariffs_delta_update")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	expectedID := action.Output["tariff_id"]
	return evalInboundPaginatedPull(rt, action.ID, "/ocpi/2.2.1/sender/tariffs", true, "tariff", expectedID)
}

func evalTokenPushCreate(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalInboundTokenPush(rt, def, "arm_push_token_create", false, false)
}

func evalTokenPushUpdate(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalInboundTokenPush(rt, def, "arm_push_token_update", true, false)
}

func evalTokenPushInvalidate(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalInboundTokenPush(rt, def, "arm_push_token_invalidate", true, true)
}

func evalRTAInvalid(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalOutboundAuthorize(rt, "run_rta_invalid", false)
}

func evalRTAValid(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalOutboundAuthorize(rt, "run_rta_valid", true)
}

func evalRemoteStart(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "arm_remote_start")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := windowEvents(rt, action.ID)
	commandEvent := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Direction == "inbound" && event.Method == "POST" && event.Path == "/ocpi/2.2.1/receiver/commands/START_SESSION"
	})
	if commandEvent == nil {
		return pendingEval("Waiting for the peer to send START_SESSION.")
	}
	var issues []string
	issues = append(issues, validateInboundOCPIRequest(*commandEvent, true)...)
	issues = append(issues, validateCommandPayload(*commandEvent, "START_SESSION", rt.sandbox.Seed)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*commandEvent, true)...)

	callback := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Direction == "outbound" && event.Method == "POST" && event.ActionID == "" && strings.Contains(strings.ToLower(event.RequestBody), "accepted")
	})
	if callback == nil {
		return pendingEvalWithEvidence("Waiting for the asynchronous START_SESSION callback exchange.", *commandEvent)
	}
	issues = append(issues, validateResponseTime(*callback, 2000)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*callback, true)...)
	if len(issues) > 0 {
		return failedEval([]string{"The START_SESSION flow did not satisfy the expected command and callback behavior."}, issues, *commandEvent, *callback)
	}
	return passedEval("The peer sent a valid START_SESSION command and accepted the callback flow.", *commandEvent, *callback)
}

func evalRemoteStop(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "arm_remote_stop")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := windowEvents(rt, action.ID)
	commandEvent := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Direction == "inbound" && event.Method == "POST" && event.Path == "/ocpi/2.2.1/receiver/commands/STOP_SESSION"
	})
	if commandEvent == nil {
		return pendingEval("Waiting for the peer to send STOP_SESSION.")
	}
	var issues []string
	issues = append(issues, validateInboundOCPIRequest(*commandEvent, true)...)
	issues = append(issues, validateCommandPayload(*commandEvent, "STOP_SESSION", rt.sandbox.Seed)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*commandEvent, true)...)

	callback := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Direction == "outbound" && event.Method == "POST" && event.ActionID == "" && strings.Contains(strings.ToLower(event.RequestBody), "accepted")
	})
	if callback == nil {
		return pendingEvalWithEvidence("Waiting for the asynchronous STOP_SESSION callback exchange.", *commandEvent)
	}
	issues = append(issues, validateResponseTime(*callback, 2000)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*callback, true)...)
	if len(issues) > 0 {
		return failedEval([]string{"The STOP_SESSION flow did not satisfy the expected command and callback behavior."}, issues, *commandEvent, *callback)
	}
	return passedEval("The peer sent a valid STOP_SESSION command and accepted the callback flow.", *commandEvent, *callback)
}

func evalEVSEStatusUnknown(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalOutboundLocationPatch(rt, "run_evse_status_unknown", false)
}

func evalEVSEStatusKnown(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalOutboundLocationPatch(rt, "run_evse_status_known", true)
}

func evalSessionPushPending(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalOutboundSessionPush(rt, "run_session_push_pending", "PENDING")
}

func evalSessionPushActive(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalOutboundSessionPush(rt, "run_session_push_active", "ACTIVE")
}

func evalSessionPushCompleted(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	return evalOutboundSessionPush(rt, "run_session_push_completed", "COMPLETED")
}

func evalCDRPush(rt *sessionRuntime, def CaseDefinition) CaseEvaluation {
	action, pending, failed := actionGate(rt, "run_cdr_push")
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := filterDirection(windowEvents(rt, action.ID), "outbound")
	event := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Method == "POST" && strings.Contains(event.Path, "/cdrs")
	})
	if event == nil {
		return pendingEval("Waiting for the outbound CDR push.")
	}
	var issues []string
	issues = append(issues, validateRequestAuth(*event)...)
	issues = append(issues, validateJSONContentType(*event)...)
	issues = append(issues, validateResponseTime(*event, 2000)...)
	issues = append(issues, validateCDRPayload(*event)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*event, true)...)
	if len(issues) > 0 {
		return failedEval([]string{"The peer did not handle the pushed CDR as expected."}, issues, *event)
	}
	return passedEval("The peer accepted the pushed CDR with a valid OCPI response.", *event)
}

func evalInboundPaginatedPull(rt *sessionRuntime, actionID, expectedPath string, requireDates bool, dataKind, expectedID string) CaseEvaluation {
	action, pending, failed := actionGate(rt, actionID)
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := matchingInboundListEvents(rt, action.ID, expectedPath)
	if len(events) == 0 {
		return pendingEval("Waiting for inbound pull traffic from the peer.")
	}
	var issues []string
	issues = append(issues, validatePaginatedRequests(events, requireDates)...)
	if dataKind != "" && expectedID != "" && !payloadContainsValue(events, expectedID) {
		issues = append(issues, fmt.Sprintf("The expected updated %s %s did not appear in the response payload.", dataKind, expectedID))
	}
	if len(issues) > 0 {
		return failedEval([]string{"The observed pull sequence did not match the expected OCPI request or pagination behavior."}, issues, events...)
	}
	return passedEval("The observed pull sequence matched the expected OCPI request and pagination behavior.", events...)
}

func evalInboundTokenPush(rt *sessionRuntime, def CaseDefinition, actionID string, requireKnownToken bool, requireInvalid bool) CaseEvaluation {
	action, pending, failed := actionGate(rt, actionID)
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := windowEvents(rt, action.ID)
	event := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Direction == "inbound" && event.Method == "PUT" && strings.HasPrefix(event.Path, "/ocpi/2.2.1/receiver/tokens/")
	})
	if event == nil {
		return pendingEval("Waiting for the peer to push a Token object.")
	}

	var issues []string
	issues = append(issues, validateInboundOCPIRequest(*event, true)...)
	issues = append(issues, validateTokenPayload(*event, requireInvalid)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*event, true)...)

	if requireKnownToken {
		expectedUID := action.Output["uid"]
		if expectedUID != "" && !strings.Contains(event.Path, "/"+expectedUID) {
			issues = append(issues, fmt.Sprintf("Expected token UID %s based on the prepared session state, but received %s.", expectedUID, event.Path))
		}
	}

	if len(issues) > 0 {
		return failedEval([]string{"The pushed Token object did not satisfy the expected OCPI request and payload rules."}, issues, *event)
	}
	return passedEval("The pushed Token object satisfied the expected OCPI request and payload rules.", *event)
}

func evalOutboundAuthorize(rt *sessionRuntime, actionID string, expectAllowed bool) CaseEvaluation {
	action, pending, failed := actionGate(rt, actionID)
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := filterDirection(windowEvents(rt, action.ID), "outbound")
	event := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Method == "POST" && strings.HasSuffix(event.Path, "/authorize")
	})
	if event == nil {
		return pendingEval("Waiting for the outbound authorization request.")
	}

	var issues []string
	issues = append(issues, validateRequestAuth(*event)...)
	issues = append(issues, validateJSONContentType(*event)...)
	issues = append(issues, validateResponseTime(*event, 2000)...)
	issues = append(issues, validateAuthorizationEnvelope(*event, expectAllowed)...)
	if len(issues) > 0 {
		label := "invalid"
		if expectAllowed {
			label = "valid"
		}
		return failedEval([]string{fmt.Sprintf("The peer did not return the expected %s authorization response.", label)}, issues, *event)
	}
	return passedEval("The peer returned a valid authorization response.", *event)
}

func evalOutboundLocationPatch(rt *sessionRuntime, actionID string, expectSuccess bool) CaseEvaluation {
	action, pending, failed := actionGate(rt, actionID)
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := filterDirection(windowEvents(rt, action.ID), "outbound")
	event := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Method == "PATCH" && strings.Contains(event.Path, "/locations/")
	})
	if event == nil {
		return pendingEval("Waiting for the outbound EVSE status update.")
	}

	var issues []string
	issues = append(issues, validateRequestAuth(*event)...)
	issues = append(issues, validateJSONContentType(*event)...)
	issues = append(issues, validateLocationPatchPayload(*event)...)
	issues = append(issues, validateResponseTime(*event, 2000)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*event, expectSuccess)...)
	if !expectSuccess && event.ResponseStatus < 400 {
		issues = append(issues, "Expected the peer to reject the unknown EVSE update with a non-2xx HTTP status.")
	}
	if len(issues) > 0 {
		return failedEval([]string{"The EVSE status push did not produce the expected peer response."}, issues, *event)
	}
	return passedEval("The EVSE status push produced the expected peer response.", *event)
}

func evalOutboundSessionPush(rt *sessionRuntime, actionID, expectedStatus string) CaseEvaluation {
	action, pending, failed := actionGate(rt, actionID)
	if pending != nil || failed != nil {
		if pending != nil {
			return *pending
		}
		return *failed
	}
	events := filterDirection(windowEvents(rt, action.ID), "outbound")
	event := firstMatchingEvent(events, func(event TrafficEvent) bool {
		return event.Method == "PUT" && strings.Contains(event.Path, "/sessions/")
	})
	if event == nil {
		return pendingEval("Waiting for the outbound Session push.")
	}

	var issues []string
	issues = append(issues, validateRequestAuth(*event)...)
	issues = append(issues, validateJSONContentType(*event)...)
	issues = append(issues, validateResponseTime(*event, 2000)...)
	issues = append(issues, validateSessionPayload(*event, expectedStatus)...)
	issues = append(issues, validateOCPIEnvelopeResponse(*event, true)...)
	if len(issues) > 0 {
		return failedEval([]string{"The peer did not handle the pushed Session as expected."}, issues, *event)
	}
	return passedEval("The peer accepted the pushed Session update with a valid OCPI response.", *event)
}

func actionGate(rt *sessionRuntime, actionID string) (*ActionState, *CaseEvaluation, *CaseEvaluation) {
	action := rt.actions[actionID]
	if action == nil {
		failed := failedEval([]string{fmt.Sprintf("Missing action definition for %s.", actionID)}, nil)
		return nil, nil, &failed
	}
	if action.Status == "idle" {
		pending := pendingEval(action.Description)
		return action, &pending, nil
	}
	if action.Status == "failed" {
		failed := failedEval([]string{action.LastError}, nil)
		return action, nil, &failed
	}
	return action, nil, nil
}

func matchingInboundListEvents(rt *sessionRuntime, actionID, path string) []TrafficEvent {
	events := windowEvents(rt, actionID)
	matches := make([]TrafficEvent, 0)
	for _, event := range events {
		if event.Direction == "inbound" && event.Method == "GET" && event.Path == path {
			matches = append(matches, event)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].StartedAt < matches[j].StartedAt
	})
	return matches
}

func windowEvents(rt *sessionRuntime, actionID string) []TrafficEvent {
	action := rt.actions[actionID]
	if action == nil {
		return nil
	}
	start := action.EventAnchor
	if start < 0 || start > len(rt.events) {
		start = 0
	}
	end := len(rt.events)
	for _, candidate := range rt.actions {
		if candidate == nil || candidate.ID == actionID {
			continue
		}
		if candidate.LastRunAt == "" {
			continue
		}
		if candidate.EventAnchor > start && candidate.EventAnchor < end {
			end = candidate.EventAnchor
		}
	}
	if start >= end || start >= len(rt.events) {
		return nil
	}
	out := make([]TrafficEvent, end-start)
	copy(out, rt.events[start:end])
	return out
}

func filterDirection(events []TrafficEvent, direction string) []TrafficEvent {
	filtered := make([]TrafficEvent, 0, len(events))
	for _, event := range events {
		if event.Direction == direction {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func firstMatchingEvent(events []TrafficEvent, fn func(TrafficEvent) bool) *TrafficEvent {
	for i := range events {
		if fn(events[i]) {
			return &events[i]
		}
	}
	return nil
}

func nthMatchingEvent(events []TrafficEvent, n int, fn func(TrafficEvent) bool) *TrafficEvent {
	count := 0
	for i := range events {
		if fn(events[i]) {
			count++
			if count == n {
				return &events[i]
			}
		}
	}
	return nil
}

func validatePaginatedRequests(events []TrafficEvent, requireDates bool) []string {
	var issues []string
	for i, event := range events {
		issues = append(issues, validateInboundOCPIRequest(event, true)...)
		query, _ := url.ParseQuery(event.RawQuery)
		if requireDates {
			dateFrom := query.Get("date_from")
			if dateFrom == "" {
				issues = append(issues, "Expected date_from to be present for the delta pull request.")
			}
			if dateFrom != "" {
				if _, err := parseOCPIDateTime(dateFrom); err != nil {
					issues = append(issues, "date_from was not a valid OCPI DateTime in UTC.")
				}
			}
			if dateTo := query.Get("date_to"); dateTo != "" {
				from, fromErr := parseOCPIDateTime(query.Get("date_from"))
				to, toErr := parseOCPIDateTime(dateTo)
				if toErr != nil {
					issues = append(issues, "date_to was not a valid OCPI DateTime in UTC.")
				}
				if fromErr == nil && toErr == nil {
					if !to.After(from) {
						issues = append(issues, "date_to must be later than date_from.")
					}
					if to.Sub(from) > 31*24*time.Hour {
						issues = append(issues, "The requested delta range exceeded one month.")
					}
				}
			}
		} else if query.Get("date_from") != "" || query.Get("date_to") != "" {
			issues = append(issues, "The full pull should not include date_from or date_to.")
		}

		limit := query.Get("limit")
		offset := query.Get("offset")
		if i == 0 && offset != "" && offset != "0" {
			issues = append(issues, "The first paginated request should start with offset=0.")
		}
		if limit != "" {
			if parsedLimit, err := strconv.Atoi(limit); err != nil || parsedLimit <= 0 {
				issues = append(issues, "The limit query parameter was not a positive integer.")
			}
		}
		if offset != "" {
			if parsedOffset, err := strconv.Atoi(offset); err != nil || parsedOffset < 0 {
				issues = append(issues, "The offset query parameter was not a non-negative integer.")
			}
		}

		if i > 0 {
			prev := events[i-1]
			prevLink := headerValue(prev.ResponseHeaders, "link")
			if prevLink == "" {
				issues = append(issues, "The peer sent another page request even though the previous response did not advertise a next Link.")
			}
			if nextOffset := offsetFromLink(prevLink); nextOffset != "" && offset != nextOffset {
				issues = append(issues, fmt.Sprintf("Expected the next page offset %s from the previous Link header, got %s.", nextOffset, offset))
			}
		}

	}

	return uniqueStrings(issues)
}

func validateInboundOCPIRequest(event TrafficEvent, requireFrom bool) []string {
	var issues []string
	issues = append(issues, validateRequestAuth(event)...)
	if headerValue(event.RequestHeaders, "x-request-id") == "" {
		issues = append(issues, "The request did not include X-Request-ID.")
	}
	if headerValue(event.RequestHeaders, "x-correlation-id") == "" {
		issues = append(issues, "The request did not include X-Correlation-ID.")
	}
	if requireFrom {
		if headerValue(event.RequestHeaders, "ocpi-from-country-code") == "" {
			issues = append(issues, "The request did not include OCPI-From-Country-Code.")
		}
		if headerValue(event.RequestHeaders, "ocpi-from-party-id") == "" {
			issues = append(issues, "The request did not include OCPI-From-Party-Id.")
		}
	}
	return issues
}

func validateRequestAuth(event TrafficEvent) []string {
	value := headerValue(event.RequestHeaders, "authorization")
	if value == "" {
		return []string{"The request did not include Authorization: Token ..."}
	}
	if !strings.HasPrefix(strings.ToLower(value), "token ") {
		return []string{"The Authorization header did not use the Token scheme."}
	}
	return nil
}

func validateJSONContentType(event TrafficEvent) []string {
	contentType := headerValue(event.RequestHeaders, "content-type")
	if contentType == "" {
		return []string{"The request did not include Content-Type: application/json."}
	}
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return []string{"The request Content-Type was not application/json."}
	}
	return nil
}

func validateResponseTime(event TrafficEvent, maxMS int64) []string {
	if event.DurationMS > maxMS {
		return []string{fmt.Sprintf("The response took %dms which exceeded the %dms limit.", event.DurationMS, maxMS)}
	}
	return nil
}

func validateVersionsEnvelope(event TrafficEvent) []string {
	envelope, issues := parseEnvelope(event.ResponseBody, true)
	if len(issues) > 0 {
		return issues
	}
	var versions []map[string]any
	if err := json.Unmarshal(envelope.Data, &versions); err != nil {
		return []string{"The versions response data was not a JSON array."}
	}
	found := false
	for _, version := range versions {
		if versionString, _ := version["version"].(string); versionString == "2.2.1" {
			found = true
		}
		if rawURL, _ := version["url"].(string); rawURL != "" {
			if issue := validateURLLike(rawURL); issue != "" {
				issues = append(issues, issue)
			}
		}
	}
	if !found {
		issues = append(issues, "The versions response did not advertise OCPI version 2.2.1.")
	}
	return issues
}

func validateVersionDetailsEnvelope(event TrafficEvent) []string {
	envelope, issues := parseEnvelope(event.ResponseBody, true)
	if len(issues) > 0 {
		return issues
	}
	var payload struct {
		Version   string `json:"version"`
		Endpoints []struct {
			Identifier string `json:"identifier"`
			Role       string `json:"role"`
			URL        string `json:"url"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return []string{"The version details response data was not a valid object."}
	}
	if payload.Version != "2.2.1" {
		issues = append(issues, "The version details response did not report version 2.2.1.")
	}
	hasCredentials := false
	for _, endpoint := range payload.Endpoints {
		if endpoint.Identifier == "credentials" {
			hasCredentials = true
		}
		if issue := validateURLLike(endpoint.URL); issue != "" {
			issues = append(issues, issue)
		}
	}
	if !hasCredentials {
		issues = append(issues, "The version details response did not advertise a credentials endpoint.")
	}
	return issues
}

func validateCredentialsEnvelope(event TrafficEvent) []string {
	envelope, issues := parseEnvelope(event.ResponseBody, true)
	if len(issues) > 0 {
		return issues
	}
	var payload map[string]any
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return []string{"The credentials response data was not a valid object."}
	}
	for _, field := range []string{"token", "url", "country_code", "party_id"} {
		if strings.TrimSpace(stringValue(payload[field])) == "" {
			issues = append(issues, fmt.Sprintf("The credentials response did not include %s.", field))
		}
	}
	if rawURL := stringValue(payload["url"]); rawURL != "" {
		if issue := validateURLLike(rawURL); issue != "" {
			issues = append(issues, issue)
		}
	}
	return issues
}

func validateDeleteCredentialsEnvelope(event TrafficEvent) []string {
	envelope, issues := parseEnvelope(event.ResponseBody, true)
	if len(issues) > 0 {
		return issues
	}
	if envelope.Data != nil && strings.TrimSpace(string(envelope.Data)) != "null" && strings.TrimSpace(string(envelope.Data)) != "" {
		issues = append(issues, "DELETE Credentials returned a data payload, expected empty or null data.")
	}
	return issues
}

func validateAuthorizationEnvelope(event TrafficEvent, expectAllowed bool) []string {
	envelope, issues := parseEnvelope(event.ResponseBody, true)
	if len(issues) > 0 {
		return issues
	}
	var payload map[string]any
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return []string{"The authorization response data was not a valid object."}
	}
	allowed := stringValue(payload["allowed"])
	if expectAllowed && allowed != "ALLOWED" {
		issues = append(issues, fmt.Sprintf("Expected allowed=ALLOWED, got %s.", allowed))
	}
	if !expectAllowed {
		switch allowed {
		case "BLOCKED", "EXPIRED", "NO_CREDIT", "NOT_ALLOWED":
		default:
			issues = append(issues, fmt.Sprintf("Expected an invalid authorization status, got %s.", allowed))
		}
	}
	if payload["token"] == nil {
		issues = append(issues, "The authorization response did not include token.")
	}
	if expectAllowed && strings.TrimSpace(stringValue(payload["authorization_reference"])) == "" {
		issues = append(issues, "The authorization response did not include authorization_reference for ALLOWED.")
	}
	return issues
}

func validateCommandPayload(event TrafficEvent, command string, seed *fakegen.SeedData) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.RequestBody), &payload); err != nil {
		return []string{"The command request body was not valid JSON."}
	}
	var issues []string
	if command == "START_SESSION" {
		if issue := validateURLLike(stringValue(payload["response_url"])); issue != "" {
			issues = append(issues, "The START_SESSION payload did not include a valid response_url.")
		}
		if strings.TrimSpace(stringValue(payload["location_id"])) == "" {
			issues = append(issues, "The START_SESSION payload did not include location_id.")
		}
		token, _ := payload["token"].(map[string]any)
		if token == nil {
			issues = append(issues, "The START_SESSION payload did not include token.")
		} else if strings.TrimSpace(stringValue(token["uid"])) == "" {
			issues = append(issues, "The START_SESSION payload token did not include uid.")
		}
	} else {
		if strings.TrimSpace(stringValue(payload["session_id"])) == "" {
			issues = append(issues, "The STOP_SESSION payload did not include session_id.")
		}
	}
	return issues
}

func validateLocationPatchPayload(event TrafficEvent) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.RequestBody), &payload); err != nil {
		return []string{"The EVSE status update body was not valid JSON."}
	}
	var issues []string
	status := stringValue(payload["status"])
	if status == "" {
		issues = append(issues, "The EVSE status update did not include status.")
	}
	lastUpdated := stringValue(payload["last_updated"])
	if _, err := parseOCPIDateTime(lastUpdated); err != nil {
		issues = append(issues, "The EVSE status update last_updated field was not a valid OCPI DateTime.")
	}
	return issues
}

func validateTokenPayload(event TrafficEvent, requireInvalid bool) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.RequestBody), &payload); err != nil {
		return []string{"The Token body was not valid JSON."}
	}
	var issues []string
	for _, field := range []string{"country_code", "party_id", "uid", "type", "contract_id", "issuer", "whitelist", "last_updated"} {
		if strings.TrimSpace(stringValue(payload[field])) == "" {
			issues = append(issues, fmt.Sprintf("The Token body did not include %s.", field))
		}
	}
	if _, ok := payload["valid"].(bool); !ok {
		issues = append(issues, "The Token body did not include a boolean valid field.")
	}
	if requireInvalid {
		if valid, ok := payload["valid"].(bool); !ok || valid {
			issues = append(issues, "Expected the invalidation Token push to set valid=false.")
		}
	}
	if _, err := parseOCPIDateTime(stringValue(payload["last_updated"])); err != nil {
		issues = append(issues, "The Token last_updated field was not a valid OCPI DateTime.")
	}
	pathParts := strings.Split(strings.TrimPrefix(event.Path, "/ocpi/2.2.1/receiver/tokens/"), "/")
	if len(pathParts) < 3 {
		issues = append(issues, "The Token request path did not contain country_code, party_id, and uid.")
	} else {
		if cc := stringValue(payload["country_code"]); cc != "" && pathParts[0] != cc {
			issues = append(issues, "The Token path country_code did not match the body.")
		}
		if pid := stringValue(payload["party_id"]); pid != "" && pathParts[1] != pid {
			issues = append(issues, "The Token path party_id did not match the body.")
		}
		if uid := stringValue(payload["uid"]); uid != "" && pathParts[2] != uid {
			issues = append(issues, "The Token path uid did not match the body.")
		}
	}
	query, _ := url.ParseQuery(event.RawQuery)
	if queryType := query.Get("type"); queryType != "" && queryType != stringValue(payload["type"]) {
		issues = append(issues, "The Token type query parameter did not match the body.")
	}
	return issues
}

func validateSessionPayload(event TrafficEvent, expectedStatus string) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.RequestBody), &payload); err != nil {
		return []string{"The Session body was not valid JSON."}
	}
	var issues []string
	for _, field := range []string{"country_code", "party_id", "id", "start_date_time", "status", "last_updated"} {
		if strings.TrimSpace(stringValue(payload[field])) == "" {
			issues = append(issues, fmt.Sprintf("The Session body did not include %s.", field))
		}
	}
	if payload["kwh"] == nil {
		issues = append(issues, "The Session body did not include kwh.")
	}
	if status := stringValue(payload["status"]); status != expectedStatus {
		issues = append(issues, fmt.Sprintf("Expected Session status %s, got %s.", expectedStatus, status))
	}
	if _, err := parseOCPIDateTime(stringValue(payload["start_date_time"])); err != nil {
		issues = append(issues, "The Session start_date_time field was not a valid OCPI DateTime.")
	}
	if _, err := parseOCPIDateTime(stringValue(payload["last_updated"])); err != nil {
		issues = append(issues, "The Session last_updated field was not a valid OCPI DateTime.")
	}
	if expectedStatus == "COMPLETED" {
		if _, err := parseOCPIDateTime(stringValue(payload["end_date_time"])); err != nil {
			issues = append(issues, "A COMPLETED Session should include a valid end_date_time.")
		}
	}
	return issues
}

func validateCDRPayload(event TrafficEvent) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.RequestBody), &payload); err != nil {
		return []string{"The CDR body was not valid JSON."}
	}
	var issues []string
	for _, field := range []string{"country_code", "party_id", "id", "start_date_time", "end_date_time", "auth_method", "currency", "last_updated"} {
		if strings.TrimSpace(stringValue(payload[field])) == "" {
			issues = append(issues, fmt.Sprintf("The CDR body did not include %s.", field))
		}
	}
	if payload["cdr_token"] == nil {
		issues = append(issues, "The CDR body did not include cdr_token.")
	}
	if payload["cdr_location"] == nil {
		issues = append(issues, "The CDR body did not include cdr_location.")
	}
	if payload["total_cost"] == nil || payload["total_energy"] == nil || payload["total_time"] == nil {
		issues = append(issues, "The CDR body did not include total_cost, total_energy, and total_time.")
	}
	if _, err := parseOCPIDateTime(stringValue(payload["start_date_time"])); err != nil {
		issues = append(issues, "The CDR start_date_time field was not a valid OCPI DateTime.")
	}
	if _, err := parseOCPIDateTime(stringValue(payload["end_date_time"])); err != nil {
		issues = append(issues, "The CDR end_date_time field was not a valid OCPI DateTime.")
	}
	if _, err := parseOCPIDateTime(stringValue(payload["last_updated"])); err != nil {
		issues = append(issues, "The CDR last_updated field was not a valid OCPI DateTime.")
	}
	periods, ok := payload["charging_periods"].([]any)
	if !ok || len(periods) == 0 {
		issues = append(issues, "The CDR body did not include charging_periods.")
	} else if !hasDimensionType(periods, "TIME") {
		issues = append(issues, "The CDR charging_periods did not include a TIME dimension.")
	}
	return issues
}

func validateOCPIEnvelopeResponse(event TrafficEvent, expectSuccess bool) []string {
	envelope, issues := parseEnvelope(event.ResponseBody, expectSuccess)
	if len(issues) > 0 {
		return issues
	}
	if expectSuccess && (event.ResponseStatus < 200 || event.ResponseStatus >= 300) {
		issues = append(issues, fmt.Sprintf("Expected an HTTP success status, got %d.", event.ResponseStatus))
	}
	if !expectSuccess && event.ResponseStatus >= 200 && event.ResponseStatus < 300 {
		issues = append(issues, "Expected a non-success HTTP status for this response.")
	}
	if envelope.Timestamp == "" {
		issues = append(issues, "The OCPI response did not include timestamp.")
	}
	return issues
}

func parseEnvelope(body string, expectSuccess bool) (OCPIEnvelope, []string) {
	var envelope OCPIEnvelope
	if strings.TrimSpace(body) == "" {
		return envelope, []string{"The response body was empty."}
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		return envelope, []string{"The response body was not valid JSON."}
	}
	var issues []string
	if envelope.StatusCode == 0 {
		issues = append(issues, "The OCPI response did not include status_code.")
	}
	if expectSuccess && (envelope.StatusCode < 1000 || envelope.StatusCode >= 2000) {
		issues = append(issues, fmt.Sprintf("Expected a successful OCPI status code, got %d.", envelope.StatusCode))
	}
	if _, err := parseOCPIDateTime(envelope.Timestamp); err != nil {
		issues = append(issues, "The OCPI response timestamp was not a valid UTC OCPI DateTime.")
	}
	return envelope, issues
}

func parseOCPIDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty datetime")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			if _, offset := parsed.Zone(); offset != 0 {
				return time.Time{}, fmt.Errorf("datetime must be UTC")
			}
			return parsed.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func validateURLLike(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "Expected a URL but the value was empty."
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Sprintf("The URL %q was not valid.", raw)
	}
	if parsed.Scheme != "https" && !isLocalHost(parsed.Hostname()) {
		return fmt.Sprintf("The URL %q did not use https.", raw)
	}
	return ""
}

func isLocalHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func payloadContainsValue(events []TrafficEvent, expected string) bool {
	for _, event := range events {
		if strings.Contains(event.ResponseBody, expected) {
			return true
		}
	}
	return false
}

func payloadHasEVSEStatus(events []TrafficEvent, evseUID, status string) bool {
	for _, event := range events {
		var envelope OCPIEnvelope
		if err := json.Unmarshal([]byte(event.ResponseBody), &envelope); err != nil {
			continue
		}
		var locations []map[string]any
		if err := json.Unmarshal(envelope.Data, &locations); err != nil {
			continue
		}
		for _, location := range locations {
			for _, evse := range anySlice(location["evses"]) {
				evseMap, _ := evse.(map[string]any)
				if stringValue(evseMap["uid"]) == evseUID && stringValue(evseMap["status"]) == status {
					return true
				}
			}
		}
	}
	return false
}

func payloadHasAllLocationEVSEsRemoved(events []TrafficEvent, locationID string) bool {
	for _, event := range events {
		var envelope OCPIEnvelope
		if err := json.Unmarshal([]byte(event.ResponseBody), &envelope); err != nil {
			continue
		}
		var locations []map[string]any
		if err := json.Unmarshal(envelope.Data, &locations); err != nil {
			continue
		}
		for _, location := range locations {
			if stringValue(location["id"]) != locationID {
				continue
			}
			evses := anySlice(location["evses"])
			if len(evses) == 0 {
				return false
			}
			allRemoved := true
			for _, item := range evses {
				evseMap, _ := item.(map[string]any)
				if stringValue(evseMap["status"]) != "REMOVED" {
					allRemoved = false
				}
			}
			if allRemoved {
				return true
			}
		}
	}
	return false
}

func hasDimensionType(periods []any, expected string) bool {
	for _, item := range periods {
		periodMap, _ := item.(map[string]any)
		for _, dimension := range anySlice(periodMap["dimensions"]) {
			dimensionMap, _ := dimension.(map[string]any)
			if stringValue(dimensionMap["type"]) == expected {
				return true
			}
		}
	}
	return false
}

func offsetFromLink(link string) string {
	if link == "" {
		return ""
	}
	re := regexp.MustCompile(`<([^>]+)>`)
	match := re.FindStringSubmatch(link)
	if len(match) != 2 {
		return ""
	}
	parsed, err := url.Parse(match[1])
	if err != nil {
		return ""
	}
	return parsed.Query().Get("offset")
}

func headerValue(headers map[string]string, key string) string {
	if headers == nil {
		return ""
	}
	return headers[strings.ToLower(key)]
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case json.RawMessage:
		return string(typed)
	default:
		if typed == nil {
			return ""
		}
		return fmt.Sprintf("%v", typed)
	}
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func checkpointState(rt *sessionRuntime, caseID, checkpointID string) *CheckpointState {
	result := rt.cases[caseID]
	if result == nil {
		return nil
	}
	for i := range result.Checkpoints {
		if result.Checkpoints[i].ID == checkpointID {
			return &result.Checkpoints[i]
		}
	}
	return nil
}

func manualOrPassed(checkpoint *CheckpointState, intro string, events ...TrafficEvent) CaseEvaluation {
	if checkpoint == nil || strings.TrimSpace(checkpoint.Answer) == "" {
		return CaseEvaluation{
			Status:           "manual",
			Messages:         []string{intro},
			EvidenceEventIDs: collectEventIDs(events),
		}
	}
	answer := strings.ToLower(strings.TrimSpace(checkpoint.Answer))
	if strings.Contains(answer, "still present") || strings.Contains(answer, "not removed") {
		return CaseEvaluation{
			Status:           "failed",
			Messages:         []string{"The manual checkpoint indicates the eMSP side did not reflect the expected deletion semantics."},
			EvidenceEventIDs: collectEventIDs(events),
		}
	}
	return CaseEvaluation{
		Status:           "passed",
		Messages:         []string{intro, "Manual checkpoint answer recorded: " + checkpoint.Answer},
		EvidenceEventIDs: collectEventIDs(events),
	}
}

func passedEval(message string, events ...TrafficEvent) CaseEvaluation {
	return CaseEvaluation{
		Status:           "passed",
		Messages:         []string{message},
		EvidenceEventIDs: collectEventIDs(events),
	}
}

func pendingEval(message string) CaseEvaluation {
	return CaseEvaluation{
		Status:   "pending",
		Messages: []string{message},
	}
}

func pendingEvalWithEvidence(message string, events ...TrafficEvent) CaseEvaluation {
	return CaseEvaluation{
		Status:           "pending",
		Messages:         []string{message},
		EvidenceEventIDs: collectEventIDs(events),
	}
}

func failedEval(intro []string, issues []string, events ...TrafficEvent) CaseEvaluation {
	messages := append([]string(nil), intro...)
	messages = append(messages, issues...)
	return CaseEvaluation{
		Status:           "failed",
		Messages:         uniqueStrings(messages),
		EvidenceEventIDs: collectEventIDs(events),
	}
}

func collectEventIDs(events []TrafficEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		if event.ID != "" {
			ids = append(ids, event.ID)
		}
	}
	return ids
}
