package server

import (
	"sync"
	"time"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

const (
	voteTTL  = 30 * time.Second // R-8.6 vote deadline (L2-1)
	graceTTL = 5 * time.Minute  // reconnect grace before a lobby Leave (§5.4)
)

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

// commit records the durable snapshot and updates the vote timer after a
// state-changing session call. A VoteOpened in the events arms the deadline; a
// VoteResolved disarms it. Caller holds r.mu.
func (r *Room) commit(events []engine.Event) {
	for _, e := range events {
		switch e.(type) {
		case engine.VoteOpened:
			r.armVote()
		case engine.VoteResolved:
			r.disarmVote()
		}
	}
	r.persist()
}

// armVote sets a voteTTL deadline whose expiry closes the vote. Caller holds r.mu.
func (r *Room) armVote() {
	if r.voteTimer != nil {
		r.voteTimer.Stop()
	}
	deadline := r.clock.Now().Add(voteTTL).UnixMilli()
	r.voteDeadline = &deadline
	r.voteTimer = r.clock.AfterFunc(voteTTL, r.fireVote)
}

// disarmVote cancels the vote timer and clears the deadline. Caller holds r.mu.
func (r *Room) disarmVote() {
	if r.voteTimer != nil {
		r.voteTimer.Stop()
		r.voteTimer = nil
	}
	r.voteDeadline = nil
}

// fireVote runs on the clock goroutine when the deadline elapses: it closes the vote
// with the current ballots (CloseVote fans the resolution out itself) and updates
// the timer state. A resolve that already happened makes CloseVote a no-op (§8.2).
func (r *Room) fireVote() {
	r.mu.Lock()
	defer r.mu.Unlock()
	events, err := r.session.CloseVote()
	if err != nil {
		return
	}
	r.commit(events)
}

// currentVoteDeadline returns a copy of the active vote deadline (unix-ms) or nil.
// Caller holds r.mu.
func (r *Room) currentVoteDeadline() *int64 {
	if r.voteDeadline == nil {
		return nil
	}
	d := *r.voteDeadline
	return &d
}

// onDisconnect starts a grace timer for pid after its socket drops (§5.4). It does
// NOT Leave immediately: the seat is held so a reconnect within grace is seamless.
func (r *Room) onDisconnect(pid game.PlayerID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.graceTimers[pid]; ok {
		t.Stop()
	}
	r.graceTimers[pid] = r.clock.AfterFunc(graceTTL, func() { r.graceExpired(pid) })
}

// graceExpired runs when a disconnect's grace elapses. In the Lobby it Leaves (which
// migrates the host per L2-3); while Playing the seat is kept — the engine has no
// fold, so a general turn-timeout is future work (§12).
func (r *Room) graceExpired(pid game.PlayerID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.graceTimers, pid)
	if r.session.Stage() == game.Lobby {
		r.session.Leave(pid)
		r.persist()
	}
}

// cancelGrace stops any pending grace timer for pid (on reconnect). Caller holds r.mu.
func (r *Room) cancelGrace(pid game.PlayerID) {
	if t, ok := r.graceTimers[pid]; ok {
		t.Stop()
		delete(r.graceTimers, pid)
	}
}

// noteConnOpened / noteConnClosed maintain the live-socket count and the empty
// timestamp used by Hub.sweep. Caller holds r.mu.
func (r *Room) noteConnOpened() { r.live++ }

func (r *Room) noteConnClosed() {
	if r.live > 0 {
		r.live--
	}
	if r.live == 0 {
		r.emptyAt = r.clock.Now()
	}
}

// collectible reports whether the room may be garbage-collected at now: no live
// sockets for longer than grace, or Finished and idle past idleTTL (§5.3).
func (r *Room) collectible(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.live > 0 {
		return false
	}
	if r.session.Stage() == game.Finished {
		return now.Sub(r.emptyAt) >= idleTTL
	}
	return now.Sub(r.emptyAt) >= graceTTL
}
