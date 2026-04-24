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
	results := ResolveAttacks(attackers, []*entity.Monster{entity.NewMonster(entity.MonsterSlime, 1, 1.15)}, 0.10)
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
		entity.NewMonster(entity.MonsterSlime, 1, 1.15),
		entity.NewMonster(entity.MonsterSlime, 1, 1.15),
	}
	results := ResolveAttacks(attackers, monsters, 0.10)
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

func TestMonsterAISkipsFledPlayers(t *testing.T) {
	p1 := entity.NewPlayer("Fled", entity.ClassWarrior)
	p1.Fled = true
	p2 := entity.NewPlayer("Alive", entity.ClassMage)

	m := &entity.Monster{Name: "Orc", HP: 50, ATK: 10}
	// Run many iterations to cover the random branch
	for i := 0; i < 50; i++ {
		idx, isAoE := MonsterAI(m, []*entity.Player{p1, p2}, 1)
		if isAoE {
			continue
		}
		if idx == 0 {
			t.Fatalf("iteration %d: monster targeted fled player at index 0", i)
		}
	}
}

func TestMonsterAITauntSkipsFledWarrior(t *testing.T) {
	warrior := entity.NewPlayer("Tank", entity.ClassWarrior)
	warrior.Fled = true
	mage := entity.NewPlayer("Mage", entity.ClassMage)

	m := &entity.Monster{Name: "Orc", HP: 50, ATK: 10, DEF: 5, TauntTarget: true, TauntTurns: 2}
	idx, _ := MonsterAI(m, []*entity.Player{warrior, mage}, 1)

	if idx != 1 {
		t.Errorf("taunt should skip fled warrior and target mage, got idx=%d", idx)
	}
	if m.TauntTarget {
		t.Error("TauntTarget should be cleared when warrior is unavailable (fled)")
	}
}

func TestFleeChance(t *testing.T) {
	successes := 0
	for i := 0; i < 100; i++ {
		if AttemptFlee(0.50) {
			successes++
		}
	}
	if successes < 20 || successes > 80 {
		t.Errorf("Flee success rate suspicious: %d/100", successes)
	}
}

func TestMonsterAIBossAoE(t *testing.T) {
	boss := &entity.Monster{Name: "Boss", HP: 100, IsBoss: true}
	players := []*entity.Player{entity.NewPlayer("P1", entity.ClassWarrior)}

	// Turn 0 should NOT AoE
	_, isAoE := MonsterAI(boss, players, 0)
	if isAoE {
		t.Error("boss should not AoE on turn 0")
	}
	// Turn 3 should AoE
	_, isAoE = MonsterAI(boss, players, 3)
	if !isAoE {
		t.Error("boss should AoE on turn 3")
	}
	// Turn 6 should AoE
	_, isAoE = MonsterAI(boss, players, 6)
	if !isAoE {
		t.Error("boss should AoE on turn 6")
	}
}

func TestMonsterAILowestHP(t *testing.T) {
	p1 := entity.NewPlayer("Tank", entity.ClassWarrior) // 120 HP
	p2 := entity.NewPlayer("Mage", entity.ClassMage)     // 70 HP
	p2.HP = 10                                            // very low

	// Run many times — at least some should target p2 (30% chance)
	targetedLow := 0
	for i := 0; i < 100; i++ {
		m := &entity.Monster{Name: "Orc", HP: 50}
		idx, _ := MonsterAI(m, []*entity.Player{p1, p2}, 1)
		if idx == 1 {
			targetedLow++
		}
	}
	// Should target low HP player roughly 30% of time
	if targetedLow < 10 || targetedLow > 60 {
		t.Errorf("lowest HP targeting out of expected range: %d/100", targetedLow)
	}
}

func TestCalcDamageWithMultiplier(t *testing.T) {
	// AoE multiplier 0.8: ATK=20, DEF=5, mult=0.8 → base = 20*0.8 - 5 = 11
	// Range: 11 * 0.85 to 11 * 1.15 = ~9.35 to ~12.65
	for i := 0; i < 50; i++ {
		dmg := CalcDamage(20, 5, 0.8)
		if dmg < 9 || dmg > 13 {
			t.Errorf("AoE damage %d out of expected range 9-13", dmg)
		}
	}
}

func TestCalcDamageHighDEF(t *testing.T) {
	// When DEF > ATK*mult, should deal minimum 1 damage
	dmg := CalcDamage(5, 100, 1.0)
	if dmg != 1 {
		t.Errorf("expected minimum damage 1, got %d", dmg)
	}
}
