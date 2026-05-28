# Skill Tree Branching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert linear `SkillBranch.Nodes [4]SkillNode` into a diamond `SkillTree {T1, T2[2], T3, T4[2]}` with two fork points so each run picks 1-of-4 builds per branch instead of following a single fixed sequence.

**Architecture:** `entity/skill_tree.go` is fully rewritten in Task 1 (data types, 48-node content, new `Allocate(branchIdx, class, choice byte)` API, path traversal helper). `game/session.go` forwards `choice` to entity. `ui/game_view.go` interprets `[`/`]` as A/B at fork tiers and as branch index otherwise.

**Tech Stack:** Go 1.21+, Bubble Tea TUI. Spec: `docs/superpowers/specs/2026-05-28-skill-tree-branching-design.md`.

---

## File Structure (locked decisions)

| Action | Path | Responsibility |
|--------|------|----------------|
| Replace | `entity/skill_tree.go` | New `SkillTree` type; 48-node `branchTrees` map; `PlayerSkills.Choices`; `Allocate(branchIdx, class, choice byte)`; `NextIsBranch`; `NextStepLabel`; tree-walking `allocatedNodes` |
| Modify | `entity/skill_tree_test.go` | Update 8 existing tests to new types/API; add 11 new tests per spec |
| Modify | `game/session.go` | `AllocateSkillPoint(fp, branchIdx, choice byte) error` — adds `choice` param, forwards |
| Modify | `ui/game_view.go` | Input handler at ~L386 reads `NextIsBranch` to decide branch-vs-choice meaning; HUD render at ~L782 shows tree-aware step prompts |

Each file has one clear responsibility. The Task 1 entity rewrite is heavy because the rewrite is cohesive — tearing it across multiple commits leaves the build red. Tasks 2-3 then layer game/UI changes on the stable entity foundation.

---

## Task 1: Rewrite entity/skill_tree.go with tree types and 48 nodes

**Files:**
- Replace: `entity/skill_tree.go`
- Replace: `entity/skill_tree_test.go`

This task does a full rewrite of the entity layer because the types, content, and API all change together. The other consumers (session.go, ui/game_view.go) currently call `AllocateSkillPoint(fp, branchIdx int)` (2 args at the session level) which delegates to `PlayerSkills.Allocate(branchIdx int, class Class)` (2 args at entity level). After this task, the entity `Allocate` takes 3 args. Until Task 2, the session forwarding will not compile — so Task 1 must also do a **minimal compile fix in session.go** to keep the build green.

- [ ] **Step 1: Replace `entity/skill_tree.go` with the new structure**

Replace the entire file contents with:

