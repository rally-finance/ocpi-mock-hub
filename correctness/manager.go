package correctness

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rally-finance/ocpi-mock-hub/fakegen"
)

var (
	ErrActiveSessionExists = errors.New("an OCPI eMSP correctness session is already active")
	ErrSessionNotFound     = errors.New("correctness session not found")
	ErrUnknownSuite        = errors.New("correctness suite not found")
	ErrUnknownAction       = errors.New("correctness action not found")
	ErrUnknownCheckpoint   = errors.New("correctness checkpoint not found")
)

type sessionRuntime struct {
	suite   SuiteDefinition
	session TestSession
	sandbox *Sandbox
	actions map[string]*ActionState
	cases   map[string]*CaseResult
	events  []TrafficEvent
}

type Manager struct {
	mu       sync.RWMutex
	baseSeed *fakegen.SeedData
	suites   map[string]SuiteDefinition
	sessions map[string]*sessionRuntime
	order    []string
	activeID string
}

func NewManager(baseSeed *fakegen.SeedData) *Manager {
	suites := BuiltinSuites()
	suiteMap := make(map[string]SuiteDefinition, len(suites))
	for _, suite := range suites {
		suiteMap[suite.ID] = suite
	}
	return &Manager{
		baseSeed: baseSeed,
		suites:   suiteMap,
		sessions: make(map[string]*sessionRuntime),
	}
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func (m *Manager) ListSuites() []SuiteDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.suites))
	for id := range m.suites {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	out := make([]SuiteDefinition, 0, len(ids))
	for _, id := range ids {
		out = append(out, m.suites[id])
	}
	return out
}

func (m *Manager) ListSessions() []SessionSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SessionSnapshot, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		rt := m.sessions[m.order[i]]
		if rt == nil {
			continue
		}
		out = append(out, SessionSnapshot{
			ID:          rt.session.ID,
			SuiteID:     rt.session.SuiteID,
			SuiteName:   rt.session.SuiteName,
			Status:      rt.session.Status,
			CreatedAt:   rt.session.CreatedAt,
			UpdatedAt:   rt.session.UpdatedAt,
			CompletedAt: rt.session.CompletedAt,
			Summary:     rt.session.Summary,
			CurrentStep: rt.session.CurrentStep,
		})
	}
	return out
}

func (m *Manager) GetSession(id string) (*TestSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt := m.sessions[id]
	if rt == nil {
		return nil, ErrSessionNotFound
	}
	copied := rt.session
	return &copied, nil
}

func (m *Manager) ActiveSessionID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeID
}

func (m *Manager) CurrentSeed(base *fakegen.SeedData) *fakegen.SeedData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeID == "" {
		return base
	}
	rt := m.sessions[m.activeID]
	if rt == nil || rt.sandbox == nil || rt.sandbox.Seed == nil {
		return base
	}
	return rt.sandbox.Seed
}

func (m *Manager) ActiveOverlay() *OverlayStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeID == "" {
		return nil
	}
	rt := m.sessions[m.activeID]
	if rt == nil || rt.sandbox == nil {
		return nil
	}
	return rt.sandbox.Store
}

func (m *Manager) StartSession(cfg SessionConfig) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeID != "" {
		return nil, ErrActiveSessionExists
	}
	return m.newSessionLocked(cfg)
}

func (m *Manager) RerunSession(id string) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt := m.sessions[id]
	if rt == nil {
		return nil, ErrSessionNotFound
	}
	if m.activeID != "" {
		return nil, ErrActiveSessionExists
	}
	return m.newSessionLocked(rt.session.Config)
}

func (m *Manager) newSessionLocked(cfg SessionConfig) (*TestSession, error) {
	suiteID := cfg.SuiteID
	if suiteID == "" {
		suiteID = DefaultSuiteID
	}
	suite, ok := m.suites[suiteID]
	if !ok {
		return nil, ErrUnknownSuite
	}

	rt := &sessionRuntime{
		suite:   suite,
		sandbox: NewSandbox(m.baseSeed),
		actions: make(map[string]*ActionState, len(suite.Actions)),
		cases:   make(map[string]*CaseResult, len(suite.Cases)),
	}

	id := "CTS-" + uuid.NewString()[:8]
	now := nowUTC()
	rt.session = TestSession{
		ID:        id,
		SuiteID:   suite.ID,
		SuiteName: suite.Name,
		Status:    "running",
		CreatedAt: now,
		UpdatedAt: now,
		Config: SessionConfig{
			SuiteID:         suite.ID,
			Name:            cfg.Name,
			PeerVersionsURL: cfg.PeerVersionsURL,
			PeerToken:       cfg.PeerToken,
		},
		Deferred: suite.Deferred,
	}

	for _, action := range suite.Actions {
		state := ActionState{
			ID:          action.ID,
			Group:       action.Group,
			Title:       action.Title,
			Description: action.Description,
			Kind:        action.Kind,
			Status:      "idle",
		}
		rt.actions[action.ID] = &state
	}
	for _, def := range suite.Cases {
		result := CaseResult{
			ID:          def.ID,
			Group:       def.Group,
			Title:       def.Title,
			Description: def.Description,
			Status:      "pending",
		}
		for _, checkpoint := range def.Checkpoints {
			result.Checkpoints = append(result.Checkpoints, CheckpointState{
				ID:          checkpoint.ID,
				Prompt:      checkpoint.Prompt,
				Placeholder: checkpoint.Placeholder,
				Status:      "pending",
			})
		}
		rt.cases[def.ID] = &result
	}

	m.sessions[id] = rt
	m.order = append(m.order, id)
	m.activeID = id
	m.rebuildSessionLocked(rt)

	copied := rt.session
	return &copied, nil
}

