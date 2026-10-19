package entity

import (
	"errors"
	"fmt"
)

type SkillEffect int

const (
	EffectATKBoost SkillEffect = iota
	EffectDEFBoost
	EffectMaxHPBoost
	EffectSkillPower
	EffectCritChance
	EffectHealBoost
)

type SkillNode struct {
	Name   string
	Effect SkillEffect
	Value  int
}

func (n SkillNode) EffectLabel() string {
	switch n.Effect {
	case EffectATKBoost:
		return fmt.Sprintf("+%d ATK", n.Value)
	case EffectDEFBoost:
		return fmt.Sprintf("+%d DEF", n.Value)
	case EffectMaxHPBoost:
		return fmt.Sprintf("+%d 최대HP", n.Value)
	case EffectSkillPower:
		return fmt.Sprintf("스킬위력 +%d", n.Value)
	case EffectCritChance:
		return fmt.Sprintf("치명타 +%d%%", n.Value)
	case EffectHealBoost:
		return fmt.Sprintf("힐 +%d", n.Value)
	}
	return fmt.Sprintf("+%d", n.Value)
}

type SkillTree struct {
	Name string
	T1   SkillNode
	T2   [2]SkillNode
	T3   SkillNode
	T4   [2]SkillNode
}

type PlayerSkills struct {
	BranchIndex int
	Points      int
	Allocated   int
	Choices     string
}

func NewPlayerSkills() *PlayerSkills {
	return &PlayerSkills{BranchIndex: -1}
}

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

func GetBranches(class Class) [2]SkillTree {
	return branchTrees[class]
}

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
	if ps.BranchIndex == -1 {
		ps.BranchIndex = branchIdx
	}
	switch ps.Allocated {
	case 0:
		ps.Allocated = 1
	case 1:
		if choice != 'A' && choice != 'B' {
			return errors.New("T2 requires choice 'A' or 'B'")
		}
		ps.Choices += string(choice)
		ps.Allocated = 2
	case 2:
		ps.Allocated = 3
	case 3:
		if choice != 'A' && choice != 'B' {
			return errors.New("T4 capstone requires choice 'A' or 'B'")
		}
		ps.Choices += string(choice)
		ps.Allocated = 4
	}
	return nil
}

func (ps *PlayerSkills) NextIsBranch() bool {
	if ps == nil || ps.Allocated >= 4 {
		return false
	}
	return ps.Allocated == 1 || ps.Allocated == 3
}

func choiceToIdx(c byte) int {
	if c == 'B' {
		return 1
	}
	return 0
}

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

func (ps *PlayerSkills) GetATKBonus(class Class) int {
	return ps.sumEffect(class, EffectATKBoost)
}

func (ps *PlayerSkills) GetDEFBonus(class Class) int {
	return ps.sumEffect(class, EffectDEFBoost)
}

func (ps *PlayerSkills) GetMaxHPBonus(class Class) int {
	return ps.sumEffect(class, EffectMaxHPBoost)
}

func (ps *PlayerSkills) GetSkillPower(class Class) int {
	return ps.sumEffect(class, EffectSkillPower)
}

func (ps *PlayerSkills) GetCritChance(class Class) int {
	return ps.sumEffect(class, EffectCritChance)
}

func (ps *PlayerSkills) GetHealBoost(class Class) int {
	return ps.sumEffect(class, EffectHealBoost)
}
