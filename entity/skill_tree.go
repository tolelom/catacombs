package entity

import "errors"

// SkillEffect represents the type of bonus a skill node provides.
type SkillEffect int

const (
	EffectATKBoost    SkillEffect = iota
	EffectDEFBoost
	EffectMaxHPBoost
	EffectSkillPower
	EffectCritChance
	EffectHealBoost
)

// SkillNode is a single node in a skill branch.
type SkillNode struct {
	Name   string
	Effect SkillEffect
	Value  int
}

// SkillBranch is a named sequence of 4 skill nodes (3 base tiers + 1 capstone).
type SkillBranch struct {
	Name  string
	Nodes [4]SkillNode
}

// PlayerSkills tracks a player's skill tree state for the current run.
type PlayerSkills struct {
	BranchIndex int // -1 = not chosen, 0 or 1
	Points      int // total points earned (1 per floor clear)
	Allocated   int // points spent in chosen branch (max 3)
}

// NewPlayerSkills returns an initialized PlayerSkills with no branch chosen.
func NewPlayerSkills() *PlayerSkills {
	return &PlayerSkills{BranchIndex: -1}
}

// branchDefs holds 2 branches per class. Tier 4 (Nodes[3]) is the capstone.
// Allocate enforces the points-spent prerequisite (3 prior tiers in this
// branch) but does NOT gate on floor — players who buy the StartSkillPts
// perma-upgrade can land the capstone within the first few floors.
var branchDefs = map[Class][2]SkillBranch{
	ClassWarrior: {
		{
			Name: "Tank",
			Nodes: [4]SkillNode{
				{"Iron Skin", EffectDEFBoost, 3},
				{"Fortitude", EffectMaxHPBoost, 20},
				{"Bastion", EffectDEFBoost, 5},
				{"Unbreakable", EffectMaxHPBoost, 50},
			},
		},
		{
			Name: "Berserker",
			Nodes: [4]SkillNode{
				{"Fury", EffectATKBoost, 4},
				{"Wrath", EffectSkillPower, 20},
				{"Rampage", EffectATKBoost, 6},
				{"Bloodthirst", EffectATKBoost, 12},
			},
		},
	},
	ClassMage: {
		{
			Name: "Elementalist",
			Nodes: [4]SkillNode{
				{"Arcane Focus", EffectSkillPower, 15},
				{"Elemental Fury", EffectATKBoost, 5},
				{"Overload", EffectSkillPower, 25},
				{"Cataclysm", EffectSkillPower, 50},
			},
		},
		{
			Name: "Chronomancer",
			Nodes: [4]SkillNode{
				{"Temporal Shield", EffectDEFBoost, 3},
				{"Time Warp", EffectATKBoost, 3},
				{"Stasis", EffectMaxHPBoost, 15},
				{"Eternity", EffectMaxHPBoost, 40},
			},
		},
	},
	ClassHealer: {
		{
			Name: "Guardian",
			Nodes: [4]SkillNode{
				{"Blessing", EffectHealBoost, 20},
				{"Divine Armor", EffectDEFBoost, 4},
				{"Miracle", EffectHealBoost, 30},
				{"Resurrection", EffectHealBoost, 60},
			},
		},
		{
			Name: "Priest",
			Nodes: [4]SkillNode{
				{"Smite", EffectATKBoost, 5},
				{"Holy Power", EffectSkillPower, 20},
				{"Judgment", EffectATKBoost, 7},
				{"Wrath of God", EffectATKBoost, 14},
			},
		},
	},
	ClassRogue: {
		{
			Name: "Assassin",
			Nodes: [4]SkillNode{
				{"Backstab", EffectATKBoost, 5},
				{"Precision", EffectCritChance, 15},
				{"Execute", EffectATKBoost, 8},
				{"Death Mark", EffectCritChance, 30},
			},
		},
		{
			Name: "Alchemist",
			Nodes: [4]SkillNode{
				{"Tonic", EffectHealBoost, 15},
				{"Brew", EffectSkillPower, 20},
				{"Elixir", EffectMaxHPBoost, 25},
				{"Panacea", EffectMaxHPBoost, 60},
			},
		},
	},
}

// GetBranches returns the 2 skill branches for the given class.
func GetBranches(class Class) [2]SkillBranch {
	return branchDefs[class]
}

// Allocate spends one skill point into the given branch. Returns an error if
// the player tries to switch branches after first allocation or has already
// allocated the maximum of 4 points (3 base + 1 capstone).
func (ps *PlayerSkills) Allocate(branchIdx int, class Class) error {
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
	ps.BranchIndex = branchIdx
	ps.Allocated++
	return nil
}

// allocatedNodes returns the slice of nodes the player has unlocked.
func (ps *PlayerSkills) allocatedNodes(class Class) []SkillNode {
	if ps == nil || ps.BranchIndex < 0 || ps.Allocated == 0 {
		return nil
	}
	branches := GetBranches(class)
	branch := branches[ps.BranchIndex]
	return branch.Nodes[:ps.Allocated]
}

// sumEffect sums values of nodes matching the given effect.
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
