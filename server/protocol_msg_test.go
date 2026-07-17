package server

import (
	"encoding/json"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestEncodeUpdateVoteKeys(t *testing.T) {
	view := &engine.SeatView{
		Phase: engine.Playing,
		You:   0,
		Turn:  1,
		Live:  map[engine.SeatID]bool{0: true, 1: true},
		Vote:  &engine.VoteView{Claimant: 0, Target: 1, Code: engine.Sh6, Voted: []engine.SeatID{0}},
	}
	dl := int64(1700000000000)
	msg := encodeUpdate(0, "ROOM01", game.Update{
		Stage:  game.Playing,
		Roster: []game.SeatMeta{{Seat: 0, Name: "Host"}, {Seat: 1, Name: "Bob"}},
		View:   view,
		Events: []engine.Event{engine.VoteOpened{Claimant: 0, Target: 1, Code: engine.Sh6}},
	}, &dl)

	m := asMap(t, msg)
	if m["type"] != "update" || m["roomCode"] != "ROOM01" || m["stage"] != "playing" {
		t.Fatalf("envelope keys wrong: %+v", m)
	}
	if m["you"].(float64) != 0 || m["voteDeadline"].(float64) != 1.7e12 {
		t.Fatalf("you/voteDeadline wrong: %+v", m)
	}
	ev := m["events"].([]any)[0].(map[string]any)
	if ev["type"] != "voteOpened" || ev["claimant"].(float64) != 0 || ev["code"].(float64) != 6 {
		t.Fatalf("voteOpened event wrong: %+v", ev)
	}
	vv := m["view"].(map[string]any)["vote"].(map[string]any)
	if vv["target"].(float64) != 1 || len(vv["voted"].([]any)) != 1 {
		t.Fatalf("view.vote wrong: %+v", vv)
	}
}

func TestAckAndErrorEnvelopes(t *testing.T) {
	a := asMap(t, ackMsg("42"))
	if a["type"] != "ack" || a["reqId"] != "42" {
		t.Fatalf("ack shape: %+v", a)
	}
	e := asMap(t, errorMsg("42", "notYours", "boom"))
	if e["type"] != "error" || e["code"] != "notYours" || e["message"] != "boom" {
		t.Fatalf("error shape: %+v", e)
	}
}

func TestDecodeClientMsgAction(t *testing.T) {
	var msg ClientMsg
	if err := json.Unmarshal([]byte(`{"type":"action","reqId":"7","action":{"type":"takeBottomAndPass"}}`), &msg); err != nil {
		t.Fatalf("unmarshal ClientMsg: %v", err)
	}
	if msg.Type != "action" || msg.ReqID != "7" {
		t.Fatalf("ClientMsg fields: %+v", msg)
	}
	a, err := decodeAction(msg.Action)
	if err != nil {
		t.Fatalf("decodeAction: %v", err)
	}
	if _, ok := a.(engine.TakeBottomAndPass); !ok {
		t.Fatalf("want TakeBottomAndPass, got %T", a)
	}
}

func TestConfigDTOToGame(t *testing.T) {
	cfg, err := ConfigDTO{DeckSize: 36, Mode: "middle"}.toGame()
	if err != nil || cfg.Rules.DeckSize != engine.Deck36 || cfg.Mode != engine.Middle {
		t.Fatalf("toGame: cfg=%+v err=%v", cfg, err)
	}
	if _, err := (ConfigDTO{DeckSize: 36, Mode: "bogus"}).toGame(); err == nil {
		t.Fatal("unknown mode must error")
	}
	if _, err := (ConfigDTO{DeckSize: 99, Mode: "middle"}).toGame(); err == nil {
		t.Fatal("unsupported deck size must error")
	}
}
