package game

import (
	"fmt"
	"testing"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func testCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, _ := config.Load("")
	return cfg
}

func TestAddPlayerDedup(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p1 := entity.NewPlayer("Alice", entity.ClassWarrior)
	p1.Fingerprint = "fp-alice"
	s.AddPlayer(p1)
	// Simulate rejoin: another Player with the same fingerprint
	p2 := entity.NewPlayer("Alice", entity.ClassMage)
	p2.Fingerprint = "fp-alice"
	s.AddPlayer(p2)

	players := s.GetState().Players
	if len(players) != 1 {
		t.Fatalf("expected 1 player after dedup, got %d", len(players))
	}
	if players[0].Class != entity.ClassWarrior {
		t.Errorf("first add should win; expected Warrior, got %v", players[0].Class)
	}

	// Empty-fingerprint players bypass dedup (legacy/test path)
	anon1 := entity.NewPlayer("Anon", entity.ClassRogue)
	anon2 := entity.NewPlayer("Anon", entity.ClassHealer)
	s.AddPlayer(anon1)
	s.AddPlayer(anon2)
	if got := len(s.GetState().Players); got != 3 {
		t.Errorf("expected 3 players total, got %d", got)
	}
}

func TestGetStateNoRace(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Racer", entity.ClassWarrior)
	p.Fingerprint = "test-fp"
	s.AddPlayer(p)
	s.StartGame()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			st := s.GetState()
			for _, p := range st.Players {
				_ = p.HP
				_ = p.Gold
			}
			for _, m := range st.Monsters {
				_ = m.HP
			}
		}
	}()

	for i := 0; i < 10; i++ {
		select {
		case s.actionCh <- playerActionMsg{PlayerID: "test-fp", Action: PlayerAction{Type: ActionWait}}:
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	<-done
}

func TestSessionTurnTimeout(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("test", entity.ClassWarrior)
	p.Fingerprint = "test-fp"
	s.AddPlayer(p)
	s.StartFloor()

	// Don't submit any action, wait for timeout
	done := make(chan struct{})
	go func() {
		s.RunTurn()
		close(done)
	}()

	select {
	case <-done:
		// Turn completed via timeout
	case <-time.After(12 * time.Second):
		t.Error("Turn did not timeout within 12 seconds")
	}
}

func TestRevealNextLog(t *testing.T) {
	s := NewGameSession(testCfg(t))

	// No logs to reveal
	if s.RevealNextLog() {
		t.Error("should return false when no pending logs")
	}

	// Manually add pending logs
	s.mu.Lock()
	s.state.PendingLogs = []string{"msg1", "msg2", "msg3"}
	s.mu.Unlock()

	if !s.RevealNextLog() {
		t.Error("should return true when log revealed")
	}

	st := s.GetState()
	if len(st.CombatLog) != 1 || st.CombatLog[0] != "msg1" {
		t.Errorf("expected [msg1], got %v", st.CombatLog)
	}
	if len(st.PendingLogs) != 2 {
		t.Errorf("expected 2 pending, got %d", len(st.PendingLogs))
	}

	// Reveal remaining
	s.RevealNextLog()
	s.RevealNextLog()
	if s.RevealNextLog() {
		t.Error("should return false after all revealed")
	}
}

func TestDeepCopyIndependence(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Test", entity.ClassWarrior)
	p.Fingerprint = "fp-test"
	p.Inventory = append(p.Inventory, entity.Item{Name: "Sword", Type: entity.ItemWeapon, Bonus: 5})
	s.AddPlayer(p)

	state := s.GetState()

	// Mutate the copy
	state.Players[0].HP = 999
	state.Players[0].Inventory = append(state.Players[0].Inventory, entity.Item{Name: "Shield"})

	// Original should be unchanged
	origState := s.GetState()
	if origState.Players[0].HP == 999 {
		t.Error("deep copy failed: HP mutation leaked to original")
	}
	if len(origState.Players[0].Inventory) != 1 {
		t.Error("deep copy failed: inventory mutation leaked to original")
	}
}

func TestBuyItemInventoryFull(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Buyer", entity.ClassWarrior)
	p.Fingerprint = "fp-buyer"
	p.Gold = 1000
	// Fill inventory to 10
	for i := 0; i < 10; i++ {
		p.Inventory = append(p.Inventory, entity.Item{Name: "Junk"})
	}
	s.AddPlayer(p)

	s.mu.Lock()
	s.state.Phase = PhaseShop
	s.state.ShopItems = []entity.Item{
		{Name: "Potion", Type: entity.ItemConsumable, Bonus: 30, Price: 10},
	}
	s.mu.Unlock()

	if result := s.BuyItem("fp-buyer", 0); result != BuyInventoryFull {
		t.Errorf("expected BuyInventoryFull, got %d", result)
	}
}

// TestDailyModeDeterministic regresses the daily-challenge determinism bug:
// previously the dungeon layout was seeded but spawnMonsters/triggerEvent and
// combat used package-level rand, so two players running the same daily on
// the same date faced different monsters, item rolls, and event outcomes.
func TestDailyModeDeterministic(t *testing.T) {
	cfg := testCfg(t)
	const date = "2026-04-25"

	runOnce := func() (monsterNames []string, itemBonuses []int, shopBonuses []int) {
		s := NewGameSession(cfg)
		s.DailyMode = true
		s.DailyDate = date
		p := entity.NewPlayer("Daily", entity.ClassWarrior)
		p.Fingerprint = "fp-daily"
		s.AddPlayer(p)
		s.StartFloor()

		// Force the same content path on each run.
		s.mu.Lock()
		s.spawnMonsters()
		for _, m := range s.state.Monsters {
			monsterNames = append(monsterNames, m.Name)
			itemBonuses = append(itemBonuses, m.HP, m.ATK, m.DEF)
		}
		s.generateShopItems()
		for _, it := range s.state.ShopItems {
			shopBonuses = append(shopBonuses, it.Bonus, it.Price)
		}
		s.mu.Unlock()
		return
	}

	a, b, c := runOnce()
	d, e, f := runOnce()
	if !equalStrings(a, d) {
		t.Errorf("monster names differ across runs: %v vs %v", a, d)
	}
	if !equalInts(b, e) {
		t.Errorf("monster stats differ across runs: %v vs %v", b, e)
	}
	if !equalInts(c, f) {
		t.Errorf("shop bonuses differ across runs: %v vs %v", c, f)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestStartGameCapturesPartySize ensures full_party achievement input is
// frozen at game start, so an inactivity-removed teammate mid-run cannot
// cost the surviving party the achievement.
func TestStartGameCapturesPartySize(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, name := range []string{"A", "B", "C", "D"} {
		p := entity.NewPlayer(name, entity.ClassWarrior)
		p.Fingerprint = "fp-" + name
		s.AddPlayer(p)
	}
	s.StartGame()
	defer s.Stop()
	if got := s.GetState().StartingPartySize; got != 4 {
		t.Errorf("StartingPartySize: got %d, want 4", got)
	}
}

// TestAnyPlayerDiedSticky regresses the no_death achievement: once a
// player has died this run, the flag must stay true regardless of revival
// on the next floor.
func TestAnyPlayerDiedSticky(t *testing.T) {
	s := NewGameSession(testCfg(t))
	if s.GetState().AnyPlayerDied {
		t.Error("fresh session should report AnyPlayerDied=false")
	}
	s.mu.Lock()
	s.state.AnyPlayerDied = true
	s.mu.Unlock()
	if !s.GetState().AnyPlayerDied {
		t.Error("AnyPlayerDied flag should be persisted via GetState")
	}
}

// TestDailyCrossSessionDeterminism verifies that two independently-created
// sessions with the same daily date produce the same monsters and shop items
// across multiple floors. Regression: previously only the dungeon layout was
// seeded; spawn/combat/event used package-level rand and diverged.
func TestDailyCrossSessionDeterminism(t *testing.T) {
	cfg := testCfg(t)
	const date = "2026-04-25"

	collect := func() []string {
		s := NewGameSession(cfg)
		s.DailyMode = true
		s.DailyDate = date
		p := entity.NewPlayer("Daily", entity.ClassWarrior)
		p.Fingerprint = "fp"
		s.AddPlayer(p)
		var seq []string
		for floor := 1; floor <= 3; floor++ {
			s.mu.Lock()
			s.state.FloorNum = floor
			s.mu.Unlock()
			s.StartFloor()
			s.mu.Lock()
			s.spawnMonsters()
			for _, m := range s.state.Monsters {
				seq = append(seq, fmt.Sprintf("F%d:%s/HP=%d/ATK=%d", floor, m.Name, m.HP, m.ATK))
			}
			s.generateShopItems()
			for _, it := range s.state.ShopItems {
				seq = append(seq, fmt.Sprintf("F%d:shop:%s/B=%d/P=%d", floor, it.Name, it.Bonus, it.Price))
			}
			s.mu.Unlock()
		}
		return seq
	}

	a := collect()
	b := collect()
	if len(a) != len(b) {
		t.Fatalf("sequence length differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("step %d differs: %q vs %q", i, a[i], b[i])
		}
	}
}

// TestMultiplayerFlowEndToEnd exercises the full lobby-to-game-to-quit path
// for a 2-player party. It pins the session-lifetime fixes from prior
// rounds (LeaveSession keeping the session alive for the survivor; the
// last leaver tearing down room + session).
func TestMultiplayerFlowEndToEnd(t *testing.T) {
	cfg := testCfg(t)
	l := NewLobby(cfg)
	code := l.CreateRoom("E2E")
	if err := l.JoinRoom(code, "A", "fp-a"); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if err := l.JoinRoom(code, "B", "fp-b"); err != nil {
		t.Fatalf("join B: %v", err)
	}
	session := l.EnsureSession(code, func() *GameSession { return NewGameSession(cfg) })
	if session == nil {
		t.Fatal("EnsureSession returned nil")
	}
	session.AddPlayer(newPlayerWithFP("A", "fp-a"))
	session.AddPlayer(newPlayerWithFP("B", "fp-b"))
	session.StartGame()
	defer session.Stop()

	if got := session.GetState().StartingPartySize; got != 2 {
		t.Errorf("StartingPartySize: got %d, want 2", got)
	}

	// A leaves mid-run.
	if rem := l.LeaveSession(code, "fp-a"); rem != 1 {
		t.Errorf("after A leaves: remaining=%d, want 1", rem)
	}
	if l.GetRoom(code) == nil {
		t.Fatal("room should persist while B is still playing")
	}

	// GameOver flag is captured by AnyPlayerDied if anyone died — we didn't
	// kill anyone, so the survivor flag stays false.
	if session.GetState().AnyPlayerDied {
		t.Error("AnyPlayerDied should be false in a clean run")
	}

	// B leaves; this should tear everything down.
	if rem := l.LeaveSession(code, "fp-b"); rem != 0 {
		t.Errorf("after B leaves: remaining=%d, want 0", rem)
	}
	if l.GetRoom(code) != nil {
		t.Error("room should be removed after last player leaves")
	}
	select {
	case <-session.done:
	default:
		t.Error("session should be stopped after last player leaves")
	}
}

// TestStopAllSessionsDuringShutdown simulates the graceful-shutdown path:
// active sessions across multiple rooms must all see their done channel
// closed when main calls Lobby.StopAllSessions.
func TestStopAllSessionsDuringShutdown(t *testing.T) {
	cfg := testCfg(t)
	l := NewLobby(cfg)
	codes := []string{l.CreateRoom("R1"), l.CreateRoom("R2"), l.CreateRoom("R3")}
	sessions := make([]*GameSession, len(codes))
	for i, c := range codes {
		sessions[i] = l.EnsureSession(c, func() *GameSession { return NewGameSession(cfg) })
		sessions[i].AddPlayer(newPlayerWithFP("P", "fp-"+c))
		sessions[i].StartGame()
	}

	l.StopAllSessions()
	for i, s := range sessions {
		select {
		case <-s.done:
		default:
			t.Errorf("session %d (%s) was not stopped", i, codes[i])
		}
	}
}

// BenchmarkGetState measures the per-call cost of the deep-copy snapshot.
// GetState runs on every UI tick (~5 Hz combat, slower elsewhere) for every
// connected client, so its allocation profile dominates idle CPU.
func BenchmarkGetState(b *testing.B) {
	s := NewGameSession(testCfg(&testing.T{}))
	for _, name := range []string{"A", "B", "C", "D"} {
		p := entity.NewPlayer(name, entity.ClassWarrior)
		p.Fingerprint = "fp-" + name
		for i := 0; i < 5; i++ {
			p.Inventory = append(p.Inventory, entity.Item{Name: "Iron Sword", Type: entity.ItemWeapon, Bonus: 5})
		}
		s.AddPlayer(p)
	}
	s.StartFloor()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.GetState()
	}
}

func TestSendChat(t *testing.T) {
	s := NewGameSession(testCfg(t))
	s.SendChat("Alice", "hello")
	st := s.GetState()
	if len(st.CombatLog) != 1 || st.CombatLog[0] != "[Alice] hello" {
		t.Errorf("expected chat log, got %v", st.CombatLog)
	}
}

func TestRemovePlayer(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	if got := s.RemovePlayer("fp-a"); got != 1 {
		t.Errorf("expected 1 remaining after dropping A, got %d", got)
	}
	players := s.GetState().Players
	if len(players) != 1 || players[0].Fingerprint != "fp-b" {
		t.Errorf("unexpected player list after remove: %+v", players)
	}
	if got := s.RemovePlayer("fp-b"); got != 0 {
		t.Errorf("expected 0 remaining after dropping B, got %d", got)
	}
	if got := s.RemovePlayer("fp-missing"); got != -1 {
		t.Errorf("expected -1 for unknown fingerprint, got %d", got)
	}
}

func TestBuyItemRejectsSoldNonConsumable(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Buyer", entity.ClassWarrior)
	p.Fingerprint = "fp-buyer"
	p.Gold = 10000
	s.AddPlayer(p)

	s.mu.Lock()
	s.state.Phase = PhaseShop
	s.state.ShopItems = []entity.Item{
		{Name: "HP Potion", Type: entity.ItemConsumable, Bonus: 30, Price: 10},
		{Name: "Iron Sword", Type: entity.ItemWeapon, Bonus: 5, Price: 50},
	}
	s.state.ShopSold = make([]bool, len(s.state.ShopItems))
	s.mu.Unlock()

	if r := s.BuyItem("fp-buyer", 1); r != BuyOK {
		t.Fatalf("first weapon buy should succeed, got %d", r)
	}
	if r := s.BuyItem("fp-buyer", 1); r != BuyFailed {
		t.Errorf("second weapon buy should fail (sold), got %d", r)
	}
	// Consumable can be bought again
	if r := s.BuyItem("fp-buyer", 0); r != BuyOK {
		t.Errorf("first potion buy should succeed, got %d", r)
	}
	if r := s.BuyItem("fp-buyer", 0); r != BuyOK {
		t.Errorf("second potion buy should succeed (consumable restocks), got %d", r)
	}
}

func TestLeaveShopWaitsForAllPlayers(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)
	s.mu.Lock()
	s.state.Phase = PhaseShop
	s.state.Floor = nil
	s.mu.Unlock()

	if s.LeaveShop("fp-a") {
		t.Error("LeaveShop should not transition while other players are still browsing")
	}
	if s.GetState().Phase != PhaseShop {
		t.Errorf("phase should remain PhaseShop, got %v", s.GetState().Phase)
	}
	if !s.LeaveShop("fp-b") {
		t.Error("LeaveShop should transition once everyone is ready")
	}
	if s.GetState().Phase != PhaseExploring {
		t.Errorf("phase should be PhaseExploring after all leave, got %v", s.GetState().Phase)
	}
}

// TestLeaveShopExposesLeaversForUI verifies the shop ready-counter is
// visible to the UI via GetState so the polling client can render
// "N/M ready to leave".
func TestLeaveShopExposesLeaversForUI(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		s.AddPlayer(p)
	}
	s.mu.Lock()
	s.state.Phase = PhaseShop
	s.state.Floor = nil
	s.mu.Unlock()

	if got := s.GetState().ShopLeavers; got != nil {
		t.Errorf("ShopLeavers should be nil before any q, got %v", got)
	}
	s.LeaveShop("fp-a")
	leavers := s.GetState().ShopLeavers
	if !leavers["fp-a"] || leavers["fp-b"] {
		t.Errorf("expected only fp-a to be a leaver, got %v", leavers)
	}
	s.LeaveShop("fp-b")
	if l := s.GetState().ShopLeavers; !l["fp-a"] || !l["fp-b"] || l["fp-c"] {
		t.Errorf("expected fp-a + fp-b leavers, got %v", l)
	}
}