```go
package entity

import "errors"

// SkillEffect represents the type of bonus a skill node provides.
type SkillEffect int

const (
	EffectATKBoost SkillEffect = iota
	EffectDEFBoost
	EffectMaxHPBoost
	EffectSkillPower
	EffectCritChance
	EffectHealBoost
)

// SkillNode is a single node in a skill tree.
type SkillNode struct {
	Name   string
	Effect SkillEffect
	Value  int
}

// SkillTree is one branch's diamond-shaped skill tree:
//
//	    T1 (auto)
//	     |
//	  /     \
//	T2A    T2B    (first fork)
//	  \     /
//	    T3 (auto)
//	     |
//	  /     \
//	T4A    T4B    (capstone fork)
//
// Index 0 of T2/T4 is the 'A' choice; index 1 is 'B'.
type SkillTree struct {
	Name string
	T1   SkillNode
	T2   [2]SkillNode
	T3   SkillNode
	T4   [2]SkillNode
}

// PlayerSkills tracks a player's skill tree state for the current run.
//
// Allocated is the count of skill points spent in the chosen branch (0..4).
// Choices is the recorded fork selections: empty before T2, length 1 between
// T2 and T4, length 2 after the capstone. Each char is 'A' or 'B'.
//
// Invariant: len(Choices) == 0 when Allocated < 2; 1 when Allocated 2 or 3; 2 when Allocated == 4.
type PlayerSkills struct {
	BranchIndex int    // -1 = not chosen, 0 or 1
	Points      int    // total points earned this run
	Allocated   int    // points spent in chosen branch (0..4)
	Choices     string // fork choices, 0..2 chars, each 'A' or 'B'
}

// NewPlayerSkills returns an initialized PlayerSkills with no branch chosen.
func NewPlayerSkills() *PlayerSkills {
	return &PlayerSkills{BranchIndex: -1}
}

// branchTrees holds the two trees for each class. Tier 1 and Tier 3 are
// unconditional; Tier 2 and Tier 4 are forks (A at index 0, B at index 1).
//
// Design intent per spec: T2[A] keeps each branch's stay-on-theme identity
// (i.e. extends the original linear progression). T2[B] dips into a
// complementary stat. T4 doubles down on whichever sub-identity was chosen.
var branchTrees = map[Class][2]SkillTree{
	ClassWarrior: {
		{
			Name: "Tank",
			T1:   SkillNode{"Iron Skin", EffectDEFBoost, 3},
			T2: [2]SkillNode{
				{"Fortitude", EffectMaxHPBoost, 20},
				{"Spike Mail", EffectDEFBoost, 3},
			},
			T3: SkillNode{"Bastion", EffectDEFBoost, 5},
			T4: [2]SkillNode{
				{"Unbreakable", EffectMaxHPBoost, 50},
				{"Last Stand", EffectATKBoost, 12},
			},
		},
		{
			Name: "Berserker",
			T1:   SkillNode{"Fury", EffectATKBoost, 4},
			T2: [2]SkillNode{
				{"Wrath", EffectSkillPower, 20},
				{"Reckless", EffectATKBoost, 5},
			},
			T3: SkillNode{"Rampage", EffectATKBoost, 6},
			T4: [2]SkillNode{
				{"Bloodthirst", EffectATKBoost, 12},
				{"Unending Rage", EffectMaxHPBoost, 40},
			},
		},
	},
	ClassMage: {
		{
			Name: "Elementalist",
			T1:   SkillNode{"Arcane Focus", EffectSkillPower, 15},
			T2: [2]SkillNode{
				{"Elemental Fury", EffectATKBoost, 5},
				{"Mana Surge", EffectSkillPower, 15},
			},
			T3: SkillNode{"Overload", EffectSkillPower, 25},
			T4: [2]SkillNode{
				{"Cataclysm", EffectSkillPower, 50},
				{"Pyroclasm", EffectATKBoost, 14},
			},
		},
		{
			Name: "Chronomancer",
			T1:   SkillNode{"Temporal Shield", EffectDEFBoost, 3},
			T2: [2]SkillNode{
				{"Time Warp", EffectATKBoost, 3},
				{"Slow Field", EffectDEFBoost, 3},
			},
			T3: SkillNode{"Stasis", EffectMaxHPBoost, 15},
			T4: [2]SkillNode{
				{"Eternity", EffectMaxHPBoost, 40},
				{"Aeon Strike", EffectATKBoost, 12},
			},
		},
	},
	ClassHealer: {
		{
			Name: "Guardian",
			T1:   SkillNode{"Blessing", EffectHealBoost, 20},
			T2: [2]SkillNode{
				{"Divine Armor", EffectDEFBoost, 4},
				{"Aegis", EffectMaxHPBoost, 20},
			},
			T3: SkillNode{"Miracle", EffectHealBoost, 30},
			T4: [2]SkillNode{
				{"Resurrection", EffectHealBoost, 60},
				{"Sanctum", EffectDEFBoost, 8},
			},
		},
		{
			Name: "Priest",
			T1:   SkillNode{"Smite", EffectATKBoost, 5},
			T2: [2]SkillNode{
				{"Holy Power", EffectSkillPower, 20},
				{"Vow", EffectATKBoost, 4},
			},
			T3: SkillNode{"Judgment", EffectATKBoost, 7},
			T4: [2]SkillNode{
				{"Wrath of God", EffectATKBoost, 14},
				{"Crusade", EffectSkillPower, 40},
			},
		},
	},
	ClassRogue: {
		{
			Name: "Assassin",
			T1:   SkillNode{"Backstab", EffectATKBoost, 5},
			T2: [2]SkillNode{
				{"Precision", EffectCritChance, 15},
				{"Shadow Step", EffectATKBoost, 4},
			},
			T3: SkillNode{"Execute", EffectATKBoost, 8},
			T4: [2]SkillNode{
				{"Death Mark", EffectCritChance, 30},
				{"Bloodletter", EffectATKBoost, 14},
			},
		},
		{
			Name: "Alchemist",
			T1:   SkillNode{"Tonic", EffectHealBoost, 15},
			T2: [2]SkillNode{
				{"Brew", EffectSkillPower, 20},
				{"Bitter Tonic", EffectHealBoost, 15},
			},
			T3: SkillNode{"Elixir", EffectMaxHPBoost, 25},
			T4: [2]SkillNode{
				{"Panacea", EffectMaxHPBoost, 60},
				{"Universal Solvent", EffectHealBoost, 50},
			},
		},
	},
}

// GetBranches returns the two skill trees for the given class.
func GetBranches(class Class) [2]SkillTree {
	return branchTrees[class]
}

// Allocate spends one skill point. choice is interpreted only at fork tiers
// (T2 and T4); it is ignored at auto tiers (T1 and T3). At a fork, choice
// must be 'A' or 'B'.
//
// Returns errors for: invalid branchIdx, no points available, mid-run branch
// switch, fully allocated, or fork tier with missing/invalid choice byte.
func (ps *PlayerSkills) Allocate(branchIdx int, class Class, choice byte) error {
	if ps == nil {
		return errors.New("skills not initialized")
	}
	if branchIdx < 0 || branchIdx > 1 {
		return errors.New("invalid branch index")
	}
	if ps.Allocated >= 4 {
		return errors.New("branch fully allocated")
	}
	if ps.Points <= ps.Allocated {
		return errors.New("no available skill points")
	}
	if ps.BranchIndex != -1 && ps.BranchIndex != branchIdx {
		return errors.New("cannot switch branch after first allocation")
	}
	// Bind branch on first allocation.
	if ps.BranchIndex == -1 {
		ps.BranchIndex = branchIdx
	}
	// Advance based on current Allocated count.
	switch ps.Allocated {
	case 0:
		// T1 auto.
		ps.Allocated = 1
	case 1:
		// T2 fork.
		if choice != 'A' && choice != 'B' {
			return errors.New("T2 requires choice 'A' or 'B'")
		}
		ps.Choices += string(choice)
		ps.Allocated = 2
	case 2:
		// T3 auto.
		ps.Allocated = 3
	case 3:
		// T4 capstone fork.
		if choice != 'A' && choice != 'B' {
			return errors.New("T4 capstone requires choice 'A' or 'B'")
		}
		ps.Choices += string(choice)
		ps.Allocated = 4
	}
	return nil
}

// NextIsBranch reports whether the next Allocate call will land on a fork
// (T2 or T4). The UI uses this to decide whether to prompt for an A/B
// choice or treat both keys equivalently.
func (ps *PlayerSkills) NextIsBranch() bool {
	if ps == nil || ps.Allocated >= 4 {
		return false
	}
	return ps.Allocated == 1 || ps.Allocated == 3
}

// NextStepLabel returns a short Korean label describing what the next
// allocation will be. Empty string if nothing further can be allocated.
func (ps *PlayerSkills) NextStepLabel(class Class) string {
	if ps == nil || ps.Allocated >= 4 {
		return ""
	}
	if ps.BranchIndex < 0 {
		branches := GetBranches(class)
		return "분기 선택: " + branches[0].Name + " or " + branches[1].Name
	}
	branches := GetBranches(class)
	tree := branches[ps.BranchIndex]
	switch ps.Allocated {
	case 0:
		return "T1 자동: " + tree.T1.Name
	case 1:
		return "T2 선택: " + tree.T2[0].Name + " or " + tree.T2[1].Name
	case 2:
		return "T3 자동: " + tree.T3.Name
	case 3:
		return "T4 캡스톤: " + tree.T4[0].Name + " or " + tree.T4[1].Name
	}
	return ""
}

// choiceToIdx maps a recorded choice byte to the array index of the chosen
// fork node. 'A' -> 0, 'B' -> 1.
func choiceToIdx(c byte) int {
	if c == 'B' {
		return 1
	}
	return 0
}

// allocatedNodes returns the nodes the player has unlocked in their chosen
// branch, walking the tree path defined by Choices.
func (ps *PlayerSkills) allocatedNodes(class Class) []SkillNode {
	if ps == nil || ps.BranchIndex < 0 || ps.Allocated == 0 {
		return nil
	}
	branches := GetBranches(class)
	tree := branches[ps.BranchIndex]
	out := make([]SkillNode, 0, 4)
	if ps.Allocated >= 1 {
		out = append(out, tree.T1)
	}
	if ps.Allocated >= 2 && len(ps.Choices) >= 1 {
		out = append(out, tree.T2[choiceToIdx(ps.Choices[0])])
	}
	if ps.Allocated >= 3 {
		out = append(out, tree.T3)
	}
	if ps.Allocated >= 4 && len(ps.Choices) >= 2 {
		out = append(out, tree.T4[choiceToIdx(ps.Choices[1])])
	}
	return out
}

func (ps *PlayerSkills) sumEffect(class Class, effect SkillEffect) int {
	total := 0
	for _, node := range ps.allocatedNodes(class) {
		if node.Effect == effect {
			total += node.Value
		}
	}
	return total
}

// GetATKBonus returns the total ATK bonus from allocated skill nodes.
func (ps *PlayerSkills) GetATKBonus(class Class) int {
	return ps.sumEffect(class, EffectATKBoost)
}

// GetDEFBonus returns the total DEF bonus from allocated skill nodes.
func (ps *PlayerSkills) GetDEFBonus(class Class) int {
	return ps.sumEffect(class, EffectDEFBoost)
}

// GetMaxHPBonus returns the total MaxHP bonus from allocated skill nodes.
func (ps *PlayerSkills) GetMaxHPBonus(class Class) int {
	return ps.sumEffect(class, EffectMaxHPBoost)
}

// GetSkillPower returns the total SkillPower bonus from allocated skill nodes.
func (ps *PlayerSkills) GetSkillPower(class Class) int {
	return ps.sumEffect(class, EffectSkillPower)
}

// GetCritChance returns the total CritChance bonus from allocated skill nodes.
func (ps *PlayerSkills) GetCritChance(class Class) int {
	return ps.sumEffect(class, EffectCritChance)
}

// GetHealBoost returns the total HealBoost bonus from allocated skill nodes.
func (ps *PlayerSkills) GetHealBoost(class Class) int {
	return ps.sumEffect(class, EffectHealBoost)
}
```

