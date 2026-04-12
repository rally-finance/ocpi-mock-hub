package handlers

import "testing"

func TestRequestLogPersistsAcrossInstances(t *testing.T) {
	store := newRequestLogMemoryStore()
	first := NewRequestLog(store)
	second := NewRequestLog(store)

	first.Add(RequestLogEntry{
		Timestamp:  "2026-01-01T00:00:00Z",
		Method:     "GET",
		Path:       "/ocpi/versions",
		Status:     200,
		DurationMS: 12,
	})

	entries := second.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected second request log instance to see 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/ocpi/versions" {
		t.Fatalf("expected shared request log path %q, got %q", "/ocpi/versions", entries[0].Path)
	}
}