func TestLeaveShopIgnoresFledPlayers(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	b.Fled = true
	s.AddPlayer(a)
	s.AddPlayer(b)
	s.mu.Lock()
	s.state.Phase = PhaseShop
	s.state.Floor = nil
	s.mu.Unlock()

	// Only A is available; A's leave should be sufficient.
	if !s.LeaveShop("fp-a") {
		t.Error("LeaveShop should transition when only A is available (B is fled)")
	}
}

func TestAllocateSkillPointAppliesMaxHPBoost(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("W", entity.ClassWarrior)
	p.Fingerprint = "fp-w"
	s.AddPlayer(p)

	// Tank branch T1 is auto and grants Iron Skin (DEF+3) only — no MaxHP/HP
	// delta should apply here. T2/T4 fork plumbing is covered by the
	// separate TestAllocateSkillPoint_ForwardsChoiceToEntity test below.
	s.mu.Lock()
	for _, pl := range s.state.Players {
		pl.Skills.Points = 1
	}
	beforeMax := s.state.Players[0].MaxHP
	beforeHP := s.state.Players[0].HP
	s.mu.Unlock()

	if err := s.AllocateSkillPoint("fp-w", 0, 0); err != nil {
		t.Fatalf("allocate T1 failed: %v", err)
	}
	st := s.GetState()
	// Iron Skin is DEF-only; MaxHP and HP must not change at T1.
	if got := st.Players[0].MaxHP; got != beforeMax {
		t.Errorf("expected MaxHP unchanged after T1 DEF node, got %d (was %d)", got, beforeMax)
	}
	if got := st.Players[0].HP; got != beforeHP {
		t.Errorf("expected HP unchanged after T1 DEF node, got %d (was %d)", got, beforeHP)
	}
	// Branch must be locked to Tank (0).
	if got := st.Players[0].Skills.BranchIndex; got != 0 {
		t.Errorf("expected BranchIndex 0 after T1, got %d", got)
	}
}

