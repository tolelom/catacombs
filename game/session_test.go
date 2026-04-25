package game

import (
	"testing"
	"time"

	"github.com/tolelom/catacombs/config"
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

	// Tank branch order: Iron Skin (DEF), Fortitude (MaxHP+20), Bastion (DEF)
	s.mu.Lock()
	for _, pl := range s.state.Players {
		pl.Skills.Points = 3
	}
	beforeMax := s.state.Players[0].MaxHP
	beforeHP := s.state.Players[0].HP
	s.mu.Unlock()

	if err := s.AllocateSkillPoint("fp-w", 0); err != nil {
		t.Fatalf("allocate 1 failed: %v", err)
	}
	if err := s.AllocateSkillPoint("fp-w", 0); err != nil {
		t.Fatalf("allocate 2 failed: %v", err)
	}
	st := s.GetState()
	if got := st.Players[0].MaxHP; got != beforeMax+20 {
		t.Errorf("expected MaxHP +20 from Fortitude, got %d (was %d)", got, beforeMax)
	}
	if got := st.Players[0].HP; got != beforeHP+20 {
		t.Errorf("expected HP +20 from Fortitude, got %d (was %d)", got, beforeHP)
	}
}
