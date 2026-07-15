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

func (GameStarted) isEvent() {}
func (CardPlayed) isEvent()  {}
