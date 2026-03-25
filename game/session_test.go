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
	case <-time.After(7 * time.Second):
		t.Error("Turn did not timeout within 7 seconds")
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

func TestSendChat(t *testing.T) {
	s := NewGameSession(testCfg(t))
	s.SendChat("Alice", "hello")
	st := s.GetState()
	if len(st.CombatLog) != 1 || st.CombatLog[0] != "[Alice] hello" {
		t.Errorf("expected chat log, got %v", st.CombatLog)
	}
}
