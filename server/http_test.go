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
	srv := httptest.NewServer(NewServer(h, Options{}).Handler())
	defer srv.Close()

	// create room
	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Host"}`))
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
	jresp, err := http.Post(srv.URL+"/api/rooms/"+created.Code+"/join", "application/json", strings.NewReader(`{"name":"Bob"}`))
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

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/" + created.Code
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

func TestRoutesAndCookieScope(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer resp.Body.Close()
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	if created.Code == "" {
		t.Fatal("create must return a code")
	}
	ck := findCookie(resp.Cookies(), cookieName(created.Code))
	if ck == nil {
		t.Fatal("create must Set-Cookie")
	}
	// Кука обязана уходить и на /api/..., и на /ws/... — значит Path=/ (§7.4).
	if ck.Path != "/" {
		t.Fatalf("cookie Path = %q, want \"/\"", ck.Path)
	}
	if ck.SameSite != http.SameSiteLaxMode || ck.Secure {
		t.Fatalf("default cookie must be Lax and non-Secure, got SameSite=%v Secure=%v", ck.SameSite, ck.Secure)
	}
}

func TestProbeMe(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	ck := findCookie(resp.Cookies(), cookieName(created.Code))

	probe := func(code string, cookie *http.Cookie) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/rooms/"+code+"/me", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("probe: %v", err)
		}
		defer r.Body.Close()
		var body struct {
			Seat  *int   `json:"seat"`
			Error string `json:"error"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Seat != nil {
			return r.StatusCode, "seat"
		}
		return r.StatusCode, body.Error
	}

	if st, what := probe(created.Code, ck); st != http.StatusOK || what != "seat" {
		t.Fatalf("with cookie: %d/%s, want 200/seat", st, what)
	}
	if st, what := probe(created.Code, nil); st != http.StatusUnauthorized || what != "seatNotFound" {
		t.Fatalf("without cookie: %d/%s, want 401/seatNotFound", st, what)
	}
	if st, what := probe("ZZZZ", ck); st != http.StatusNotFound || what != "roomNotFound" {
		t.Fatalf("unknown room: %d/%s, want 404/roomNotFound", st, what)
	}
}

func TestCORSAllowlist(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{Origins: []string{"http://localhost:5173"}}).Handler())
	defer srv.Close()

	// preflight от разрешённого origin
	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/rooms", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Allow-Origin = %q, want the echoed origin (\"*\" is illegal with credentials)", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}

	// чужой origin не получает разрешения
	req2, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/rooms", nil)
	req2.Header.Set("Origin", "http://evil.example")
	req2.Header.Set("Access-Control-Request-Method", "POST")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("preflight2: %v", err)
	}
	resp2.Body.Close()
	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin for foreign origin = %q, want empty", got)
	}
}

func TestCrossSiteCookieMode(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{CrossSite: true}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer resp.Body.Close()
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	ck := findCookie(resp.Cookies(), cookieName(created.Code))
	if ck == nil {
		t.Fatal("create must Set-Cookie")
	}
	if ck.SameSite != http.SameSiteNoneMode || !ck.Secure {
		t.Fatalf("cross-site cookie must be None+Secure, got SameSite=%v Secure=%v", ck.SameSite, ck.Secure)
	}
}
