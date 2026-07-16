// Package server is Layer 2 (D-6): the network adapter over game.Session. It maps
// connection→PlayerID and room-code→*game.Session, drives the synchronous session
// API, and relays Updates to browsers over WebSocket. It holds no game rules — the
// rules live in engine (Layer 0) and the authoritative procedures in game (Layer 1).
package server

import (
	"sync"
	"time"
)

// Timer is a pending one-shot callback. Stop cancels it, reporting whether it was
// still pending (true) or had already fired / been stopped (false).
type Timer interface {
	Stop() bool
}

// Clock is the seam over time (L2-9): real in production, fake in tests. Vote,
// grace, and GC timers go through it so tests advance time deterministically
// instead of sleeping.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// NewRealClock returns a Clock backed by the standard library.
func NewRealClock() Clock { return realClock{} }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) Timer {
	return time.AfterFunc(d, f) // *time.Timer satisfies Timer (Stop() bool)
}

// fakeClock is a deterministic test double: time only moves on Advance, which fires
// every timer whose deadline it passes.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

func newFakeClock(start time.Time) *fakeClock { return &fakeClock{now: start} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clock: c, deadline: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock forward and fires every due timer, in insertion order,
// with the lock released so a callback may arm further timers.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.fired && !t.stopped && !t.deadline.After(now) {
			t.fired = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

type fakeTimer struct {
	clock    *fakeClock
	deadline time.Time
	f        func()
	fired    bool
	stopped  bool
}

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.fired || t.stopped {
		return false
	}
	t.stopped = true
	return true
}
