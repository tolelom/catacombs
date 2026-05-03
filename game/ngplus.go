package game

import "math/rand"

// Curse is an end-game handicap with an upside, applied at NG+ start.
type Curse struct {
	ID   string
	Name string
	Desc string
	// Multipliers (1.0 = no change). Applied at AddPlayer / spawn.
	HPMult        float64
	ATKMult       float64
	DEFMult       float64
	SkillUseDelta int     // added to per-combat skill uses (negative reduces)
	GoldMult      float64 // applied to gold rewards
}

// AvailableCurses lists the 5 NG+ curses; one is rolled at NG+ start.
var AvailableCurses = []Curse{
	{ID: "frail", Name: "빈약", Desc: "MaxHP -20%, ATK +30%", HPMult: 0.80, ATKMult: 1.30, DEFMult: 1.0, GoldMult: 1.0},
	{ID: "naked", Name: "무방비", Desc: "DEF -50%, 골드 +50%", HPMult: 1.0, ATKMult: 1.0, DEFMult: 0.50, GoldMult: 1.50},
	{ID: "silent", Name: "침묵", Desc: "스킬 -1, 데미지 +20%", HPMult: 1.0, ATKMult: 1.20, DEFMult: 1.0, SkillUseDelta: -1, GoldMult: 1.0},
	{ID: "weak", Name: "약체", Desc: "ATK -20%, DEF +50%", HPMult: 1.0, ATKMult: 0.80, DEFMult: 1.50, GoldMult: 1.0},
	{ID: "cruel", Name: "잔혹", Desc: "적 광폭화 임계값 70%, 골드 +50%", HPMult: 1.0, ATKMult: 1.0, DEFMult: 1.0, GoldMult: 1.50},
}

// PickCurse returns one curse using the supplied RNG.
func PickCurse(rng *rand.Rand) Curse {
	if rng == nil {
		return AvailableCurses[0]
	}
	return AvailableCurses[rng.Intn(len(AvailableCurses))]
}
