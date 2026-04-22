package hub

import (
	"encoding/json"
	"testing"
)

func TestMultiParty_TwoEMSPs(t *testing.T) {
	s := NewMemoryStore()

	p1 := PartyState{
		Key: "NL/MSP1", CountryCode: "NL", PartyID: "MSP1",
		TokenB: "token-b-msp1", OwnToken: "own-1", Role: "EMSP",
	}
	p2 := PartyState{
		Key: "DE/MSP2", CountryCode: "DE", PartyID: "MSP2",
		TokenB: "token-b-msp2", OwnToken: "own-2", Role: "EMSP",
	}

	d1, _ := json.Marshal(p1)
	d2, _ := json.Marshal(p2)

	if err := s.PutParty("NL/MSP1", d1); err != nil {
		t.Fatalf("PutParty 1: %v", err)
	}
	if err := s.PutParty("DE/MSP2", d2); err != nil {
		t.Fatalf("PutParty 2: %v", err)
	}

	got1, _ := s.GetPartyByTokenB("token-b-msp1")
	if got1 == nil {
		t.Fatal("GetPartyByTokenB(msp1) returned nil")
	}
	var ps1 PartyState
	json.Unmarshal(got1, &ps1)
	if ps1.Key != "NL/MSP1" {
		t.Errorf("expected NL/MSP1, got %s", ps1.Key)
	}

	got2, _ := s.GetPartyByTokenB("token-b-msp2")
	if got2 == nil {
		t.Fatal("GetPartyByTokenB(msp2) returned nil")
	}
	var ps2 PartyState
	json.Unmarshal(got2, &ps2)
	if ps2.Key != "DE/MSP2" {
		t.Errorf("expected DE/MSP2, got %s", ps2.Key)
	}

	none, _ := s.GetPartyByTokenB("nonexistent")
	if none != nil {
		t.Error("expected nil for nonexistent token")
	}
}

func TestMultiParty_DeleteParty(t *testing.T) {
	s := NewMemoryStore()

	p1 := PartyState{Key: "NL/MSP1", TokenB: "token-b-1"}
	p2 := PartyState{Key: "DE/MSP2", TokenB: "token-b-2"}
	d1, _ := json.Marshal(p1)
	d2, _ := json.Marshal(p2)
	s.PutParty("NL/MSP1", d1)
	s.PutParty("DE/MSP2", d2)

	s.DeleteParty("NL/MSP1")

	got1, _ := s.GetParty("NL/MSP1")
	if got1 != nil {
		t.Error("expected nil after delete")
	}

	byToken, _ := s.GetPartyByTokenB("token-b-1")
	if byToken != nil {
		t.Error("expected nil token lookup after delete")
	}

	got2, _ := s.GetParty("DE/MSP2")
	if got2 == nil {
		t.Error("other party should still exist")
	}
}

func TestMultiParty_SharedTokenB_TwoRoles(t *testing.T) {
	// OCPI 2.2.1 §8.4.3: a single credentials exchange can advertise
	// multiple roles that share one TokenB. Both roles must resolve by
	// TokenB, and deleting one must leave the other still routable.
	s := NewMemoryStore()

	sharedToken := "shared-token-b"
	p1 := PartyState{Key: "NL/MSP", CountryCode: "NL", PartyID: "MSP", TokenB: sharedToken, Role: "EMSP"}
	p2 := PartyState{Key: "DE/MSP", CountryCode: "DE", PartyID: "MSP", TokenB: sharedToken, Role: "EMSP"}
	d1, _ := json.Marshal(p1)
	d2, _ := json.Marshal(p2)
	s.PutParty("NL/MSP", d1)
	s.PutParty("DE/MSP", d2)

	got, _ := s.GetPartyByTokenB(sharedToken)
	if got == nil {
		t.Fatal("expected any party to resolve for shared token")
	}

	// Deleting NL must not invalidate DE.
	s.DeleteParty("NL/MSP")
	got, _ = s.GetPartyByTokenB(sharedToken)
	if got == nil {
		t.Fatal("expected DE to still resolve after NL deleted")
	}
	var ps PartyState
	json.Unmarshal(got, &ps)
	if ps.Key != "DE/MSP" {
		t.Errorf("expected DE/MSP after NL deletion, got %s", ps.Key)
	}

	// Deleting the last one drops the token binding.
	s.DeleteParty("DE/MSP")
	got, _ = s.GetPartyByTokenB(sharedToken)
	if got != nil {
		t.Error("expected token binding to be cleared once all parties gone")
	}
}

func TestMultiParty_ListParties(t *testing.T) {
	s := NewMemoryStore()

	p1 := PartyState{Key: "NL/MSP1", TokenB: "tb1"}
	p2 := PartyState{Key: "DE/MSP2", TokenB: "tb2"}
	p3 := PartyState{Key: "FR/MSP3", TokenB: "tb3"}
	d1, _ := json.Marshal(p1)
	d2, _ := json.Marshal(p2)
	d3, _ := json.Marshal(p3)
	s.PutParty("NL/MSP1", d1)
	s.PutParty("DE/MSP2", d2)
	s.PutParty("FR/MSP3", d3)

	list, err := s.ListParties()
	if err != nil {
		t.Fatalf("ListParties: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 parties, got %d", len(list))
	}
}
