package entity

import "testing"

func TestCheckPhaseTransitionTriggersBelow50Pct(t *testing.T) {
	m := NewMonster(MonsterBoss5, 5, 1.15)
	if m.Phase != 1 {
		t.Errorf("new boss should start at Phase 1, got %d", m.Phase)
	}
	// Damage below 50%
	m.TakeDamage(m.MaxHP/2 + 1)
	if !m.CheckPhaseTransition() {
		t.Error("expected phase transition when HP < 50%")
	}
	if m.Phase != 2 {
		t.Errorf("Phase should be 2 after transition, got %d", m.Phase)
	}
	if !m.PhaseTransitioned {
		t.Error("PhaseTransitioned should be true")
	}
	// Second call returns false (only fires once)
	if m.CheckPhaseTransition() {
		t.Error("transition should fire only once per encounter")
	}
}

func TestCheckPhaseTransitionAbove50PctNoTrigger(t *testing.T) {
	m := NewMonster(MonsterBoss5, 5, 1.15)
	m.TakeDamage(10) // small damage, still > 50%
	if m.CheckPhaseTransition() {
		t.Error("phase transition should not fire above 50% HP")
	}
}

func TestCheckPhaseTransitionNonBossNoTrigger(t *testing.T) {
	m := NewMonster(MonsterSlime, 1, 1.15)
	m.TakeDamage(m.MaxHP - 1)
	if m.CheckPhaseTransition() {
		t.Error("non-boss should not trigger phase transition")
	}
}

func TestVulnerableFlagSet(t *testing.T) {
	m := NewMonster(MonsterBoss15, 15, 1.15)
	if m.Vulnerable {
		t.Error("Vulnerable should default to false")
	}
}
