package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/coder/websocket"

	"github.com/oustrix/shukh/game"
)

// wsConn is one live socket. All writes for a socket go through its write pump (the
// single writer), so acks/errors and nudged snapshots are queued on out/nudge rather
// than written from the read goroutine.
type wsConn struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
	out    chan ServerMsg // serialized outbound acks/errors
	nudge  chan struct{}  // request to re-snapshot (after non-fanout transitions)
}

// serveConn runs the two pumps for one authenticated socket. Subscribe uses
// close-and-replace (Фаза A), so a reconnect is clean; a double-connect evicts the
// prior socket in registerConn.
func (r *Room) serveConn(ctx context.Context, c *websocket.Conn, pid game.PlayerID) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wc := r.registerConn(pid, c, cancel)
	defer r.unregisterConn(pid, c)

	ch, unsub, err := r.session.Subscribe(pid)
	if err != nil {
		return
	}
	defer unsub()

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.writePump(ctx, wc, ch, pid)
	}()
	r.readPump(ctx, wc, pid)
	cancel()
	<-done
}

// registerConn installs wc as pid's live socket, evicting any prior one (double
// connect closes the first cleanly, §11) and cancelling any pending grace (reconnect).
func (r *Room) registerConn(pid game.PlayerID, c *websocket.Conn, cancel context.CancelFunc) *wsConn {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.socks == nil {
		r.socks = map[game.PlayerID]*wsConn{}
	}
	had := false
	if old, ok := r.socks[pid]; ok {
		old.cancel() // evict prior socket; its pumps unwind, its serveConn returns
		had = true
	}
	r.cancelGrace(pid)
	wc := &wsConn{conn: c, cancel: cancel, out: make(chan ServerMsg, 8), nudge: make(chan struct{}, 1)}
	r.socks[pid] = wc
	if !had {
		r.noteConnOpened()
	}
	return wc
}

// unregisterConn removes c if it is still pid's current socket, then starts grace.
// On eviction (c already replaced) it is a no-op — the replacement holds the seat.
func (r *Room) unregisterConn(pid game.PlayerID, c *websocket.Conn) {
	r.mu.Lock()
	removed := false
	if wc, ok := r.socks[pid]; ok && wc.conn == c {
		delete(r.socks, pid)
		r.noteConnClosed()
		removed = true
	}
	r.mu.Unlock()
	if removed {
		r.onDisconnect(pid) // start the grace timer (§5.4)
	}
}

// writePump is the sole writer for wc. It serializes subscription Updates, queued
// acks/errors, and nudged re-snapshots onto the socket.
func (r *Room) writePump(ctx context.Context, wc *wsConn, ch <-chan game.Update, pid game.PlayerID) {
	for {
		select {
		case <-ctx.Done():
			return
		case up, ok := <-ch:
			if !ok {
				return // Subscribe closed (unsub / close-and-replace)
			}
			r.writeMsg(ctx, wc, encodeUpdate(r.seatOf(pid), r.code, up, r.voteDeadlineSafe()))
		case m := <-wc.out:
			r.writeMsg(ctx, wc, m)
		case <-wc.nudge:
			if up, err := r.session.SnapshotFor(pid); err == nil {
				r.writeMsg(ctx, wc, encodeUpdate(r.seatOf(pid), r.code, up, r.voteDeadlineSafe()))
			}
		}
	}
}

func (r *Room) writeMsg(ctx context.Context, wc *wsConn, m ServerMsg) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = wc.conn.Write(ctx, websocket.MessageText, data)
}

func (r *Room) voteDeadlineSafe() *int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentVoteDeadline()
}

// readPump decodes ClientMsgs and dispatches them until the socket closes.
func (r *Room) readPump(ctx context.Context, wc *wsConn, pid game.PlayerID) {
	for {
		typ, data, err := wc.conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		var msg ClientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			r.reply(wc, errorMsg("", "badRequest", err.Error()))
			continue
		}
		r.dispatch(ctx, wc, pid, msg)
	}
}

// dispatch routes one ClientMsg to the session and replies ack/error.
func (r *Room) dispatch(ctx context.Context, wc *wsConn, pid game.PlayerID, msg ClientMsg) {
	switch msg.Type {
	case "action":
		act, err := decodeAction(msg.Action)
		if err != nil {
			r.reply(wc, errorMsg(msg.ReqID, "badAction", err.Error()))
			return
		}
		act = withActor(act, r.seatOf(pid))
		events, err := r.session.Submit(pid, act)
		if err != nil {
			r.reply(wc, errorMsg(msg.ReqID, codeFor(err), err.Error()))
			return
		}
		r.mu.Lock()
		r.commit(events) // arms/disarms vote timer + persists; Submit already fanned out
		r.mu.Unlock()
		r.reply(wc, ackMsg(msg.ReqID))
	case "setConfig":
		if msg.Config == nil {
			r.reply(wc, errorMsg(msg.ReqID, "badRequest", "missing config"))
			return
		}
		cfg, err := msg.Config.toGame()
		if err != nil {
			r.reply(wc, errorMsg(msg.ReqID, "badRequest", err.Error()))
			return
		}
		if err := r.session.SetConfig(pid, cfg); err != nil {
			r.reply(wc, errorMsg(msg.ReqID, codeFor(err), err.Error()))
			return
		}
		r.afterLobbyChange()
		r.reply(wc, ackMsg(msg.ReqID))
	case "start":
		seed := r.clock.Now().UnixNano() // SERVER-generated seed (L2-8)
		if err := r.session.Start(pid, seed); err != nil {
			r.reply(wc, errorMsg(msg.ReqID, codeFor(err), err.Error()))
			return
		}
		r.afterLobbyChange() // Start does not fan out → nudge sockets to re-snapshot
		r.reply(wc, ackMsg(msg.ReqID))
	case "leave":
		r.session.Leave(pid)
		r.afterLobbyChange()
		r.reply(wc, ackMsg(msg.ReqID))
	default:
		r.reply(wc, errorMsg(msg.ReqID, "badRequest", "unknown message type"))
	}
}

// afterLobbyChange persists and nudges every socket to re-snapshot. Used after
// transitions that change state without a Session fanout (SetConfig/Start/Leave).
func (r *Room) afterLobbyChange() {
	r.mu.Lock()
	r.persist()
	for _, wc := range r.socks {
		select {
		case wc.nudge <- struct{}{}:
		default:
		}
	}
	r.mu.Unlock()
}

// reply queues an ack/error for the write pump. A full outbound buffer drops it (the
// client re-syncs from the next update).
func (r *Room) reply(wc *wsConn, m ServerMsg) {
	select {
	case wc.out <- m:
	default:
	}
}

// codeFor maps game sentinel errors to stable protocol error codes (§10). Any other
// error is an engine rule rejection (engine.IllegalAction) with state untouched; we
// return a generic code rather than couple to the engine error's concrete type.
func codeFor(err error) string {
	switch {
	case errors.Is(err, game.ErrNotYours):
		return "notYours"
	case errors.Is(err, game.ErrNotPlaying):
		return "notPlaying"
	case errors.Is(err, game.ErrNotHost):
		return "notHost"
	case errors.Is(err, game.ErrNotLobby):
		return "notLobby"
	case errors.Is(err, game.ErrTooFewPlayers):
		return "tooFewPlayers"
	case errors.Is(err, game.ErrUnknownPlayer):
		return "seatNotFound"
	case errors.Is(err, game.ErrFull):
		return "full"
	case errors.Is(err, game.ErrDuplicate):
		return "duplicate"
	}
	return "illegalAction"
}
