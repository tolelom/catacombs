package combat

// SoloChain is a solo-mode action-sequence combo: the player's own action
// order across turns replaces the cross-class party combos that are
// impossible to trigger alone. Values sit slightly below their party-combo
// counterparts because a solo player can set them up at will, while party
// combos need coordination between two or more players.
type SoloChain struct {
	Name             string
	Message          string
	DamageMultiplier float64 // multiplied onto the attack's Multiplier
	BonusDamage      int     // added to the attack's PlayerATK
	GuaranteeCrit    bool    // attack crits (2.0x) if it hasn't already
}

var (
	// Echo Strike — skill → attack (party counterpart: Ice Shatter 1.5x).
	soloEchoStrike = SoloChain{
		Name:             "Echo Strike",
		Message:          "💫 ECHO STRIKE! 스킬의 잔향이 공격을 증폭!",
		DamageMultiplier: 1.4,
	}
	// Counter Stance — coop → attack (party counterpart: Expose Weakness).
	soloCounterStance = SoloChain{
		Name:          "Counter Stance",
		Message:       "🎯 COUNTER STANCE! 반격 태세에서 치명타!",
		GuaranteeCrit: true,
	}
	// Relentless Assault — 3rd+ consecutive attack (party counterpart:
	// Full Assault 1.3x +5).
	soloRelentless = SoloChain{
		Name:             "Relentless Assault",
		Message:          "⚔️ RELENTLESS! 연속 공세!",
		DamageMultiplier: 1.25,
		BonusDamage:      3,
	}
)

// DetectSoloChain returns the chain triggered by an attack that follows
// lastAction, with attackStreak consecutive attacks made BEFORE this one
// (streak 2 means this attack is the 3rd in a row). lastAction must reflect
// the previous turn's *successful* action — failed skills/coops, items,
// waiting, and freezes record "" and break every chain. Returns an
// independent copy, or nil when no chain fires.
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