- [ ] **Step 2: Replace `entity/skill_tree_test.go` with updated + new tests**

Replace the entire file contents with:

```go
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
```

- [ ] **Step 3: Update `game/session.go::AllocateSkillPoint` for the new entity signature**

The existing `AllocateSkillPoint(fingerprint string, branchIdx int) error` calls `p.Skills.Allocate(branchIdx, p.Class)`. The entity method now takes 3 args. The minimum change to keep the build green for this task: pass `0` as the choice byte (entity will return an error at T2/T4, which the session surfaces). This is a temporary partial state — Task 2 makes the session method accept `choice byte` from the caller.

In `game/session.go` find line ~845:

```go
if err := p.Skills.Allocate(branchIdx, p.Class); err != nil {
```

Replace with:

```go
if err := p.Skills.Allocate(branchIdx, p.Class, 0); err != nil {
```

This makes the build pass. Allocating past T1 will fail with the entity-layer error message until Task 2 plumbs the real choice byte through.

- [ ] **Step 3b: Apply a minimal HUD compile patch in `ui/game_view.go`**

The HUD around lines 781–807 currently dereferences `branch.Nodes[nextNode]`, but the new `SkillTree` type has no `Nodes` field — the build will not compile without this patch. Task 3 replaces this with the rich tree-aware HUD; for now, ship a minimal placeholder so the entire repo compiles after this task.

