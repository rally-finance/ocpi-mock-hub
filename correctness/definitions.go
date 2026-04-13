package correctness

const DefaultSuiteID = "ocpi-2-2-1-emsp-correctness"

func BuiltinSuites() []SuiteDefinition {
	return []SuiteDefinition{builtinOCPIEMSPSuite()}
}

func builtinOCPIEMSPSuite() SuiteDefinition {
	return SuiteDefinition{
		ID:          DefaultSuiteID,
		Name:        "OCPI 2.2.1 eMSP Correctness Tests",
		Description: "Interactive OCPI eMSP implementation checks built from scenario prompts and official OCPI 2.2.1 rules.",
		Actions: []ActionDefinition{
			{
				ID:          "run_handshake",
				Group:       "Handshake",
				Title:       "Run Handshake",
				Description: "The mock hub discovers the peer OCPI endpoints and performs the credentials exchange from the beginning.",
				Kind:        "outbound",
			},
			{
				ID:          "arm_pull_locations_full",
				Group:       "Locations Pull",
				Title:       "Wait For Full Locations Pull",
				Description: "Arm the session, then trigger a full pull on the eMSP so the hub can validate the request sequence and paging behavior.",
				Kind:        "observe",
			},
			{
				ID:          "prepare_pull_locations_delta_update",
				Group:       "Locations Pull",
				Title:       "Prepare Delta Location Update",
				Description: "Mutate one location in the session sandbox, then trigger a delta pull on the eMSP.",
				Kind:        "prepare",
			},
			{
				ID:          "arm_pull_tariffs_full",
				Group:       "Tariffs Pull",
				Title:       "Wait For Full Tariffs Pull",
				Description: "Arm the session, then trigger a full tariffs pull on the eMSP so the hub can validate paging and headers.",
				Kind:        "observe",
			},
			{
				ID:          "prepare_pull_tariffs_delta_update",
				Group:       "Tariffs Pull",
				Title:       "Prepare Delta Tariff Update",
				Description: "Mutate one tariff in the session sandbox, then trigger a delta pull on the eMSP.",
				Kind:        "prepare",
			},
			{
				ID:          "arm_push_token_create",
				Group:       "Token Push",
				Title:       "Wait For Token Create",
				Description: "Arm the session, then ask the eMSP to push a new Token object using the instructed RFID token profile with valid=true and whitelist=ALLOWED.",
				Kind:        "observe",
			},
			{
				ID:          "arm_push_token_update",
				Group:       "Token Push",
				Title:       "Wait For Token Update",
				Description: "Arm the session, then ask the eMSP to update the same Token object identified in the action output.",
				Kind:        "observe",
			},
			{
				ID:          "run_rta_valid",
				Group:       "Real-time Authorization",
				Title:       "Run Valid Authorization",
				Description: "Use the discovered Tokens sender endpoint to send an authorization request for the known valid Token object prepared in this session.",
				Kind:        "outbound",
			},
			{
				ID:          "arm_remote_start",
				Group:       "Commands",
				Title:       "Wait For Remote Start",
				Description: "Arm the session, then ask the eMSP to send START_SESSION using the non-removed happy-path location and token shown in the action output.",
				Kind:        "observe",
			},
			{
				ID:          "arm_remote_stop",
				Group:       "Commands",
				Title:       "Wait For Remote Stop",
				Description: "Arm the session, then ask the eMSP to send STOP_SESSION for the active mock session.",
				Kind:        "observe",
			},
			{
				ID:          "prepare_pull_locations_full_delete_connector",
				Group:       "Locations Pull",
				Title:       "Prepare Full Connector Deletion",
				Description: "Remove one connector from the sandbox, then trigger a full locations pull and confirm the connector disappeared on the eMSP side.",
				Kind:        "prepare",
			},
			{
				ID:          "prepare_pull_locations_delta_delete_evse",
				Group:       "Locations Pull",
				Title:       "Prepare Delta EVSE Removal",
				Description: "Mark one EVSE as removed in the sandbox, then trigger a delta pull and confirm the removal on the eMSP side.",
				Kind:        "prepare",
			},
			{
				ID:          "prepare_pull_locations_delta_delete_location",
				Group:       "Locations Pull",
				Title:       "Prepare Delta Location Removal",
				Description: "Mark one location as removed in the sandbox, then trigger a delta pull and confirm the removal on the eMSP side.",
				Kind:        "prepare",
			},
			{
				ID:          "arm_push_token_invalidate",
				Group:       "Token Push",
				Title:       "Wait For Token Invalidation",
				Description: "Arm the session, then ask the eMSP to invalidate the same Token object by pushing it again with valid set to false.",
				Kind:        "observe",
			},
			{
				ID:          "run_rta_invalid",
				Group:       "Real-time Authorization",
				Title:       "Run Invalid Authorization",
				Description: "Use the discovered Tokens sender endpoint to send an authorization request for the invalidated token from this session and validate the peer response.",
				Kind:        "outbound",
			},
			{
				ID:          "run_evse_status_unknown",
				Group:       "Push To eMSP",
				Title:       "Push Unknown EVSE Status",
				Description: "Send a PATCH-like EVSE status update for an unknown EVSE using the peer locations receiver endpoint.",
				Kind:        "outbound",
			},
			{
				ID:          "run_evse_status_known",
				Group:       "Push To eMSP",
				Title:       "Push Known EVSE Status",
				Description: "Send a PATCH-like EVSE status update for a known EVSE using the peer locations receiver endpoint.",
				Kind:        "outbound",
			},
			{
				ID:          "run_session_push_pending",
				Group:       "Push To eMSP",
				Title:       "Push Pending Session",
				Description: "Push a PENDING Session object to the peer sessions receiver endpoint.",
				Kind:        "outbound",
			},
			{
				ID:          "run_session_push_active",
				Group:       "Push To eMSP",
				Title:       "Push Active Session",
				Description: "Push an ACTIVE Session update for the same session identifier.",
				Kind:        "outbound",
			},
			{
				ID:          "run_session_push_completed",
				Group:       "Push To eMSP",
				Title:       "Push Completed Session",
				Description: "Push a COMPLETED Session update for the same session identifier.",
				Kind:        "outbound",
			},
			{
				ID:          "run_cdr_push",
				Group:       "Push To eMSP",
				Title:       "Push CDR",
				Description: "Push a CDR to the peer CDRs receiver endpoint.",
				Kind:        "outbound",
			},
			{
				ID:          "run_unregister",
				Group:       "Handshake",
				Title:       "Run Unregister",
				Description: "Send DELETE Credentials to the peer after the rest of the correctness flow is complete.",
				Kind:        "outbound",
			},
		},
		Cases: []CaseDefinition{
			{
				ID:          "handshake_flow",
				Group:       "Handshake",
				Title:       "Hub-Initiated Versions, Version Details, and Credentials Exchange",
				Description: "Validate the official OCPI handshake flow initiated by the mock hub against the peer versions endpoint using the session's configured peer token.",
				Evaluator:   "handshake_flow",
				ActionIDs:   []string{"run_handshake"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Register FromIOP", CaseIDs: []string{"REG_FromIOP_2", "REG_FromIOP_3", "REG_FromIOP_6", "REG_FromIOP_7", "REG_FromIOP_8", "REG_FromIOP_9"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "versions", Reference: "version_information_endpoint", Title: "Versions and Version Details"},
					{ID: "credentials", Reference: "credentials", Title: "Credentials Module"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "peer_fetches_hub_versions",
				Group:       "Handshake",
				Title:       "Peer Fetches Mock Hub Versions and Version Details",
				Description: "Validate that the peer follows the credentials exchange by calling the mock hub versions endpoints with the session token returned by the hub.",
				Evaluator:   "peer_fetches_hub_versions",
				ActionIDs:   []string{"run_handshake"},
				Requires:    []string{"handshake_flow"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Register FromIOP", CaseIDs: []string{"REG_FromIOP_1", "REG_FromIOP_4", "REG_FromIOP_5"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "versions", Reference: "version_information_endpoint", Title: "Versions and Version Details"},
					{ID: "transport-auth", Reference: "transport_and_format#authorization", Title: "Authorization Header"},
					{ID: "transport-get", Reference: "transport_and_format#GET", Title: "GET Request Semantics"},
				},
			},
			{
				ID:          "pull_locations_full",
				Group:       "Locations Pull",
				Title:       "Full Locations Pull",
				Description: "Validate the full pull request, request headers, and pagination sequence for the Locations sender endpoint.",
				Evaluator:   "pull_locations_full",
				ActionIDs:   []string{"arm_pull_locations_full"},
				Requires:    []string{"peer_fetches_hub_versions"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Pull Locations ToIOP", CaseIDs: []string{"Pull_Locations_1", "Pull_Locations_3", "Pull_Locations_4"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "transport-auth", Reference: "transport_and_format#authorization", Title: "Authorization Header"},
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Pagination"},
					{ID: "locations-sender", Reference: "mod_locations", Title: "Locations Sender Interface"},
				},
			},
			{
				ID:          "pull_locations_delta_update",
				Group:       "Locations Pull",
				Title:       "Delta Locations Pull After Update",
				Description: "Mutate one location and validate that the eMSP performs a delta pull with a valid date range and receives the updated object.",
				Evaluator:   "pull_locations_delta_update",
				ActionIDs:   []string{"prepare_pull_locations_delta_update"},
				Requires:    []string{"pull_locations_full"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Pull Locations ToIOP", CaseIDs: []string{"Pull_Locations_10", "Pull_Locations_14"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Pagination and Date Ranges"},
					{ID: "locations-sender", Reference: "mod_locations", Title: "Locations Sender Interface"},
				},
			},
			{
				ID:          "pull_tariffs_full",
				Group:       "Tariffs Pull",
				Title:       "Full Tariffs Pull",
				Description: "Validate the full pull request, request headers, and pagination sequence for the Tariffs sender endpoint.",
				Evaluator:   "pull_tariffs_full",
				ActionIDs:   []string{"arm_pull_tariffs_full"},
				Requires:    []string{"peer_fetches_hub_versions"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Pull Tariff (ToIOP)", CaseIDs: []string{"Pull_Tariff_1", "Pull_Tariff_2", "Pull_Tariff_3", "Pull_Tariff_4"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "transport-auth", Reference: "transport_and_format#authorization", Title: "Authorization Header"},
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Pagination"},
					{ID: "tariffs-sender", Reference: "mod_tariffs", Title: "Tariffs Sender Interface"},
				},
			},
			{
				ID:          "pull_tariffs_delta_update",
				Group:       "Tariffs Pull",
				Title:       "Delta Tariffs Pull After Update",
				Description: "Mutate one tariff in the sandbox and validate that the eMSP performs a delta pull and receives the updated tariff.",
				Evaluator:   "pull_tariffs_delta_update",
				ActionIDs:   []string{"prepare_pull_tariffs_delta_update"},
				Requires:    []string{"pull_tariffs_full"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Pull Tariff (ToIOP)", CaseIDs: []string{"Pull_Tariff_6", "Pull_Tariff_7"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Pagination and Date Ranges"},
					{ID: "tariffs-sender", Reference: "mod_tariffs", Title: "Tariffs Sender Interface"},
				},
			},
			{
				ID:          "token_push_create",
				Group:       "Token Push",
				Title:       "Token Push Create",
				Description: "Validate a new Token push from the eMSP to the hub.",
				Evaluator:   "token_push_create",
				ActionIDs:   []string{"arm_push_token_create"},
				Requires:    []string{"peer_fetches_hub_versions"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push Token ToIOP", CaseIDs: []string{"Push_Token_1", "Push_Token_2", "Push_Token_3", "Push_Token_4"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "tokens-receiver", Reference: "mod_tokens", Title: "Tokens Receiver Interface"},
					{ID: "transport-client-owned", Reference: "transport_and_format#client_owned_push", Title: "Client Owned Object Push"},
					{ID: "types", Reference: "types", Title: "Common OCPI Types"},
				},
			},
			{
				ID:          "token_push_update",
				Group:       "Token Push",
				Title:       "Token Push Update",
				Description: "Validate an update to a previously pushed Token object.",
				Evaluator:   "token_push_update",
				ActionIDs:   []string{"arm_push_token_update"},
				Requires:    []string{"token_push_create"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push Token ToIOP", CaseIDs: []string{"Push_Token_3", "Push_Token_4"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "tokens-receiver", Reference: "mod_tokens", Title: "Tokens Receiver Interface"},
					{ID: "transport-put", Reference: "transport_and_format#PUT", Title: "PUT Semantics"},
				},
			},
			{
				ID:          "rta_valid",
				Group:       "Real-time Authorization",
				Title:       "Valid Token Authorization",
				Description: "Validate the peer response when the hub asks for authorization of a known valid token.",
				Evaluator:   "rta_valid",
				ActionIDs:   []string{"run_rta_valid"},
				Requires:    []string{"peer_fetches_hub_versions", "token_push_update"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "RTA FromIOP", CaseIDs: []string{"RT_Authorization_3"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "tokens-rta", Reference: "mod_tokens", Title: "Real-time Authorization"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "remote_start",
				Group:       "Commands",
				Title:       "Remote Start Session",
				Description: "Validate the inbound START_SESSION command and the resulting callback flow.",
				Evaluator:   "remote_start",
				ActionIDs:   []string{"arm_remote_start"},
				Requires:    []string{"rta_valid"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Start Session", CaseIDs: []string{"Remote_Start_1", "Remote_Start_2", "Remote_Start_3", "Remote_Start_4", "Remote_Start_5"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "commands", Reference: "mod_commands", Title: "Commands Module"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "remote_stop",
				Group:       "Commands",
				Title:       "Remote Stop Session",
				Description: "Validate the inbound STOP_SESSION command and the resulting callback flow.",
				Evaluator:   "remote_stop",
				ActionIDs:   []string{"arm_remote_stop"},
				Requires:    []string{"remote_start"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Stop Session", CaseIDs: []string{"Remote_Stop_1", "Remote_Stop_2", "Remote_Stop_3", "Remote_Stop_4", "Remote_Stop_5"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "commands", Reference: "mod_commands", Title: "Commands Module"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "pull_locations_full_delete_connector",
				Group:       "Locations Pull",
				Title:       "Full Locations Pull After Connector Removal",
				Description: "Remove one connector from the sandbox and validate both the full payload semantics and the tester's observation on the eMSP side.",
				Evaluator:   "pull_locations_full_delete_connector",
				ActionIDs:   []string{"prepare_pull_locations_full_delete_connector"},
				Requires:    []string{"pull_locations_full"},
				Checkpoints: []CheckpointDefinition{
					{
						ID:          "confirm_connector_removed_after_full_pull",
						Prompt:      "After the full sync finished, what did you observe on the eMSP side for the removed connector?",
						Placeholder: "Example: connector absent from location view",
					},
				},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Pull Locations ToIOP", CaseIDs: []string{"Pull_Locations_7"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "transport-get", Reference: "transport_and_format#GET", Title: "Full Sync Semantics"},
					{ID: "locations-sender", Reference: "mod_locations", Title: "Locations Sender Interface"},
				},
			},
			{
				ID:          "pull_locations_delta_delete_evse",
				Group:       "Locations Pull",
				Title:       "Delta Locations Pull After EVSE Removal",
				Description: "Mark one EVSE as REMOVED and validate the delta payload plus the tester's observation on the eMSP side.",
				Evaluator:   "pull_locations_delta_delete_evse",
				ActionIDs:   []string{"prepare_pull_locations_delta_delete_evse"},
				Requires:    []string{"pull_locations_delta_update"},
				Checkpoints: []CheckpointDefinition{
					{
						ID:          "confirm_evse_removed_after_delta_pull",
						Prompt:      "After the delta sync finished, what did you observe on the eMSP side for the removed EVSE?",
						Placeholder: "Example: EVSE marked removed or hidden",
					},
				},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Pull Locations ToIOP", CaseIDs: []string{"Pull_Locations_12"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "locations-removed", Reference: "mod_locations", Title: "EVSE Removed Status"},
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Date Range Pagination"},
				},
			},
			{
				ID:          "pull_locations_delta_delete_location",
				Group:       "Locations Pull",
				Title:       "Delta Locations Pull After Location Removal",
				Description: "Mark all EVSEs on one location as REMOVED and validate the delta payload plus the tester's observation on the eMSP side.",
				Evaluator:   "pull_locations_delta_delete_location",
				ActionIDs:   []string{"prepare_pull_locations_delta_delete_location"},
				Requires:    []string{"pull_locations_delta_update"},
				Checkpoints: []CheckpointDefinition{
					{
						ID:          "confirm_location_removed_after_delta_pull",
						Prompt:      "After the delta sync finished, what did you observe on the eMSP side for the removed location?",
						Placeholder: "Example: location hidden or all EVSEs removed",
					},
				},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Pull Locations ToIOP", CaseIDs: []string{"Pull_Locations_13"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "locations-removed", Reference: "mod_locations", Title: "Location Removal via EVSE Status"},
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Date Range Pagination"},
				},
			},
			{
				ID:          "token_push_invalidate",
				Group:       "Token Push",
				Title:       "Token Push Invalidation",
				Description: "Validate token invalidation using a Token push with valid set to false.",
				Evaluator:   "token_push_invalidate",
				ActionIDs:   []string{"arm_push_token_invalidate"},
				Requires:    []string{"token_push_update"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push Token ToIOP", CaseIDs: []string{"Push_Token_4"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "tokens-invalid", Reference: "mod_tokens", Title: "Token Invalidation"},
					{ID: "transport-put", Reference: "transport_and_format#PUT", Title: "PUT Semantics"},
				},
			},
			{
				ID:          "rta_invalid",
				Group:       "Real-time Authorization",
				Title:       "Invalid Token Authorization",
				Description: "Validate the peer response when the hub asks for authorization of an invalid token.",
				Evaluator:   "rta_invalid",
				ActionIDs:   []string{"run_rta_invalid"},
				Requires:    []string{"token_push_invalidate"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "RTA FromIOP", CaseIDs: []string{"RT_Authorization_2"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "tokens-rta", Reference: "mod_tokens", Title: "Real-time Authorization"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "evse_status_unknown",
				Group:       "Push To eMSP",
				Title:       "Unknown EVSE Status Push",
				Description: "Validate the peer behavior when the hub sends a status update for an unknown EVSE.",
				Evaluator:   "evse_status_unknown",
				ActionIDs:   []string{"run_evse_status_unknown"},
				Requires:    []string{"peer_fetches_hub_versions"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push EVSE status FromIOP", CaseIDs: []string{"Push_EVSEStatus_2"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "locations-receiver", Reference: "mod_locations", Title: "Locations Receiver Interface"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "evse_status_known",
				Group:       "Push To eMSP",
				Title:       "Known EVSE Status Push",
				Description: "Validate the peer behavior when the hub sends a status update for a known EVSE.",
				Evaluator:   "evse_status_known",
				ActionIDs:   []string{"run_evse_status_known"},
				Requires:    []string{"peer_fetches_hub_versions"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push EVSE status FromIOP", CaseIDs: []string{"Push_EVSEStatus_3", "Push_EVSEStatus_4"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "locations-receiver", Reference: "mod_locations", Title: "Locations Receiver Interface"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "session_push_pending",
				Group:       "Push To eMSP",
				Title:       "Pending Session Push",
				Description: "Validate the peer response when the hub pushes a PENDING Session object.",
				Evaluator:   "session_push_pending",
				ActionIDs:   []string{"run_session_push_pending"},
				Requires:    []string{"peer_fetches_hub_versions"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push Sessions FromIOP", CaseIDs: []string{"Push_Session_2"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "sessions-receiver", Reference: "mod_sessions", Title: "Sessions Receiver Interface"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "session_push_active",
				Group:       "Push To eMSP",
				Title:       "Active Session Push",
				Description: "Validate the peer response when the hub pushes an ACTIVE Session update.",
				Evaluator:   "session_push_active",
				ActionIDs:   []string{"run_session_push_active"},
				Requires:    []string{"session_push_pending"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push Sessions FromIOP", CaseIDs: []string{"Push_Session_3"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "sessions-receiver", Reference: "mod_sessions", Title: "Sessions Receiver Interface"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "session_push_completed",
				Group:       "Push To eMSP",
				Title:       "Completed Session Push",
				Description: "Validate the peer response when the hub pushes a COMPLETED Session update.",
				Evaluator:   "session_push_completed",
				ActionIDs:   []string{"run_session_push_completed"},
				Requires:    []string{"session_push_active"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push Sessions FromIOP", CaseIDs: []string{"Push_Session_4"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "sessions-status", Reference: "mod_sessions", Title: "Session Lifecycle"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "cdr_push",
				Group:       "Push To eMSP",
				Title:       "CDR Push",
				Description: "Validate the peer response when the hub pushes a CDR.",
				Evaluator:   "cdr_push",
				ActionIDs:   []string{"run_cdr_push"},
				Requires:    []string{"session_push_completed"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Push CDR FromIOP", CaseIDs: []string{"Push_CDR_3"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "cdrs-receiver", Reference: "mod_cdrs", Title: "CDRs Receiver Interface"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:          "unregister_flow",
				Group:       "Handshake",
				Title:       "Credentials Unregister",
				Description: "Validate the peer DELETE Credentials response and timing after the rest of the correctness flow has completed.",
				Evaluator:   "unregister_flow",
				ActionIDs:   []string{"run_unregister"},
				Requires:    []string{"peer_fetches_hub_versions"},
				ScenarioSource: []ScenarioSource{
					{Sheet: "Unregister FromIOP", CaseIDs: []string{"Unregister_FromIOP_1", "Unregister_FromIOP_2"}},
				},
				NormativeSource: []NormativeSource{
					{ID: "credentials", Reference: "credentials", Title: "Credentials Module"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
		},
		Deferred: []DeferredScenario{
			{
				ID:     "locations_push_receiver_routes",
				Title:  "Inbound Locations Receiver Checks",
				Reason: "Deferred because the current mock hub does not expose receiver-side locations routes for partner pushes in the base implementation.",
				Scenario: []ScenarioSource{
					{Sheet: "Push Locations ToIOP", CaseIDs: []string{"Push_Location_1", "Push_Location_2", "Push_Location_3", "Push_Location_4"}},
					{Sheet: "Push Static Data ToIOP"},
				},
				Normative: []NormativeSource{
					{ID: "locations-receiver", Reference: "mod_locations", Title: "Locations Receiver Interface"},
					{ID: "transport-response", Reference: "transport_and_format#response", Title: "OCPI Response Envelope"},
				},
			},
			{
				ID:     "partner_pull_cdrs",
				Title:  "Hub Pulls CDRs From Peer",
				Reason: "Deferred because it requires a new outbound pull client and partner-side list crawlers beyond the v1 scope.",
				Scenario: []ScenarioSource{
					{Sheet: "Pull CDR(s) FromIOP", CaseIDs: []string{"Pull_CDR_1", "Pull_CDR_2", "Pull_CDR_3"}},
				},
				Normative: []NormativeSource{
					{ID: "cdrs-sender", Reference: "mod_cdrs", Title: "CDRs Sender Interface"},
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Pagination"},
				},
			},
			{
				ID:     "partner_pull_tariffs",
				Title:  "Hub Pulls Tariffs From Peer",
				Reason: "Deferred because it requires a new outbound pull client and partner-side list crawlers beyond the v1 scope.",
				Scenario: []ScenarioSource{
					{Sheet: "Pull Tariffs FromIOP", CaseIDs: []string{"Pull_Tariffs_1", "Pull_Tariffs_2", "Pull_Tariffs_3"}},
				},
				Normative: []NormativeSource{
					{ID: "tariffs-sender", Reference: "mod_tariffs", Title: "Tariffs Sender Interface"},
					{ID: "transport-pagination", Reference: "transport_and_format#pagination", Title: "Pagination"},
				},
			},
		},
	}
}
