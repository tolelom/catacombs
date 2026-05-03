package game

import (
	"crypto/sha256"
	"encoding/binary"
)

func DailySeed(date string) int64 {
	h := sha256.Sum256([]byte("catacombs:" + date))
	return int64(binary.BigEndian.Uint64(h[:8]))
}

// DailyModifier is a per-day game-wide tweak applied to daily challenge runs.
// Deterministic from the date string so all players today face the same one.
type DailyModifier struct {
	ID            string
	Name          string
	Desc          string
	GoldMult      float64 // 1.0 = no change
	MonsterHPMult float64 // 1.0 = no change (e.g. 0.75 for slow_enemy weakness)
	MonsterATKMult float64
	SkillUseDelta int
	StartGoldBonus int
}

var dailyModifiers = []DailyModifier{
	{ID: "double_gold", Name: "이중 보상", Desc: "골드 보상 2배", GoldMult: 2.0, MonsterHPMult: 1.0, MonsterATKMult: 1.0},
	{ID: "fragile", Name: "부서지는 갑옷", Desc: "적 HP -25%, 적 ATK +25%", GoldMult: 1.0, MonsterHPMult: 0.75, MonsterATKMult: 1.25},
	{ID: "slow_enemy", Name: "둔한 적", Desc: "적 ATK -20%", GoldMult: 1.0, MonsterHPMult: 1.0, MonsterATKMult: 0.80},
	{ID: "skilled", Name: "숙련", Desc: "전투당 스킬 +1", GoldMult: 1.0, MonsterHPMult: 1.0, MonsterATKMult: 1.0, SkillUseDelta: 1},
	{ID: "rich_start", Name: "풍족한 시작", Desc: "시작 골드 +200", GoldMult: 1.0, MonsterHPMult: 1.0, MonsterATKMult: 1.0, StartGoldBonus: 200},
}

// PickDailyModifier returns the deterministic daily modifier for a date.
func PickDailyModifier(date string) DailyModifier {
	if date == "" {
		return dailyModifiers[0]
	}
	idx := int(uint(DailySeed(date+":mod")) % uint(len(dailyModifiers)))
	return dailyModifiers[idx]
}
