package entity

import (
	"fmt"
	"math"
)

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

// All bosses already AoE every third turn via combat.MonsterAI; the patterns
// below are the optional follow-up effect that fires after that AoE. Values
// are not persisted — Monster lives only in memory — so reordering is safe.
const (
	PatternNone    BossPattern = iota
	PatternPoison              // applies poison to all players after AoE
	PatternBurn                // applies burn to all players after AoE
	PatternHeal                // heals self for 10% MaxHP after AoE
	PatternFreeze              // applies freeze to all players after AoE
)

type Monster struct {
	Name              string
	Type              MonsterType
	HP, MaxHP         int
	ATK, DEF          int
	IsBoss            bool
	IsMiniBoss        bool
	IsElite           bool
	ElitePrefix       ElitePrefixType
	TauntTarget       bool
	TauntTurns        int
	Pattern           BossPattern
	Phase             int  // 1 = base; 2 = enraged (boss < 50% HP)
	PhaseTransitioned bool // true once Phase 2 effects have been applied
	Vulnerable        bool // 1-turn flag for B15 phase 2 (1.5x dmg taken)
	Effects           []ActiveEffect
}

// AddEffect refreshes an existing effect of the same type or appends a new one.
func (m *Monster) AddEffect(e ActiveEffect) {
	for i, existing := range m.Effects {
		if existing.Type == e.Type {
			m.Effects[i] = e
			return
		}
	}
	m.Effects = append(m.Effects, e)
}

// HasEffect reports whether the given status effect is active on the monster.
func (m *Monster) HasEffect(t StatusEffect) bool {
	for _, e := range m.Effects {
		if e.Type == t {
			return true
		}
	}
	return false
}

// TickEffects advances per-turn status effects on the monster, applying
// damage and decrementing duration. Returns log messages.
func (m *Monster) TickEffects() []string {
	var msgs []string
	remaining := m.Effects[:0]
	for i := 0; i < len(m.Effects); i++ {
		e := &m.Effects[i]
		switch e.Type {
		case StatusPoison:
			m.HP -= e.Value
			if m.HP < 0 {
				m.HP = 0
			}
			msgs = append(msgs, fmt.Sprintf("%s 독 피해 %d", m.Name, e.Value))
		case StatusBurn:
			m.HP -= e.Value
			if m.HP < 0 {
				m.HP = 0
			}
			msgs = append(msgs, fmt.Sprintf("%s 화상 피해 %d", m.Name, e.Value))
		case StatusBleed:
			m.HP -= e.Value
			if m.HP < 0 {
				m.HP = 0
			}
			msgs = append(msgs, fmt.Sprintf("%s 출혈 피해 %d", m.Name, e.Value))
			e.Value++
		}
		e.Duration--
		if e.Duration > 0 {
			remaining = append(remaining, *e)
		}
	}
	m.Effects = remaining
	return msgs
}

// CheckPhaseTransition returns true if a boss/mini-boss should enter phase 2
// this turn. Phase 2 default threshold is 50% HP. Pass thresholdPercent (e.g.
// 70 for the Cruel curse) to trigger earlier.
func (m *Monster) CheckPhaseTransition() bool {
	return m.CheckPhaseTransitionAt(50)
}

// CheckPhaseTransitionAt allows the caller to override the HP percentage at
// which Phase 2 fires.
func (m *Monster) CheckPhaseTransitionAt(thresholdPercent int) bool {
	if !m.IsBoss {
		return false
	}
	if m.PhaseTransitioned {
		return false
	}
	if m.HP*100 <= m.MaxHP*thresholdPercent {
		m.PhaseTransitioned = true
		m.Phase = 2
		return true
	}
	return false
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
		Phase:      1,
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
