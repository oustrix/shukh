package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

// forward reports whether a is a game-advancing move (not a claim/vote/ask/social
// action). The integration driver only plays these, so it never opens a vote or
// dispute — the duel converges to Finished.
func forward(a engine.Action) bool {
	switch a.(type) {
	case engine.PlayCard, engine.TakeBottomAndPass, engine.PodkladkaWest,
		engine.DiscardWest, engine.GiveShukhCard:
		return true
	default:
		return false
	}
}

// nextForward scans every seat for the first available forward move, returning the
// player to submit it and the action. ok=false means no seat has a forward move.
func nextForward(s *Session) (PlayerID, engine.Action, bool) {
	for _, id := range []PlayerID{"h", "p2"} {
		up, err := s.Snapshot(id)
		if err != nil {
			continue
		}
		for _, a := range up.Legal {
			if forward(a) {
				return id, a, true
			}
		}
	}
	return "", nil, false
}

func TestIntegrationDuelPlaysToFinish(t *testing.T) {
	s := startedDuel(t)
	for step := 0; step < 400; step++ {
		if s.Stage() == Finished {
			break
		}
		mover, a, ok := nextForward(s)
		if !ok {
			t.Fatalf("step %d: no seat has a forward move; state stuck", step)
		}
		if _, err := s.Submit(mover, a); err != nil {
			t.Fatalf("step %d: submit %T by %s rejected: %v", step, a, mover, err)
		}
	}
	if s.Stage() != Finished {
		t.Fatal("game did not finish within the step budget")
	}
	// After finish, the engine state carries the full ranking (R-9.2/R-10.1).
	final, _ := s.Snapshot("h")
	if len(final.View.Finish) != 2 {
		t.Fatalf("finished game must rank both players, got %v", final.View.Finish)
	}
}

func TestIntegrationSubjectiveShukhVote(t *testing.T) {
	s := startedDuel(t)
	// Host claims Ш-6 against p2; both seats vote; vote resolves and clears.
	if _, err := s.Submit("h", engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6}); err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	up, _ := s.Snapshot("p2")
	if len(up.Legal) != 2 {
		t.Fatalf("during the vote p2 must see 2 Vote options, got %d", len(up.Legal))
	}
	if _, err := s.Submit("p2", engine.Vote{Voter: 1, Support: true}); err != nil {
		t.Fatalf("p2 vote rejected: %v", err)
	}
	evs, err := s.Submit("h", engine.Vote{Voter: 0, Support: false})
	if err != nil {
		t.Fatalf("host vote rejected: %v", err)
	}
	sawResolved := false
	for _, e := range evs {
		if _, ok := e.(engine.VoteResolved); ok {
			sawResolved = true
		}
	}
	if !sawResolved {
		t.Fatal("full turnout must emit VoteResolved")
	}
	after, _ := s.Snapshot("h")
	// vote cleared → normal play resumes (or a §8 payment gate is open, but the
	// Adjudication itself is gone): the mover has legal actions again.
	if len(after.Legal) == 0 && after.Stage == Playing {
		t.Fatal("after the vote the game must be playable again")
	}
}
