package server

import (
	"encoding/json"
	"fmt"

	"github.com/oustrix/shukh/engine"
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
