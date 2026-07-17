package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

// startedDuel returns a 2-player Playing session (host=seat0, "p2"=seat1) on a
// fixed seed, plus the host's view to read legal actions from.
func startedDuel(t *testing.T) *Session {
	t.Helper()
	s := NewSession(cfg36(), "h", "Host")
	if err := s.Join("p2", "Bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.Start("h", 42); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSubmitRejectsOffTurnImpersonation(t *testing.T) {
	s := startedDuel(t)
	up, _ := s.SnapshotFor("h")
	// Find whichever player is NOT to move and have them try a turn-action.
	mover := up.View.Turn
	var idler PlayerID = "h"
	if mover == 0 {
		idler = "p2"
	}
	// idler tries to play the first card in *its own* hand out of turn.
	idlerUp, _ := s.SnapshotFor(idler)
	if len(idlerUp.View.Hand) == 0 {
		t.Skip("idler has no cards to attempt with")
	}
	_, err := s.Submit(idler, engine.PlayCard{Card: idlerUp.View.Hand[0]})
	if err != ErrNotYours {
		t.Fatalf("out-of-turn play: want ErrNotYours, got %v", err)
	}
}

func TestSubmitVoterMustBeSelf(t *testing.T) {
	s := startedDuel(t)
	// host raises a subjective ШУХ against p2 (seat 1).
	if _, err := s.Submit("h", engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6}); err != nil {
		t.Fatalf("claim rejected: %v", err)
	}
	// host tries to cast p2's ballot → impersonation.
	if _, err := s.Submit("h", engine.Vote{Voter: 1, Support: true}); err != ErrNotYours {
		t.Fatalf("voting as another seat: want ErrNotYours, got %v", err)
	}
	// host casts its own ballot → ok.
	if _, err := s.Submit("h", engine.Vote{Voter: 0, Support: false}); err != nil {
		t.Fatalf("own vote rejected: %v", err)
	}
}

func TestSubmitUnknownPlayer(t *testing.T) {
	s := startedDuel(t)
	if _, err := s.Submit("ghost", engine.TakeBottomAndPass{}); err != ErrUnknownPlayer {
		t.Fatalf("want ErrUnknownPlayer, got %v", err)
	}
}

func TestSubmitNotPlaying(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host") // still Lobby
	if _, err := s.Submit("h", engine.TakeBottomAndPass{}); err != ErrNotPlaying {
		t.Fatalf("want ErrNotPlaying, got %v", err)
	}
}
