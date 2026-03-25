package game

import (
	"testing"

	"github.com/tolelom/catacombs/config"
)

func TestGetWeeklyMutation(t *testing.T) {
	m := GetWeeklyMutation()
	if m.ID == "" {
		t.Error("expected non-empty mutation ID")
	}
	if m.Name == "" {
		t.Error("expected non-empty mutation Name")
	}
	if m.Apply == nil {
		t.Error("expected non-nil Apply function")
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

func TestMutationApplyNoSkills(t *testing.T) {
	cfg := config.GameConfig{SkillUses: 3}
	// Find the no_skills mutation
	var m Mutation
	for _, mut := range Mutations {
		if mut.ID == "no_skills" {
			m = mut
			break
		}
	}
	if m.ID == "" {
		t.Fatal("no_skills mutation not found")
	}
	m.Apply(&cfg)
	if cfg.SkillUses != 0 {
		t.Errorf("expected SkillUses=0 after no_skills mutation, got %d", cfg.SkillUses)
	}
}

func TestMutationApplySpeedRun(t *testing.T) {
	cfg := config.GameConfig{TurnTimeoutSec: 10}
	var m Mutation
	for _, mut := range Mutations {
		if mut.ID == "speed_run" {
			m = mut
			break
		}
	}
	if m.ID == "" {
		t.Fatal("speed_run mutation not found")
	}
	m.Apply(&cfg)
	if cfg.TurnTimeoutSec != 5 {
		t.Errorf("expected TurnTimeoutSec=5 after speed_run mutation, got %d", cfg.TurnTimeoutSec)
	}
}