Find the block around lines 781–808 in `ui/game_view.go`:

```go
// Show skill tree allocation UI if player has unspent points
for _, p := range state.Players {
	if p.Fingerprint == fingerprint && p.Skills != nil && p.Skills.Points > p.Skills.Allocated && p.Skills.Allocated < 4 {
		branches := entity.GetBranches(p.Class)
		sb.WriteString("\n")
		skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
		sb.WriteString(skillStyle.Render(fmt.Sprintf("  스킬 포인트 사용 가능! (미사용: %d)", p.Skills.Points-p.Skills.Allocated)))
		sb.WriteString("\n")
		for i, branch := range branches {
			key := "["
			if i == 1 {
				key = "]"
			}
			nextNode := p.Skills.Allocated
			if p.Skills.BranchIndex >= 0 && p.Skills.BranchIndex != i {
				fmt.Fprintf(&sb, "    [%s] %s (잠김)\n", key, branch.Name)
			} else if nextNode < 4 {
				node := branch.Nodes[nextNode]
				capstoneTag := ""
				if nextNode == 3 {
					capstoneTag = " ★ 캡스톤"
				}
				fmt.Fprintf(&sb, "    [%s] %s -> %s%s\n", key, branch.Name, node.Name, capstoneTag)
			}
		}
		break
	}
}
```

