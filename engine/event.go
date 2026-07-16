package engine

// GameStarted is emitted by NewGame once dealing is complete; Turn is the seat
// that opens the first con (holder of 9♦, R-5.1).
type GameStarted struct {
	Turn SeatID
}

// CardPlayed is emitted when a player puts a card into the con — a заход or a бой
// (R-5.2/§3).
type CardPlayed struct {
	Seat SeatID
	Card Card
}

// ConClosed is emitted when the con reaches a closing condition (count R-5.5 or
// Дама♥ R-3.7.1); By is the closer.
type ConClosed struct {
	By SeatID
}

// ConSwept is emitted when the closed con's cards move to the discard (R-5.6),
// bottom→top.
type ConSwept struct {
	Cards []Card
}

// PlayerFinished is emitted when a player exits the game (R-9.1); Place is the
// 1-based finishing place.
type PlayerFinished struct {
	Seat  SeatID
	Place int
}

// GameFinished is emitted once, when the game ends (R-10.1/R-10.1.1); Finish is
// the complete ranking, winner first, loser last.
type GameFinished struct {
	Finish []SeatID
}

// CardsTaken is emitted when a player takes cards into hand: one bottom card
// (R-5.8) or a whole eaten con after западло (R-3.6.2).
type CardsTaken struct {
	Seat  SeatID
	Cards []Card
}

// PodkladkaPlayed is emitted for the западло move (R-3.6.2): Seat tucked 6(2)♥
// under the con and Eater received the whole con.
type PodkladkaPlayed struct {
	Seat  SeatID
	Eater SeatID
}

// TurnSkipped is emitted when a Guard turn is skipped because the seat's only
// possible move is the forbidden lone-Дама♥ заход (§14.4). Guard-only: Middle and
// Culture instead allow it and catch it as Ш-2.
type TurnSkipped struct {
	Seat SeatID
}

// ShukhAssessed is emitted when a ШУХ is confirmed against Offender (§8); Code is
// the §7 trigger.
type ShukhAssessed struct {
	Offender SeatID
	Code     ShukhCode
}

// ActionReverted is emitted when a claimed ШУХ reverses the offender's last
// action by restoring the pre-action snapshot (§15.3).
type ActionReverted struct {
	Seat SeatID
}

// ShukhPaid is emitted when a payer gives one card into the offender's Shukh zone
// (R-8.1/R-8.2); From is the giver.
type ShukhPaid struct {
	Offender SeatID
	From     SeatID
	Card     Card
}

// ShukhCardsTaken is emitted when a player lifts his Shukh pile into hand (R-8.3).
type ShukhCardsTaken struct {
	Seat  SeatID
	Cards []Card
}

// OneCardDeclared is emitted when a player announces «Одна карта!» (R-6.1).
type OneCardDeclared struct {
	Seat SeatID
}

// WestDiscarded is emitted when 6(2)♥ is discarded in the endgame (R-9.3).
type WestDiscarded struct {
	Seat SeatID
}

func (ShukhAssessed) isEvent()   {}
func (ActionReverted) isEvent()  {}
func (ShukhPaid) isEvent()       {}
func (ShukhCardsTaken) isEvent() {}
func (OneCardDeclared) isEvent() {}
func (WestDiscarded) isEvent()   {}

func (GameStarted) isEvent()     {}
func (CardPlayed) isEvent()      {}
func (ConClosed) isEvent()       {}
func (ConSwept) isEvent()        {}
func (PlayerFinished) isEvent()  {}
func (GameFinished) isEvent()    {}
func (CardsTaken) isEvent()      {}
func (PodkladkaPlayed) isEvent() {}
func (TurnSkipped) isEvent()     {}
