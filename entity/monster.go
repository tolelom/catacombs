package entity

import "math"

type MonsterType int

const (
	MonsterSlime MonsterType = iota
	MonsterSkeleton
	MonsterOrc
	MonsterDarkKnight
	MonsterBoss5
	MonsterBoss10
	MonsterBoss15
	MonsterBoss20
)

type monsterBase struct {
	Name     string
	HP, ATK, DEF int
	MinFloor int
	IsBoss   bool
}

var monsterDefs = map[MonsterType]monsterBase{
	MonsterSlime:      {"Slime", 20, 5, 1, 1, false},
	MonsterSkeleton:   {"Skeleton", 35, 10, 4, 3, false},
	MonsterOrc:        {"Orc", 55, 14, 6, 6, false},
	MonsterDarkKnight: {"Dark Knight", 80, 18, 10, 12, false},
	MonsterBoss5:      {"Guardian", 150, 15, 8, 5, true},
	MonsterBoss10:     {"Warden", 250, 22, 12, 10, true},
	MonsterBoss15:     {"Overlord", 400, 30, 16, 15, true},
	MonsterBoss20:     {"Archlich", 600, 40, 20, 20, true},
}

type Monster struct {
	Name        string
	Type        MonsterType
	HP, MaxHP   int
	ATK, DEF    int
	IsBoss      bool
	TauntTarget bool
	TauntTurns  int
}

func NewMonster(mt MonsterType, floor int) *Monster {
	base := monsterDefs[mt]
	scale := 1.0
	if !base.IsBoss && floor > base.MinFloor {
		scale = math.Pow(1.15, float64(floor-base.MinFloor))
	}
	hp := int(math.Round(float64(base.HP) * scale))
	atk := int(math.Round(float64(base.ATK) * scale))
	return &Monster{
		Name:   base.Name,
		Type:   mt,
		HP:     hp,
		MaxHP:  hp,
		ATK:    atk,
		DEF:    base.DEF,
		IsBoss: base.IsBoss,
	}
}

func (m *Monster) TakeDamage(dmg int) {
	m.HP -= dmg
	if m.HP < 0 {
		m.HP = 0
	}
}

func (m *Monster) IsDead() bool {
	return m.HP <= 0
}

func (m *Monster) TickTaunt() {
	if m.TauntTurns > 0 {
		m.TauntTurns--
		if m.TauntTurns == 0 {
			m.TauntTarget = false
		}
	}
}
