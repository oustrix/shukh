package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

func TestSessionCloseVoteResolvesPartialTally(t *testing.T) {
	s := startedDuel(t) // host = seat 0, "p2" = seat 1 (submit_test.go)
	ch, cancel, err := s.Subscribe("p2")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer cancel()
	<-ch // drain the initial snapshot

	// Host raises a subjective ШУХ against p2 → the vote opens.
	if _, err := s.Submit("h", engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6}); err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	opened := <-ch // fanout of VoteOpened
	if opened.View == nil || opened.View.Vote == nil {
		t.Fatal("with the vote open the projection must carry a VoteView")
	}

	// A single (partial) ballot — a 2-seat duel needs both to auto-resolve.
	if _, err := s.Submit("h", engine.Vote{Voter: 0, Support: false}); err != nil {
		t.Fatalf("host ballot rejected: %v", err)
	}
	<-ch // fanout of the still-open vote

	// The system timer forces resolution with only that partial tally.
	evs, err := s.CloseVote()
	if err != nil {
		t.Fatalf("CloseVote: %v", err)
	}
	sawResolved := false
	for _, e := range evs {
		if _, ok := e.(engine.VoteResolved); ok {
			sawResolved = true
		}
	}
	if !sawResolved {
		t.Fatal("CloseVote must emit VoteResolved")
	}

	// The resolution reaches the subscriber via fanout, and the vote is gone.
	resolved := <-ch
	if resolved.View == nil || resolved.View.Vote != nil {
		t.Fatalf("after CloseVote the projection must show no open vote, got %+v", resolved.View)
	}
	gotResolved := false
	for _, e := range resolved.Events {
		if _, ok := e.(engine.VoteResolved); ok {
			gotResolved = true
		}
	}
	if !gotResolved {
		t.Fatal("subscriber must receive the VoteResolved fanout")
	}
}

func TestSessionCloseVoteNoopWhenClosed(t *testing.T) {
	s := startedDuel(t)
	evs, err := s.CloseVote()
	if err != nil {
		t.Fatalf("CloseVote with no open vote must not error, got %v", err)
	}
	if evs != nil {
		t.Fatalf("CloseVote with no open vote must return nil events, got %+v", evs)
	}
}
