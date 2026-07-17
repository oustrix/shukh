package engine

import "testing"

// playingState builds a minimal gates-closed 3-seat Playing state with the given
// hand sizes, for adjudication unit tests. Cards are dealt off a canonical deck
// (NewDeck) in seat order and the remainder goes to the discard, so I-1 holds for
// any sizes; the adjudication path never inspects card identity, only counts.
func playingState(t *testing.T, sizes map[SeatID]int) State {
	t.Helper()
	s := State{
		Rules:         RuleSet{DeckSize: Deck36},
		Mode:          Middle,
		Seats:         []SeatID{0, 1, 2},
		Phase:         Playing,
		Hands:         map[SeatID][]Card{},
		Shukh:         map[SeatID][]Card{},
		Live:          map[SeatID]bool{0: true, 1: true, 2: true},
		OwesOneCard:   map[SeatID]bool{},
		ShukhTakeable: map[SeatID]bool{},
	}
	deck := NewDeck(s.Rules)
	i := 0
	for _, seat := range s.Seats {
		for j := 0; j < sizes[seat]; j++ {
			s.Hands[seat] = append(s.Hands[seat], deck[i])
			i++
		}
	}
	s.Discard = append(s.Discard, deck[i:]...) // remainder keeps every card present once (I-1)
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

func TestGatesAreMutuallyExclusive(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	s.Adjudication = &Adjudication{Claimant: 1, Target: 0, Code: Sh6, Votes: map[SeatID]bool{}}
	s.Pending = &Payment{Offender: 0, Owed: []SeatID{1}}
	if err := CheckInvariants(s); err == nil {
		t.Fatal("CheckInvariants must reject two open gates at once (§15.8)")
	}
}

func TestCloseVotePartialTurnoutConfirmsTarget(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// one lone «против ШУХа» ballot — far short of a table majority.
	ns, _, err = Apply(ns, Vote{Voter: 2, Support: true})
	if err != nil {
		t.Fatalf("vote rejected: %v", err)
	}
	if ns.Adjudication == nil {
		t.Fatal("a single ballot must not auto-resolve a 3-seat vote")
	}
	ns, events, err := Apply(ns, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote rejected: %v", err)
	}
	if ns.Adjudication != nil {
		t.Fatal("CloseVote must clear the Adjudication")
	}
	assertResolved(t, events, false) // 1 of 3 support ⇒ not overturned
	if ns.Pending == nil || ns.Pending.Offender != 0 {
		t.Fatalf("expected §8 payment gate confirming the ШУХ on target 0, got %+v", ns.Pending)
	}
}

func TestCloseVoteEarlyMajorityFlipsToClaimant(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// two «против ШУХа» ballots — a table majority (2 of 3) reached before full turnout.
	ns, _, err = Apply(ns, Vote{Voter: 0, Support: true})
	if err != nil {
		t.Fatalf("vote 0 rejected: %v", err)
	}
	ns, _, err = Apply(ns, Vote{Voter: 2, Support: true})
	if err != nil {
		t.Fatalf("vote 2 rejected: %v", err)
	}
	if ns.Adjudication == nil {
		t.Fatal("with seat 1 not yet voted the 3-seat vote must still be open")
	}
	ns, events, err := Apply(ns, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote rejected: %v", err)
	}
	assertResolved(t, events, true) // 2 of 3 support ⇒ overturned
	if ns.Pending == nil || ns.Pending.Offender != 1 {
		t.Fatalf("expected Ш-8 payment gate on claimant 1, got %+v", ns.Pending)
	}
}

func TestCloseVoteZeroVotesConfirmsTarget(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	ns, events, err := Apply(ns, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote rejected: %v", err)
	}
	if ns.Adjudication != nil {
		t.Fatal("CloseVote must clear the Adjudication")
	}
	assertResolved(t, events, false) // no support at all ⇒ confirmed on target
	if ns.Pending == nil || ns.Pending.Offender != 0 {
		t.Fatalf("expected §8 payment gate on target 0, got %+v", ns.Pending)
	}
}

func TestCloseVoteNoAdjudicationIsNoop(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, events, err := Apply(s, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote with no open vote must not error, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("no-op CloseVote must emit no events, got %+v", events)
	}
	if ns.Adjudication != nil || ns.Pending != nil {
		t.Fatalf("no-op CloseVote must not open any gate, got adj=%+v pending=%+v", ns.Adjudication, ns.Pending)
	}
}

// TestCloseVoteNoAdjudicationDoesNotSettleUnsettled guards the no-op contract:
// CloseVote is a system action and must never settle an open Ш-2/Ш-12 Middle
// catch-window, even though Adjudication == nil. §15.8 guarantees Unsettled is
// nil while a vote is actually open, so this only matters for the no-op path.
func TestCloseVoteNoAdjudicationDoesNotSettleUnsettled(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	s.Unsettled = &Unsettled{Prev: s, Seat: 1, Code: Sh2}

	ns, events, err := Apply(s, CloseVote{})
	if err != nil {
		t.Fatalf("CloseVote with no open vote must not error, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("no-op CloseVote must emit no events, got %+v", events)
	}
	if ns.Unsettled == nil || ns.Unsettled.Seat != 1 || ns.Unsettled.Code != Sh2 {
		t.Fatalf("no-op CloseVote must not settle the open catch-window, got %+v", ns.Unsettled)
	}
}

func TestVoteViewPopulatedAndSorted(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// Cast two ballots out of ascending order (seat 2 then seat 0); a 3-seat vote
	// stays open at two ballots.
	ns, _, err = Apply(ns, Vote{Voter: 2, Support: false})
	if err != nil {
		t.Fatalf("vote 2 rejected: %v", err)
	}
	ns, _, err = Apply(ns, Vote{Voter: 0, Support: true})
	if err != nil {
		t.Fatalf("vote 0 rejected: %v", err)
	}
	v := View(ns, 1)
	if v.Vote == nil {
		t.Fatal("an open Adjudication must populate SeatView.Vote")
	}
	if v.Vote.Claimant != 1 || v.Vote.Target != 0 || v.Vote.Code != Sh6 {
		t.Fatalf("wrong VoteView dispute: %+v", v.Vote)
	}
	if len(v.Vote.Voted) != 2 || v.Vote.Voted[0] != 0 || v.Vote.Voted[1] != 2 {
		t.Fatalf("Voted must list who voted, ascending [0 2], got %v", v.Vote.Voted)
	}
}

func TestVoteViewNilWithoutAdjudication(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	if v := View(s, 0); v.Vote != nil {
		t.Fatalf("no Adjudication ⇒ SeatView.Vote must be nil, got %+v", v.Vote)
	}
}

func TestVoteViewVotedIsACopy(t *testing.T) {
	s := playingState(t, map[SeatID]int{0: 3, 1: 3, 2: 3})
	ns, _, err := Apply(s, ClaimSubjective{Claimant: 1, Target: 0, Code: Sh6})
	if err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	ns, _, err = Apply(ns, Vote{Voter: 0, Support: true})
	if err != nil {
		t.Fatalf("vote rejected: %v", err)
	}
	v := View(ns, 1)
	v.Vote.Voted[0] = 99 // mutate the returned slice
	if again := View(ns, 1); again.Vote.Voted[0] != 0 {
		t.Fatalf("View must return a fresh Voted slice; state leaked (%v)", again.Vote.Voted)
	}
}