// TestRunTurnUnblocksOnPlayerLeave regresses the mid-turn-quit stall:
// previously RunTurn captured aliveCount once and then waited on actionCh
// for an action that the leaver would never send, freezing the party for
// the full TurnTimeoutSec. RemovePlayer now wakes playerLeftCh and the
// collect loop re-evaluates the alive set.
func TestRunTurnUnblocksOnPlayerLeave(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Alice", entity.ClassWarrior)
	p.Fingerprint = "fp-alice"
	s.AddPlayer(p)
	s.StartFloor()

	done := make(chan struct{})
	go func() {
		s.RunTurn()
		close(done)
	}()

	// Drop the only player mid-turn before any action arrives.
	time.Sleep(50 * time.Millisecond)
	if rem := s.RemovePlayer("fp-alice"); rem != 0 {
		t.Fatalf("expected 0 players remaining, got %d", rem)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunTurn did not unblock within 2s after RemovePlayer")
	}
}

func TestRemovePlayerClearsShopLeavers(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Alice", entity.ClassWarrior)
	p.Fingerprint = "fp-alice"
	s.AddPlayer(p)

	s.mu.Lock()
	s.shopLeavers = map[string]bool{"fp-alice": true}
	s.mu.Unlock()

	s.RemovePlayer("fp-alice")

	s.mu.Lock()
	_, exists := s.shopLeavers["fp-alice"]
	s.mu.Unlock()
	if exists {
		t.Error("RemovePlayer should clear the leaving player's shopLeavers entry")
	}
}

