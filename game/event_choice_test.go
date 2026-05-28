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

// TestPendingEventDeepCopyIsolation ensures that mutating the snapshot's
// EventVotes map does not affect the session's internal state.
func TestPendingEventDeepCopyIsolation(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	s.AddPlayer(p)

	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomEvent}},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.Phase = PhaseExploring
	s.mu.Unlock()

	s.EnterRoom(0)

	snap1 := s.GetState()
	if snap1.EventVotes == nil {
		t.Fatal("snapshot EventVotes should be a non-nil map after entry")
	}
	snap1.EventVotes["mutator"] = 999

	snap2 := s.GetState()
	if _, leaked := snap2.EventVotes["mutator"]; leaked {
		t.Error("mutation on snapshot must not leak into session state")
	}
}

// enterDeterministicEvent forces a chosen event into the session for tests
// by stashing it directly. Bypasses the random PickRandomEvent path so
// the test can control which choices exist.
func enterDeterministicEvent(t *testing.T, s *GameSession, eventName string) {
	t.Helper()
	var picked *RandomEvent
	for _, ev := range GetRandomEvents() {
		if ev.Name == eventName {
			e := ev
			picked = &e
			break
		}
	}
	if picked == nil {
		t.Fatalf("event %q not found", eventName)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomEvent}},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.LastEventName = picked.Name
	s.pendingEvent = picked
	view := &PendingEventView{
		Name:        picked.Name,
		Description: picked.Description,
		Choices:     make([]EventChoiceView, 0, len(picked.Choices)),
	}
	for _, c := range picked.Choices {
		tagsCopy := make([]ChoiceTag, len(c.Tags))
		copy(tagsCopy, c.Tags)
		view.Choices = append(view.Choices, EventChoiceView{Label: c.Label, Tags: tagsCopy})
	}
	s.state.PendingEvent = view
	s.state.EventVotes = make(map[string]int)
	s.eventVotes = make(map[string]int)
	s.state.Phase = PhaseEvent
}

func TestSubmitEventVote_PartialVotesDoNotResolve(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "merchant")

	if s.SubmitEventVote("fp-a", 0) {
		t.Error("partial vote should not resolve")
	}
	st := s.GetState()
	if st.Phase != PhaseEvent {
		t.Errorf("phase should remain PhaseEvent, got %v", st.Phase)
	}
	if st.Floor.Rooms[0].Cleared {
		t.Error("room must not be cleared on partial vote")
	}
}

func TestSubmitEventVote_MajorityResolvesAndAppliesToParty(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.HP = p.MaxHP - 50 // make room to receive heal
		p.Gold = 100
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "shrine")
	// shrine choice 0 = "무릎 꿇고 기도하기" → heal 30 + floor*2
	beforeHP := make(map[string]int)
	for _, p := range s.state.Players {
		beforeHP[p.Fingerprint] = p.HP
	}

	_ = s.SubmitEventVote("fp-a", 0)
	_ = s.SubmitEventVote("fp-b", 0)
	if !s.SubmitEventVote("fp-c", 1) {
		t.Fatal("3rd vote should trigger resolution")
	}
	st := s.GetState()
	if st.Phase != PhaseExploring {
		t.Errorf("phase should be PhaseExploring after resolve, got %v", st.Phase)
	}
	if !st.Floor.Rooms[0].Cleared {
		t.Error("room should be cleared after resolution")
	}
	for _, p := range st.Players {
		if p.HP <= beforeHP[p.Fingerprint] {
			t.Errorf("player %s HP did not increase (before=%d after=%d)", p.Fingerprint, beforeHP[p.Fingerprint], p.HP)
		}
	}
	if st.PendingEvent != nil || st.EventVotes != nil {
		t.Error("PendingEvent and EventVotes must be cleared after resolve")
	}
}

func TestSubmitEventVote_TieBreakLowestIndex(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c", "fp-d"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.HP = p.MaxHP - 30
		p.Gold = 200
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "merchant")
	// 2v2 tie: idx 0 (heal-for-gold) and idx 1 (gold for mystery item)
	_ = s.SubmitEventVote("fp-a", 1)
	_ = s.SubmitEventVote("fp-b", 0)
	_ = s.SubmitEventVote("fp-c", 1)
	if !s.SubmitEventVote("fp-d", 0) {
		t.Fatal("4th vote should trigger resolution")
	}
	st := s.GetState()
	// Idx 0 won (lower index on tie) → "골드로 치료 거래" → HPChange > 0
	for _, p := range st.Players {
		if p.HP <= p.MaxHP-30 {
			t.Errorf("expected idx 0 (heal) to win tie; player %s HP=%d", p.Fingerprint, p.HP)
		}
	}
}

func TestSubmitEventVote_SoloModeImmediateResolve(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("solo", entity.ClassWarrior)
	p.Fingerprint = "fp-solo"
	p.HP = p.MaxHP - 40
	s.AddPlayer(p)
	enterDeterministicEvent(t, s, "shrine")
	if !s.SubmitEventVote("fp-solo", 0) {
		t.Fatal("solo single vote should resolve immediately")
	}
	if s.GetState().Phase != PhaseExploring {
		t.Error("solo resolve should return to PhaseExploring")
	}
}
