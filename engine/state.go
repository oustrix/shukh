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

// ShukhCode identifies a ШУХ trigger from the §7 catalogue (its numeric value is
// the Ш-number). Iteration 4 handles the detectable set {Sh2, Sh3, Sh11, Sh12}.
type ShukhCode int

const (
	Sh2  ShukhCode = 2  // зашёл с Дамы♥ (R-3.7.2) — extra effect: skip turn
	Sh3  ShukhCode = 3  // взял ШУХ раньше завершения кона (R-8.3)
	Sh11 ShukhCode = 11 // не объявил «Одна карта!» (R-6.2)
	Sh12 ShukhCode = 12 // эндшпиль: держит/использовал 6(2)♥ (R-9.4) — skip + discard
)

// Unsettled is an open Middle catch-window (§6.1/§15.3): a разрешённо-нелегальное
// action has been applied but not yet settled. Prev is the snapshot of State from
// BEFORE that action — reversing the ШУХ is exactly restoring it (no per-action
// undo). Seat is the offender; Code is what may be claimed (Sh2 | Sh3 | Sh12).
type Unsettled struct {
	Prev State
	Seat SeatID
	Code ShukhCode
}

// Payment is an active §8 payment gate: obligated players give the offender one
// card each into his Shukh zone. Owed is the FIFO queue of live seats ≠ offender
// with ≥2 cards that have not yet paid (R-8.1/R-8.1.1, P-3). Skip applies the
// offender's turn-skip once payment completes (Ш-2/Ш-12); ThenDiscardWest sets the
// Ш-12 6(2)♥-discard obligation (R-9.4.3).
type Payment struct {
	Offender        SeatID
	Owed            []SeatID
	Skip            bool
	ThenDiscardWest bool
}

// EndgameState tracks the §9.2 two-player endgame. Active is set when liveCount
// falls to 2 (R-9.3 kicks in). Asked records that the 6(2)♥ question was asked
// (ending the безнаказанно window, R-9.4.1). MustDiscard obligates the 6(2)♥
// holder to DiscardWest before any other move (post-Ш-12, R-9.4.3).
type EndgameState struct {
	Active      bool
	Asked       bool
	MustDiscard bool
}

// Player is a seat occupant. Name is carried for higher layers (display, events);
// engine logic uses only the number and order of players.
type Player struct {
	Name string
}

// TableCard is a card on the con together with the seat that played it. The
// owner is needed to decide exit (R-9.1/R-5.9) and to route "take the bottom"
// (R-5.8) — the con is physically a stack of specific players' cards.
type TableCard struct {
	Card Card
	By   SeatID
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
// is in exactly one zone at all times). Iteration 4 adds the ШУХ-penalty-core
// fields (Endgame, Pending, Unsettled, OwesOneCard, ShukhTakeable, §15.2); the
// behavior that populates and consumes them lands in later iteration-4 tasks.
type State struct {
	Rules RuleSet         // deck size + §12 variant flags
	Mode  EnforcementMode // Guard | Middle (Culture later)
	Seats []SeatID        // clockwise order (R-2.13), fixed for the game
	Phase Phase           // Playing | Finished

	Talon   []Card            // undealt deck; empty after NewGame (dealing done, R-4.10)
	Hands   map[SeatID][]Card // each player's hand (a set; order irrelevant to play)
	Table   []TableCard       // the con, bottom→top; empty at start of game
	Discard []Card            // closed discard pile (R-2.9)
	Shukh   map[SeatID][]Card // set-aside ШУХ cards, face down (R-2.10, I-3)

	Turn   SeatID          // whose turn it is
	Live   map[SeatID]bool // players still in the game (R-5.5.1, R-9.1)
	Finish []SeatID        // finishing order → places (R-9.2)

	Endgame       EndgameState    // §9.2 endgame flags
	Pending       *Payment        // active §8 payment gate; nil = none
	Unsettled     *Unsettled      // Middle catch-window; nil = stable
	OwesOneCard   map[SeatID]bool // §6: seat at 1 card, not yet declared/moved (R-6.1)
	ShukhTakeable map[SeatID]bool // §8: seat may lift its Shukh pile into hand (R-8.3)
}

// Event is a state-transition fact emitted for higher layers (animations, logs,
// spec §9). It is a sealed interface — only engine types implement it.
type Event interface{ isEvent() }
