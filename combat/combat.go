package combat

import (
	"math"
	"math/rand"

	"github.com/tolelom/catacombs/entity"
)

// rngOrDefault returns the supplied rng, or a process-global fallback when
// callers (mostly tests) pass nil. Production paths thread the per-session
// RNG through so daily challenges remain deterministic.
func rngOrDefault(rng *rand.Rand) *rand.Rand {
	if rng != nil {
		return rng
	}
	return defaultRng
}

var defaultRng = rand.New(rand.NewSource(1))

// CalcDamage: max(1, ATK * multiplier - DEF) * random(0.85~1.15)
func CalcDamage(rng *rand.Rand, atk, def int, multiplier float64) int {
	r := rngOrDefault(rng)
	base := float64(atk)*multiplier - float64(def)
	if base < 1 {
		base = 1
	}
	randomFactor := 0.85 + r.Float64()*0.30
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
	Missed      bool // monster dodged
	Glanced     bool // partial hit (50% damage)
}

func ResolveAttacks(rng *rand.Rand, intents []AttackIntent, monsters []*entity.Monster, coopBonus float64) []AttackResult {
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
					dmg := CalcDamage(rng, intent.PlayerATK, m.DEF, intent.Multiplier)
					if m.Vulnerable {
						dmg = int(math.Round(float64(dmg) * 1.5))
					}
					m.TakeDamage(dmg)
					totalDmg += dmg
				}
			}
			results[i] = AttackResult{TargetIdx: -1, Damage: totalDmg, IsAoE: true}
		} else {
			if intent.TargetIdx < 0 || intent.TargetIdx >= len(monsters) {
				results[i] = AttackResult{TargetIdx: -1} // mark as invalid
				continue
			}
			m := monsters[intent.TargetIdx]
			if m.IsDead() {
				results[i] = AttackResult{TargetIdx: -1}
				continue
			}
			r := rngOrDefault(rng)
			// 5% dodge, 5% glance — non-elite/non-boss only (so bosses stay reliable)
			if !m.IsBoss && !m.IsMiniBoss {
				roll := r.Float64()
				if roll < 0.05 {
					results[i] = AttackResult{TargetIdx: intent.TargetIdx, Missed: true}
					continue
				}
				if roll < 0.10 {
					dmg := CalcDamage(rng, intent.PlayerATK, m.DEF, intent.Multiplier*0.5)
					m.TakeDamage(dmg)
					results[i] = AttackResult{TargetIdx: intent.TargetIdx, Damage: dmg, Glanced: true}
					continue
				}
			}
			dmg := CalcDamage(rng, intent.PlayerATK, m.DEF, intent.Multiplier)
			coopApplied := false
			if targetCount[intent.TargetIdx] >= 2 && targetOrder[intent.TargetIdx] != i {
				dmg = int(math.Round(float64(dmg) * (1.0 + coopBonus)))
				coopApplied = true
			}
			if m.Vulnerable {
				dmg = int(math.Round(float64(dmg) * 1.5))
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

func AttemptFlee(rng *rand.Rand, fleeChance float64) bool {
	return rngOrDefault(rng).Float64() < fleeChance
}

func MonsterAI(rng *rand.Rand, m *entity.Monster, players []*entity.Player, turnNumber int) (targetIdx int, isAoE bool) {
	r := rngOrDefault(rng)
	// Phase 2 bosses AoE every 2 turns; base bosses + mini-bosses every 3.
	aoeFreq := 3
	if m.IsBoss && m.Phase >= 2 {
		aoeFreq = 2
	}
	if (m.IsBoss || m.IsMiniBoss) && turnNumber > 0 && turnNumber%aoeFreq == 0 {
		return -1, true
	}
	if m.TauntTarget {
		for i, p := range players {
			if !p.IsOut() && p.Class == entity.ClassWarrior {
				return i, false
			}
		}
		// No available warrior found — clear taunt
		m.TauntTarget = false
		m.TauntTurns = 0
	}
	if r.Float64() < 0.3 {
		minHP := int(^uint(0) >> 1)
		minIdx := -1
		for i, p := range players {
			if !p.IsOut() && p.HP < minHP {
				minHP = p.HP
				minIdx = i
			}
		}
		if minIdx >= 0 {
			return minIdx, false
		}
		// Fall through to default targeting if no available player found
	}
	for i, p := range players {
		if !p.IsOut() {
			return i, false
		}
	}
	return -1, false
}
