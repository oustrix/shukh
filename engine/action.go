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

func (PlayCard) isAction()          {}
func (TakeBottomAndPass) isAction() {}
func (PodkladkaWest) isAction()     {}