// TestAllocateSkillPoint_ForwardsChoiceToEntity ensures a Bravo choice at
// T2 records 'B' in the player's Skills.Choices, proving the session
// faithfully forwards the choice byte to the entity layer.
func TestAllocateSkillPoint_ForwardsChoiceToEntity(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Skills = entity.NewPlayerSkills()
	p.Skills.Points = 2
	s.AddPlayer(p)

	if err := s.AllocateSkillPoint("fp-a", 0, 0); err != nil {
		t.Fatalf("T1 allocation should succeed, got %v", err)
	}
	if err := s.AllocateSkillPoint("fp-a", 0, 'B'); err != nil {
		t.Fatalf("T2 allocation with choice 'B' should succeed, got %v", err)
	}
	got := s.GetState().Players[0].Skills.Choices
	if got != "B" {
		t.Errorf("expected Choices 'B' after T2 with choice B, got %q", got)
	}
}

// TestAllocateSkillPoint_T3AcceptsAnyBranchIdxArg simulates the UI sending
// either branchIdx 0 or 1 at the T3 auto stage. With the fix in
// ui/game_view.go that always preserves the locked branch index, a
// well-behaved UI never passes a mismatched branchIdx here, but the test
// guards the session-level contract: T3 advancement only works when
// branchIdx matches the locked BranchIndex.
//
// This prevents regressions of the bug fixed alongside this test, where
// the UI fell back to deriving branchIdx from the key at T3 and silently
// generated a "cannot switch branch" error.
func TestAllocateSkillPoint_T3AcceptsAnyBranchIdxArg(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Skills = entity.NewPlayerSkills()
	p.Skills.Points = 3
	s.AddPlayer(p)

	// Lock Tank (branch 0) via T1, then commit T2A.
	if err := s.AllocateSkillPoint("fp-a", 0, 0); err != nil {
		t.Fatalf("T1: %v", err)
	}
	if err := s.AllocateSkillPoint("fp-a", 0, 'A'); err != nil {
		t.Fatalf("T2A: %v", err)
	}

	// T3 with the LOCKED branch index — must succeed.
	if err := s.AllocateSkillPoint("fp-a", 0, 0); err != nil {
		t.Fatalf("T3 with locked branchIdx=0 must succeed, got: %v", err)
	}

	// Sanity: passing the wrong branchIdx at T3 still errors (defines the
	// session-level contract the UI fix relies on).
	p.Skills.Points = 4
	if err := s.AllocateSkillPoint("fp-a", 1, 0); err == nil {
		t.Error("T4 with mismatched branchIdx=1 must error")
	}
}

