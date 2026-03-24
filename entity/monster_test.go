package entity

import (
	"testing"
	"math"
)

func TestMonsterScaling(t *testing.T) {
	slime := NewMonster(MonsterSlime, 1)
	if slime.HP != 20 || slime.ATK != 5 {
		t.Errorf("Slime floor 1: got HP=%d ATK=%d, want HP=20 ATK=5", slime.HP, slime.ATK)
	}
	slimeF3 := NewMonster(MonsterSlime, 3)
	expectedHP := int(math.Round(20 * math.Pow(1.15, 2)))
	if slimeF3.HP != expectedHP {
		t.Errorf("Slime floor 3: got HP=%d, want %d", slimeF3.HP, expectedHP)
	}
}

func TestBossStats(t *testing.T) {
	boss := NewMonster(MonsterBoss5, 5)
	if boss.HP != 150 || boss.ATK != 15 || boss.DEF != 8 {
		t.Errorf("Boss5: got HP=%d ATK=%d DEF=%d, want 150/15/8", boss.HP, boss.ATK, boss.DEF)
	}
}

func TestMonsterDEFScaling(t *testing.T) {
	// Slime base DEF=1, minFloor=1. At floor 5, scale = 1.15^4 ≈ 1.749
	m := NewMonster(MonsterSlime, 5)
	if m.DEF <= 1 {
		t.Errorf("Slime DEF at floor 5 should be scaled above base 1, got %d", m.DEF)
	}
	// Boss DEF should NOT scale
	boss := NewMonster(MonsterBoss5, 5)
	if boss.DEF != 8 {
		t.Errorf("Boss5 DEF should be base 8, got %d", boss.DEF)
	}
}

func TestTickTaunt(t *testing.T) {
	m := &Monster{Name: "Orc", HP: 50, TauntTarget: true, TauntTurns: 2}
	m.TickTaunt()
	if m.TauntTurns != 1 || !m.TauntTarget {
		t.Error("should still be taunted with 1 turn left")
	}
	m.TickTaunt()
	if m.TauntTurns != 0 || m.TauntTarget {
		t.Error("taunt should be cleared at 0")
	}
}

func TestMonsterAtMinFloor(t *testing.T) {
	// Slime at floor 1 (minFloor=1) should have base stats
	m := NewMonster(MonsterSlime, 1)
	if m.HP != 20 || m.ATK != 5 || m.DEF != 1 {
		t.Errorf("Slime at min floor should be base stats, got HP=%d ATK=%d DEF=%d", m.HP, m.ATK, m.DEF)
	}
}
