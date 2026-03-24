package combat

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func TestCalcDamage(t *testing.T) {
	dmg := CalcDamage(12, 1, 1.0)
	if dmg < 9 || dmg > 13 {
		t.Errorf("Damage out of expected range: got %d, want 9~13", dmg)
	}
}

func TestCalcDamageMinimum(t *testing.T) {
	dmg := CalcDamage(1, 100, 1.0)
	if dmg != 1 {
		t.Errorf("Minimum damage: got %d, want 1", dmg)
	}
}

func TestCoopBonus(t *testing.T) {
	attackers := []AttackIntent{
		{PlayerATK: 12, TargetIdx: 0, Multiplier: 1.0, IsAoE: false},
		{PlayerATK: 15, TargetIdx: 0, Multiplier: 1.0, IsAoE: false},
	}
	results := ResolveAttacks(attackers, []*entity.Monster{entity.NewMonster(entity.MonsterSlime, 1)})
	if !results[1].CoopApplied {
		t.Error("Second attacker should get co-op bonus")
	}
}

func TestAoENoCoopBonus(t *testing.T) {
	attackers := []AttackIntent{
		{PlayerATK: 12, TargetIdx: 0, Multiplier: 1.0, IsAoE: false},
		{PlayerATK: 20, TargetIdx: -1, Multiplier: 0.8, IsAoE: true},
	}
	monsters := []*entity.Monster{
		entity.NewMonster(entity.MonsterSlime, 1),
		entity.NewMonster(entity.MonsterSlime, 1),
	}
	results := ResolveAttacks(attackers, monsters)
	if results[0].CoopApplied {
		t.Error("AoE should not trigger co-op bonus")
	}
}

func TestMonsterAITauntDeadWarrior(t *testing.T) {
	warrior := entity.NewPlayer("Tank", entity.ClassWarrior)
	warrior.TakeDamage(warrior.HP) // kill warrior
	mage := entity.NewPlayer("Mage", entity.ClassMage)

	m := &entity.Monster{Name: "Orc", HP: 50, ATK: 10, DEF: 5, TauntTarget: true, TauntTurns: 2}
	idx, isAoE := MonsterAI(m, []*entity.Player{warrior, mage}, 1)

	if isAoE {
		t.Error("should not AoE")
	}
	if idx != 1 {
		t.Errorf("expected target mage at index 1, got %d", idx)
	}
	if m.TauntTarget {
		t.Error("TauntTarget should be cleared when warrior is dead")
	}
}

func TestFleeChance(t *testing.T) {
	successes := 0
	for i := 0; i < 100; i++ {
		if AttemptFlee() {
			successes++
		}
	}
	if successes < 20 || successes > 80 {
		t.Errorf("Flee success rate suspicious: %d/100", successes)
	}
}
