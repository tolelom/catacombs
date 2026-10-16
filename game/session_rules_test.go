package game

import (
	"math/rand"
	"testing"
	"time"

	"github.com/tolelom/catacombs/entity"
)

func addPlayers(s *GameSession, hp int, fps ...string) {
	for _, fp := range fps {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		if hp > 0 {
			p.HP = hp
		}
		s.AddPlayer(p)
	}
}

func TestEventDamageThatWipesThePartyEndsTheRun(t *testing.T) {
	s := NewGameSession(testCfg(t))
	addPlayers(s, 1, "fp-solo")
	enterDeterministicEvent(t, s, "trap_room")

	s.SubmitEventVote("fp-solo", 1)

	st := s.GetState()
	if !st.GameOver || st.Phase != PhaseResult {
		t.Fatalf("party wiped by an event but run continues: gameOver=%v phase=%v", st.GameOver, st.Phase)
	}
}

func TestPerPlayerEventDamageThatWipesThePartyEndsTheRun(t *testing.T) {
	s := NewGameSession(testCfg(t))
	addPlayers(s, 1, "fp-a", "fp-b")
	enterDeterministicEvent(t, s, "trap_room")
	s.mu.Lock()
	s.eventPerPlayer = true
	s.mu.Unlock()

	s.SubmitEventVote("fp-a", 1)
	s.SubmitEventVote("fp-b", 1)

	st := s.GetState()
	if !st.GameOver || st.Phase != PhaseResult {
		t.Fatalf("party wiped by per-player picks but run continues: gameOver=%v phase=%v", st.GameOver, st.Phase)
	}
}

func TestEventDamageWithSurvivorsKeepsExploring(t *testing.T) {
	s := NewGameSession(testCfg(t))
	addPlayers(s, 0, "fp-solo")
	enterDeterministicEvent(t, s, "trap_room")

	s.SubmitEventVote("fp-solo", 1)

	st := s.GetState()
	if st.GameOver || st.Phase != PhaseExploring {
		t.Fatalf("survivor's run ended: gameOver=%v phase=%v", st.GameOver, st.Phase)
	}
}

func TestActionFromNonMemberIsRejected(t *testing.T) {
	s := NewGameSession(testCfg(t))
	addPlayers(s, 0, "fp-a")

	s.SubmitAction("fp-ghost", PlayerAction{Type: ActionAttack})

	if _, ok := s.GetState().SubmittedActions["fp-ghost"]; ok {
		t.Fatal("non-member action was recorded as submitted")
	}
	if n := len(s.actionCh); n != 0 {
		t.Fatalf("non-member action reached the turn queue (%d queued)", n)
	}
}

// startCombatTurn runs RunTurn against a sturdy monster and returns a channel
// closed when the turn finishes collecting and resolving.
func startCombatTurn(t *testing.T, s *GameSession) <-chan struct{} {
	t.Helper()
	s.mu.Lock()
	s.state.Phase = PhaseCombat
	m := entity.NewMonster(entity.MonsterSlime, 1, 1.0)
	m.HP, m.MaxHP = 100000, 100000
	s.state.Monsters = []*entity.Monster{m}
	s.mu.Unlock()
	finished := make(chan struct{})
	go func() {
		s.RunTurn()
		close(finished)
	}()
	t.Cleanup(s.Stop)
	return finished
}

// waitTurnOpen blocks until RunTurn has started collecting for the new turn.
func waitTurnOpen(t *testing.T, s *GameSession) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for s.GetState().TurnDeadline.IsZero() {
		if time.Now().After(deadline) {
			t.Fatal("turn never opened")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func nextTurnMsg(s *GameSession, fp string) playerActionMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	return playerActionMsg{PlayerID: fp, Action: PlayerAction{Type: ActionWait}, Turn: s.state.TurnNum + 1}
}

func TestTurnWaitsForMembersEvenIfAStrangerActs(t *testing.T) {
	cfg := testCfg(t)
	cfg.Game.TurnTimeoutSec = 5
	s := NewGameSession(cfg)
	addPlayers(s, 0, "fp-a", "fp-b")
	ghost := nextTurnMsg(s, "fp-ghost")
	finished := startCombatTurn(t, s)
	waitTurnOpen(t, s)

	s.actionCh <- ghost
	s.SubmitAction("fp-a", PlayerAction{Type: ActionWait})

	select {
	case <-finished:
		t.Fatal("turn resolved before fp-b acted because a stranger's action was counted")
	case <-time.After(300 * time.Millisecond):
	}
	s.SubmitAction("fp-b", PlayerAction{Type: ActionWait})
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("turn did not resolve after every member acted")
	}
}

