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

func (GameStarted) isEvent()    {}
func (CardPlayed) isEvent()     {}
func (ConClosed) isEvent()      {}
func (ConSwept) isEvent()       {}
func (PlayerFinished) isEvent() {}
func (GameFinished) isEvent()   {}
