package entity

import "testing"

func TestGetBranches(t *testing.T) {
	classes := []Class{ClassWarrior, ClassMage, ClassHealer, ClassRogue}
	for _, c := range classes {
		branches := GetBranches(c)
		if len(branches) != 2 {
			t.Errorf("expected 2 branches for %s, got %d", c, len(branches))
		}
		for i, b := range branches {
			if b.Name == "" {
				t.Errorf("branch %d for %s has empty name", i, c)
			}
			for j, node := range b.Nodes {
				if node.Name == "" {
					t.Errorf("node %d in branch %d for %s has empty name", j, i, c)
				}
				if node.Value <= 0 {
					t.Errorf("node %d in branch %d for %s has non-positive value %d", j, i, c, node.Value)
				}
			}
		}
	}
}

func TestAllocateSkillPoint(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 1

	err := ps.Allocate(0, ClassWarrior)
	if err != nil {
		t.Fatalf("unexpected error on first allocation: %v", err)
	}
	if ps.BranchIndex != 0 {
		t.Errorf("expected BranchIndex 0, got %d", ps.BranchIndex)
	}
	if ps.Allocated != 1 {
		t.Errorf("expected Allocated 1, got %d", ps.Allocated)
	}
}

func TestCannotSwitchBranch(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 2

	err := ps.Allocate(0, ClassWarrior)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = ps.Allocate(1, ClassWarrior)
	if err == nil {
		t.Fatal("expected error when switching branch, got nil")
	}
}

func TestCannotAllocateWithoutPoints(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 0

	err := ps.Allocate(0, ClassWarrior)
	if err == nil {
		t.Fatal("expected error when no points available, got nil")
	}
}

func TestFullyAllocated(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 5

	for i := 0; i < 4; i++ {
		err := ps.Allocate(0, ClassWarrior)
		if err != nil {
			t.Fatalf("unexpected error on allocation %d: %v", i+1, err)
		}
	}

	err := ps.Allocate(0, ClassWarrior)
	if err == nil {
		t.Fatal("expected error when fully allocated, got nil")
	}
}

func TestSkillBonuses(t *testing.T) {
	// Warrior Tank branch: DEF+3, MaxHP+20, DEF+5
	ps := NewPlayerSkills()
	ps.Points = 3
	for i := 0; i < 3; i++ {
		if err := ps.Allocate(0, ClassWarrior); err != nil {
			t.Fatalf("allocate error: %v", err)
		}
	}

	if got := ps.GetDEFBonus(ClassWarrior); got != 8 {
		t.Errorf("expected DEF bonus 8 (3+5), got %d", got)
	}
	if got := ps.GetMaxHPBonus(ClassWarrior); got != 20 {
		t.Errorf("expected MaxHP bonus 20, got %d", got)
	}
	if got := ps.GetATKBonus(ClassWarrior); got != 0 {
		t.Errorf("expected ATK bonus 0 for Tank, got %d", got)
	}

	// Warrior Berserker branch: ATK+4, SkillPower+20, ATK+6
	ps2 := NewPlayerSkills()
	ps2.Points = 3
	for i := 0; i < 3; i++ {
		if err := ps2.Allocate(1, ClassWarrior); err != nil {
			t.Fatalf("allocate error: %v", err)
		}
	}
	if got := ps2.GetATKBonus(ClassWarrior); got != 10 {
		t.Errorf("expected ATK bonus 10 (4+6), got %d", got)
	}
	if got := ps2.GetSkillPower(ClassWarrior); got != 20 {
		t.Errorf("expected SkillPower bonus 20, got %d", got)
	}
}

func TestNilPlayerSkillsBonuses(t *testing.T) {
	var ps *PlayerSkills
	if got := ps.GetATKBonus(ClassWarrior); got != 0 {
		t.Errorf("expected 0 ATK bonus from nil skills, got %d", got)
	}
	if got := ps.GetDEFBonus(ClassWarrior); got != 0 {
		t.Errorf("expected 0 DEF bonus from nil skills, got %d", got)
	}
}

func TestPartialAllocation(t *testing.T) {
	// Rogue Assassin: ATK+5, CritChance+15, ATK+8
	// Allocate only 2 points: should get ATK+5 and CritChance+15
	ps := NewPlayerSkills()
	ps.Points = 2
	for i := 0; i < 2; i++ {
		if err := ps.Allocate(0, ClassRogue); err != nil {
			t.Fatalf("allocate error: %v", err)
		}
	}
	if got := ps.GetATKBonus(ClassRogue); got != 5 {
		t.Errorf("expected ATK bonus 5, got %d", got)
	}
	if got := ps.GetCritChance(ClassRogue); got != 15 {
		t.Errorf("expected CritChance bonus 15, got %d", got)
	}
}
