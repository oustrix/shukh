package server

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

// openVoteRoom returns a started 2-player room with an R-8.6 vote open (host claimed
// Ш-6 against seat 1), the timer armed via commit.
func openVoteRoom(t *testing.T) (*Room, *fakeClock) {
	t.Helper()
	r, _, clock := newTestRoom(t)
	if _, err := r.Join("Bob"); err != nil {
		t.Fatalf("Join: %v", err)
	}
	host := r.session.Snapshot().Host
	if err := r.session.Start(host, 42); err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.mu.Lock()
	evs, err := r.session.Submit(host, engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6})
	if err != nil {
		r.mu.Unlock()
		t.Fatalf("claim: %v", err)
	}
	r.commit(evs)
	r.mu.Unlock()
	return r, clock
}

func TestVoteTimerFiresAndResolves(t *testing.T) {
	r, clock := openVoteRoom(t)
	r.mu.Lock()
	armed := r.currentVoteDeadline() != nil
	r.mu.Unlock()
	if !armed {
		t.Fatal("VoteOpened must arm the vote deadline")
	}
	if r.session.Snapshot().Game.Adjudication == nil {
		t.Fatal("precondition: an Adjudication must be open")
	}
	clock.Advance(voteTTL) // fire → CloseVote → resolve
	if r.session.Snapshot().Game.Adjudication != nil {
		t.Fatal("timer expiry must resolve and clear the vote")
	}
	r.mu.Lock()
	stillArmed := r.currentVoteDeadline() != nil
	r.mu.Unlock()
	if stillArmed {
		t.Fatal("deadline must be cleared after resolution")
	}
}

func TestFullTurnoutStopsTimer(t *testing.T) {
	r, clock := openVoteRoom(t)
	snap := r.session.Snapshot()
	host := snap.Host
	p2 := snap.Order[1]
	// both seats vote → auto-resolve before the deadline.
	r.mu.Lock()
	e1, _ := r.session.Submit(host, engine.Vote{Voter: 0, Support: false})
	r.commit(e1)
	e2, _ := r.session.Submit(p2, engine.Vote{Voter: 1, Support: false})
	r.commit(e2)
	cleared := r.currentVoteDeadline() == nil
	r.mu.Unlock()
	if !cleared {
		t.Fatal("full turnout must disarm the vote timer")
	}
	if r.session.Snapshot().Game.Adjudication != nil {
		t.Fatal("full turnout must resolve the vote")
	}
	clock.Advance(voteTTL) // no-op: CloseVote on a nil Adjudication; must not panic
}
