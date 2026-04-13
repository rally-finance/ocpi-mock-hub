package correctness

import (
	"encoding/json"
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

type persistedActionState struct {
	ID          string            `json:"id"`
	Group       string            `json:"group"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Kind        string            `json:"kind"`
	Status      string            `json:"status"`
	LastRunAt   string            `json:"last_run_at,omitempty"`
	LastError   string            `json:"last_error,omitempty"`
	Output      map[string]string `json:"output,omitempty"`
	EventAnchor int               `json:"event_anchor,omitempty"`
}

type persistedSessionRuntime struct {
	SuiteID string                          `json:"suite_id"`
	Session TestSession                     `json:"session"`
	Seed    *fakegen.SeedData               `json:"seed,omitempty"`
	Actions map[string]persistedActionState `json:"actions,omitempty"`
	Cases   map[string]CaseResult           `json:"cases,omitempty"`
	Events  []TrafficEvent                  `json:"events,omitempty"`
}

type persistedManagerState struct {
	Sessions map[string]persistedSessionRuntime `json:"sessions,omitempty"`
	Order    []string                           `json:"order,omitempty"`
	ActiveID string                             `json:"active_id,omitempty"`
}

type sessionRuntime struct {
	suite   SuiteDefinition
	session TestSession
	sandbox *Sandbox
	actions map[string]*ActionState
	cases   map[string]*CaseResult
	events  []TrafficEvent
}

type Manager struct {
	mu         sync.Mutex
	baseSeed   *fakegen.SeedData
	suites     map[string]SuiteDefinition
	sessions   map[string]*sessionRuntime
	order      []string
	activeID   string
	stateStore StateStore
}

func NewManager(baseSeed *fakegen.SeedData, stores ...StateStore) *Manager {
	suites := BuiltinSuites()
	suiteMap := make(map[string]SuiteDefinition, len(suites))
	for _, suite := range suites {
		suiteMap[suite.ID] = suite
	}

	var stateStore StateStore
	if len(stores) > 0 && stores[0] != nil {
		stateStore = stores[0]
	} else {
		stateStore = newMemoryStateStore()
	}

	manager := &Manager{
		baseSeed:   baseSeed,
		suites:     suiteMap,
		sessions:   make(map[string]*sessionRuntime),
		stateStore: stateStore,
	}
	_ = manager.loadStateLocked()
	return manager
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func decodeManagerState(raw []byte) (persistedManagerState, error) {
	state := persistedManagerState{
		Sessions: make(map[string]persistedSessionRuntime),
	}
	if len(raw) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return persistedManagerState{}, err
	}
	if state.Sessions == nil {
		state.Sessions = make(map[string]persistedSessionRuntime)
	}
	return state, nil
}

func (m *Manager) applyStateLocked(state persistedManagerState) {
	sessions := make(map[string]*sessionRuntime, len(state.Sessions))
	for id, persisted := range state.Sessions {
		suiteID := persisted.SuiteID
		if suiteID == "" {
			suiteID = persisted.Session.SuiteID
		}
		suite := m.suites[suiteID]
		seed := persisted.Seed
		if seed == nil {
			seed = m.baseSeed
		}
		sessions[id] = &sessionRuntime{
			suite:   suite,
			session: persisted.Session,
			sandbox: newSessionSandbox(seed, m.stateStore, id),
			actions: materializeActionStates(persisted, suite),
			cases:   materializeCaseResults(persisted, suite),
			events:  append([]TrafficEvent(nil), persisted.Events...),
		}
		if sessions[id].session.SuiteName == "" && suite.Name != "" {
			sessions[id].session.SuiteName = suite.Name
		}
	}

	m.sessions = sessions
	m.order = normalizeSessionOrder(state.Order, sessions)
	m.activeID = state.ActiveID
	if m.sessions[m.activeID] == nil {
		m.activeID = ""
	}
}

func normalizeSessionOrder(order []string, sessions map[string]*sessionRuntime) []string {
	seen := make(map[string]struct{}, len(sessions))
	normalized := make([]string, 0, len(sessions))
	for _, id := range order {
		if _, ok := seen[id]; ok || sessions[id] == nil {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	var missing []string
	for id := range sessions {
		if _, ok := seen[id]; ok {
			continue
		}
		missing = append(missing, id)
	}
	slices.Sort(missing)
	return append(normalized, missing...)
}

func materializeActionStates(persisted persistedSessionRuntime, suite SuiteDefinition) map[string]*ActionState {
	actions := make(map[string]*ActionState, max(len(persisted.Actions), len(suite.Actions)))
	for id, action := range persisted.Actions {
		copy := ActionState{
			ID:          action.ID,
			Group:       action.Group,
			Title:       action.Title,
			Description: action.Description,
			Kind:        action.Kind,
			Status:      action.Status,
			LastRunAt:   action.LastRunAt,
			LastError:   action.LastError,
			Output:      copyStringMap(action.Output),
			EventAnchor: action.EventAnchor,
		}
		actions[id] = &copy
	}
	if len(actions) == 0 {
		for _, action := range persisted.Session.Actions {
			copy := action
			copy.Output = copyStringMap(action.Output)
			actions[action.ID] = &copy
		}
	}
	for _, definition := range suite.Actions {
		if actions[definition.ID] != nil {
			continue
		}
		state := ActionState{
			ID:          definition.ID,
			Group:       definition.Group,
			Title:       definition.Title,
			Description: definition.Description,
			Kind:        definition.Kind,
			Status:      "idle",
		}
		actions[definition.ID] = &state
	}
	return actions
}

func materializeCaseResults(persisted persistedSessionRuntime, suite SuiteDefinition) map[string]*CaseResult {
	results := make(map[string]*CaseResult, max(len(persisted.Cases), len(suite.Cases)))
	for id, result := range persisted.Cases {
		copy := result
		copy.Messages = append([]string(nil), result.Messages...)
		copy.EvidenceEventIDs = append([]string(nil), result.EvidenceEventIDs...)
		copy.Checkpoints = append([]CheckpointState(nil), result.Checkpoints...)
		results[id] = &copy
	}
	if len(results) == 0 {
		for _, item := range persisted.Session.Cases {
			copy := item
			copy.Messages = append([]string(nil), item.Messages...)
			copy.EvidenceEventIDs = append([]string(nil), item.EvidenceEventIDs...)
			copy.Checkpoints = append([]CheckpointState(nil), item.Checkpoints...)
			results[item.ID] = &copy
		}
	}
	for _, definition := range suite.Cases {
		if results[definition.ID] != nil {
			continue
		}
		result := CaseResult{
			ID:          definition.ID,
			Group:       definition.Group,
			Title:       definition.Title,
			Description: definition.Description,
			Status:      "pending",
		}
		for _, checkpoint := range definition.Checkpoints {
			result.Checkpoints = append(result.Checkpoints, CheckpointState{
				ID:          checkpoint.ID,
				Prompt:      checkpoint.Prompt,
				Placeholder: checkpoint.Placeholder,
				Status:      "pending",
			})
		}
		results[definition.ID] = &result
	}
	return results
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func (m *Manager) serializeStateLocked() persistedManagerState {
	state := persistedManagerState{
		Sessions: make(map[string]persistedSessionRuntime, len(m.sessions)),
		Order:    append([]string(nil), m.order...),
		ActiveID: m.activeID,
	}
	for id, rt := range m.sessions {
		if rt == nil {
			continue
		}
		persisted := persistedSessionRuntime{
			SuiteID: rt.suite.ID,
			Session: rt.session,
			Seed:    CloneSeed(rt.sandbox.Seed),
			Actions: make(map[string]persistedActionState, len(rt.actions)),
			Cases:   make(map[string]CaseResult, len(rt.cases)),
			Events:  append([]TrafficEvent(nil), rt.events...),
		}
		for actionID, action := range rt.actions {
			if action == nil {
				continue
			}
			persisted.Actions[actionID] = persistedActionState{
				ID:          action.ID,
				Group:       action.Group,
				Title:       action.Title,
				Description: action.Description,
				Kind:        action.Kind,
				Status:      action.Status,
				LastRunAt:   action.LastRunAt,
				LastError:   action.LastError,
				Output:      copyStringMap(action.Output),
				EventAnchor: action.EventAnchor,
			}
		}
		for caseID, result := range rt.cases {
			if result == nil {
				continue
			}
			copy := *result
			copy.Messages = append([]string(nil), result.Messages...)
			copy.EvidenceEventIDs = append([]string(nil), result.EvidenceEventIDs...)
			copy.Checkpoints = append([]CheckpointState(nil), result.Checkpoints...)
			persisted.Cases[caseID] = copy
		}
		state.Sessions[id] = persisted
	}
	return state
}

func (m *Manager) loadStateLocked() error {
	raw, err := m.stateStore.GetBlob(managerStateBlobKey)
	if err != nil {
		return err
	}
	state, err := decodeManagerState(raw)
	if err != nil {
		return err
	}
	m.applyStateLocked(state)
	return nil
}

func (m *Manager) withStateMutationLocked(fn func() error) error {
	err := m.stateStore.UpdateBlob(managerStateBlobKey, func(raw []byte) ([]byte, error) {
		state, err := decodeManagerState(raw)
		if err != nil {
			return nil, err
		}
		m.applyStateLocked(state)
		if err := fn(); err != nil {
			return nil, err
		}
		return json.Marshal(m.serializeStateLocked())
	})
	if err != nil {
		_ = m.loadStateLocked()
	}
	return err
}

func (m *Manager) ListSuites() []SuiteDefinition {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return nil
	}

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
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return nil, err
	}
	rt := m.sessions[id]
	if rt == nil {
		return nil, ErrSessionNotFound
	}
	copied := rt.session
	return &copied, nil
}

func (m *Manager) ActiveSessionID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return ""
	}
	return m.activeID
}

func (m *Manager) OverlayForSession(sessionID string) *OverlayStore {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return nil
	}
	if sessionID == "" || m.sessions[sessionID] == nil {
		return nil
	}
	return NewOverlayStore(m.stateStore, sessionID)
}

func (m *Manager) CurrentSeed(base *fakegen.SeedData) *fakegen.SeedData {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return base
	}
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return nil
	}
	if m.activeID == "" || m.sessions[m.activeID] == nil {
		return nil
	}
	return NewOverlayStore(m.stateStore, m.activeID)
}

func (m *Manager) StartSession(cfg SessionConfig) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var created *TestSession
	err := m.withStateMutationLocked(func() error {
		if m.activeID != "" {
			return ErrActiveSessionExists
		}
		session, err := m.newSessionLocked(cfg)
		if err != nil {
			return err
		}
		created = session
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (m *Manager) RerunSession(id string) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var created *TestSession
	err := m.withStateMutationLocked(func() error {
		rt := m.sessions[id]
		if rt == nil {
			return ErrSessionNotFound
		}
		if m.activeID != "" {
			return ErrActiveSessionExists
		}
		session, err := m.newSessionLocked(rt.session.Config)
		if err != nil {
			return err
		}
		created = session
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
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

	id := "CTS-" + uuid.NewString()[:8]
	rt := &sessionRuntime{
		suite:   suite,
		sandbox: newSessionSandbox(m.baseSeed, m.stateStore, id),
		actions: make(map[string]*ActionState, len(suite.Actions)),
		cases:   make(map[string]*CaseResult, len(suite.Cases)),
	}

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

	_ = m.withStateMutationLocked(func() error {
		if m.activeID == "" {
			return nil
		}
		rt := m.sessions[m.activeID]
		if rt == nil {
			return nil
		}
		event.SessionID = rt.session.ID
		if event.ID == "" {
			event.ID = "evt-" + uuid.NewString()[:10]
		}
		rt.events = append(rt.events, event)
		m.rebuildSessionLocked(rt)
		return nil
	})
}

func (m *Manager) MarkActionStarted(sessionID, actionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.withStateMutationLocked(func() error {
		if m.activeID != "" && m.activeID != sessionID {
			return ErrActiveSessionExists
		}

		rt := m.sessions[sessionID]
		if rt == nil {
			return ErrSessionNotFound
		}
		action := rt.actions[actionID]
		if action == nil {
			return ErrUnknownAction
		}
		if action.Status == "running" {
			return fmt.Errorf("correctness action %s is already running", actionID)
		}
		if action.Status == "idle" && rt.session.CurrentStep.ActionID != actionID {
			current := rt.session.CurrentStep
			if current.ActionID == "" && current.Title != "" {
				return fmt.Errorf("finish the current step first: %s", current.Title)
			}
			if current.ActionID != "" {
				return fmt.Errorf("run %s first", current.Title)
			}
			return fmt.Errorf("action %s is not ready yet", actionID)
		}
		action.Status = "running"
		action.LastRunAt = nowUTC()
		action.LastError = ""
		action.EventAnchor = len(rt.events)
		m.resetCheckpointsForActionLocked(rt, actionID)
		m.activeID = sessionID
		m.rebuildSessionLocked(rt)
		return nil
	})
}

func (m *Manager) CompleteAction(sessionID, actionID string, output map[string]string) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var updated *TestSession
	err := m.withStateMutationLocked(func() error {
		rt := m.sessions[sessionID]
		if rt == nil {
			return ErrSessionNotFound
		}
		action := rt.actions[actionID]
		if action == nil {
			return ErrUnknownAction
		}
		action.Status = "completed"
		action.LastRunAt = nowUTC()
		action.LastError = ""
		action.Output = copyStringMap(output)
		m.rebuildSessionLocked(rt)
		copied := rt.session
		updated = &copied
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (m *Manager) FailAction(sessionID, actionID string, err error) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var updated *TestSession
	mutationErr := m.withStateMutationLocked(func() error {
		rt := m.sessions[sessionID]
		if rt == nil {
			return ErrSessionNotFound
		}
		action := rt.actions[actionID]
		if action == nil {
			return ErrUnknownAction
		}
		action.Status = "failed"
		action.LastRunAt = nowUTC()
		action.LastError = err.Error()
		m.rebuildSessionLocked(rt)
		copied := rt.session
		updated = &copied
		return nil
	})
	if mutationErr != nil {
		return nil, mutationErr
	}
	return updated, nil
}

func (m *Manager) SetPeerState(sessionID string, peer SessionPeerState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.withStateMutationLocked(func() error {
		rt := m.sessions[sessionID]
		if rt == nil {
			return ErrSessionNotFound
		}
		rt.session.Peer = peer
		m.rebuildSessionLocked(rt)
		return nil
	})
}

func (m *Manager) SubmitCheckpoint(sessionID, checkpointID, answer, notes string) (*TestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var updated *TestSession
	err := m.withStateMutationLocked(func() error {
		rt := m.sessions[sessionID]
		if rt == nil {
			return ErrSessionNotFound
		}

		changed := false
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
				changed = true
			}
		}
		if !changed {
			return ErrUnknownCheckpoint
		}

		m.rebuildSessionLocked(rt)
		copied := rt.session
		updated = &copied
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (m *Manager) ActionState(sessionID, actionID string) (*ActionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadStateLocked(); err != nil {
		return nil, err
	}

	rt := m.sessions[sessionID]
	if rt == nil {
		return nil, ErrSessionNotFound
	}
	action := rt.actions[actionID]
	if action == nil {
		return nil, ErrUnknownAction
	}
	copy := *action
	copy.Output = copyStringMap(action.Output)
	return &copy, nil
}

func (m *Manager) UpdateSandbox(sessionID string, fn func(*Sandbox) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.withStateMutationLocked(func() error {
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
	})
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
		if result == nil {
			return dep
		}
		if dependencyBlocksProgress(result.Status) {
			if result != nil && result.Title != "" {
				return result.Title
			}
			return dep
		}
	}
	return ""
}

func dependencyBlocksProgress(status string) bool {
	switch status {
	case "pending", "manual", "blocked":
		return true
	default:
		// Failed cases stay visible in the results summary, but they should not
		// freeze the rest of the suite when later checks can still run.
		return false
	}
}

func (m *Manager) resetCheckpointsForActionLocked(rt *sessionRuntime, actionID string) {
	for _, def := range rt.suite.Cases {
		if !slices.Contains(def.ActionIDs, actionID) {
			continue
		}
		result := rt.cases[def.ID]
		if result == nil {
			continue
		}
		for i := range result.Checkpoints {
			result.Checkpoints[i].Answer = ""
			result.Checkpoints[i].Notes = ""
			result.Checkpoints[i].Status = "pending"
			result.Checkpoints[i].UpdatedAt = ""
		}
	}
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

	var blockedStep *SessionStep
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
			continue
		case "blocked":
			if blockedStep == nil {
				step := SessionStep{
					Title:       result.Title,
					Description: firstMessage(result.Messages),
					CaseID:      result.ID,
				}
				blockedStep = &step
			}
			continue
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
	if blockedStep != nil {
		return *blockedStep
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

func tailEvents(events []TrafficEvent, limit int) []TrafficEvent {
	if len(events) == 0 || limit <= 0 {
		return nil
	}
	if len(events) <= limit {
		return append([]TrafficEvent(nil), events...)
	}
	return append([]TrafficEvent(nil), events[len(events)-limit:]...)
}
