package server

import (
	"encoding/json"
	"testing"

	"github.com/oustrix/shukh/engine"
)

func decodeActionJSON(t *testing.T, s string) engine.Action {
	t.Helper()
	a, err := decodeAction(json.RawMessage(s))
	if err != nil {
		t.Fatalf("decodeAction(%s): %v", s, err)
	}
	return a
}

func TestCardRoundTrip(t *testing.T) {
	c := engine.Card{Suit: engine.Hearts, Rank: engine.Queen}
	data, err := json.Marshal(encodeCard(c))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `{"suit":"♥","rank":12}` {
		t.Fatalf("card JSON = %s", data)
	}
	back, err := decodeAction(json.RawMessage(`{"type":"playCard","card":` + string(data) + `}`))
	if err != nil {
		t.Fatalf("decode playCard: %v", err)
	}
	if pc, ok := back.(engine.PlayCard); !ok || pc.Card != c {
		t.Fatalf("card did not round-trip: %+v", back)
	}
}

func TestDecodeEachActionVariant(t *testing.T) {
	cases := map[string]engine.Action{
		`{"type":"takeBottomAndPass"}`:                                engine.TakeBottomAndPass{},
		`{"type":"podkladkaWest"}`:                                    engine.PodkladkaWest{},
		`{"type":"discardWest"}`:                                      engine.DiscardWest{},
		`{"type":"claimShukh","target":2,"code":2}`:                   engine.ClaimShukh{Target: 2, Code: engine.Sh2},
		`{"type":"giveShukhCard","card":{"suit":"♠","rank":7}}`:       engine.GiveShukhCard{Card: engine.Card{Suit: engine.Spades, Rank: 7}},
		`{"type":"takeShukhCards","seat":1}`:                          engine.TakeShukhCards{Seat: 1},
		`{"type":"declareOneCard","seat":1}`:                          engine.DeclareOneCard{Seat: 1},
		`{"type":"askCount","target":1}`:                              engine.AskCount{Target: 1},
		`{"type":"askAboutWest","target":1}`:                          engine.AskAboutWest{Target: 1},
		`{"type":"claimSubjective","claimant":0,"target":1,"code":6}`: engine.ClaimSubjective{Claimant: 0, Target: 1, Code: engine.Sh6},
	}
	for raw, want := range cases {
		got := decodeActionJSON(t, raw)
		if got != want {
			t.Fatalf("decode %s = %#v, want %#v", raw, got, want)
		}
	}
}

func TestDecodeVoteMapsSupport(t *testing.T) {
	against := decodeActionJSON(t, `{"type":"vote","vote":"againstShukh"}`)
	if v, ok := against.(engine.Vote); !ok || !v.Support {
		t.Fatalf("againstShukh must map to Support:true, got %#v", against)
	}
	forShukh := decodeActionJSON(t, `{"type":"vote","vote":"forShukh"}`)
	if v, ok := forShukh.(engine.Vote); !ok || v.Support {
		t.Fatalf("forShukh must map to Support:false, got %#v", forShukh)
	}
}

func TestDecodeUnknownType(t *testing.T) {
	if _, err := decodeAction(json.RawMessage(`{"type":"noSuchAction"}`)); err == nil {
		t.Fatal("unknown action type must error")
	}
}

func TestWithActorStampsSelfFields(t *testing.T) {
	if a := withActor(engine.Vote{Support: true}, 3); a.(engine.Vote).Voter != 3 {
		t.Fatal("withActor must stamp Vote.Voter")
	}
	if a := withActor(engine.ClaimSubjective{Target: 1, Code: engine.Sh6}, 2); a.(engine.ClaimSubjective).Claimant != 2 {
		t.Fatal("withActor must stamp ClaimSubjective.Claimant")
	}
	if a := withActor(engine.DeclareOneCard{}, 4); a.(engine.DeclareOneCard).Seat != 4 {
		t.Fatal("withActor must stamp DeclareOneCard.Seat")
	}
	if a := withActor(engine.PlayCard{}, 5); a != (engine.PlayCard{}) {
		t.Fatal("withActor must leave non-self actions unchanged")
	}
}
