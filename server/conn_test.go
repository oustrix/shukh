package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/oustrix/shukh/game"
)

// wsEcho stands up an httptest server that accepts a WS and runs serveConn for pid.
func wsEcho(t *testing.T, r *Room, pid game.PlayerID) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		r.serveConn(req.Context(), c, pid)
	}))
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, url
}

func readMsg(t *testing.T, ctx context.Context, c *websocket.Conn) map[string]any {
	t.Helper()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestConnActionAcksAndUpdates(t *testing.T) {
	r, _, _ := newTestRoom(t)
	if _, err := r.Join("Bob"); err != nil {
		t.Fatalf("Join: %v", err)
	}
	host := r.session.Snapshot().Host
	if err := r.session.Start(host, 42); err != nil {
		t.Fatalf("Start: %v", err)
	}

	srv, url := wsEcho(t, r, host)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	// initial snapshot on connect
	first := readMsg(t, ctx, c)
	if first["type"] != "update" {
		t.Fatalf("first message must be an update, got %v", first["type"])
	}

	// host raises a subjective ШУХ (legal with gates closed, any turn) → ack + update
	send := map[string]any{
		"type":   "action",
		"reqId":  "1",
		"action": map[string]any{"type": "claimSubjective", "target": 1, "code": 6},
	}
	data, _ := json.Marshal(send)
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}

	sawAck, sawVoteOpen := false, false
	for i := 0; i < 6 && !(sawAck && sawVoteOpen); i++ {
		m := readMsg(t, ctx, c)
		switch m["type"] {
		case "ack":
			if m["reqId"] == "1" {
				sawAck = true
			}
		case "update":
			if evs, ok := m["events"].([]any); ok {
				for _, e := range evs {
					if em, ok := e.(map[string]any); ok && em["type"] == "voteOpened" {
						sawVoteOpen = true
					}
				}
			}
		case "error":
			t.Fatalf("unexpected error: %+v", m)
		}
	}
	if !sawAck || !sawVoteOpen {
		t.Fatalf("expected ack (%v) and a voteOpened update (%v)", sawAck, sawVoteOpen)
	}
}
