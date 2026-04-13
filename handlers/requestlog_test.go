package handlers

import (
	"errors"
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

func TestRequestLogFailedSlotWriteDoesNotAdvanceVisibleHistory(t *testing.T) {
	store := &failingRequestLogStore{base: newRequestLogMemoryStore()}
	log := NewRequestLog(store)

	log.Add(RequestLogEntry{Timestamp: "2026-01-01T00:00:00Z", Method: "GET", Path: "/ok-1", Status: 200})
	store.failEntryWrites = true
	log.Add(RequestLogEntry{Timestamp: "2026-01-01T00:00:01Z", Method: "GET", Path: "/should-fail", Status: 200})
	store.failEntryWrites = false

	entries := log.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected failed slot write not to inflate visible entry count, got %d", len(entries))
	}
	if entries[0].Path != "/ok-1" {
		t.Fatalf("expected only successful entry to remain visible, got %q", entries[0].Path)
	}

	log.Add(RequestLogEntry{Timestamp: "2026-01-01T00:00:02Z", Method: "GET", Path: "/ok-2", Status: 200})
	entries = log.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected next successful write to restore two visible entries, got %d", len(entries))
	}
	if entries[0].Path != "/ok-2" || entries[1].Path != "/ok-1" {
		t.Fatalf("unexpected entry order after retry: %#v", entries)
	}
}

type failingRequestLogStore struct {
	base            *requestLogMemoryStore
	failEntryWrites bool
}

func (s *failingRequestLogStore) GetBlob(key string) ([]byte, error) {
	return s.base.GetBlob(key)
}

func (s *failingRequestLogStore) UpdateBlob(key string, fn func([]byte) ([]byte, error)) error {
	if s.failEntryWrites && key != requestLogMetaBlobKey {
		return errors.New("simulated entry write failure")
	}
	return s.base.UpdateBlob(key, fn)
}
