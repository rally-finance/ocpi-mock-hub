package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxLogEntries         = 500
	requestLogMetaBlobKey = "request-log:meta"
	requestLogEntryPrefix = "request-log:entry:"
)

var ignoredPaths = map[string]bool{
	"/":            true,
	"/favicon.ico": true,
	"/robots.txt":  true,
}

type RequestLogStateStore interface {
	GetBlob(key string) ([]byte, error)
	UpdateBlob(key string, fn func([]byte) ([]byte, error)) error
}

type requestLogMemoryStore struct {
	mu    sync.Mutex
	blobs map[string][]byte
}

func newRequestLogMemoryStore() *requestLogMemoryStore {
	return &requestLogMemoryStore{
		blobs: make(map[string][]byte),
	}
}

func (m *requestLogMemoryStore) GetBlob(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val := m.blobs[key]
	if val == nil {
		return nil, nil
	}
	return append([]byte(nil), val...), nil
}

func (m *requestLogMemoryStore) UpdateBlob(key string, fn func([]byte) ([]byte, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current := append([]byte(nil), m.blobs[key]...)
	next, err := fn(current)
	if err != nil {
		return err
	}
	if next == nil {
		delete(m.blobs, key)
		return nil
	}
	m.blobs[key] = append([]byte(nil), next...)
	return nil
}

type RequestLogEntry struct {
	Timestamp  string `json:"timestamp"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	OCPIFrom   string `json:"ocpi_from,omitempty"`
	OCPITo     string `json:"ocpi_to,omitempty"`
}

type requestLogState struct {
	LastSequence int64 `json:"last_sequence"`
	NextIndex    int   `json:"next_index"`
	Count        int   `json:"count"`
}

type requestLogSlot struct {
	Sequence int64           `json:"sequence"`
	Entry    RequestLogEntry `json:"entry"`
}

type RequestLog struct {
	stateStore RequestLogStateStore
}

func NewRequestLog(stores ...RequestLogStateStore) *RequestLog {
	var stateStore RequestLogStateStore
	if len(stores) > 0 && stores[0] != nil {
		stateStore = stores[0]
	} else {
		stateStore = newRequestLogMemoryStore()
	}
	return &RequestLog{stateStore: stateStore}
}

func decodeRequestLogState(raw []byte) (requestLogState, error) {
	if len(raw) == 0 {
		return requestLogState{}, nil
	}
	var state requestLogState
	if err := json.Unmarshal(raw, &state); err != nil {
		return requestLogState{}, err
	}
	return state, nil
}

func decodeRequestLogSlot(raw []byte) (requestLogSlot, error) {
	if len(raw) == 0 {
		return requestLogSlot{}, nil
	}
	var slot requestLogSlot
	if err := json.Unmarshal(raw, &slot); err != nil {
		return requestLogSlot{}, err
	}
	return slot, nil
}

func requestLogEntryBlobKey(index int) string {
	return requestLogEntryPrefix + strconv.Itoa(index)
}

func (rl *RequestLog) Add(entry RequestLogEntry) {
	if rl == nil || rl.stateStore == nil {
		return
	}

	var writeIndex int
	var sequence int64
	var previousCount int
	if err := rl.stateStore.UpdateBlob(requestLogMetaBlobKey, func(raw []byte) ([]byte, error) {
		state, err := decodeRequestLogState(raw)
		if err != nil {
			return nil, err
		}

		writeIndex = state.NextIndex
		previousCount = state.Count
		state.LastSequence++
		sequence = state.LastSequence
		state.NextIndex = (state.NextIndex + 1) % maxLogEntries
		if state.Count < maxLogEntries {
			state.Count++
		}
		return json.Marshal(state)
	}); err != nil {
		log.Printf("[requestlog] failed to reserve log slot: %v", err)
		return
	}

	payload, err := json.Marshal(requestLogSlot{
		Sequence: sequence,
		Entry:    entry,
	})
	if err != nil {
		return
	}

	if err := rl.stateStore.UpdateBlob(requestLogEntryBlobKey(writeIndex), func([]byte) ([]byte, error) {
		return payload, nil
	}); err != nil {
		log.Printf("[requestlog] failed to persist log slot %d sequence %d: %v", writeIndex, sequence, err)
		rl.rollbackReservedEntry(writeIndex, sequence, previousCount)
	}
}

func (rl *RequestLog) rollbackReservedEntry(writeIndex int, sequence int64, previousCount int) {
	if rl == nil || rl.stateStore == nil {
		return
	}
	expectedNextIndex := (writeIndex + 1) % maxLogEntries
	if err := rl.stateStore.UpdateBlob(requestLogMetaBlobKey, func(raw []byte) ([]byte, error) {
		state, err := decodeRequestLogState(raw)
		if err != nil {
			return nil, err
		}
		if state.LastSequence != sequence || state.NextIndex != expectedNextIndex {
			return json.Marshal(state)
		}

		state.LastSequence--
		state.NextIndex = writeIndex
		state.Count = previousCount
		return json.Marshal(state)
	}); err != nil {
		log.Printf("[requestlog] failed to roll back log reservation for slot %d sequence %d: %v", writeIndex, sequence, err)
	}
}

func (rl *RequestLog) Entries() []RequestLogEntry {
	if rl == nil || rl.stateStore == nil {
		return nil
	}
	raw, err := rl.stateStore.GetBlob(requestLogMetaBlobKey)
	if err != nil {
		return nil
	}
	state, err := decodeRequestLogState(raw)
	if err != nil {
		return nil
	}

	if state.Count <= 0 {
		return nil
	}

	entries := make([]RequestLogEntry, 0, state.Count)
	for i := 0; i < state.Count; i++ {
		index := (state.NextIndex - 1 - i + maxLogEntries) % maxLogEntries
		expectedSequence := state.LastSequence - int64(i)
		slotRaw, err := rl.stateStore.GetBlob(requestLogEntryBlobKey(index))
		if err != nil {
			continue
		}
		slot, err := decodeRequestLogSlot(slotRaw)
		if err != nil || slot.Sequence != expectedSequence {
			continue
		}
		entries = append(entries, slot.Entry)
	}

	return entries
}

type statusCapture struct {
	http.ResponseWriter
	status int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.status = code
	sc.ResponseWriter.WriteHeader(code)
}

func RequestLogMiddleware(rl *RequestLog) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/admin") || ignoredPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sc := &statusCapture{ResponseWriter: w, status: 200}

			next.ServeHTTP(sc, r)

			path := r.URL.Path
			if r.URL.RawQuery != "" {
				path += "?" + r.URL.RawQuery
			}

			rl.Add(RequestLogEntry{
				Timestamp:  start.UTC().Format(time.RFC3339),
				Method:     r.Method,
				Path:       path,
				Status:     sc.status,
				DurationMS: time.Since(start).Milliseconds(),
				OCPIFrom:   r.Header.Get("OCPI-from-country-code") + "*" + r.Header.Get("OCPI-from-party-id"),
				OCPITo:     r.Header.Get("OCPI-to-country-code") + "*" + r.Header.Get("OCPI-to-party-id"),
			})
		})
	}
}

func (h *Handler) GetRequestLog(w http.ResponseWriter, r *http.Request) {
	if h.ReqLog == nil {
		writeJSON(w, http.StatusOK, []RequestLogEntry{})
		return
	}
	writeJSON(w, http.StatusOK, h.ReqLog.Entries())
}
