package entity

import "testing"

// TestGetBranches verifies every class has two well-formed trees with
// non-empty node names. Replaces the old test that walked Nodes[j].
func TestGetBranches(t *testing.T) {
	classes := []Class{ClassWarrior, ClassMage, ClassHealer, ClassRogue}
	for _, c := range classes {
		branches := GetBranches(c)
		if len(branches) != 2 {
			t.Errorf("expected 2 branches for %s, got %d", c, len(branches))
		}
		for i, tree := range branches {
			if tree.Name == "" {
				t.Errorf("branch %d for %s has empty name", i, c)
			}
			if tree.T1.Name == "" {
				t.Errorf("T1 in branch %d for %s has empty name", i, c)
			}
			if tree.T3.Name == "" {
				t.Errorf("T3 in branch %d for %s has empty name", i, c)
			}
			for j := 0; j < 2; j++ {
				if tree.T2[j].Name == "" {
					t.Errorf("T2[%d] in branch %d for %s has empty name", j, i, c)
				}
				if tree.T4[j].Name == "" {
					t.Errorf("T4[%d] in branch %d for %s has empty name", j, i, c)
				}
			}
			if tree.T1.Value <= 0 || tree.T3.Value <= 0 {
				t.Errorf("auto nodes in branch %d for %s must have positive values", i, c)
			}
			for j := 0; j < 2; j++ {
				if tree.T2[j].Value <= 0 {
					t.Errorf("T2[%d] in branch %d for %s has non-positive value", j, i, c)
				}
				if tree.T4[j].Value <= 0 {
					t.Errorf("T4[%d] in branch %d for %s has non-positive value", j, i, c)
				}
			}
		}
	}
}

