package server

import (
	"sync"
	"time"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

// wsConn is a placeholder for the live-socket handle wired in Task 16 (§6, WebSocket
// connection). It exists only so Room.socks compiles now; Task 16 replaces it with
// the real connection type and fleshes out double-connect eviction.
type wsConn struct{}

// Room wraps one *game.Session with the Layer-2 machinery: room code, token→PlayerID
// table, storage write-through, timers (vote, grace), and connection bookkeeping.
// The session holds all game state; Room adds only transport.
type Room struct {
	mu      sync.Mutex
	code    string
	session *game.Session
	tokens  map[Token]game.PlayerID
	store   RoomStore
	clock   Clock

	// GC bookkeeping (§5): number of live sockets and when it last hit zero.
	live    int
	emptyAt time.Time

	// vote timer (§8) + reconnect grace (§5.4); wired in Task 14.
	voteTimer    Timer
	voteDeadline *int64
	graceTimers  map[game.PlayerID]Timer

	// live sockets, for double-connect eviction (§6); populated in Task 16.
	socks map[game.PlayerID]*wsConn
}

// NewRoom creates a room seated by the host: it builds the session, mints the host
// token, and persists the initial snapshot. Returns the room and the host token.
func NewRoom(code string, cfg game.Config, hostName string, store RoomStore, clock Clock) (*Room, Token) {
	host := newPlayerID()
	r := &Room{
		code:        code,
		session:     game.NewSession(cfg, host, hostName),
		tokens:      map[Token]game.PlayerID{},
		store:       store,
		clock:       clock,
		emptyAt:     clock.Now(), // no sockets yet → eligible for GC after grace if abandoned
		graceTimers: map[game.PlayerID]Timer{},
	}
	tok := r.mintToken(host)
	r.persist()
	return r, tok
}

// newPlayerID mints an opaque, server-private seat identity. It reuses the token
// generator's entropy; a PlayerID never leaves the server.
func newPlayerID() game.PlayerID {
	t, err := newToken()
	if err != nil {
		panic("server: crypto/rand failed minting PlayerID: " + err.Error())
	}
	return game.PlayerID(t)
}

// mintToken registers a fresh token for pid. Caller holds r.mu (or the room is not
// yet shared, as in NewRoom).
func (r *Room) mintToken(pid game.PlayerID) Token {
	tok, err := newToken()
	if err != nil {
		panic("server: crypto/rand failed minting token: " + err.Error())
	}
	r.tokens[tok] = pid
	return tok
}

// Join seats a new player: Session.Join + mint token + persist.
func (r *Room) Join(name string) (Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid := newPlayerID()
	if err := r.session.Join(pid, name); err != nil {
		return "", err
	}
	tok := r.mintToken(pid)
	r.persist()
	return tok, nil
}

// playerFor resolves a token to its PlayerID.
func (r *Room) playerFor(tok Token) (game.PlayerID, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, ok := r.tokens[tok]
	return pid, ok
}

// seatOf resolves pid to its current seat index via the durable session order, or -1
// if not seated. Cheap enough for per-update use in the MVP.
func (r *Room) seatOf(pid game.PlayerID) engine.SeatID {
	for i, p := range r.session.Snapshot().Order {
		if p == pid {
			return engine.SeatID(i)
		}
	}
	return -1
}

// persist write-throughs the durable snapshot (§4). Caller holds r.mu (except the
// pre-publication call in NewRoom).
func (r *Room) persist() {
	snap := RoomSnapshot{
		Code:    r.code,
		Tokens:  r.tokens,
		Session: r.session.Snapshot(),
	}
	_ = r.store.Save(snap) // MemStore never errors; a real store would log/retry
}
