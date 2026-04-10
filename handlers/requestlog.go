package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxLogEntries = 500

var ignoredPaths = map[string]bool{
	"/":            true,
	"/favicon.ico": true,
	"/robots.txt":  true,
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

type RequestLog struct {
	mu      sync.RWMutex
	entries []RequestLogEntry
	pos     int
	full    bool
}

func NewRequestLog() *RequestLog {
	return &RequestLog{
		entries: make([]RequestLogEntry, maxLogEntries),
	}
}

func (rl *RequestLog) Add(entry RequestLogEntry) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.entries[rl.pos] = entry
	rl.pos = (rl.pos + 1) % maxLogEntries
	if rl.pos == 0 {
		rl.full = true
	}
}

func (rl *RequestLog) Entries() []RequestLogEntry {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	var result []RequestLogEntry
	if rl.full {
		result = make([]RequestLogEntry, maxLogEntries)
		copy(result, rl.entries[rl.pos:])
		copy(result[maxLogEntries-rl.pos:], rl.entries[:rl.pos])
	} else {
		result = make([]RequestLogEntry, rl.pos)
		copy(result, rl.entries[:rl.pos])
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
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
