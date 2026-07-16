package engine

import (
	"maps"
	"slices"
)

// SeatView is the per‑seat projection of game state (D‑9): exactly what one
// player may see. Opponents' hands are represented by counts only — there is no
// card field on OpponentView, so a hidden card is structurally unrepresentable.
type SeatView struct {
	Rules RuleSet
	Mode  EnforcementMode
	Phase Phase // Playing | Finished

	You  SeatID // whose view this is
	Turn SeatID // whose turn it is (public)

	Hand         []Card // own hand, in full (owner sees it)
	ShukhPending int    // own set‑aside ШУХ cards awaiting pickup (I‑3)

	// Opponents, clockwise starting at the seat after You (R‑2.13); self excluded.
	Opponents []OpponentView

	Table   []TableCard // the con, bottom→top (cards and order are public)
	Discard int         // size of the closed discard pile (contents hidden, R‑2.9)
	Talon   int         // undealt deck remaining (0 after dealing; field is general)

	Live   map[SeatID]bool // who is still in the game
	Finish []SeatID        // finishing order → places (R‑9.2)

	// Vote summarizes the open R-8.6 adjudication for a reconnecting/observing seat
	// (§8.3): who raised what against whom and which seats have already cast a ballot
	// (the fact only — never how). nil when no vote is open.
	Vote *VoteView
}

// OpponentView is the public projection of one other seat: counts only.
type OpponentView struct {
	Seat         SeatID
	HandCount    int  // number of cards in hand (public)
	ShukhPending int  // number of awaiting ШУХ cards (public, I‑3)
	Live         bool
}

// VoteView is the public summary of an open R-8.6 table vote (§8.3). It exposes the
// dispute (Claimant/Target/Code) and Voted — the seats that have cast a ballot, in
// ascending order — but never how anyone voted: the ballot stays secret until the
// vote resolves (§8.4).
type VoteView struct {
	Claimant SeatID
	Target   SeatID
	Code     ShukhCode
	Voted    []SeatID
}

// View builds the projection of s for seat (D‑9). It is pure and does not mutate
// s. Unlike LegalActions it is not turn‑gated: every valid seat can always see
// its own hand and the public state. State is taken by value, matching
// LegalActions. Precondition: seat is one of s.Seats (Layer 1 guarantees this);
// an unknown seat yields a well‑formed but meaningless SeatView (empty own hand).
func View(s State, seat SeatID) SeatView {
	opps := s.seatsFrom(seat) // clockwise from seat, inclusive
	v := SeatView{
		Rules:        s.Rules,
		Mode:         s.Mode,
		Phase:        s.Phase,
		You:          seat,
		Turn:         s.Turn,
		Hand:         slices.Clone(s.Hands[seat]),
		ShukhPending: len(s.Shukh[seat]),
		Table:        slices.Clone(s.Table),
		Discard:      len(s.Discard),
		Talon:        len(s.Talon),
		Live:         make(map[SeatID]bool, len(s.Live)),
		Finish:       slices.Clone(s.Finish),
		Opponents:    make([]OpponentView, 0, len(s.Seats)-1),
	}
	for _, k := range opps[1:] { // skip seat itself
		v.Opponents = append(v.Opponents, OpponentView{
			Seat:         k,
			HandCount:    len(s.Hands[k]),
			ShukhPending: len(s.Shukh[k]),
			Live:         s.Live[k],
		})
	}
	maps.Copy(v.Live, s.Live)
	if s.Adjudication != nil {
		voted := make([]SeatID, 0, len(s.Adjudication.Votes))
		for seat := range s.Adjudication.Votes {
			voted = append(voted, seat)
		}
		slices.Sort(voted) // ascending; expose only the fact of a ballot (§8.4)
		v.Vote = &VoteView{
			Claimant: s.Adjudication.Claimant,
			Target:   s.Adjudication.Target,
			Code:     s.Adjudication.Code,
			Voted:    voted,
		}
	}
	return v
}
