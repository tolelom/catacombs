package game

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func TestEarlyThreatMultShape(t *testing.T) {
	s := NewGameSession(testCfg(t)) // EarlyThreatRamp 0.08

	// Onboarding floors and the fade floor are untouched.
	for _, f := range []int{1, 2, 10, 15, 20} {
		if got := s.earlyThreatMult(f); got != 1.0 {
			t.Errorf("floor %d should have no early ramp, got %.3f", f, got)
		}
	}
	// Symmetric bump peaking at floor 6 (steps = min(floor-2, 10-floor) = 4).
	if got := s.earlyThreatMult(6); got <= 1.0 {
		t.Errorf("floor 6 should be lifted, got %.3f", got)
	}
	if s.earlyThreatMult(6) <= s.earlyThreatMult(3) {
		t.Error("peak (floor 6) should exceed an early ramp floor (3)")
	}
	if s.earlyThreatMult(9) >= s.earlyThreatMult(6) {
		t.Error("ramp should taper from peak (6) toward fade floor (9 < 6 value)")
	}
}

func TestEarlyThreatMultDisabled(t *testing.T) {
	cfg := testCfg(t)
	cfg.Combat.EarlyThreatRamp = 0 // explicit opt-out
	s := NewGameSession(cfg)
	for f := 1; f <= 20; f++ {
		if got := s.earlyThreatMult(f); got != 1.0 {
			t.Errorf("ramp disabled: floor %d should be 1.0, got %.3f", f, got)
		}
	}
}

func TestApplyEarlyThreatLiftsHPandATKNotDEF(t *testing.T) {
	s := NewGameSession(testCfg(t))
	m := &entity.Monster{Name: "Orc", HP: 100, MaxHP: 100, ATK: 20, DEF: 10}
	s.applyEarlyThreat(m, 6) // peak floor

	if m.HP <= 100 || m.MaxHP <= 100 {
		t.Errorf("early threat should lift HP, got HP=%d MaxHP=%d", m.HP, m.MaxHP)
	}
	if m.ATK <= 20 {
		t.Errorf("early threat should lift ATK, got %d", m.ATK)
	}
	if m.DEF != 10 {
		t.Errorf("early threat must NOT touch DEF, got %d", m.DEF)
	}
}
