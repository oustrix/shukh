package engine

import "testing"

func TestStateCloneIsDeep(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 2, 1: 2, 2: 2})
	s.Adjudication = &Adjudication{Claimant: 0, Target: 1, Code: Sh6, Votes: map[SeatID]bool{0: true}}
	c := s.Clone()
	// Mutate the clone's maps and pointer target in place; the original must not move.
	c.Live[0] = false
	c.Hands[0] = nil
	c.Adjudication.Votes[1] = false
	c.Adjudication.Claimant = 9
	if s.Live[0] != true {
		t.Fatal("Clone aliased the Live map")
	}
	if s.Hands[0] == nil {
		t.Fatal("Clone aliased the Hands map")
	}
	if _, ok := s.Adjudication.Votes[1]; ok {
		t.Fatal("Clone aliased Adjudication.Votes")
	}
	if s.Adjudication.Claimant != 0 {
		t.Fatal("Clone aliased the Adjudication pointer")
	}
}
