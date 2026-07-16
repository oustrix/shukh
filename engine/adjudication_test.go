package engine

import "testing"

func TestSubjectiveCodesClassified(t *testing.T) {
	for _, c := range []ShukhCode{Sh6, Sh9, Sh10} {
		if !c.isSubjective() {
			t.Errorf("Ш-%d must be subjective", c)
		}
	}
	for _, c := range []ShukhCode{Sh2, Sh3, Sh8, Sh11, Sh12} {
		if c.isSubjective() {
			t.Errorf("Ш-%d must not be subjective (Sh8 is an outcome, not a claim)", c)
		}
	}
}

func TestAdjudicationClosesGates(t *testing.T) {
	s := State{Seats: []SeatID{0, 1, 2}}
	if !s.gatesClosed() {
		t.Fatal("empty state must have gates closed")
	}
	s.Adjudication = &Adjudication{Claimant: 0, Target: 1, Code: Sh6, Votes: map[SeatID]bool{}}
	if s.gatesClosed() {
		t.Fatal("an open Adjudication must close the gates")
	}
}
