package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"

	"github.com/coder/websocket"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// Options configures the transport-facing policy of the HTTP layer: which browser
// origins may talk to it (CORS + WS origin check) and whether the reconnect cookie
// must survive a cross-site deployment (W3-4).
type Options struct {
	Origins   []string // allowed browser origins, e.g. "http://localhost:5173"; empty = same-origin only
	CrossSite bool     // true → SameSite=None; Secure (requires TLS)
}

// Server wires HTTP handlers to a Hub: room creation, join (mint token + cookie),
// and the WS upgrade. Identity is a per-room HttpOnly cookie (L2-6).
type Server struct {
	hub  *Hub
	opts Options
}

func NewServer(hub *Hub, opts Options) *Server { return &Server{hub: hub, opts: opts} }

// Handler builds the router. Go 1.22 method+path patterns; {code} via PathValue.
// /api/* and /ws/* are the server's namespace; /r/CODE is deliberately left free so
// the SPA can own the invite link (W3-2, D-2).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("POST /api/rooms/{code}/join", s.joinRoom)
	mux.HandleFunc("GET /api/rooms/{code}/me", s.me)
	mux.HandleFunc("GET /ws/{code}", s.connect)
	return s.cors(mux)
}

// cors applies the allowlist to browser requests. The origin is echoed rather than
// "*": the reconnect cookie makes every request credentialed, and the wildcard is
// illegal with credentials (§7.5).
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		if origin != "" {
			// Vary — на любой запрос с Origin, а не только на разрешённый: ответ
			// зависит от Origin и когда заголовки НЕ выданы, и общий кеш иначе
			// отдал бы отказ разрешённому origin (и наоборот).
			w.Header().Add("Vary", "Origin")
		}
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (s *Server) originAllowed(origin string) bool {
	return slices.Contains(s.opts.Origins, origin)
}

// originPatterns converts the allowlist into host patterns for the WS Origin check.
// Empty list → nil, and coder/websocket falls back to same-origin only.
func (s *Server) originPatterns() []string {
	out := make([]string, 0, len(s.opts.Origins))
	for _, o := range s.opts.Origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
}

// cookieName scopes one cookie per room so several rooms coexist in one browser.
func cookieName(code string) string { return "shukh_" + code }

// roomCookie builds the reconnect cookie. Path is "/" because the token must reach
// both /api/rooms/{code}/... and /ws/{code} (§7.4); the per-room name keeps rooms
// isolated from each other.
func (s *Server) roomCookie(code string, tok Token) *http.Cookie {
	c := &http.Cookie{
		Name:     cookieName(code),
		Value:    string(tok),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if s.opts.CrossSite {
		c.SameSite = http.SameSiteNoneMode
		c.Secure = true
	}
	return c
}

func (s *Server) createRoom(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Config *ConfigDTO `json:"config"`
		Name   string     `json:"name"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	cfg := game.Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}
	if body.Config != nil {
		c, err := body.Config.toGame()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "badRequest"})
			return
		}
		cfg = c
	}
	name := body.Name
	if name == "" {
		name = "Host"
	}
	code, tok, _ := s.hub.CreateRoom(cfg, name)
	http.SetCookie(w, s.roomCookie(code, tok))
	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

func (s *Server) joinRoom(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.resolveRoom(w, code)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "Player"
	}
	tok, err := room.Join(body.Name)
	if err != nil {
		// codeFor is the single mapping of game sentinels to protocol codes (§10);
		// Session.Join realistically fails only with ErrFull/ErrDuplicate, both of
		// which it already covers.
		writeJSON(w, http.StatusConflict, map[string]string{"error": codeFor(err)})
		return
	}
	pid, _ := room.playerFor(tok)
	http.SetCookie(w, s.roomCookie(code, tok))
	writeJSON(w, http.StatusOK, map[string]int{"seat": int(room.seatOf(pid))})
}

// me reports whether the caller already holds a seat in this room. The browser WS
// API hides the handshake status, so a failed socket is indistinguishable from a
// dead server without this probe; it also drives the invite-link flow (§7.7).
func (s *Server) me(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.resolveRoom(w, code)
	if !ok {
		return
	}
	pid, ok := s.resolvePlayer(w, req, code, room)
	if !ok {
		return
	}
	seat := room.seatOf(pid)
	if seat < 0 {
		// Token is valid but the seat was released (grace expired in the lobby).
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"seat": int(seat)})
}

func (s *Server) connect(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.resolveRoom(w, code)
	if !ok {
		return
	}
	pid, ok := s.resolvePlayer(w, req, code, room)
	if !ok {
		return
	}
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: s.originPatterns()})
	if err != nil {
		return
	}
	defer c.CloseNow()
	room.serveConn(req.Context(), c, pid)
}

// resolveRoom looks a room up by code, answering roomNotFound itself when there is
// none. Shared by every handler that takes a {code}: the reply for a missing room is
// a protocol decision (§10), and it must not drift between HTTP and WS entry points.
func (s *Server) resolveRoom(w http.ResponseWriter, code string) (*Room, bool) {
	room, ok := s.hub.Room(code)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "roomNotFound"})
		return nil, false
	}
	return room, true
}

// resolvePlayer maps the room cookie to its PlayerID, answering seatNotFound itself.
// A missing cookie and an unknown token are deliberately indistinguishable to the
// caller: both mean "you hold no seat here", and saying which would leak whether a
// token exists.
func (s *Server) resolvePlayer(w http.ResponseWriter, req *http.Request, code string, room *Room) (game.PlayerID, bool) {
	ck, err := req.Cookie(cookieName(code))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return "", false
	}
	pid, ok := room.playerFor(Token(ck.Value))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return "", false
	}
	return pid, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
