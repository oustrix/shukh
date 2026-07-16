package server

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// cardDTO is the wire form of a card: suit glyph + numeric rank, mirroring the TS
// Card in web/src/contract/types.ts (W-3).
type cardDTO struct {
	Suit string `json:"suit"`
	Rank int    `json:"rank"`
}

func encodeCard(c engine.Card) any {
	return cardDTO{Suit: suitGlyph(c.Suit), Rank: int(c.Rank)}
}

// suitGlyph / glyphToSuit are the explicit two-way map between engine suits and the
// TS glyphs, independent of any engine String() formatting.
func suitGlyph(s engine.Suit) string {
	switch s {
	case engine.Spades:
		return "♠"
	case engine.Hearts:
		return "♥"
	case engine.Diamonds:
		return "♦"
	case engine.Clubs:
		return "♣"
	}
	return "?"
}

func (d cardDTO) toCard() (engine.Card, error) {
	su, err := glyphToSuit(d.Suit)
	if err != nil {
		return engine.Card{}, err
	}
	return engine.Card{Suit: su, Rank: engine.Rank(d.Rank)}, nil
}

func glyphToSuit(g string) (engine.Suit, error) {
	switch g {
	case "♠":
		return engine.Spades, nil
	case "♥":
		return engine.Hearts, nil
	case "♦":
		return engine.Diamonds, nil
	case "♣":
		return engine.Clubs, nil
	}
	return 0, fmt.Errorf("server: unknown suit glyph %q", g)
}