func (m *Manager) RecordTrafficEvent(event TrafficEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeID == "" {
		return
	}
	rt := m.sessions[m.activeID]
	if rt == nil {
		return
	}
	event.SessionID = rt.session.ID
	if event.ID == "" {
		event.ID = "evt-" + uuid.NewString()[:10]
	}
	rt.events = append(rt.events, event)
	m.rebuildSessionLocked(rt)
}

func (m *Manager) MarkActionStarted(sessionID, actionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt := m.sessions[sessionID]
	if rt == nil {
		return ErrSessionNotFound
	}
	action := rt.actions[actionID]
	if action == nil {
		return ErrUnknownAction
	}
	action.Status = "running"
	action.LastRunAt = nowUTC()
	action.LastError = ""
	action.Output = nil
	action.EventAnchor = len(rt.events)
	m.rebuildSessionLocked(rt)
	return nil
}

func (m *Manager) CompleteAction(sessionID, actionID string, output map[string]string) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt := m.sessions[sessionID]
	if rt == nil {
		return nil, ErrSessionNotFound
	}
	action := rt.actions[actionID]
	if action == nil {
		return nil, ErrUnknownAction
	}
	action.Status = "completed"
	action.LastRunAt = nowUTC()
	action.LastError = ""
	action.Output = output
	m.rebuildSessionLocked(rt)
	copied := rt.session
	return &copied, nil
}

func (m *Manager) FailAction(sessionID, actionID string, err error) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt := m.sessions[sessionID]
	if rt == nil {
		return nil, ErrSessionNotFound
	}
	action := rt.actions[actionID]
	if action == nil {
		return nil, ErrUnknownAction
	}
	action.Status = "failed"
	action.LastRunAt = nowUTC()
	action.LastError = err.Error()
	m.rebuildSessionLocked(rt)
	copied := rt.session
	return &copied, nil
}

func (m *Manager) SetPeerState(sessionID string, peer SessionPeerState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt := m.sessions[sessionID]
	if rt == nil {
		return ErrSessionNotFound
	}
	rt.session.Peer = peer
	m.rebuildSessionLocked(rt)
	return nil
}

func (m *Manager) SubmitCheckpoint(sessionID, checkpointID, answer, notes string) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt := m.sessions[sessionID]
	if rt == nil {
		return nil, ErrSessionNotFound
	}

	updated := false
	now := nowUTC()
	for _, result := range rt.cases {
		for i := range result.Checkpoints {
			if result.Checkpoints[i].ID != checkpointID {
				continue
			}
			result.Checkpoints[i].Answer = answer
			result.Checkpoints[i].Notes = notes
			if answer != "" {
				result.Checkpoints[i].Status = "answered"
			} else {
				result.Checkpoints[i].Status = "pending"
			}
			result.Checkpoints[i].UpdatedAt = now
			updated = true
		}
	}
	if !updated {
		return nil, ErrUnknownCheckpoint
	}

	m.rebuildSessionLocked(rt)
	copied := rt.session
	return &copied, nil
}

func (m *Manager) ActionState(sessionID, actionID string) (*ActionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt := m.sessions[sessionID]
	if rt == nil {
		return nil, ErrSessionNotFound
	}
	action := rt.actions[actionID]
	if action == nil {
		return nil, ErrUnknownAction
	}
	copy := *action
	return &copy, nil
}

func (m *Manager) UpdateSandbox(sessionID string, fn func(*Sandbox) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt := m.sessions[sessionID]
	if rt == nil {
		return ErrSessionNotFound
	}
	if rt.sandbox == nil {
		return fmt.Errorf("session sandbox not initialized")
	}
	if err := fn(rt.sandbox); err != nil {
		return err
	}
	m.rebuildSessionLocked(rt)
	return nil
}