func TestActionsQueuedAsTheTurnStartsAreKept(t *testing.T) {
	cfg := testCfg(t)
	cfg.Game.TurnTimeoutSec = 5
	s := NewGameSession(cfg)
	addPlayers(s, 0, "fp-a", "fp-b")
	s.actionCh <- nextTurnMsg(s, "fp-a")
	s.actionCh <- nextTurnMsg(s, "fp-b")

	finished := startCombatTurn(t, s)

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("actions sent for this turn were dropped; turn waited for the timeout")
	}
}

func TestStaleActionsFromAnEarlierTurnAreIgnored(t *testing.T) {
	cfg := testCfg(t)
	cfg.Game.TurnTimeoutSec = 5
	s := NewGameSession(cfg)
	addPlayers(s, 0, "fp-a")
	stale := nextTurnMsg(s, "fp-a")
	stale.Turn--
	s.actionCh <- stale

	finished := startCombatTurn(t, s)

	select {
	case <-finished:
		t.Fatal("an action from an earlier turn resolved this turn")
	case <-time.After(300 * time.Millisecond):
	}
}

func setClock(s *GameSession, now time.Time) {
	s.mu.Lock()
	s.clock = func() time.Time { return now }
	s.mu.Unlock()
}

func sweep(s *GameSession) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepDisconnectedLocked(s.clock())
	return len(s.state.Players)
}

func TestConnectedPlayerWithoutInputIsKept(t *testing.T) {
	s := NewGameSession(testCfg(t))
	addPlayers(s, 0, "fp-a")
	start := time.Now()
	setClock(s, start)
	setClock(s, start.Add(10*time.Minute))

	if n := sweep(s); n != 1 {
		t.Fatalf("connected but idle player was removed (players=%d)", n)
	}
}

func TestDisconnectedPlayerIsRemovedAfterGrace(t *testing.T) {
	s := NewGameSession(testCfg(t))
	addPlayers(s, 0, "fp-a", "fp-b")
	start := time.Now()
	setClock(s, start)
	s.MarkDisconnected("fp-a")

	setClock(s, start.Add(50*time.Second))
	if n := sweep(s); n != 2 {
		t.Fatalf("player removed inside the reconnect grace (players=%d)", n)
	}
	setClock(s, start.Add(61*time.Second))
	if n := sweep(s); n != 1 {
		t.Fatalf("disconnected player kept past the grace (players=%d)", n)
	}
}

func TestReconnectWithinGraceKeepsPlayer(t *testing.T) {
	s := NewGameSession(testCfg(t))
	addPlayers(s, 0, "fp-a")
	start := time.Now()
	setClock(s, start)
	s.MarkDisconnected("fp-a")
	setClock(s, start.Add(50*time.Second))
	s.MarkConnected("fp-a")

	setClock(s, start.Add(5*time.Minute))
	if n := sweep(s); n != 1 {
		t.Fatalf("reconnected player was removed later (players=%d)", n)
	}
}

func TestPickCurseWithoutRNGIsNotFixed(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		seen[PickCurse(nil).ID] = true
	}
	if len(seen) < 2 {
		t.Fatalf("PickCurse(nil) always returned %v", seen)
	}
}

func TestPickCurseWithRNGIsDeterministic(t *testing.T) {
	a := PickCurse(rand.New(rand.NewSource(7)))
	b := PickCurse(rand.New(rand.NewSource(7)))
	if a.ID != b.ID {
		t.Fatalf("same seed gave %q and %q", a.ID, b.ID)
	}
}

func TestLogRevealIsPacedRegardlessOfCallers(t *testing.T) {
	s := NewGameSession(testCfg(t))
	start := time.Now()
	setClock(s, start)
	s.mu.Lock()
	s.state.PendingLogs = []string{"one", "two", "three"}
	s.mu.Unlock()

	first := s.RevealNextLog()
	second := s.RevealNextLog()
	if !first || second {
		t.Fatalf("two immediate reveals returned %v, %v; want true, false", first, second)
	}
	setClock(s, start.Add(logRevealInterval))
	if !s.RevealNextLog() {
		t.Fatal("reveal after the interval did not advance")
	}
	if got := len(s.GetState().PendingLogs); got != 1 {
		t.Fatalf("pending logs left: got %d, want 1", got)
	}
}

func TestPrivateRoomIsHiddenAndClosedToOthers(t *testing.T) {
	l := NewLobby(testConfig(t))
	code, err := l.CreatePrivateRoom("fp-owner", "daily")
	if err != nil {
		t.Fatalf("create private room: %v", err)
	}

	for _, r := range l.ListRoomSnapshots() {
		if r.Code == code {
			t.Fatal("private room is listed in the lobby")
		}
	}
	if err := l.JoinRoom(code, "intruder", "fp-other"); err == nil {
		t.Fatal("another player joined a private room by code")
	}
	if err := l.JoinRoom(code, "owner", "fp-owner"); err != nil {
		t.Fatalf("owner could not join their private room: %v", err)
	}
}
