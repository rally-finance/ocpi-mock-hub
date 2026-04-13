package handlers

import (
	"fmt"
	"testing"
)

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

func TestRequestLogKeepsRecentEntriesInRingOrder(t *testing.T) {
	store := newRequestLogMemoryStore()
	log := NewRequestLog(store)

	for i := 0; i < maxLogEntries+2; i++ {
		log.Add(RequestLogEntry{
			Timestamp:  "2026-01-01T00:00:00Z",
			Method:     "GET",
			Path:       fmt.Sprintf("/ocpi/%d", i),
			Status:     200,
			DurationMS: int64(i),
		})
	}

	entries := log.Entries()
	if len(entries) != maxLogEntries {
		t.Fatalf("expected %d retained entries, got %d", maxLogEntries, len(entries))
	}
	if entries[0].Path != "/ocpi/501" {
		t.Fatalf("expected newest retained entry to be /ocpi/501, got %q", entries[0].Path)
	}
	if entries[len(entries)-1].Path != "/ocpi/2" {
		t.Fatalf("expected oldest retained entry to be /ocpi/2, got %q", entries[len(entries)-1].Path)
	}
}
