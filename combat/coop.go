package combat

import "github.com/tolelom/catacombs/entity"

type CoopApplyResult struct {
	DamageReduction   map[string]float64
	HealPerAlly       int
	ReviveFingerprint string
	ReviveHPPercent   float64
	Logs              []string
}

func ApplyWarriorCoop(warrior *entity.Player) CoopApplyResult {
	return CoopApplyResult{
		DamageReduction: map[string]float64{warrior.Fingerprint: 0.75},
		Logs:            []string{warrior.Name + " 견고함! 1턴간 받는 피해 -75%"},
	}
}

func ApplyMageCoop(mage *entity.Player, party []*entity.Player, solo bool) CoopApplyResult {
	out := CoopApplyResult{DamageReduction: make(map[string]float64)}
	if solo {
		out.DamageReduction[mage.Fingerprint] = 0.50
		out.Logs = []string{mage.Name + " 결계! 1턴간 받는 피해 -50%"}
		return out
	}
	for _, p := range party {
		if !p.IsOut() {
			out.DamageReduction[p.Fingerprint] = 0.50
		}
	}
	out.Logs = []string{mage.Name + " 결계! 파티 받는 피해 -50%"}
	return out
}

func ApplyHealerCoop(healer *entity.Player, party []*entity.Player, reviveAllowed bool) CoopApplyResult {
	out := CoopApplyResult{}
	if reviveAllowed {
		for _, p := range party {
			if p.IsDead() {
				out.ReviveFingerprint = p.Fingerprint
				out.ReviveHPPercent = 0.50
				out.Logs = []string{healer.Name + " 신성 가호! " + p.Name + " 50%HP로 부활!"}
				return out
			}
		}
	}
	out.HealPerAlly = 0
	out.Logs = []string{healer.Name + " 신성 가호! 파티 30% 회복"}
	return out
}
