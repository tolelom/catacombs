package combat

import (
	"math"
	"math/rand"

	"github.com/tolelom/catacombs/entity"
)

// CalcDamage: max(1, ATK * multiplier - DEF) * random(0.85~1.15)
func CalcDamage(atk, def int, multiplier float64) int {
	base := float64(atk)*multiplier - float64(def)
	if base < 1 {
		base = 1
	}
	randomFactor := 0.85 + rand.Float64()*0.30
	return int(math.Round(base * randomFactor))
}

type AttackIntent struct {
	PlayerATK  int
	TargetIdx  int
	Multiplier float64
	IsAoE      bool
}

type AttackResult struct {
	TargetIdx   int
	Damage      int
	CoopApplied bool
	IsAoE       bool
}

func ResolveAttacks(intents []AttackIntent, monsters []*entity.Monster) []AttackResult {
	targetCount := make(map[int]int)
	targetOrder := make(map[int]int)
	for i, intent := range intents {
		if !intent.IsAoE {
			targetCount[intent.TargetIdx]++
			if _, ok := targetOrder[intent.TargetIdx]; !ok {
				targetOrder[intent.TargetIdx] = i
			}
		}
	}

	results := make([]AttackResult, len(intents))
	for i, intent := range intents {
		if intent.IsAoE {
			totalDmg := 0
			for _, m := range monsters {
				if !m.IsDead() {
					dmg := CalcDamage(intent.PlayerATK, m.DEF, intent.Multiplier)
					m.TakeDamage(dmg)
					totalDmg += dmg
				}
			}
			results[i] = AttackResult{TargetIdx: -1, Damage: totalDmg, IsAoE: true}
		} else {
			if intent.TargetIdx < 0 || intent.TargetIdx >= len(monsters) {
				continue
			}
			m := monsters[intent.TargetIdx]
			dmg := CalcDamage(intent.PlayerATK, m.DEF, intent.Multiplier)
			coopApplied := false
			if targetCount[intent.TargetIdx] >= 2 && targetOrder[intent.TargetIdx] != i {
				dmg = int(math.Round(float64(dmg) * 1.10))
				coopApplied = true
			}
			m.TakeDamage(dmg)
			results[i] = AttackResult{
				TargetIdx:   intent.TargetIdx,
				Damage:      dmg,
				CoopApplied: coopApplied,
			}
		}
	}
	return results
}

func AttemptFlee() bool {
	return rand.Float64() < 0.5
}

func MonsterAI(m *entity.Monster, players []*entity.Player, turnNumber int) (targetIdx int, isAoE bool) {
	if m.IsBoss && turnNumber%3 == 0 {
		return -1, true
	}
	if m.TauntTarget {
		for i, p := range players {
			if !p.IsDead() && p.Class == entity.ClassWarrior {
				return i, false
			}
		}
		// No living warrior found — clear taunt
		m.TauntTarget = false
		m.TauntTurns = 0
	}
	if rand.Float64() < 0.3 {
		minHP := int(^uint(0) >> 1)
		minIdx := -1
		for i, p := range players {
			if !p.IsDead() && p.HP < minHP {
				minHP = p.HP
				minIdx = i
			}
		}
		if minIdx >= 0 {
			return minIdx, false
		}
		// Fall through to default targeting if no alive player found
	}
	for i, p := range players {
		if !p.IsDead() {
			return i, false
		}
	}
	return 0, false
}
