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
)

type wsClient struct {
	t   *testing.T
	c   *websocket.Conn
	ctx context.Context
}

func dialClient(t *testing.T, ctx context.Context, wsURL string, cookie *http.Cookie) *wsClient {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": {cookie.Name + "=" + cookie.Value}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return &wsClient{t: t, c: c, ctx: ctx}
}

func (w *wsClient) read() map[string]any {
	_, data, err := w.c.Read(w.ctx)
	if err != nil {
		w.t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		w.t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func (w *wsClient) send(v map[string]any) {
	data, _ := json.Marshal(v)
	if err := w.c.Write(w.ctx, websocket.MessageText, data); err != nil {
		w.t.Fatalf("write: %v", err)
	}
}

// readUpdateWithEvent reads until an update carrying an event of type typ.
func (w *wsClient) readUpdateWithEvent(typ string) {
	for i := 0; i < 50; i++ {
		m := w.read()
		if m["type"] == "update" && hasEvent(m, typ) {
			return
		}
	}
	w.t.Fatalf("did not observe an update carrying event %q", typ)
}

// readUntilStage reads until an update reports the given stage.
func (w *wsClient) readUntilStage(stage string) {
	for i := 0; i < 50; i++ {
		if w.read()["stage"] == stage {
			return
		}
	}
	w.t.Fatalf("never reached stage %q", stage)
}

func hasEvent(m map[string]any, typ string) bool {
	evs, _ := m["events"].([]any)
	for _, e := range evs {
		if em, ok := e.(map[string]any); ok && em["type"] == typ {
			return true
		}
	}
	return false
}

func TestIntegrationVoteTimeoutAndReconnect(t *testing.T) {
	clock := newFakeClock(time.Unix(1_000, 0)) // deterministic Now → deterministic start seed
	h := NewHub(NewMemStore(), clock)
	srv := httptest.NewServer(NewServer(h).Handler())
	defer srv.Close()
	base := srv.URL
	wsBase := "ws" + strings.TrimPrefix(base, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// create + join over HTTP (cookies mint identity)
	cResp, _ := http.Post(base+"/r", "application/json", strings.NewReader(`{"name":"Host"}`))
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(cResp.Body).Decode(&created)
	cResp.Body.Close()
	code := created.Code
	hostCookie := findCookie(cResp.Cookies(), cookieName(code))

	jResp, _ := http.Post(base+"/r/"+code+"/join", "application/json", strings.NewReader(`{"name":"Bob"}`))
	jResp.Body.Close()
	bobCookie := findCookie(jResp.Cookies(), cookieName(code))

	wsURL := wsBase + "/r/" + code
	host := dialClient(t, ctx, wsURL, hostCookie)
	defer host.c.CloseNow()
	bob := dialClient(t, ctx, wsURL, bobCookie)
	defer bob.c.CloseNow()

	host.read() // initial lobby snapshot
	bob.read()

	// host starts (server seed from the fake clock → deterministic)
	host.send(map[string]any{"type": "start", "reqId": "start"})
	host.readUntilStage("playing")
	bob.readUntilStage("playing")

	// host opens a subjective vote (gates are closed right after the deal)
	host.send(map[string]any{
		"type": "action", "reqId": "claim",
		"action": map[string]any{"type": "claimSubjective", "target": 1, "code": 6},
	})
	host.readUpdateWithEvent("voteOpened")
	bob.readUpdateWithEvent("voteOpened")

	// the deadline fires → CloseVote resolves with a partial tally → voteResolved to BOTH
	clock.Advance(voteTTL)
	host.readUpdateWithEvent("voteResolved")
	bob.readUpdateWithEvent("voteResolved")

	// --- reconnect: drop host, redial with the same cookie, get a fresh snapshot ---
	host.c.CloseNow()
	host2 := dialClient(t, ctx, wsURL, hostCookie)
	defer host2.c.CloseNow()
	if host2.read()["type"] != "update" {
		t.Fatal("reconnect must deliver a fresh snapshot")
	}

	// --- double-connect: a second dial with the same cookie evicts the first ---
	host3 := dialClient(t, ctx, wsURL, hostCookie)
	defer host3.c.CloseNow()
	host3.read() // fresh snapshot on the new socket
	if _, _, err := host2.c.Read(ctx); err == nil {
		t.Fatal("double-connect must close the previously connected socket")
	}
}