// decodeAction parses the discriminated-union client action JSON into an
// engine.Action. Self-referential seat fields (vote.voter, claimSubjective.claimant)
// are left zero here — conn.go stamps them via withActor.
func decodeAction(raw json.RawMessage) (engine.Action, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	switch head.Type {
	case "playCard":
		var p struct {
			Card cardDTO `json:"card"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		c, err := p.Card.toCard()
		if err != nil {
			return nil, err
		}
		return engine.PlayCard{Card: c}, nil
	case "takeBottomAndPass":
		return engine.TakeBottomAndPass{}, nil
	case "podkladkaWest":
		return engine.PodkladkaWest{}, nil
	case "discardWest":
		return engine.DiscardWest{}, nil
	case "claimShukh":
		var p struct {
			Target engine.SeatID    `json:"target"`
			Code   engine.ShukhCode `json:"code"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.ClaimShukh{Target: p.Target, Code: p.Code}, nil
	case "giveShukhCard":
		var p struct {
			Card cardDTO `json:"card"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		c, err := p.Card.toCard()
		if err != nil {
			return nil, err
		}
		return engine.GiveShukhCard{Card: c}, nil
	case "takeShukhCards":
		var p struct {
			Seat engine.SeatID `json:"seat"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.TakeShukhCards{Seat: p.Seat}, nil
	case "declareOneCard":
		var p struct {
			Seat engine.SeatID `json:"seat"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.DeclareOneCard{Seat: p.Seat}, nil
	case "askCount":
		var p struct {
			Target engine.SeatID `json:"target"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.AskCount{Target: p.Target}, nil
	case "askAboutWest":
		var p struct {
			Target engine.SeatID `json:"target"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.AskAboutWest{Target: p.Target}, nil
	case "claimSubjective":
		var p struct {
			Claimant engine.SeatID    `json:"claimant"`
			Target   engine.SeatID    `json:"target"`
			Code     engine.ShukhCode `json:"code"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return engine.ClaimSubjective{Claimant: p.Claimant, Target: p.Target, Code: p.Code}, nil
	case "vote":
		var p struct {
			Vote string `json:"vote"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		switch p.Vote {
		case "forShukh": // «За ШУХ» → confirm on target
			return engine.Vote{Support: false}, nil
		case "againstShukh": // «Против ШУХа» → challenge, Support:true (§7.2)
			return engine.Vote{Support: true}, nil
		default:
			return nil, fmt.Errorf("server: unknown vote %q", p.Vote)
		}
	default:
		return nil, fmt.Errorf("server: unknown action type %q", head.Type)
	}
}

// encodeAction is the reverse of decodeAction: it serializes an engine.Action back
// to the tagged-union JSON (used for the `legal` list in updates).
func encodeAction(a engine.Action) any {
	switch act := a.(type) {
	case engine.PlayCard:
		return map[string]any{"type": "playCard", "card": encodeCard(act.Card)}
	case engine.TakeBottomAndPass:
		return map[string]any{"type": "takeBottomAndPass"}
	case engine.PodkladkaWest:
		return map[string]any{"type": "podkladkaWest"}
	case engine.DiscardWest:
		return map[string]any{"type": "discardWest"}
	case engine.ClaimShukh:
		return map[string]any{"type": "claimShukh", "target": int(act.Target), "code": int(act.Code)}
	case engine.GiveShukhCard:
		return map[string]any{"type": "giveShukhCard", "card": encodeCard(act.Card)}
	case engine.TakeShukhCards:
		return map[string]any{"type": "takeShukhCards", "seat": int(act.Seat)}
	case engine.DeclareOneCard:
		return map[string]any{"type": "declareOneCard", "seat": int(act.Seat)}
	case engine.AskCount:
		return map[string]any{"type": "askCount", "target": int(act.Target)}
	case engine.AskAboutWest:
		return map[string]any{"type": "askAboutWest", "target": int(act.Target)}
	case engine.ClaimSubjective:
		return map[string]any{"type": "claimSubjective", "claimant": int(act.Claimant), "target": int(act.Target), "code": int(act.Code)}
	case engine.Vote:
		v := "forShukh"
		if act.Support {
			v = "againstShukh"
		}
		return map[string]any{"type": "vote", "vote": v}
	default:
		return map[string]any{"type": "unknown"}
	}
}

// withActor stamps the authenticated seat onto the self-referential fields the
// client must not choose (§7.2). Actions without such a field pass through.
func withActor(a engine.Action, seat engine.SeatID) engine.Action {
	switch act := a.(type) {
	case engine.Vote:
		act.Voter = seat
		return act
	case engine.ClaimSubjective:
		act.Claimant = seat
		return act
	case engine.DeclareOneCard:
		act.Seat = seat
		return act
	case engine.TakeShukhCards:
		act.Seat = seat
		return act
	}
	return a
}

// --- envelopes (§7.1) ---

// ClientMsg is a decoded browser→server message. Action carries the raw union for
// decodeAction; Config is present only for setConfig.
type ClientMsg struct {
	Type   string          `json:"type"` // action | setConfig | start | leave
	Action json.RawMessage `json:"action,omitempty"`
	Config *ConfigDTO      `json:"config,omitempty"`
	ReqID  string          `json:"reqId,omitempty"`
}

// ServerMsg is a server→browser message. One struct covers update|ack|error; unset
// fields are omitted. `you` and `voteDeadline` are pointers so a zero value (seat 0)
// is still emitted.
type ServerMsg struct {
	Type string `json:"type"` // update | ack | error

	// update
	You          *int   `json:"you,omitempty"`
	RoomCode     string `json:"roomCode,omitempty"`
	Stage        string `json:"stage,omitempty"`
	Roster       []any  `json:"roster,omitempty"`
	View         any    `json:"view,omitempty"` // nil in the lobby → omitted (client treats absent as null)
	Legal        []any  `json:"legal,omitempty"`
	Events       []any  `json:"events,omitempty"`
	VoteDeadline *int64 `json:"voteDeadline,omitempty"`

	// ack / error
	ReqID   string `json:"reqId,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ConfigDTO is the wire form of game.Config for setConfig / create.
type ConfigDTO struct {
	DeckSize int    `json:"deckSize"`
	Mode     string `json:"mode"`
}

func (c ConfigDTO) toGame() (game.Config, error) {
	rules := engine.RuleSet{}
	switch c.DeckSize {
	case 36:
		rules.DeckSize = engine.Deck36
	case 52:
		rules.DeckSize = engine.Deck52
	default:
		return game.Config{}, fmt.Errorf("server: unsupported deck size %d", c.DeckSize)
	}
	var mode engine.EnforcementMode
	switch c.Mode {
	case "guard":
		mode = engine.Guard
	case "middle":
		mode = engine.Middle
	case "culture":
		mode = engine.Culture
	default:
		return game.Config{}, fmt.Errorf("server: unknown enforcement mode %q", c.Mode)
	}
	return game.Config{Rules: rules, Mode: mode}, nil
}

func ackMsg(reqID string) ServerMsg { return ServerMsg{Type: "ack", ReqID: reqID} }

func errorMsg(reqID, code, message string) ServerMsg {
	return ServerMsg{Type: "error", ReqID: reqID, Code: code, Message: message}
}

// encodeUpdate serializes a game.Update plus Layer-2 meta into an `update` message.
func encodeUpdate(you engine.SeatID, roomCode string, u game.Update, voteDeadline *int64) ServerMsg {
	roster := make([]any, len(u.Roster))
	for i, m := range u.Roster {
		roster[i] = map[string]any{"seat": int(m.Seat), "name": m.Name}
	}
	legal := make([]any, len(u.Legal))
	for i, a := range u.Legal {
		legal[i] = encodeAction(a)
	}
	events := make([]any, len(u.Events))
	for i, e := range u.Events {
		events[i] = encodeEvent(e)
	}
	yi := int(you)
	return ServerMsg{
		Type:         "update",
		You:          &yi,
		RoomCode:     roomCode,
		Stage:        encodeStage(u.Stage),
		Roster:       roster,
		View:         encodeView(u.View),
		Legal:        legal,
		Events:       events,
		VoteDeadline: voteDeadline,
	}
}

func encodeStage(s game.Lifecycle) string {
	switch s {
	case game.Lobby:
		return "lobby"
	case game.Playing:
		return "playing"
	case game.Finished:
		return "finished"
	}
	return "unknown"
}

func encodePhase(p engine.Phase) string {
	if p == engine.Finished {
		return "finished"
	}
	return "playing"
}

func encodeMode(m engine.EnforcementMode) string {
	switch m {
	case engine.Guard:
		return "guard"
	case engine.Middle:
		return "middle"
	case engine.Culture:
		return "culture"
	}
	return "unknown"
}

func encodeRules(r engine.RuleSet) any {
	return map[string]any{"deckSize": r.DeckSize, "podkladkaSnizu": r.PodkladkaSnizu, "jokers": r.Jokers}
}

func encodeCards(cards []engine.Card) []any {
	out := make([]any, len(cards))
	for i, c := range cards {
		out[i] = encodeCard(c)
	}
	return out
}

func encodeSeats(seats []engine.SeatID) []any {
	out := make([]any, len(seats))
	for i, s := range seats {
		out[i] = int(s)
	}
	return out
}

// encodeView serializes a per-seat projection (D-9), including the new optional
// VoteView (§8.3). Returns nil for a nil view (lobby).
func encodeView(v *engine.SeatView) any {
	if v == nil {
		return nil
	}
	opps := make([]any, len(v.Opponents))
	for i, o := range v.Opponents {
		opps[i] = map[string]any{
			"seat": int(o.Seat), "handCount": o.HandCount,
			"shukhPending": o.ShukhPending, "live": o.Live,
		}
	}
	table := make([]any, len(v.Table))
	for i, tc := range v.Table {
		table[i] = map[string]any{"card": encodeCard(tc.Card), "by": int(tc.By)}
	}
	live := make(map[string]bool, len(v.Live))
	for k, b := range v.Live {
		live[strconv.Itoa(int(k))] = b
	}
	m := map[string]any{
		"rules":        encodeRules(v.Rules),
		"mode":         encodeMode(v.Mode),
		"phase":        encodePhase(v.Phase),
		"you":          int(v.You),
		"turn":         int(v.Turn),
		"hand":         encodeCards(v.Hand),
		"shukhPending": v.ShukhPending,
		"opponents":    opps,
		"table":        table,
		"discard":      v.Discard,
		"talon":        v.Talon,
		"live":         live,
		"finish":       encodeSeats(v.Finish),
	}
	if v.Vote != nil {
		m["vote"] = map[string]any{
			"claimant": int(v.Vote.Claimant),
			"target":   int(v.Vote.Target),
			"code":     int(v.Vote.Code),
			"voted":    encodeSeats(v.Vote.Voted),
		}
	}
	return m
}

// encodeEvent serializes an engine.Event to the GameEvent union (mirrors types.ts).
func encodeEvent(e engine.Event) any {
	switch ev := e.(type) {
	case engine.GameStarted:
		return map[string]any{"type": "gameStarted", "turn": int(ev.Turn)}
	case engine.CardPlayed:
		return map[string]any{"type": "cardPlayed", "seat": int(ev.Seat), "card": encodeCard(ev.Card)}
	case engine.ConClosed:
		return map[string]any{"type": "conClosed", "by": int(ev.By)}
	case engine.ConSwept:
		return map[string]any{"type": "conSwept", "cards": encodeCards(ev.Cards)}
	case engine.PlayerFinished:
		return map[string]any{"type": "playerFinished", "seat": int(ev.Seat), "place": ev.Place}
	case engine.GameFinished:
		return map[string]any{"type": "gameFinished", "finish": encodeSeats(ev.Finish)}
	case engine.CardsTaken:
		return map[string]any{"type": "cardsTaken", "seat": int(ev.Seat), "cards": encodeCards(ev.Cards)}
	case engine.PodkladkaPlayed:
		return map[string]any{"type": "podkladkaPlayed", "seat": int(ev.Seat), "eater": int(ev.Eater)}
	case engine.TurnSkipped:
		return map[string]any{"type": "turnSkipped", "seat": int(ev.Seat)}
	case engine.ShukhAssessed:
		return map[string]any{"type": "shukhAssessed", "offender": int(ev.Offender), "code": int(ev.Code)}
	case engine.ActionReverted:
		return map[string]any{"type": "actionReverted", "seat": int(ev.Seat)}
	case engine.ShukhPaid:
		return map[string]any{"type": "shukhPaid", "offender": int(ev.Offender), "from": int(ev.From), "card": encodeCard(ev.Card)}
	case engine.ShukhCardsTaken:
		return map[string]any{"type": "shukhCardsTaken", "seat": int(ev.Seat), "cards": encodeCards(ev.Cards)}
	case engine.OneCardDeclared:
		return map[string]any{"type": "oneCardDeclared", "seat": int(ev.Seat)}
	case engine.WestDiscarded:
		return map[string]any{"type": "westDiscarded", "seat": int(ev.Seat)}
	case engine.VoteOpened:
		return map[string]any{"type": "voteOpened", "claimant": int(ev.Claimant), "target": int(ev.Target), "code": int(ev.Code)}
	case engine.VoteResolved:
		return map[string]any{"type": "voteResolved", "code": int(ev.Code), "overturned": ev.Overturned}
	default:
		return map[string]any{"type": "unknown"}
	}
}