Replace with this minimal compile-safe version:

```go
// Show skill tree allocation UI if player has unspent points.
// Minimal placeholder rendering — Task 3 replaces this with rich
// tree-aware display.
for _, p := range state.Players {
	if p.Fingerprint == fingerprint && p.Skills != nil && p.Skills.Points > p.Skills.Allocated && p.Skills.Allocated < 4 {
		branches := entity.GetBranches(p.Class)
		sb.WriteString("\n")
		skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
		sb.WriteString(skillStyle.Render(fmt.Sprintf("  스킬 포인트 사용 가능! (미사용: %d)", p.Skills.Points-p.Skills.Allocated)))
		sb.WriteString("\n")
		for i, tree := range branches {
			key := "["
			if i == 1 {
				key = "]"
			}
			if p.Skills.BranchIndex >= 0 && p.Skills.BranchIndex != i {
				fmt.Fprintf(&sb, "    [%s] %s (잠김)\n", key, tree.Name)
			} else {
				fmt.Fprintf(&sb, "    [%s] %s\n", key, tree.Name)
			}
		}
		break
	}
}
```

- [ ] **Step 4: Run the new test suite**

Run: `go test ./entity/`
Expected: all tests in `entity/skill_tree_test.go` PASS.

If any test fails, fix the implementation in `entity/skill_tree.go` and re-run. Do NOT modify the tests to match a buggy implementation.

- [ ] **Step 5: Run the full game suite to confirm nothing else broke**

Run: `go test ./...`
Expected: every package PASS. The session.go change made in Step 3 may surface other test failures if any test calls `AllocateSkillPoint` and expects T2 to succeed — if so, those tests need to wait for Task 2. List any failures and verify they are all "allocation past T1 returns an error" type, not unrelated regressions.

