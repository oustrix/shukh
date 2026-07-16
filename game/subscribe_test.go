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
	s.fanout(map[PlayerID][]engine.Event{"h": {engine.OneCardDeclared{Seat: 0}}})
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
