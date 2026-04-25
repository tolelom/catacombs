package game

import (
	"testing"
)

func TestGetWeeklyMutation(t *testing.T) {
	m := GetWeeklyMutation()
	if m.ID == "" {
		t.Error("expected non-empty mutation ID")
	}
	if m.Name == "" {
		t.Error("expected non-empty mutation Name")
	}
	// Verify it returns one of the known mutations
	found := false
	for _, known := range Mutations {
		if known.ID == m.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("mutation ID %q not found in Mutations list", m.ID)
	}
}

func TestEffectiveSkillUsesNoSkills(t *testing.T) {
	cfg := testCfg(t)
	cfg.Game.SkillUses = 3
	s := NewGameSession(cfg)
	s.ActiveMutation = &Mutation{ID: "no_skills"}
	if got := s.effectiveSkillUses(); got != 0 {
		t.Errorf("expected effectiveSkillUses=0 under no_skills, got %d", got)
	}
}

func TestEffectiveSkillUsesUnaffectedByOtherMutations(t *testing.T) {
	cfg := testCfg(t)
	cfg.Game.SkillUses = 3
	s := NewGameSession(cfg)
	s.ActiveMutation = &Mutation{ID: "glass_cannon"}
	if got := s.effectiveSkillUses(); got != 3 {
		t.Errorf("expected effectiveSkillUses=3 under unrelated mutation, got %d", got)
	}
}

func TestEffectiveTurnTimeoutSpeedRun(t *testing.T) {
	cfg := testCfg(t)
	cfg.Game.TurnTimeoutSec = 10
	s := NewGameSession(cfg)
	s.ActiveMutation = &Mutation{ID: "speed_run"}
	if got := s.effectiveTurnTimeout(); got != 5 {
		t.Errorf("expected effectiveTurnTimeout=5 under speed_run, got %d", got)
	}
}

func TestMutationDoesNotMutateSharedConfig(t *testing.T) {
	cfg := testCfg(t)
	cfg.Game.SkillUses = 3
	cfg.Game.TurnTimeoutSec = 10
	s := NewGameSession(cfg)
	s.ActiveMutation = &Mutation{ID: "no_skills"}
	_ = s.effectiveSkillUses()
	if cfg.Game.SkillUses != 3 {
		t.Errorf("shared cfg.SkillUses was mutated: got %d, want 3", cfg.Game.SkillUses)
	}
	s.ActiveMutation = &Mutation{ID: "speed_run"}
	_ = s.effectiveTurnTimeout()
	if cfg.Game.TurnTimeoutSec != 10 {
		t.Errorf("shared cfg.TurnTimeoutSec was mutated: got %d, want 10", cfg.Game.TurnTimeoutSec)
	}
}
