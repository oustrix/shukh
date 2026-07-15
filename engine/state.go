package engine

import "fmt"

// SeatID identifies a seat by its position in clockwise order (R-2.13). Seats are
// numbered 0..n-1 and fixed for the whole game.
type SeatID int

// Phase is the coarse lifecycle stage of a game.
type Phase int

const (
	// Playing means the game is in progress (dealing is atomic inside NewGame).
	Playing Phase = iota
	// Finished means every player has left (R-9.2); set by later iterations.
	Finished
)

// EnforcementMode selects how strictly the engine blocks vs. lets-and-catches
// rule violations (decision D-10). Culture is enum-present but not implemented in
// the MVP (it needs position rollback) — Config.Validate rejects it.
type EnforcementMode int

const (
	// Guard blocks everything detectable.
	Guard EnforcementMode = iota
	// Middle (the table default) blocks only integrity-breaking moves.
	Middle
	// Culture blocks only the physically impossible (later iteration).
	Culture
)

// Player is a seat occupant. Name is carried for higher layers (display, events);
// engine logic uses only the number and order of players.
type Player struct {
	Name string
}

// Config is the immutable setup of a game: rules, enforcement mode, and the
// players in clockwise order (spec §4).
type Config struct {
	Rules   RuleSet
	Mode    EnforcementMode
	Players []Player
}

// Validate reports whether the Config can start a game: a supported RuleSet, an
// implemented enforcement mode (Guard | Middle, D-10), and 2..8 players (D-3).
func (c Config) Validate() error {
	if err := c.Rules.Validate(); err != nil {
		return err
	}
	switch c.Mode {
	case Guard, Middle:
		// implemented
	case Culture:
		return fmt.Errorf("engine: Culture enforcement mode is not implemented in the MVP (D-10)")
	default:
		return fmt.Errorf("engine: unknown enforcement mode %d", c.Mode)
	}
	if n := len(c.Players); n < 2 || n > 8 {
		return fmt.Errorf("engine: player count %d out of range 2..8 (D-3)", n)
	}
	return nil
}

// State is the full authoritative game state (spec §2, invariant I-1: every card
// is in exactly one zone at all times). Fields for later iterations (Endgame,
// Pending adjudication, Unsettled catch-window) are added when those mechanics
// land; this iteration populates only the dealing result and turn control.
type State struct {
	Rules RuleSet         // deck size + §12 variant flags
	Mode  EnforcementMode // Guard | Middle (Culture later)
	Seats []SeatID        // clockwise order (R-2.13), fixed for the game
	Phase Phase           // Playing | Finished

	Talon   []Card            // undealt deck; empty after NewGame (dealing done, R-4.10)
	Hands   map[SeatID][]Card // each player's hand (a set; order irrelevant to play)
	Table   []Card            // the con, bottom→top; empty at start of game
	Discard []Card            // closed discard pile (R-2.9)
	Shukh   map[SeatID][]Card // set-aside ШУХ cards, face down (R-2.10, I-3)

	Turn   SeatID          // whose turn it is
	Live   map[SeatID]bool // players still in the game (R-5.5.1, R-9.1)
	Finish []SeatID        // finishing order → places (R-9.2)
}

// Event is a state-transition fact emitted for higher layers (animations, logs,
// spec §9). It is a sealed interface — only engine types implement it.
type Event interface{ isEvent() }

// GameStarted is emitted by NewGame once dealing is complete; Turn is the seat
// that opens the first con (holder of 9♦, R-5.1).
type GameStarted struct {
	Turn SeatID
}

func (GameStarted) isEvent() {}