Run: `go build -o catacombs .`
Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
git add entity/skill_tree.go entity/skill_tree_test.go game/session.go ui/game_view.go
git commit -m "feat(skills): replace linear branches with diamond skill trees (T2 + T4 forks)"
```

---

## Task 2: Plumb `choice byte` through `AllocateSkillPoint`

**Files:**
- Modify: `game/session.go` (signature change + forward choice)
- Modify: `ui/game_view.go` (call site at ~L386–399)
- Test: `game/session_test.go` (append a session-level test)

After Task 1, the entity layer accepts a real choice but the session and UI still pass `0`. This task wires the choice end-to-end so a player pressing `]` at a T2 fork actually records 'B'.

- [ ] **Step 1: Write the failing test**

Append to `game/session_test.go`:

```go
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
```

- [ ] **Step 2: Verify it fails to compile**

Run: `go test ./game/ -run TestAllocateSkillPoint_ForwardsChoiceToEntity`
Expected: compile error — `AllocateSkillPoint` only takes 2 args.

- [ ] **Step 3: Update `AllocateSkillPoint` signature in `game/session.go`**

Find the method around line 836:

```go
func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			if p.Skills == nil || p.Skills.Points <= p.Skills.Allocated {
				return fmt.Errorf("스킬 포인트가 없습니다")
			}
			before := p.Skills.GetMaxHPBonus(p.Class)
			if err := p.Skills.Allocate(branchIdx, p.Class, 0); err != nil {
				return err
			}
			// MaxHPBoost nodes raise both MaxHP and current HP by the delta.
			delta := p.Skills.GetMaxHPBonus(p.Class) - before
			if delta > 0 {
				p.MaxHP += delta
				p.HP += delta
			}
			return nil
		}
	}
	return fmt.Errorf("플레이어를 찾을 수 없습니다")
}
```

Replace with:

```go
func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int, choice byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			if p.Skills == nil || p.Skills.Points <= p.Skills.Allocated {
				return fmt.Errorf("스킬 포인트가 없습니다")
			}
			before := p.Skills.GetMaxHPBonus(p.Class)
			if err := p.Skills.Allocate(branchIdx, p.Class, choice); err != nil {
				return err
			}
			// MaxHPBoost nodes raise both MaxHP and current HP by the delta.
			delta := p.Skills.GetMaxHPBonus(p.Class) - before
			if delta > 0 {
				p.MaxHP += delta
				p.HP += delta
			}
			return nil
		}
	}
	return fmt.Errorf("플레이어를 찾을 수 없습니다")
}
```

- [ ] **Step 4: Update the single call site in `ui/game_view.go` (~line 386–399)**

The existing block:

```go
// Skill point allocation
if isKey(key, "[") || isKey(key, "]") {
	if ctx.Session != nil {
		branchIdx := 0
		if isKey(key, "]") {
			branchIdx = 1
		}
		if err := ctx.Session.AllocateSkillPoint(ctx.Fingerprint, branchIdx); err != nil {
			s.skillMsg = err.Error()
		} else {
			s.skillMsg = ""
		}
		s.gameState = ctx.Session.GetState()
	}
	return s, nil
}
```

Replace with:

```go
// Skill point allocation. The meaning of [ and ] depends on the next
// allocation step: at a fork (T2 or T4), they're A/B choice; otherwise
// they're branch index. We resolve that here by reading NextIsBranch
// from the local player.
if isKey(key, "[") || isKey(key, "]") {
	if ctx.Session != nil {
		var localSkills *entity.PlayerSkills
		for _, p := range s.gameState.Players {
			if p.Fingerprint == ctx.Fingerprint && p.Skills != nil {
				localSkills = p.Skills
				break
			}
		}
		branchIdx := 0
		var choice byte
		if localSkills != nil && localSkills.BranchIndex != -1 && localSkills.NextIsBranch() {
			// At T2/T4 fork: keys mean A or B, branch already locked.
			branchIdx = localSkills.BranchIndex
			if isKey(key, "[") {
				choice = 'A'
			} else {
				choice = 'B'
			}
		} else {
			// Branch-select stage (or T3 auto): keys mean branch index.
			if isKey(key, "]") {
				branchIdx = 1
			}
		}
		if err := ctx.Session.AllocateSkillPoint(ctx.Fingerprint, branchIdx, choice); err != nil {
			s.skillMsg = err.Error()
		} else {
			s.skillMsg = ""
		}
		s.gameState = ctx.Session.GetState()
	}
	return s, nil
}
```

- [ ] **Step 5: Run the new test plus the full suite**

Run: `go test ./game/ -run TestAllocateSkillPoint_ForwardsChoiceToEntity -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add game/session.go ui/game_view.go game/session_test.go
git commit -m "feat(skills): forward fork choice byte from UI through to entity"
```

---

## Task 3: Tree-aware HUD rendering

**Files:**
- Modify: `ui/game_view.go` (HUD block at ~L782–807)
- Test: (UI render is exercised by smoke build; no automated test added — same pattern as existing HUD code, which has no targeted unit test)

Task 1 already planted a minimal compile-safe HUD that just shows branch names and a lock marker. This task replaces that placeholder with a tree-aware display: branch-select stage, T2 fork, T3 auto, T4 capstone. The build is green entering this task.

- [ ] **Step 1: Confirm the current HUD is the Task 1 placeholder**

Run: `go build -o catacombs .`
Expected: clean build (entered from a green state after Task 2 finished).

Open `ui/game_view.go` and locate the skill-tree HUD block around line 781. It should match the placeholder from Task 1 Step 3b: a simple branch-name + `(잠김)` display with no fork/auto distinction. If it doesn't match, read the actual code and adjust the Step 2 patch accordingly.

- [ ] **Step 2: Replace the HUD skill-allocation block**

Find the placeholder block (planted in Task 1 Step 3b) around line 781 in `ui/game_view.go`:

```go
// Show skill tree allocation UI if player has unspent points.
// Minimal placeholder rendering — Task 3 replaces this with rich
// tree-aware display.
for _, p := range state.Players {
	if p.Fingerprint == fingerprint && p.Skills != nil && p.Skills.Points > p.Skills.Allocated && p.Skills.Allocated < 4 {
		branches := entity.GetBranches(p.Class)
		sb.WriteString("\n")
		skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
		sb.WriteString(skillStyle.Render(fmt.Sprintf("  스킬 포인트 사용 가능! (미사용: %d)", p.Skills.Points-p.Skills.Allocated)))
		sb.WriteString("\n")
		for i, tree := range branches {
			key := "["
			if i == 1 {
				key = "]"
			}
			if p.Skills.BranchIndex >= 0 && p.Skills.BranchIndex != i {
				fmt.Fprintf(&sb, "    [%s] %s (잠김)\n", key, tree.Name)
			} else {
				fmt.Fprintf(&sb, "    [%s] %s\n", key, tree.Name)
			}
		}
		break
	}
}
```

Replace the entire block with:

```go
// Show skill tree allocation UI if player has unspent points.
// Display adapts to which tier is next: branch-select / T2 fork / T3 auto / T4 capstone.
for _, p := range state.Players {
	if p.Fingerprint == fingerprint && p.Skills != nil && p.Skills.Points > p.Skills.Allocated && p.Skills.Allocated < 4 {
		branches := entity.GetBranches(p.Class)
		sb.WriteString("\n")
		skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
		sb.WriteString(skillStyle.Render(fmt.Sprintf("  스킬 포인트 사용 가능! (미사용: %d)", p.Skills.Points-p.Skills.Allocated)))
		sb.WriteString("\n")
		if p.Skills.BranchIndex == -1 {
			// Branch-select stage: each key picks a branch (and auto-takes T1).
			for i, tree := range branches {
				key := "["
				if i == 1 {
					key = "]"
				}
				fmt.Fprintf(&sb, "    [%s] %s -> %s\n", key, tree.Name, tree.T1.Name)
			}
		} else {
			tree := branches[p.Skills.BranchIndex]
			otherTree := branches[1-p.Skills.BranchIndex]
			otherKey := "["
			if p.Skills.BranchIndex == 0 {
				otherKey = "]"
			}
			// Show the locked branch line.
			fmt.Fprintf(&sb, "    [%s] %s (잠김)\n", otherKey, otherTree.Name)
			// Determine fork or auto next.
			switch p.Skills.Allocated {
			case 1:
				// T2 fork
				fmt.Fprintf(&sb, "    [[] %s -> %s\n", tree.Name, tree.T2[0].Name)
				fmt.Fprintf(&sb, "    []] %s -> %s\n", tree.Name, tree.T2[1].Name)
			case 2:
				// T3 auto — both keys do the same.
				fmt.Fprintf(&sb, "    [[/]] %s -> %s (자동)\n", tree.Name, tree.T3.Name)
			case 3:
				// T4 capstone fork
				fmt.Fprintf(&sb, "    [[] %s -> %s ★ 캡스톤\n", tree.Name, tree.T4[0].Name)
				fmt.Fprintf(&sb, "    []] %s -> %s ★ 캡스톤\n", tree.Name, tree.T4[1].Name)
			}
		}
		break
	}
}
```

Note: `fmt.Fprintf(&sb, "    [[] ...", ...)` produces literal `[[]` in the output because `%` is the only Fprintf format meta-char. The literal `[` and `]` characters in the format string are not special. Verified.

- [ ] **Step 3: Build**

Run: `go build -o catacombs .`
Expected: build succeeds.

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/game_view.go
git commit -m "feat(skills): tree-aware HUD rendering for branch/fork allocation steps"
```

---

## Task 4: Regression sweep + final verification

**Files:** none modified (verification only)

- [ ] **Step 1: Run full suite with coverage**

Run: `go test -cover ./...`
Expected: all packages PASS. Note coverage for `entity` and `ui` packages — should not have dropped significantly.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no warnings.

- [ ] **Step 3: gofmt check**

Run: `gofmt -l entity/skill_tree.go entity/skill_tree_test.go ui/game_view.go game/session.go game/session_test.go`
Expected: project uses CRLF, so files may be flagged. Do NOT run `gofmt -w`. Note which files are flagged — should match existing project pattern (CRLF on all `.go` files).

- [ ] **Step 4: Spec coverage check**

Open `docs/superpowers/specs/2026-05-28-skill-tree-branching-design.md` and cross-reference its "Testing" section. Confirm each listed test exists in `entity/skill_tree_test.go`:

- `TestAllocate_T1IsAutoAndIgnoresChoice` — ✓ Task 1
- `TestAllocate_T2RequiresChoice` — ✓ Task 1
- `TestAllocate_T2AppendsChoice` — ✓ Task 1
- `TestAllocate_T3IsAutoAndIgnoresChoice` — ✓ Task 1
- `TestAllocate_T4Capstone` — ✓ Task 1
- `TestAllocate_LocksBranchAfterFirstAllocation` — ✓ Task 1
- `TestAllocate_LocksFifthAllocation` — ✓ Task 1
- `TestAllocatedNodes_PathTraversal` — ✓ Task 1
- `TestGetATKBonus_SumsAcrossPath` — ✓ Task 1
- `TestNextIsBranch_TrueAtForksOnly` — ✓ Task 1 (covers `TestNextIsBranch_TrueAtT2AndT4Only` from the spec)
- `TestEveryBranchTreeIsWellFormed` — ✓ Task 1 (covered by `TestGetBranches` which now checks T1/T2/T3/T4 names+values)

All 11 spec tests are present. The spec also lists `TestPartialAllocation` (renamed `TestPartialAllocation_ReturnsPrefixOnly`) and `TestNilPlayerSkillsBonuses` (preserved from old test file).

- [ ] **Step 5: Manual smoke build**

Run: `go build -o catacombs .`
Expected: clean build. Optionally run locally (`./catacombs`) and start a Warrior solo run to step through T1 → `]` (T2B) → auto T3 → `[` (T4A) to visually confirm the HUD adapts.

- [ ] **Step 6: Final commit (only if anything changed in this task)**

Most likely nothing changed in Task 4 — it is verification only.

```bash
git status
```

If clean, that's expected.

---

## Self-review

**1. Spec coverage:** Every spec section mapped to a task:
- Architecture (diamond shape) → Task 1 data types
- Components: PlayerSkills.Choices → Task 1 (data) + Task 2 (session forward) + Task 3 (UI choice plumbing already done in Task 2)
- API (Allocate, NextIsBranch, NextStepLabel, GetBranches) → Task 1
- Branch content (48 nodes) → Task 1
- Game session API change → Task 2
- UI changes → Task 2 (input) + Task 3 (HUD)
- Mastery interaction → no code change needed; documented
- Data flow → exercised by Tasks 1-3
- Edge cases → covered by Task 1 tests (no-points, fork-without-choice, branch-switch, fifth-allocation)
- 11 spec tests → all in Task 1 with `TestPartialAllocation` and `TestNilPlayerSkillsBonuses` preserved
- Migration/compat → no DB work; one session call site updated in Task 2

**2. Placeholder scan:** No "TBD" / "implement later" / "similar to" / vague step instructions. All code blocks complete.

**3. Type consistency:** `SkillTree`, `PlayerSkills.Choices`, `Allocate(branchIdx, class, choice byte)`, `NextIsBranch()`, `NextStepLabel(class)`, `GetBranches(class)`, `choiceToIdx`, `allocatedNodes` — all spelled identically in Task 1 implementation, Task 1 tests, Task 2 implementation, Task 2 test, and Task 3 UI patch.