func (m *Manager) rebuildSessionLocked(rt *sessionRuntime) {
	caseResults := make([]CaseResult, 0, len(rt.suite.Cases))
	summary := SessionSummary{TotalCases: len(rt.suite.Cases)}
	now := nowUTC()

	evaluators := builtinEvaluators()
	for _, def := range rt.suite.Cases {
		result := rt.cases[def.ID]
		if result == nil {
			result = &CaseResult{
				ID:          def.ID,
				Group:       def.Group,
				Title:       def.Title,
				Description: def.Description,
			}
			rt.cases[def.ID] = result
		}

		if blockedBy := m.blockingRequirement(rt, def); blockedBy != "" {
			result.Status = "blocked"
			result.Messages = []string{fmt.Sprintf("Waiting for %s to pass first.", blockedBy)}
			result.EvidenceEventIDs = nil
			result.UpdatedAt = now
		} else {
			evaluator := evaluators[def.Evaluator]
			eval := CaseEvaluation{
				Status:   "pending",
				Messages: []string{"No matching OCPI evidence has been observed yet."},
			}
			if evaluator == nil {
				eval.Status = "failed"
				eval.Messages = []string{fmt.Sprintf("No evaluator registered for %s.", def.Evaluator)}
			} else {
				eval = evaluator(rt, def)
			}
			result.Status = eval.Status
			result.Messages = eval.Messages
			result.EvidenceEventIDs = eval.EvidenceEventIDs
			result.UpdatedAt = now
			if result.Status == "passed" {
				for i := range result.Checkpoints {
					if result.Checkpoints[i].Answer != "" {
						result.Checkpoints[i].Status = "answered"
					}
				}
			}
		}

		switch result.Status {
		case "passed":
			summary.PassedCases++
		case "failed":
			summary.FailedCases++
		case "manual":
			summary.ManualCases++
		case "blocked":
			summary.BlockedCases++
		default:
			summary.PendingCases++
		}
		caseResults = append(caseResults, *result)
	}

	actionStates := make([]ActionState, 0, len(rt.suite.Actions))
	for _, def := range rt.suite.Actions {
		if state := rt.actions[def.ID]; state != nil {
			actionStates = append(actionStates, *state)
		}
	}

	completedCount := summary.PassedCases + summary.FailedCases
	if summary.TotalCases > 0 {
		summary.CompletionRate = completedCount * 100 / summary.TotalCases
	}

	rt.session.Cases = caseResults
	rt.session.Actions = actionStates
	rt.session.Summary = summary
	rt.session.EventCount = len(rt.events)
	rt.session.RecentEvents = tailEvents(rt.events, 20)
	rt.session.CurrentStep = nextStep(rt.suite, rt.session)
	rt.session.UpdatedAt = now

	if summary.PendingCases == 0 && summary.ManualCases == 0 && summary.BlockedCases == 0 {
		rt.session.Status = "completed"
		if rt.session.CompletedAt == "" {
			rt.session.CompletedAt = now
		}
		if m.activeID == rt.session.ID {
			m.activeID = ""
		}
	} else {
		rt.session.Status = "running"
		rt.session.CompletedAt = ""
	}
}

func (m *Manager) blockingRequirement(rt *sessionRuntime, def CaseDefinition) string {
	for _, dep := range def.Requires {
		result := rt.cases[dep]
		if result == nil || result.Status != "passed" {
			if result != nil && result.Title != "" {
				return result.Title
			}
			return dep
		}
	}
	return ""
}

func nextStep(suite SuiteDefinition, session TestSession) SessionStep {
	actionMap := make(map[string]ActionState, len(session.Actions))
	caseMap := make(map[string]CaseResult, len(session.Cases))
	for _, action := range session.Actions {
		actionMap[action.ID] = action
	}
	for _, item := range session.Cases {
		caseMap[item.ID] = item
	}

	for _, def := range suite.Cases {
		result := caseMap[def.ID]
		switch result.Status {
		case "passed":
			continue
		case "manual":
			for _, cp := range result.Checkpoints {
				if cp.Status != "answered" {
					return SessionStep{
						Title:       result.Title,
						Description: cp.Prompt,
						CaseID:      result.ID,
					}
				}
			}
		case "failed":
			return SessionStep{
				Title:       result.Title,
				Description: firstMessage(result.Messages),
				CaseID:      result.ID,
			}
		default:
			for _, actionID := range def.ActionIDs {
				action := actionMap[actionID]
				if action.Status == "idle" || action.Status == "failed" {
					return SessionStep{
						Title:       action.Title,
						Description: action.Description,
						ActionID:    action.ID,
						CaseID:      def.ID,
					}
				}
				if action.Status == "running" {
					return SessionStep{
						Title:       action.Title,
						Description: "Action is running.",
						ActionID:    action.ID,
						CaseID:      def.ID,
					}
				}
			}
			return SessionStep{
				Title:       result.Title,
				Description: firstMessage(result.Messages),
				CaseID:      result.ID,
			}
		}
	}

	return SessionStep{
		Title:       "Session Complete",
		Description: "All currently included OCPI correctness checks reached a terminal state.",
	}
}

func firstMessage(messages []string) string {
	if len(messages) == 0 {
		return ""
	}
	return messages[0]
}

func tailEvents(events []TrafficEvent, n int) []TrafficEvent {
	if len(events) <= n {
		out := make([]TrafficEvent, len(events))
		copy(out, events)
		return out
	}
	out := make([]TrafficEvent, n)
	copy(out, events[len(events)-n:])
	return out
}