func TestAllocate_T1IsAutoAndIgnoresChoice(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 1
	// choice byte is irrelevant on first allocation; pass 0 deliberately.
	if err := ps.Allocate(0, ClassWarrior, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.BranchIndex != 0 {
		t.Errorf("expected BranchIndex 0, got %d", ps.BranchIndex)
	}
	if ps.Allocated != 1 {
		t.Errorf("expected Allocated 1, got %d", ps.Allocated)
	}
	if ps.Choices != "" {
		t.Errorf("expected Choices '', got %q", ps.Choices)
	}
}

func TestAllocate_T2RequiresChoice(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 2
	_ = ps.Allocate(0, ClassWarrior, 0) // T1 auto
	err := ps.Allocate(0, ClassWarrior, 0)
	if err == nil {
		t.Error("expected error when T2 receives no choice")
	}
}

func TestAllocate_T2AppendsChoice(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 2
	_ = ps.Allocate(0, ClassWarrior, 0) // T1
	if err := ps.Allocate(0, ClassWarrior, 'B'); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Choices != "B" {
		t.Errorf("expected Choices 'B', got %q", ps.Choices)
	}
	if ps.Allocated != 2 {
		t.Errorf("expected Allocated 2, got %d", ps.Allocated)
	}
}

func TestAllocate_T3IsAutoAndIgnoresChoice(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 3
	_ = ps.Allocate(0, ClassWarrior, 0)
	_ = ps.Allocate(0, ClassWarrior, 'A')
	if err := ps.Allocate(0, ClassWarrior, 0); err != nil {
		t.Fatalf("unexpected error on T3: %v", err)
	}
	if ps.Allocated != 3 {
		t.Errorf("expected Allocated 3, got %d", ps.Allocated)
	}
	if ps.Choices != "A" {
		t.Errorf("expected Choices 'A' (unchanged at T3), got %q", ps.Choices)
	}
}

func TestAllocate_T4Capstone(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 4
	_ = ps.Allocate(0, ClassWarrior, 0)
	_ = ps.Allocate(0, ClassWarrior, 'B')
	_ = ps.Allocate(0, ClassWarrior, 0)
	if err := ps.Allocate(0, ClassWarrior, 'A'); err != nil {
		t.Fatalf("unexpected error on T4: %v", err)
	}
	if ps.Choices != "BA" {
		t.Errorf("expected Choices 'BA', got %q", ps.Choices)
	}
	if ps.Allocated != 4 {
		t.Errorf("expected Allocated 4, got %d", ps.Allocated)
	}
}

func TestAllocate_LocksBranchAfterFirstAllocation(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 2
	if err := ps.Allocate(0, ClassWarrior, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ps.Allocate(1, ClassWarrior, 'A'); err == nil {
		t.Error("expected error when switching branches mid-run")
	}
}

func TestAllocate_LocksFifthAllocation(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 5
	_ = ps.Allocate(0, ClassWarrior, 0)
	_ = ps.Allocate(0, ClassWarrior, 'A')
	_ = ps.Allocate(0, ClassWarrior, 0)
	_ = ps.Allocate(0, ClassWarrior, 'A')
	if err := ps.Allocate(0, ClassWarrior, 'A'); err == nil {
		t.Error("expected error when allocating beyond capstone")
	}
}

func TestAllocate_NoPointsAvailable(t *testing.T) {
	ps := NewPlayerSkills()
	// Points stays 0
	if err := ps.Allocate(0, ClassWarrior, 0); err == nil {
		t.Error("expected error when no points are available")
	}
}

func TestAllocatedNodes_PathTraversal(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 4
	_ = ps.Allocate(0, ClassWarrior, 0)   // T1 = Iron Skin
	_ = ps.Allocate(0, ClassWarrior, 'A') // T2A = Fortitude
	_ = ps.Allocate(0, ClassWarrior, 0)   // T3 = Bastion
	_ = ps.Allocate(0, ClassWarrior, 'B') // T4B = Last Stand

	nodes := ps.allocatedNodes(ClassWarrior)
	want := []string{"Iron Skin", "Fortitude", "Bastion", "Last Stand"}
	if len(nodes) != len(want) {
		t.Fatalf("expected %d nodes, got %d", len(want), len(nodes))
	}
	for i, n := range nodes {
		if n.Name != want[i] {
			t.Errorf("node %d: expected %q, got %q", i, want[i], n.Name)
		}
	}
}

func TestGetATKBonus_SumsAcrossPath(t *testing.T) {
	// Warrior Berserker full path with Choices "BB":
	//   T1 = Fury (ATK +4)
	//   T2B = Reckless (ATK +5)
	//   T3 = Rampage (ATK +6)
	//   T4B = Unending Rage (MaxHP +40, NOT ATK)
	// Expected ATK bonus: 4 + 5 + 6 = 15.
	ps := NewPlayerSkills()
	ps.Points = 4
	_ = ps.Allocate(1, ClassWarrior, 0)
	_ = ps.Allocate(1, ClassWarrior, 'B')
	_ = ps.Allocate(1, ClassWarrior, 0)
	_ = ps.Allocate(1, ClassWarrior, 'B')

	got := ps.GetATKBonus(ClassWarrior)
	if got != 15 {
		t.Errorf("expected ATK bonus 15 for Berserker 'BB', got %d", got)
	}
	gotHP := ps.GetMaxHPBonus(ClassWarrior)
	if gotHP != 40 {
		t.Errorf("expected MaxHP bonus 40 for Berserker 'BB', got %d", gotHP)
	}
}

func TestNextIsBranch_TrueAtForksOnly(t *testing.T) {
	ps := NewPlayerSkills()
	// Allocated 0: false (next is T1 auto)
	if ps.NextIsBranch() {
		t.Error("Allocated=0 should report false (T1 is auto)")
	}
	ps.Points = 4
	_ = ps.Allocate(0, ClassWarrior, 0) // Allocated=1
	if !ps.NextIsBranch() {
		t.Error("Allocated=1 should report true (next is T2 fork)")
	}
	_ = ps.Allocate(0, ClassWarrior, 'A') // Allocated=2
	if ps.NextIsBranch() {
		t.Error("Allocated=2 should report false (T3 is auto)")
	}
	_ = ps.Allocate(0, ClassWarrior, 0) // Allocated=3
	if !ps.NextIsBranch() {
		t.Error("Allocated=3 should report true (next is T4 fork)")
	}
	_ = ps.Allocate(0, ClassWarrior, 'A') // Allocated=4
	if ps.NextIsBranch() {
		t.Error("Allocated=4 should report false (no further allocation)")
	}
}

func TestNilPlayerSkillsBonuses(t *testing.T) {
	var ps *PlayerSkills
	if got := ps.GetATKBonus(ClassWarrior); got != 0 {
		t.Errorf("nil skills should return 0 ATK bonus, got %d", got)
	}
	if got := ps.GetMaxHPBonus(ClassWarrior); got != 0 {
		t.Errorf("nil skills should return 0 MaxHP bonus, got %d", got)
	}
	if ps.NextIsBranch() {
		t.Error("nil skills should report NextIsBranch=false")
	}
	if ps.NextStepLabel(ClassWarrior) != "" {
		t.Error("nil skills should return empty NextStepLabel")
	}
}

func TestPartialAllocation_ReturnsPrefixOnly(t *testing.T) {
	ps := NewPlayerSkills()
	ps.Points = 2
	_ = ps.Allocate(0, ClassWarrior, 0)
	_ = ps.Allocate(0, ClassWarrior, 'A')

	nodes := ps.allocatedNodes(ClassWarrior)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes for partial allocation, got %d", len(nodes))
	}
}
