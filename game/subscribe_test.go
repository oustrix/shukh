package game

import (
	"testing"

	"github.com/oustrix/shukh/engine"
)

func TestSubscribeReceivesFanout(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	_ = s.Join("p2", "Bob")
	ch, _, err := s.Subscribe("h")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	<-ch // drain the initial snapshot Subscribe delivers
	// Directly exercise fanout with a synthetic event.
	s.mu.Lock()
	s.fanout([]engine.Event{engine.OneCardDeclared{Seat: 0}})
	s.mu.Unlock()
	select {
	case up := <-ch:
		if len(up.Events) != 1 {
			t.Fatalf("want 1 event delivered, got %d", len(up.Events))
		}
	default:
		t.Fatal("expected an Update on the subscriber channel")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	ch, cancel, _ := s.Subscribe("h")
	<-ch // drain the initial snapshot so the next receive reflects closure
	cancel()
	if _, open := <-ch; open {
		t.Fatal("unsubscribe must close the channel")
	}
}

func TestSubscribeUnknownPlayer(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	if _, _, err := s.Subscribe("ghost"); err != ErrUnknownPlayer {
		t.Fatalf("want ErrUnknownPlayer, got %v", err)
	}
}

func TestSubscribeCloseAndReplace(t *testing.T) {
	s := NewSession(cfg36(), "h", "Host")
	ch1, _, err := s.Subscribe("h")
	if err != nil {
		t.Fatalf("first subscribe: %v", err)
	}
	<-ch1 // drain the initial snapshot

	ch2, cancel2, err := s.Subscribe("h")
	if err != nil {
		t.Fatalf("second subscribe: %v", err)
	}
	defer cancel2()

	// close-and-replace: the first channel must now be closed so its reader exits.
	// A non-blocking probe distinguishes "closed" (case, open==false) from the
	// pre-fix "still open but empty" (default) without hanging the test.
	select {
	case _, open := <-ch1:
		if open {
			t.Fatal("re-Subscribe must close (not feed) the previous channel")
		}
	default:
		t.Fatal("re-Subscribe must close the previous subscriber channel (still open)")
	}

	// the second channel is the live one: a fanout reaches it.
	<-ch2 // drain its initial snapshot
	s.mu.Lock()
	s.fanout([]engine.Event{engine.OneCardDeclared{Seat: 0}})
	s.mu.Unlock()
	select {
	case up, open := <-ch2:
		if !open {
			t.Fatal("the replacement channel must stay open")
		}
		if len(up.Events) != 1 {
			t.Fatalf("want 1 event on the live channel, got %d", len(up.Events))
		}
	default:
		t.Fatal("the replacement subscriber must receive the fanout")
	}
}
