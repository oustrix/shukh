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
// it early is Ш-3 (Task 7). Carries the actor seat (P-1); a player takes only his
// own pile.
type TakeShukhCards struct{ Seat SeatID }

// DeclareOneCard announces «Одна карта!» for Seat, clearing its one-card
// obligation (R-6.1). Out of turn; carries the actor seat (P-1).
type DeclareOneCard struct{ Seat SeatID }

func (PlayCard) isAction()          {}
func (TakeBottomAndPass) isAction() {}
func (PodkladkaWest) isAction()     {}
func (ClaimShukh) isAction()        {}
func (GiveShukhCard) isAction()     {}
func (TakeShukhCards) isAction()    {}
func (DeclareOneCard) isAction()    {}
