package engine

import "testing"

import "github.com/stretchr/testify/require"

func players(n int) []Player {
	ps := make([]Player, n)
	for i := range ps {
		ps[i] = Player{Name: "p"}
	}
	return ps
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"2 players guard ok", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Guard, Players: players(2)}, true},
		{"8 players middle ok", Config{Rules: RuleSet{DeckSize: Deck52}, Mode: Middle, Players: players(8)}, true},
		{"1 player too few", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Middle, Players: players(1)}, false},
		{"9 players too many", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Middle, Players: players(9)}, false},
		{"culture not implemented", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: Culture, Players: players(2)}, false},
		{"bad deck size", Config{Rules: RuleSet{DeckSize: 40}, Mode: Middle, Players: players(2)}, false},
		{"unknown mode", Config{Rules: RuleSet{DeckSize: Deck36}, Mode: EnforcementMode(99), Players: players(2)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if c.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGameStartedIsEvent(t *testing.T) {
	var e Event = GameStarted{Turn: 3}
	require.Equal(t, SeatID(3), e.(GameStarted).Turn)
}
