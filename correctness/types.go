package correctness

import "encoding/json"

type ScenarioSource struct {
	Sheet   string   `json:"sheet"`
	CaseIDs []string `json:"case_ids"`
}

type NormativeSource struct {
	ID        string `json:"id"`
	Reference string `json:"reference"`
	Title     string `json:"title"`
}

type DeferredScenario struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Reason    string            `json:"reason"`
	Scenario  []ScenarioSource  `json:"scenario_source,omitempty"`
	Normative []NormativeSource `json:"normative_source,omitempty"`
}

type CheckpointDefinition struct {
	ID          string `json:"id"`
	Prompt      string `json:"prompt"`
	Placeholder string `json:"placeholder,omitempty"`
}

type ActionDefinition struct {
	ID          string `json:"id"`
	Group       string `json:"group"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

type CaseDefinition struct {
	ID                   string                 `json:"id"`
	Group                string                 `json:"group"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description"`
	Evaluator            string                 `json:"evaluator"`
	ActionIDs            []string               `json:"action_ids,omitempty"`
	Requires             []string               `json:"requires,omitempty"`
	Checkpoints          []CheckpointDefinition `json:"checkpoints,omitempty"`
	ScenarioSource       []ScenarioSource       `json:"scenario_source"`
	NormativeSource      []NormativeSource      `json:"normative_source"`
	CompatibilityProfile string                 `json:"compatibility_profile,omitempty"`
}

type SuiteDefinition struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Actions     []ActionDefinition `json:"actions"`
	Cases       []CaseDefinition   `json:"cases"`
	Deferred    []DeferredScenario `json:"deferred"`
}

type SessionConfig struct {
	SuiteID         string `json:"suite_id"`
	Name            string `json:"name,omitempty"`
	PeerVersionsURL string `json:"peer_versions_url"`
	PeerToken       string `json:"peer_token"`
}

type SessionPeerEndpoint struct {
	Identifier string `json:"identifier"`
	Role       string `json:"role"`
	URL        string `json:"url"`
}

type SessionPeerState struct {
	VersionDetailURL string                `json:"version_detail_url,omitempty"`
	CredentialsURL   string                `json:"credentials_url,omitempty"`
	CountryCode      string                `json:"country_code,omitempty"`
	PartyID          string                `json:"party_id,omitempty"`
	Endpoints        []SessionPeerEndpoint `json:"endpoints,omitempty"`
}

type TrafficEvent struct {
	ID              string            `json:"id"`
	SessionID       string            `json:"session_id,omitempty"`
	ActionID        string            `json:"action_id,omitempty"`
	Direction       string            `json:"direction"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Path            string            `json:"path"`
	RawQuery        string            `json:"raw_query,omitempty"`
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"`
	ResponseStatus  int               `json:"response_status"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body,omitempty"`
	DurationMS      int64             `json:"duration_ms"`
	StartedAt       string            `json:"started_at"`
}

type CheckpointState struct {
	ID          string `json:"id"`
	Prompt      string `json:"prompt"`
	Placeholder string `json:"placeholder,omitempty"`
	Answer      string `json:"answer,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Status      string `json:"status"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type ActionState struct {
	ID          string            `json:"id"`
	Group       string            `json:"group"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Kind        string            `json:"kind"`
	Status      string            `json:"status"`
	LastRunAt   string            `json:"last_run_at,omitempty"`
	LastError   string            `json:"last_error,omitempty"`
	Output      map[string]string `json:"output,omitempty"`

	EventAnchor int `json:"-"`
}

type CaseResult struct {
	ID               string            `json:"id"`
	Group            string            `json:"group"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Status           string            `json:"status"`
	Messages         []string          `json:"messages,omitempty"`
	EvidenceEventIDs []string          `json:"evidence_event_ids,omitempty"`
	Checkpoints      []CheckpointState `json:"checkpoints,omitempty"`
	UpdatedAt        string            `json:"updated_at,omitempty"`
}

type SessionSummary struct {
	TotalCases     int `json:"total_cases"`
	PassedCases    int `json:"passed_cases"`
	FailedCases    int `json:"failed_cases"`
	PendingCases   int `json:"pending_cases"`
	ManualCases    int `json:"manual_cases"`
	BlockedCases   int `json:"blocked_cases"`
	CompletionRate int `json:"completion_rate"`
}

type SessionStep struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	ActionID    string `json:"action_id,omitempty"`
	CaseID      string `json:"case_id,omitempty"`
}

type TestSession struct {
	ID           string             `json:"id"`
	SuiteID      string             `json:"suite_id"`
	SuiteName    string             `json:"suite_name"`
	Status       string             `json:"status"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
	CompletedAt  string             `json:"completed_at,omitempty"`
	Config       SessionConfig      `json:"config"`
	Peer         SessionPeerState   `json:"peer"`
	Summary      SessionSummary     `json:"summary"`
	CurrentStep  SessionStep        `json:"current_step"`
	Actions      []ActionState      `json:"actions"`
	Cases        []CaseResult       `json:"cases"`
	Deferred     []DeferredScenario `json:"deferred,omitempty"`
	EventCount   int                `json:"event_count"`
	RecentEvents []TrafficEvent     `json:"recent_events,omitempty"`
}

type SessionSnapshot struct {
	ID          string         `json:"id"`
	SuiteID     string         `json:"suite_id"`
	SuiteName   string         `json:"suite_name"`
	Status      string         `json:"status"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	CompletedAt string         `json:"completed_at,omitempty"`
	Summary     SessionSummary `json:"summary"`
	CurrentStep SessionStep    `json:"current_step"`
}

type CaseEvaluation struct {
	Status           string
	Messages         []string
	EvidenceEventIDs []string
}

type OCPIEnvelope struct {
	Data          json.RawMessage `json:"data"`
	StatusCode    int             `json:"status_code"`
	StatusMessage string          `json:"status_message"`
	Timestamp     string          `json:"timestamp"`
}
