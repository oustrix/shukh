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

func TestHTTPCreateJoinConnect(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h).Handler())
	defer srv.Close()

	// create room
	resp, err := http.Post(srv.URL+"/r", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.Code == "" {
		t.Fatal("create must return a code")
	}
	hostCookie := findCookie(resp.Cookies(), cookieName(created.Code))
	if hostCookie == nil {
		t.Fatal("create must Set-Cookie the host token")
	}
	if !hostCookie.HttpOnly {
		t.Fatal("token cookie must be HttpOnly (L2-6)")
	}

	// join room → seat + cookie
	jresp, err := http.Post(srv.URL+"/r/"+created.Code+"/join", "application/json", strings.NewReader(`{"name":"Bob"}`))
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	var joined struct {
		Seat int `json:"seat"`
	}
	_ = json.NewDecoder(jresp.Body).Decode(&joined)
	jresp.Body.Close()
	if joined.Seat != 1 {
		t.Fatalf("Bob must be seat 1, got %d", joined.Seat)
	}
	bobCookie := findCookie(jresp.Cookies(), cookieName(created.Code))
	if bobCookie == nil {
		t.Fatal("join must Set-Cookie the seat token")
	}

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/r/" + created.Code
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// WS with cookie succeeds
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": {bobCookie.Name + "=" + bobCookie.Value}},
	})
	if err != nil {
		t.Fatalf("WS dial with cookie failed: %v", err)
	}
	c.Close(websocket.StatusNormalClosure, "")

	// WS without cookie is rejected (401 seatNotFound, §10)
	_, resp2, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("WS without cookie must be rejected")
	}
	if resp2 == nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", resp2)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
