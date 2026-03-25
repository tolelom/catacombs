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
		Effect: ComboEffect{DamageMultiplier: 1.5, Message: "💥 ICE SHATTER! 동결된 적이 산산조각!"},
	},
	{
		Name: "Holy Assault",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "skill"},
			{Class: entity.ClassWarrior, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.3, HealAll: 10, Message: "✨ HOLY ASSAULT! 축복받은 공격이 파티를 치유!"},
	},
	{
		Name: "Shadow Strike",
		Required: []ComboAction{
			{Class: entity.ClassRogue, ActionType: "skill"},
			{Class: entity.ClassMage, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.4, Message: "🗡️ SHADOW STRIKE! 마법의 그림자가 공격을 증폭!"},
	},
	{
		Name: "Full Assault",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "attack"},
			{Class: entity.ClassMage, ActionType: "attack"},
			{Class: entity.ClassRogue, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.3, BonusDamage: 5, Message: "⚔️ FULL ASSAULT! 합동 공격으로 압도!"},
	},
	{
		Name: "Restoration",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "skill"},
			{Class: entity.ClassRogue, ActionType: "item"},
		},
		Effect: ComboEffect{HealAll: 20, Message: "💚 RESTORATION! 합동 치유가 폭발적으로 발동!"},
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
