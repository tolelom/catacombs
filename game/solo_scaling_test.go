package game

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func TestApplySoloScalingReducesStats(t *testing.T) {
	s := NewGameSession(testCfg(t)) // defaults: HP/DEF x0.50, ATK x0.75
	s.state.SoloMode = true

	m := &entity.Monster{Name: "Test", HP: 100, MaxHP: 100, ATK: 20, DEF: 10}
	s.applySoloScaling(m)

	if m.HP != 50 || m.MaxHP != 50 {
		t.Errorf("solo HP should halve to 50, got HP=%d MaxHP=%d", m.HP, m.MaxHP)
	}
	if m.DEF != 5 {
		t.Errorf("solo DEF should halve to 5, got %d", m.DEF)
	}
	if m.ATK != 15 { // 20 * 0.75
		t.Errorf("solo ATK should reduce to 15, got %d", m.ATK)
	}
}

func TestApplySoloScalingNoOpInParty(t *testing.T) {
	s := NewGameSession(testCfg(t))
	s.state.SoloMode = false

	m := &entity.Monster{Name: "Test", HP: 100, MaxHP: 100, ATK: 20, DEF: 10}
	s.applySoloScaling(m)

	if m.HP != 100 || m.DEF != 10 || m.ATK != 20 {
		t.Errorf("party mode must not alter stats, got HP=%d DEF=%d ATK=%d", m.HP, m.DEF, m.ATK)
	}
}

func TestApplySoloScalingFloorsAtOne(t *testing.T) {
	s := NewGameSession(testCfg(t))
	s.state.SoloMode = true

	m := &entity.Monster{Name: "Weak", HP: 1, MaxHP: 1, ATK: 1, DEF: 0}
	s.applySoloScaling(m)

	if m.HP < 1 || m.ATK < 1 {
		t.Errorf("HP and ATK must floor at 1, got HP=%d ATK=%d", m.HP, m.ATK)
	}
}
