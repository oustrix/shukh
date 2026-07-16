package server

import (
	"testing"
	"time"
)

func TestFakeClockFiresAfterDeadline(t *testing.T) {
	c := newFakeClock(time.Unix(0, 0))
	fired := false
	c.AfterFunc(10*time.Second, func() { fired = true })
	c.Advance(9 * time.Second)
	if fired {
		t.Fatal("timer fired before its deadline")
	}
	c.Advance(2 * time.Second) // now past 10s
	if !fired {
		t.Fatal("timer did not fire after Advance past the deadline")
	}
}

func TestFakeClockStopPreventsFiring(t *testing.T) {
	c := newFakeClock(time.Unix(0, 0))
	fired := false
	tm := c.AfterFunc(10*time.Second, func() { fired = true })
	if !tm.Stop() {
		t.Fatal("Stop of a pending timer must report true")
	}
	c.Advance(1 * time.Hour)
	if fired {
		t.Fatal("a stopped timer must not fire")
	}
	if tm.Stop() {
		t.Fatal("second Stop must report false")
	}
}

func TestFakeClockNowIsDeterministic(t *testing.T) {
	c := newFakeClock(time.Unix(100, 0))
	c.Advance(5 * time.Second)
	if got := c.Now().Unix(); got != 105 {
		t.Fatalf("Now = %d, want 105", got)
	}
}
