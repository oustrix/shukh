package game

import (
	"reflect"
	"testing"
)

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	s := startedDuel(t) // host = seat 0, "p2" = seat 1 (submit_test.go)

	// Deep-copy proof: a snapshot must not observe later in-place mutation of the
	// live session's maps.
	snap := s.Snapshot()
	origLive0 := snap.Game.Live[0]
	origName := snap.Names["h"]
	s.mu.Lock()
	s.state.Live[0] = !s.state.Live[0]
	s.names["h"] = "Mutated"
	s.mu.Unlock()
	if snap.Game.Live[0] != origLive0 {
		t.Fatal("Snapshot must deep-copy engine.State (Live map aliased)")
	}
	if snap.Names["h"] != origName {
		t.Fatal("Snapshot must deep-copy the names map")
	}

	// Round-trip: Restore rebuilds a session whose own Snapshot deep-equals the input.
	want := s.Snapshot()
	r := Restore(want)
	got := r.Snapshot()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Restore round-trip mismatch:\n got=%+v\nwant=%+v", got, want)
	}

	// The restored session is fully live: fresh empty subs accept a new subscriber,
	// and it can still project per-seat.
	ch, cancel, err := r.Subscribe("h")
	if err != nil {
		t.Fatalf("restored session Subscribe: %v", err)
	}
	defer cancel()
	<-ch // initial snapshot delivered
	if _, err := r.SnapshotFor("h"); err != nil {
		t.Fatalf("restored session SnapshotFor: %v", err)
	}
}
