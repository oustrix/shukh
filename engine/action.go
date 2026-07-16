package engine

// Action is a move a player can apply to the game via Apply. It is a sealed
// interface — only engine types implement it. Turn-actions (§5) apply as the
// player to move (State.Turn); they carry no seat.
type Action interface{ isAction() }

// PlayCard puts Card into the con: a заход onto an empty con (any card but Дама♥,
// R-5.2) or a бой of the top card by §3.
type PlayCard struct{ Card Card }

// TakeBottomAndPass takes the single bottom card of the con into hand and passes
// the turn (R-5.3b/R-5.8); forced for a handless-but-live player (R-5.9).
type TakeBottomAndPass struct{}

// PodkladkaWest tucks 6(2)♥ under a 7(3)♥ bottom; the whole con goes to the next
// player's hand and they open next (R-3.6.2/R-5.7.1).
type PodkladkaWest struct{}

// ClaimShukh catches an open Middle catch-window (§15.3): Target is the offender
// (State.Unsettled.Seat), Code the claimed ШУХ (must match the window). It
// reverses the offending action and assesses the ШУХ. Actor-agnostic (P-1): any
// live non-target seat may raise it; the outcome depends only on the window.
type ClaimShukh struct {
	Target SeatID
	Code   ShukhCode
}

// GiveShukhCard pays one card into the offender's Shukh zone during a §8 payment
// gate. It applies as the current payer, State.Pending.Owed[0] (P-1/P-3); Card
// must be one of that payer's non-last cards (R-8.1.1/I-2).
type GiveShukhCard struct{ Card Card }

// TakeShukhCards lifts Seat's set-aside Shukh pile into his hand (R-8.3), allowed
// only once the con it was laid in has ended (State.ShukhTakeable[Seat]). Taking
// it early is Ш-3. Carries the actor seat (P-1); a player takes only his own pile.
type TakeShukhCards struct{ Seat SeatID }

// DeclareOneCard announces «Одна карта!» for Seat, clearing its one-card
// obligation (R-6.1). Out of turn; carries the actor seat (P-1).
type DeclareOneCard struct{ Seat SeatID }

// AskCount asks Target «Сколько карт?». If Target owes an undeclared «одна карта»
// (R-6.2) it assesses Ш-11. Actor-agnostic (P-1) — validated by the target's
// obligation, not by who asks.
type AskCount struct{ Target SeatID }

// DiscardWest sends 6(2)♥ to the discard in the two-player endgame (R-9.3). A
// turn-action for the holder at заход time; it passes the turn (P-5).
type DiscardWest struct{}

// AskAboutWest asks whether Target holds 6(2)♥ in the endgame (R-9.4). It closes
// the безнаказанно window (Endgame.Asked) and, if Target still holds 6(2)♥,
// assesses Ш-12 (skip + discard obligation). Actor-agnostic (P-1).
type AskAboutWest struct{ Target SeatID }

// ClaimSubjective raises a subjective ШУХ (Ш-6 «завис» / Ш-9 «зря крикнул» / Ш-10
// «небрежность») against Target, opening an R-8.6 table vote. Claimant carries the
// raising seat (P-1). No penalty applies until the vote resolves (D-10: subjective
// ШУХи go to предъявление+голосование). Legal only with all gates closed.
type ClaimSubjective struct {
	Claimant SeatID
	Target   SeatID
	Code     ShukhCode
}

// Vote casts Voter's ballot in the open R-8.6 Adjudication. Support == true backs
// the challenge (the ШУХ is bogus → move it to the claimant as Ш-8). Any seat votes
// once (R-8.9, finished players included); on full turnout the vote auto-resolves.
type Vote struct {
	Voter   SeatID
	Support bool
}

// CloseVote force-resolves the open R-8.6 Adjudication NOW with whatever ballots
// have been cast (L2-1): a table majority backing the challenge (support*2 > n)
// moves the penalty onto the claimant as Ш-8, otherwise the ШУХ is confirmed on the
// target; a missing ballot is simply not counted as «против ШУХа». It is a system
// action — issued by the Layer-2 vote timer, never surfaced to a player — and a
// harmless no-op when no vote is open.
type CloseVote struct{}

func (PlayCard) isAction()          {}
func (DiscardWest) isAction()       {}
func (TakeBottomAndPass) isAction() {}
func (PodkladkaWest) isAction()     {}
func (ClaimShukh) isAction()        {}
func (GiveShukhCard) isAction()     {}
func (TakeShukhCards) isAction()    {}
func (DeclareOneCard) isAction()    {}
func (AskCount) isAction()          {}
func (AskAboutWest) isAction()      {}
func (ClaimSubjective) isAction()   {}
func (Vote) isAction()              {}
func (CloseVote) isAction()         {}
