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
