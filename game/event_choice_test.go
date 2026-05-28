package game

import (
	"testing"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

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

// TestTriggerEventTransitionsToPhaseEvent verifies entering a RoomEvent
// flips the session into PhaseEvent with PendingEvent populated, instead of
// resolving immediately.
func TestTriggerEventTransitionsToPhaseEvent(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	s.AddPlayer(p)

	// Hand-build a single-room floor with one event room.
	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms: []*dungeon.Room{
			{Type: dungeon.RoomEvent},
		},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.Phase = PhaseExploring
	s.mu.Unlock()

	s.EnterRoom(0)

	st := s.GetState()
	if st.Phase != PhaseEvent {
		t.Fatalf("phase should be PhaseEvent, got %v", st.Phase)
	}
	if st.PendingEvent == nil {
		t.Fatal("PendingEvent should be populated")
	}
	if len(st.PendingEvent.Choices) == 0 {
		t.Error("PendingEvent.Choices should not be empty")
	}
	if st.Floor.Rooms[0].Cleared {
		t.Error("room should not be cleared yet (waiting for vote)")
	}
}
