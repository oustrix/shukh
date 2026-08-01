package server

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

var updateGolden = flag.Bool("update", false, "rewrite server/testdata/wire/*.json")

// Golden fixtures pin the exact wire shape of every message the browser can receive.
// The TS mirror (web/src/contract/wire.test.ts) decodes these very files, so a change
// on either side that the other does not follow fails a test instead of a game (W3-3).
func TestProtocolGolden(t *testing.T) {
	c := func(rank int, suit engine.Suit) engine.Card { return engine.Card{Rank: engine.Rank(rank), Suit: suit} }
	view := &engine.SeatView{
		Rules:        engine.RuleSet{DeckSize: engine.Deck36},
		Mode:         engine.Middle,
		Phase:        engine.Playing,
		You:          1,
		Turn:         1,
		Hand:         []engine.Card{c(6, engine.Hearts), c(14, engine.Spades)},
		ShukhPending: 1,
		Opponents:    []engine.OpponentView{{Seat: 0, HandCount: 5, ShukhPending: 0, Live: true}},
		Table:        []engine.TableCard{{Card: c(7, engine.Clubs), By: 0}},
		Discard:      3,
		Talon:        18,
		Live:         map[engine.SeatID]bool{0: true, 1: true},
		Finish:       []engine.SeatID{},
	}
	voteView := *view
	voteView.Vote = &engine.VoteView{Claimant: 0, Target: 1, Code: engine.Sh6, Voted: []engine.SeatID{0}}

	roster := []game.SeatMeta{{Seat: 0, Name: "Вера"}, {Seat: 1, Name: "Боря"}}
	deadline := int64(1754130000000)

	cases := []struct {
		name string
		msg  ServerMsg
	}{
		{"lobby", encodeUpdate(1, "ABCD", game.Update{Stage: game.Lobby, Host: 0, Roster: roster}, nil)},
		{"playing", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: view,
			Legal:  []engine.Action{engine.PlayCard{Card: c(14, engine.Spades)}, engine.TakeBottomAndPass{}},
			Events: []engine.Event{engine.CardPlayed{Seat: 0, Card: c(7, engine.Clubs)}},
		}, nil)},
		{"vote_open", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: &voteView,
			Legal:  []engine.Action{engine.Vote{Voter: 1, Support: true}, engine.Vote{Voter: 1, Support: false}},
			Events: []engine.Event{engine.VoteOpened{Claimant: 0, Target: 1, Code: engine.Sh6}},
		}, &deadline)},
		{"all_actions", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: view,
			Legal: []engine.Action{
				engine.PlayCard{Card: c(6, engine.Hearts)},
				engine.TakeBottomAndPass{},
				engine.PodkladkaWest{},
				engine.DiscardWest{},
				engine.ClaimShukh{Target: 0, Code: engine.Sh2},
				engine.GiveShukhCard{Card: c(14, engine.Spades)},
				engine.TakeShukhCards{Seat: 1},
				engine.DeclareOneCard{Seat: 1},
				engine.AskCount{Target: 0},
				engine.AskAboutWest{Target: 0},
				engine.ClaimSubjective{Claimant: 1, Target: 0, Code: engine.Sh9},
				engine.Vote{Voter: 1, Support: false},
			},
		}, nil)},
		{"all_events", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: view,
			Events: []engine.Event{
				engine.GameStarted{Turn: 0},
				engine.CardPlayed{Seat: 0, Card: c(7, engine.Clubs)},
				engine.ConClosed{By: 0},
				engine.ConSwept{Cards: []engine.Card{c(7, engine.Clubs)}},
				engine.PlayerFinished{Seat: 0, Place: 1},
				engine.GameFinished{Finish: []engine.SeatID{0, 1}},
				engine.CardsTaken{Seat: 1, Cards: []engine.Card{c(7, engine.Clubs)}},
				engine.PodkladkaPlayed{Seat: 1, Eater: 0},
				engine.TurnSkipped{Seat: 1},
				engine.ShukhAssessed{Offender: 1, Code: engine.Sh11},
				engine.ActionReverted{Seat: 1},
				engine.ShukhPaid{Offender: 1, From: 0, Card: c(7, engine.Clubs)},
				engine.ShukhCardsTaken{Seat: 1, Cards: []engine.Card{c(7, engine.Clubs)}},
				engine.OneCardDeclared{Seat: 1},
				engine.WestDiscarded{Seat: 1},
				engine.VoteOpened{Claimant: 0, Target: 1, Code: engine.Sh6},
				engine.VoteResolved{Code: engine.Sh8, Overturned: true},
			},
		}, nil)},
	}

	dir := filepath.Join("testdata", "wire")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.MarshalIndent(tc.msg, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')
			path := filepath.Join(dir, tc.name+".json")
			if *updateGolden {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s (run `go test ./server/ -update` to create): %v", path, err)
			}
			if string(got) != string(want) {
				t.Fatalf("wire shape changed for %s.\n--- got ---\n%s\n--- want ---\n%s", tc.name, got, want)
			}
		})
	}
}
