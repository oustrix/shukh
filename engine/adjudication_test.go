package engine

import "testing"

// playingState builds a minimal gates-closed 3-seat Playing state with the given
// hand sizes, for adjudication unit tests. Cards are arbitrary distinct spades —
// the adjudication path never inspects card identity, only counts.
func playingState(t *testing.T, sizes map[SeatID]int) State {
	t.Helper()
	s := State{
		Rules: RuleSet{DeckSize: Deck36},
		Mode:  Middle,
		Seats: []SeatID{0, 1, 2},
		Phase: Playing,
		Hands: map[SeatID][]Card{},
		Shukh: map[SeatID][]Card{},
		Live:  map[SeatID]bool{0: true, 1: true, 2: true},
		OwesOneCard:   map[SeatID]bool{},
		ShukhTakeable: map[SeatID]bool{},
	}
	rank := Rank(7)
	for seat, n := range sizes {
		for i := 0; i < n; i++ {
			s.Hands[seat] = append(s.Hands[seat], Card{Suit: Spades, Rank: rank})
			rank++
		}
	}
	return s
}

func TestClaimSubjectiveOpensVote(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, events, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	if ns.Adjudication == nil || ns.Adjudication.Target != 0 || ns.Adjudication.Code != Sh6 {
		t.Fatalf("expected open Adjudication over seat 0/Ш-6, got %+v", ns.Adjudication)
	}
	if len(ns.Adjudication.Votes) != 0 {
		t.Fatalf("fresh vote must have no ballots, got %v", ns.Adjudication.Votes)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d: %+v", len(events), events)
	}
	if _, ok := events[0].(VoteOpened); !ok {
		t.Fatalf("want VoteOpened, got %T", events[0])
	}
}

func TestClaimSubjectiveRejectsSelfAndNonSubjective(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	if _, _, err := Apply(s, ClaimSubjective{Claimant: 0, Target: 0, Code: Sh6}); err == nil {
		t.Error("claiming against self must be illegal")
	}
	if _, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh2}); err == nil {
		t.Error("a non-subjective code must be illegal for ClaimSubjective")
	}
}

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

func TestVoteEventsAreEvents(t *testing.T) {
	var _ Event = VoteOpened{Claimant: 0, Target: 1, Code: Sh6}
	var _ Event = VoteResolved{Code: Sh6, Overturned: true}
}
