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
	MonsterMiniBoss5
	MonsterMiniBoss10
	MonsterMiniBoss15
	MonsterMiniBoss20
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
	MonsterMiniBoss5:  {"Guardian's Herald", 90, 9, 5, 4, false},
	MonsterMiniBoss10: {"Warden's Shadow", 150, 13, 7, 9, false},
	MonsterMiniBoss15: {"Overlord's Lieutenant", 240, 18, 10, 14, false},
	MonsterMiniBoss20: {"Archlich's Harbinger", 360, 24, 12, 19, false},
}

type BossPattern int

const (
	PatternNone    BossPattern = iota
	PatternAoE                 // every 3 turns AoE
	PatternPoison              // applies poison
	PatternBurn                // applies burn to random player
	PatternHeal                // heals self
	PatternFreeze              // applies freeze to all players
)

type Monster struct {
	Name        string
	Type        MonsterType
	HP, MaxHP   int
	ATK, DEF    int
	IsBoss      bool
	IsMiniBoss  bool
	IsElite     bool
	ElitePrefix ElitePrefixType
	TauntTarget bool
	TauntTurns  int
	Pattern     BossPattern
}

func NewMonster(mt MonsterType, floor int, scaling float64) *Monster {
	base := monsterDefs[mt]
	isMiniBoss := mt == MonsterMiniBoss5 || mt == MonsterMiniBoss10 ||
		mt == MonsterMiniBoss15 || mt == MonsterMiniBoss20
	scale := 1.0
	if !base.IsBoss && !isMiniBoss && floor > base.MinFloor {
		scale = math.Pow(scaling, float64(floor-base.MinFloor))
	}
	hp := int(math.Round(float64(base.HP) * scale))
	atk := int(math.Round(float64(base.ATK) * scale))
	def := int(math.Round(float64(base.DEF) * scale))
	return &Monster{
		Name:       base.Name,
		Type:       mt,
		HP:         hp,
		MaxHP:      hp,
		ATK:        atk,
		DEF:        def,
		IsBoss:     base.IsBoss,
		IsMiniBoss: isMiniBoss,
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

// BaseName returns the unprefixed monster name (without elite affixes), so
// the codex can record one entry per monster type instead of a separate
// entry for every elite variant.
func (m *Monster) BaseName() string {
	if def, ok := monsterDefs[m.Type]; ok {
		return def.Name
	}
	return m.Name
}
