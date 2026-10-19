package combat

import "github.com/tolelom/catacombs/entity"

type ComboAction struct {
	Class      entity.Class
	ActionType string
}

type ComboEffect struct {
	DamageMultiplier float64
	BonusDamage      int
	HealAll          int
	Message          string

	HealMultiplier         float64
	WarriorBulwarkOverride float64
	GuaranteeCritNextTurn  bool
	InvincibleHealTarget   bool
	EnemyMissNextTurn      bool
	PoisonNextCombat       bool
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
	{
		Name: "Iron Aegis",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "coop"},
			{Class: entity.ClassMage, ActionType: "coop"},
		},
		Effect: ComboEffect{WarriorBulwarkOverride: 0.75, Message: "🛡️ IRON AEGIS! 강철결계가 파티 전원을 감쌉니다!"},
	},
	{
		Name: "Crusader",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "coop"},
			{Class: entity.ClassHealer, ActionType: "coop"},
		},
		Effect: ComboEffect{HealMultiplier: 2.0, Message: "⚔️✨ CRUSADER! 성스러운 회복!"},
	},
	{
		Name: "Expose Weakness",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "coop"},
			{Class: entity.ClassRogue, ActionType: "coop"},
		},
		Effect: ComboEffect{GuaranteeCritNextTurn: true, Message: "🎯 EXPOSE WEAKNESS! 다음 턴 모든 공격 치명타!"},
	},
	{
		Name: "Sanctuary",
		Required: []ComboAction{
			{Class: entity.ClassMage, ActionType: "coop"},
			{Class: entity.ClassHealer, ActionType: "coop"},
		},
		Effect: ComboEffect{InvincibleHealTarget: true, Message: "✨🛡️ SANCTUARY! 회복 대상 1턴 무적!"},
	},
	{
		Name: "Mirage",
		Required: []ComboAction{
			{Class: entity.ClassMage, ActionType: "coop"},
			{Class: entity.ClassRogue, ActionType: "coop"},
		},
		Effect: ComboEffect{EnemyMissNextTurn: true, Message: "🌫️ MIRAGE! 다음 턴 적 공격 50% 빗나감!"},
	},
	{
		Name: "Venomous Edge",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "coop"},
			{Class: entity.ClassRogue, ActionType: "coop"},
		},
		Effect: ComboEffect{PoisonNextCombat: true, Message: "🐍 VENOMOUS EDGE! 다음 전투 적들에게 독!"},
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
