package server

import (
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// Server wires HTTP handlers to a Hub: room creation, join (mint token + cookie),
// and the WS upgrade. Identity is a per-room HttpOnly cookie (L2-6).
type Server struct {
	hub *Hub
}

func NewServer(hub *Hub) *Server { return &Server{hub: hub} }

// Handler builds the router. Go 1.22 method+path patterns; {code} via PathValue.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /r", s.createRoom)
	mux.HandleFunc("POST /r/{code}/join", s.joinRoom)
	mux.HandleFunc("GET /r/{code}", s.connect)
	return mux
}

// cookieName scopes one cookie per room so several rooms coexist in one browser.
func cookieName(code string) string { return "shukh_" + code }

func roomCookie(code string, tok Token) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName(code),
		Value:    string(tok),
		Path:     "/r/" + code,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: true behind TLS in production.
	}
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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg = c
	}
	name := body.Name
	if name == "" {
		name = "Host"
	}
	code, tok, _ := s.hub.CreateRoom(cfg, name)
	http.SetCookie(w, roomCookie(code, tok))
	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

func (s *Server) joinRoom(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.hub.Room(code)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
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
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	pid, _ := room.playerFor(tok)
	http.SetCookie(w, roomCookie(code, tok))
	writeJSON(w, http.StatusOK, map[string]int{"seat": int(room.seatOf(pid))})
}

func (s *Server) connect(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.hub.Room(code)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	ck, err := req.Cookie(cookieName(code))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	pid, ok := room.playerFor(Token(ck.Value))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()
	room.serveConn(req.Context(), c, pid)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
