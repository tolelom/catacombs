package combat

import "github.com/tolelom/catacombs/entity"

type ComboAction struct {
	Class      entity.Class
	ActionType string // "attack", "skill", "item"
}

type ComboEffect struct {
	DamageMultiplier float64 // multiplied onto each AttackIntent.Multiplier
	BonusDamage      int     // added to each AttackIntent.PlayerATK
	HealAll          int     // heal all players after resolution
	Message          string  // shown in combat log
}

type ComboDef struct {
	Name     string
	Required []ComboAction
	Effect   ComboEffect
}

var comboDefs = []ComboDef{
	{
		Name: "Ice Shatter",
		Required: []ComboAction{
			{Class: entity.ClassMage, ActionType: "skill"},
			{Class: entity.ClassWarrior, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.5, Message: "💥 ICE SHATTER! Frozen enemies shatter!"},
	},
	{
		Name: "Holy Assault",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "skill"},
			{Class: entity.ClassWarrior, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.3, HealAll: 10, Message: "✨ HOLY ASSAULT! Blessed strikes heal the party!"},
	},
	{
		Name: "Shadow Strike",
		Required: []ComboAction{
			{Class: entity.ClassRogue, ActionType: "skill"},
			{Class: entity.ClassMage, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.4, Message: "🗡️ SHADOW STRIKE! Magical shadows amplify the attack!"},
	},
	{
		Name: "Full Assault",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "attack"},
			{Class: entity.ClassMage, ActionType: "attack"},
			{Class: entity.ClassRogue, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.3, BonusDamage: 5, Message: "⚔️ FULL ASSAULT! Combined attack overwhelms!"},
	},
	{
		Name: "Restoration",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "skill"},
			{Class: entity.ClassRogue, ActionType: "item"},
		},
		Effect: ComboEffect{HealAll: 20, Message: "💚 RESTORATION! Combined healing surges!"},
	},
}

func DetectCombos(actions map[string]ComboAction) []ComboDef {
	var triggered []ComboDef
	for _, combo := range comboDefs {
		if matchesCombo(combo.Required, actions) {
			triggered = append(triggered, combo)
		}
	}
	return triggered
}

func matchesCombo(required []ComboAction, actions map[string]ComboAction) bool {
	used := make(map[string]bool)
	for _, req := range required {
		found := false
		for id, act := range actions {
			if !used[id] && act.Class == req.Class && act.ActionType == req.ActionType {
				used[id] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
