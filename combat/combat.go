package combat

import (
	"math"
	"math/rand"

	"github.com/tolelom/catacombs/entity"
)

func rngOrDefault(rng *rand.Rand) *rand.Rand {
	if rng != nil {
		return rng
	}
	return defaultRng
}

var defaultRng = rand.New(rand.NewSource(1))

const MinDamageFraction = 0.15

func CalcDamage(rng *rand.Rand, atk, def int, multiplier float64) int {
	r := rngOrDefault(rng)
	raw := float64(atk) * multiplier
	base := raw - float64(def)
	if floorDmg := raw * MinDamageFraction; base < floorDmg {
		base = floorDmg
	}
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
	Missed      bool
	Glanced     bool
}

func ResolveAttacks(rng *rand.Rand, intents []AttackIntent, monsters []*entity.Monster, coopBonus float64) []AttackResult {
	attackerCount, firstAttacker := countSingleTargetAttackers(intents)

	results := make([]AttackResult, len(intents))
	for i, intent := range intents {
		if intent.IsAoE {
			results[i] = resolveAoEAttack(rng, intent, monsters)
			continue
		}
		coopApplies := attackerCount[intent.TargetIdx] >= 2 && firstAttacker[intent.TargetIdx] != i
		results[i] = resolveSingleTargetAttack(rng, intent, monsters, coopApplies, coopBonus)
	}
	return results
}

func countSingleTargetAttackers(intents []AttackIntent) (attackerCount, firstAttacker map[int]int) {
	attackerCount = make(map[int]int)
	firstAttacker = make(map[int]int)
	for i, intent := range intents {
		if !intent.IsAoE {
			attackerCount[intent.TargetIdx]++
			if _, ok := firstAttacker[intent.TargetIdx]; !ok {
				firstAttacker[intent.TargetIdx] = i
			}
		}
	}
	return attackerCount, firstAttacker
}

func resolveAoEAttack(rng *rand.Rand, intent AttackIntent, monsters []*entity.Monster) AttackResult {
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
	return AttackResult{TargetIdx: -1, Damage: totalDmg, IsAoE: true}
}

func resolveSingleTargetAttack(rng *rand.Rand, intent AttackIntent, monsters []*entity.Monster, coopApplies bool, coopBonus float64) AttackResult {
	if intent.TargetIdx < 0 || intent.TargetIdx >= len(monsters) {
		return AttackResult{TargetIdx: -1}
	}
	m := monsters[intent.TargetIdx]
	if m.IsDead() {
		return AttackResult{TargetIdx: -1}
	}
	r := rngOrDefault(rng)
	if !m.IsBoss && !m.IsMiniBoss {
		roll := r.Float64()
		if roll < 0.05 {
			return AttackResult{TargetIdx: intent.TargetIdx, Missed: true}
		}
		if roll < 0.10 {
			dmg := CalcDamage(rng, intent.PlayerATK, m.DEF, intent.Multiplier*0.5)
			m.TakeDamage(dmg)
			return AttackResult{TargetIdx: intent.TargetIdx, Damage: dmg, Glanced: true}
		}
	}
	dmg := CalcDamage(rng, intent.PlayerATK, m.DEF, intent.Multiplier)
	if coopApplies {
		dmg = int(math.Round(float64(dmg) * (1.0 + coopBonus)))
	}
	if m.Vulnerable {
		dmg = int(math.Round(float64(dmg) * 1.5))
	}
	m.TakeDamage(dmg)
	return AttackResult{
		TargetIdx:   intent.TargetIdx,
		Damage:      dmg,
		CoopApplied: coopApplies,
	}
}

func AttemptFlee(rng *rand.Rand, fleeChance float64) bool {
	return rngOrDefault(rng).Float64() < fleeChance
}

func MonsterAI(rng *rand.Rand, m *entity.Monster, players []*entity.Player, turnNumber int) (targetIdx int, isAoE bool) {
	r := rngOrDefault(rng)
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
	}
	for i, p := range players {
		if !p.IsOut() {
			return i, false
		}
	}
	return -1, false
}
