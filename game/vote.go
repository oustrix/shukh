package game

import "github.com/oustrix/shukh/engine"

// CloseVote force-resolves the open R-8.6 vote with the ballots cast so far (L2-1).
// It is the system entrypoint for the Layer-2 vote timer — it bypasses authorize (no
// player owns it). With no vote open it is a (nil, nil) no-op (the timer may fire
// after a full-turnout auto-resolve already cleared the vote). Otherwise it applies
// engine.CloseVote (resolving on a partial tally — a missing ballot is not counted as
// «против ШУХа»), advances the lifecycle, and fans the resolution out to every
// subscriber. Mirrors Submit's apply→invariant→stage→fanout discipline.
func (s *Session) CloseVote() ([]engine.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Adjudication == nil {
		return nil, nil // no open vote → nothing to resolve
	}
	return s.commitApply(engine.Apply(s.state, engine.CloseVote{}))
}
