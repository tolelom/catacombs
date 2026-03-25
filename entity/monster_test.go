package entity

import (
	"testing"
	"math"
)

func TestMonsterScaling(t *testing.T) {
	slime := NewMonster(MonsterSlime, 1, 1.15)
	if slime.HP != 20 || slime.ATK != 5 {
		t.Errorf("Slime floor 1: got HP=%d ATK=%d, want HP=20 ATK=5", slime.HP, slime.ATK)
	}
	slimeF3 := NewMonster(MonsterSlime, 3, 1.15)
	expectedHP := int(math.Round(20 * math.Pow(1.15, 2)))
	if slimeF3.HP != expectedHP {
		t.Errorf("Slime floor 3: got HP=%d, want %d", slimeF3.HP, expectedHP)
	}
}

func TestBossStats(t *testing.T) {
	boss := NewMonster(MonsterBoss5, 5, 1.15)
	if boss.HP != 150 || boss.ATK != 15 || boss.DEF != 8 {
		t.Errorf("Boss5: got HP=%d ATK=%d DEF=%d, want 150/15/8", boss.HP, boss.ATK, boss.DEF)
	}
}

func TestMonsterDEFScaling(t *testing.T) {
	// Slime base DEF=1, minFloor=1. At floor 5, scale = 1.15^4 ≈ 1.749
	m := NewMonster(MonsterSlime, 5, 1.15)
	if m.DEF <= 1 {
		t.Errorf("Slime DEF at floor 5 should be scaled above base 1, got %d", m.DEF)
	}
	// Boss DEF should NOT scale
	boss := NewMonster(MonsterBoss5, 5, 1.15)
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

func TestMiniBossStats(t *testing.T) {
	tests := []struct {
		mt                     MonsterType
		name                   string
		wantHP, wantATK, wantDEF int
	}{
		{MonsterMiniBoss5, "Guardian's Herald", 90, 9, 5},
		{MonsterMiniBoss10, "Warden's Shadow", 150, 13, 7},
		{MonsterMiniBoss15, "Overlord's Lieutenant", 240, 18, 10},
		{MonsterMiniBoss20, "Archlich's Harbinger", 360, 24, 12},
	}
	for _, tc := range tests {
		m := NewMonster(tc.mt, tc.wantHP, 1.15) // floor doesn't matter, no scaling
		if m.Name != tc.name {
			t.Errorf("%v: name got %q, want %q", tc.mt, m.Name, tc.name)
		}
		if m.HP != tc.wantHP {
			t.Errorf("%v: HP got %d, want %d", tc.mt, m.HP, tc.wantHP)
		}
		if m.ATK != tc.wantATK {
			t.Errorf("%v: ATK got %d, want %d", tc.mt, m.ATK, tc.wantATK)
		}
		if m.DEF != tc.wantDEF {
			t.Errorf("%v: DEF got %d, want %d", tc.mt, m.DEF, tc.wantDEF)
		}
		if !m.IsMiniBoss {
			t.Errorf("%v: IsMiniBoss should be true", tc.mt)
		}
		if m.IsBoss {
			t.Errorf("%v: IsBoss should be false for mini-bosses", tc.mt)
		}
	}
}

func TestMonsterAtMinFloor(t *testing.T) {
	// Slime at floor 1 (minFloor=1) should have base stats
	m := NewMonster(MonsterSlime, 1, 1.15)
	if m.HP != 20 || m.ATK != 5 || m.DEF != 1 {
		t.Errorf("Slime at min floor should be base stats, got HP=%d ATK=%d DEF=%d", m.HP, m.ATK, m.DEF)
	}
}
