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

// voteOut casts all three seats' ballots and returns the resolved state + events.
func voteOut(t *testing.T, s State, ballots map[SeatID]bool) (State, []Event) {
	t.Helper()
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	var all []Event
	for _, seat := range []SeatID{0, 1, 2} {
		var evs []Event
		ns, evs, err = Apply(ns, Vote{Voter: seat, Support: ballots[seat]})
		if err != nil {
			t.Fatalf("vote by %d rejected: %v", seat, err)
		}
		all = append(all, evs...)
	}
	return ns, all
}

func TestVoteMajorityForChallengeFlipsToClaimant(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	// seats 0 and 2 support the challenge (2 of 3 > half) → Ш-8 onto claimant (seat 1).
	ns, events := voteOut(t, s, map[SeatID]bool{0: true, 1: false, 2: true})
	if ns.Adjudication != nil {
		t.Fatal("vote must clear the Adjudication")
	}
	assertResolved(t, events, true)
	// Ш-8 penalty on claimant (seat 1): others (0,2) with ≥2 cards owe → payment gate on seat 1.
	if ns.Pending == nil || ns.Pending.Offender != 1 {
		t.Fatalf("expected §8 payment gate for offender 1, got %+v", ns.Pending)
	}
}

func TestVoteNoMajorityConfirmsOnTarget(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	// only seat 2 supports the challenge (1 of 3, not a majority) → ШУХ confirmed on target (seat 0).
	ns, events := voteOut(t, s, map[SeatID]bool{0: false, 1: false, 2: true})
	assertResolved(t, events, false)
	if ns.Pending == nil || ns.Pending.Offender != 0 {
		t.Fatalf("expected §8 payment gate for offender 0, got %+v", ns.Pending)
	}
}

func assertResolved(t *testing.T, events []Event, wantOverturned bool) {
	t.Helper()
	for _, e := range events {
		if r, ok := e.(VoteResolved); ok {
			if r.Overturned != wantOverturned {
				t.Fatalf("VoteResolved.Overturned = %v, want %v", r.Overturned, wantOverturned)
			}
			return
		}
	}
	t.Fatal("no VoteResolved event emitted")
}

func TestVoteGatesNormalActions(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	s.Turn = 0
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// While the vote is open, the seat to move cannot play a card.
	if got := LegalActions(ns, 0); len(got) != 2 {
		t.Fatalf("during a vote seat 0 may only cast 2 Vote options, got %d: %+v", len(got), got)
	}
	if _, _, err := Apply(ns, PlayCard{Card: ns.Hands[0][0]}); err == nil {
		t.Fatal("a normal move during an open vote must be illegal")
	}
}