// TestRemovePlayerResolvesMoveVote — a 2-player party in PhaseExploring
// where A has voted and B disconnects must auto-advance instead of
// stalling forever waiting for B's vote.
func TestRemovePlayerResolvesMoveVote(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassWarrior)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	// Hand-build a two-room floor (room 0 = current, room 1 = empty target).
	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms: []*dungeon.Room{
			{Type: dungeon.RoomEmpty, Cleared: true},
			{Type: dungeon.RoomEmpty, Neighbors: []int{0}},
		},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.Phase = PhaseExploring
	s.mu.Unlock()

	// A votes for room 1; B does not vote.
	if s.SubmitMoveVote("fp-a", 1) {
		t.Fatal("partial vote should not resolve")
	}
	if s.GetState().Phase != PhaseExploring {
		t.Fatal("phase should remain PhaseExploring before B leaves")
	}

	// B disconnects — quorum should auto-recheck and advance the party.
	s.RemovePlayer("fp-b")

	st := s.GetState()
	if st.Floor.CurrentRoom != 1 {
		t.Errorf("expected CurrentRoom=1 after auto-advance, got %d", st.Floor.CurrentRoom)
	}
}

// TestRemovePlayerResolvesShopLeave — a 2-player party in PhaseShop where
// A has pressed leave and B drops must flip back to PhaseExploring.
func TestRemovePlayerResolvesShopLeave(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomShop}},
		CurrentRoom: 0,
	}
	s.state.Phase = PhaseShop
	s.mu.Unlock()

	if s.LeaveShop("fp-a") {
		t.Fatal("LeaveShop should not transition while B is still browsing")
	}
	s.RemovePlayer("fp-b")
	if got := s.GetState().Phase; got != PhaseExploring {
		t.Errorf("phase should be PhaseExploring after B leaves, got %v", got)
	}
}

