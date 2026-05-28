package game

import "testing"

// TestEventChoiceTagsPresent enforces every choice in every event has at
// least one Tag — UI relies on Tags to render category icons.
func TestEventChoiceTagsPresent(t *testing.T) {
	for _, ev := range GetRandomEvents() {
		for i, ch := range ev.Choices {
			if len(ch.Tags) == 0 {
				t.Errorf("event %q choice %d (%q) has no Tags", ev.Name, i, ch.Label)
			}
		}
	}
}

// TestPhaseEventConstantExists is a compile-time guard that PhaseEvent is
// defined and distinct from existing phases.
func TestPhaseEventConstantExists(t *testing.T) {
	phases := []GamePhase{PhaseExploring, PhaseCombat, PhaseShop, PhaseResult, PhaseEvent}
	seen := make(map[GamePhase]bool, len(phases))
	for _, p := range phases {
		if seen[p] {
			t.Errorf("phase %d duplicated", p)
		}
		seen[p] = true
	}
}

// TestPendingEventOnGameState ensures GameState exposes PendingEvent and
// EventVotes fields for UI snapshot consumption.
func TestPendingEventOnGameState(t *testing.T) {
	var gs GameState
	_ = gs.PendingEvent // *PendingEventView
	_ = gs.EventVotes   // map[string]int
}
