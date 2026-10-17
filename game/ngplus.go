package game

import (
	"math/rand"

	"github.com/tolelom/catacombs/config"
)

type Curse struct {
	ID   string
	Name string
	Desc string

	HPMult        float64
	ATKMult       float64
	DEFMult       float64
	SkillUseDelta int
	GoldMult      float64
}

var AvailableCurses = []Curse{
	{ID: "frail", Name: "빈약", Desc: "MaxHP -20%, ATK +30%", HPMult: 0.80, ATKMult: 1.30, DEFMult: 1.0, GoldMult: 1.0},
	{ID: "naked", Name: "무방비", Desc: "DEF -50%, 골드 +50%", HPMult: 1.0, ATKMult: 1.0, DEFMult: 0.50, GoldMult: 1.50},
	{ID: "silent", Name: "침묵", Desc: "스킬 -1, 데미지 +20%", HPMult: 1.0, ATKMult: 1.20, DEFMult: 1.0, SkillUseDelta: -1, GoldMult: 1.0},
	{ID: "weak", Name: "약체", Desc: "ATK -20%, DEF +50%", HPMult: 1.0, ATKMult: 0.80, DEFMult: 1.50, GoldMult: 1.0},
	{ID: "cruel", Name: "잔혹", Desc: "적 광폭화 임계값 70%, 골드 +50%", HPMult: 1.0, ATKMult: 1.0, DEFMult: 1.0, GoldMult: 1.50},
}

func PickCurse(rng *rand.Rand) Curse {
	if rng == nil {
		return AvailableCurses[rand.Intn(len(AvailableCurses))]
	}
	return AvailableCurses[rng.Intn(len(AvailableCurses))]
}

func NewNGPlusSession(cfg *config.Config, level int) *GameSession {
	s := NewGameSession(cfg)
	s.NGPlusLevel = level
	curse := PickCurse(nil)
	s.ActiveCurse = &curse
	s.ApplyWeeklyMutation()
	return s
}
