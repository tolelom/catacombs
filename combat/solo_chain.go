package combat

type SoloChain struct {
	Name             string
	Message          string
	DamageMultiplier float64
	BonusDamage      int
	GuaranteeCrit    bool
}

var (
	soloEchoStrike = SoloChain{
		Name:             "Echo Strike",
		Message:          "💫 ECHO STRIKE! 스킬의 잔향이 공격을 증폭!",
		DamageMultiplier: 1.4,
	}
	soloCounterStance = SoloChain{
		Name:          "Counter Stance",
		Message:       "🎯 COUNTER STANCE! 반격 태세에서 치명타!",
		GuaranteeCrit: true,
	}
	soloRelentless = SoloChain{
		Name:             "Relentless Assault",
		Message:          "⚔️ RELENTLESS! 연속 공세!",
		DamageMultiplier: 1.25,
		BonusDamage:      3,
	}
)

func DetectSoloChain(lastAction string, attackStreak int) *SoloChain {
	switch lastAction {
	case "skill":
		c := soloEchoStrike
		return &c
	case "coop":
		c := soloCounterStance
		return &c
	case "attack":
		if attackStreak >= 2 {
			c := soloRelentless
			return &c
		}
	}
	return nil
}
