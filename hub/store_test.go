package hub

import (
	"encoding/json"
	"testing"
)

func TestStore_ReservationCRUD(t *testing.T) {
	s := NewMemoryStore()

	res := map[string]string{"id": "res-1", "status": "RESERVED", "location_id": "LOC1"}
	data, _ := json.Marshal(res)

	if err := s.PutReservation("res-1", data); err != nil {
		t.Fatalf("PutReservation: %v", err)
	}

	got, err := s.GetReservation("res-1")
	if err != nil {
		t.Fatalf("GetReservation: %v", err)
	}
	if got == nil {
		t.Fatal("GetReservation returned nil")
	}

	var decoded map[string]string
	json.Unmarshal(got, &decoded)
	if decoded["id"] != "res-1" {
		t.Errorf("expected id=res-1, got %s", decoded["id"])
	}

	list, err := s.ListReservations()
	if err != nil {
		t.Fatalf("ListReservations: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 reservation, got %d", len(list))
	}

	if err := s.DeleteReservation("res-1"); err != nil {
		t.Fatalf("DeleteReservation: %v", err)
	}

	got, _ = s.GetReservation("res-1")
	if got != nil {
		t.Error("expected nil after delete")
	}

	list, _ = s.ListReservations()
	if len(list) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(list))
	}
}

func TestStore_TokenCRUD(t *testing.T) {
	s := NewMemoryStore()

	tok := map[string]string{"uid": "TOKEN1", "country_code": "DE", "party_id": "AAA", "type": "RFID"}
	data, _ := json.Marshal(tok)

	if err := s.PutToken("DE", "AAA", "TOKEN1", data); err != nil {
		t.Fatalf("PutToken: %v", err)
	}

	got, err := s.GetToken("DE", "AAA", "TOKEN1")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got == nil {
		t.Fatal("GetToken returned nil")
	}

	list, _ := s.ListTokens()
	if len(list) != 1 {
		t.Errorf("expected 1 token, got %d", len(list))
	}

	missing, _ := s.GetToken("XX", "YY", "ZZ")
	if missing != nil {
		t.Error("expected nil for missing token")
	}
}

func TestStore_SessionCRUD(t *testing.T) {
	s := NewMemoryStore()

	sess := map[string]string{"id": "sess-1", "status": "PENDING"}
	data, _ := json.Marshal(sess)

	s.PutSession("sess-1", data)

	got, _ := s.GetSession("sess-1")
	if got == nil {
		t.Fatal("GetSession returned nil")
	}

	list, _ := s.ListSessions()
	if len(list) != 1 {
		t.Errorf("expected 1 session, got %d", len(list))
	}

	s.DeleteSession("sess-1")
	got, _ = s.GetSession("sess-1")
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestStore_CDRCRUD(t *testing.T) {
	s := NewMemoryStore()

	cdr := map[string]string{"id": "cdr-1", "country_code": "DE", "party_id": "AAA"}
	data, _ := json.Marshal(cdr)

	if err := s.PutCDR("cdr-1", data); err != nil {
		t.Fatalf("PutCDR: %v", err)
	}

	got, err := s.GetCDR("cdr-1")
	if err != nil {
		t.Fatalf("GetCDR: %v", err)
	}
	if got == nil {
		t.Fatal("GetCDR returned nil")
	}

	var decoded map[string]string
	json.Unmarshal(got, &decoded)
	if decoded["id"] != "cdr-1" {
		t.Errorf("expected id=cdr-1, got %s", decoded["id"])
	}

	list, err := s.ListCDRs()
	if err != nil {
		t.Fatalf("ListCDRs: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 CDR, got %d", len(list))
	}

	missing, _ := s.GetCDR("nonexistent")
	if missing != nil {
		t.Error("expected nil for missing CDR")
	}
}

func TestStore_ModePersistence(t *testing.T) {
	s := NewMemoryStore()

	mode, _ := s.GetMode()
	if mode != "happy" {
		t.Errorf("expected default mode 'happy', got %s", mode)
	}

	s.SetMode("reject")
	mode, _ = s.GetMode()
	if mode != "reject" {
		t.Errorf("expected mode 'reject', got %s", mode)
	}
}