// TestRemovePlayerResolvesEventVote — a 2-player party in PhaseEvent where
// A has voted and B drops must resolve the vote and clear PendingEvent
// without requiring a redundant re-submit.
func TestRemovePlayerResolvesEventVote(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	a.HP = a.MaxHP - 10
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	b.HP = b.MaxHP - 10
	s.AddPlayer(a)
	s.AddPlayer(b)
	enterDeterministicEvent(t, s, "shrine")

	if s.SubmitEventVote("fp-a", 0) {
		t.Fatal("partial event vote should not resolve")
	}
	s.RemovePlayer("fp-b")

	st := s.GetState()
	if st.Phase != PhaseExploring {
		t.Errorf("phase should be PhaseExploring after auto-resolve, got %v", st.Phase)
	}
	if st.PendingEvent != nil {
		t.Error("PendingEvent must be cleared after auto-resolve")
	}
}

// TestLeaveShopEmptyFingerprintCannotDeadlock — a party containing an
// empty-fingerprint player (legacy/test path) must still be able to leave the
// shop: the "" player signals under the "" key like move votes do, instead of
// being silently untrackable and blocking quorum forever.
func TestLeaveShopEmptyFingerprintCannotDeadlock(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	anon := entity.NewPlayer("Anon", entity.ClassMage) // Fingerprint == ""
	s.AddPlayer(a)
	s.AddPlayer(anon)
	s.mu.Lock()
	s.state.Phase = PhaseShop
	s.state.Floor = nil
	s.mu.Unlock()

	if s.LeaveShop("fp-a") {
		t.Error("should still wait for the anon player")
	}
	if !s.LeaveShop("") {
		t.Error("anon leave should complete the quorum and transition")
	}
	if s.GetState().Phase != PhaseExploring {
		t.Errorf("phase should be PhaseExploring, got %v", s.GetState().Phase)
	}
}
