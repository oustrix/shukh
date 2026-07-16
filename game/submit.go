package game

import "github.com/oustrix/shukh/engine"

// Submit applies a on behalf of id. It maps id to a seat, rejects impersonation
// (acting as another seat), then defers rule legality to engine.Apply. On success
// it advances authoritative state, updates the lifecycle, fans out to subscribers,
// and returns the events. On any rejection state is untouched and nothing is
// fanned out.
//
// L2-4 delivery contract: the returned events are an ACK echo for the caller only —
// the authoritative render path is the subscription (fanout has already delivered the
// same change to every seat, including this one). Layer 2 MUST render from the
// subscription and MUST NOT re-emit or re-render from this return value; treat it as
// «accepted», nothing more.
func (s *Session) Submit(id PlayerID, a engine.Action) ([]engine.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seat, ok := s.seatOf(id)
	if !ok {
		return nil, ErrUnknownPlayer
	}
	if s.stage != Playing {
		return nil, ErrNotPlaying
	}
	if err := s.authorize(seat, a); err != nil {
		return nil, err
	}
	ns, events, err := engine.Apply(s.state, a)
	if err != nil {
		return nil, err // engine.IllegalAction, state untouched
	}
	if inv := engine.CheckInvariants(ns); inv != nil {
		// A broken invariant after Apply is an engine bug — surface it, do not commit
		// the corrupt state.
		return nil, inv
	}
	s.state = ns
	if ns.Phase == engine.Finished {
		s.stage = Finished
	}
	s.fanout(events)
	return events, nil
}

// authorize rejects acting as a seat other than sub (anti-impersonation). Rule
// legality (whose turn, whether a gate is open) is left to engine.Apply; this only
// guards identity. Actor-agnostic social actions (AskCount/AskAboutWest/ClaimShukh,
// P-1) may be raised by any seated player.
func (s *Session) authorize(sub engine.SeatID, a engine.Action) error {
	switch act := a.(type) {
	case engine.ClaimSubjective:
		if act.Claimant != sub {
			return ErrNotYours
		}
	case engine.Vote:
		if act.Voter != sub {
			return ErrNotYours
		}
	case engine.DeclareOneCard:
		if act.Seat != sub {
			return ErrNotYours
		}
	case engine.TakeShukhCards:
		if act.Seat != sub {
			return ErrNotYours
		}
	case engine.GiveShukhCard:
		if s.state.Pending == nil || len(s.state.Pending.Owed) == 0 || s.state.Pending.Owed[0] != sub {
			return ErrNotYours
		}
	case engine.AskCount, engine.AskAboutWest, engine.ClaimShukh:
		// actor-agnostic (P-1): any seated player may raise; engine validates the rest.
	default:
		// turn-actions (PlayCard, TakeBottomAndPass, PodkladkaWest, DiscardWest): the
		// actor is the seat to move.
		if s.state.Turn != sub {
			return ErrNotYours
		}
	}
	return nil
}
