package ui

import (
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/store"
)

func applyPermaUpgrades(ctx *Context, p *entity.Player) {
	if ctx == nil || ctx.Store == nil || p == nil || p.Fingerprint == "" {
		return
	}
	levels, err := ctx.Store.GetUpgrades(p.Fingerprint)
	if err != nil {
		return
	}
	if lvl := levels[store.UpgradeStartGold]; lvl > 0 {
		p.Gold += lvl * 50
	}
	if lvl := levels[store.UpgradeStartHP]; lvl > 0 {
		p.MaxHP += lvl * 20
		p.HP = p.MaxHP
	}
	if lvl := levels[store.UpgradeStartATK]; lvl > 0 {
		p.ATK += lvl * 3
	}
	if lvl := levels[store.UpgradeStartSkillPts]; lvl > 0 {
		if p.Skills == nil {
			p.Skills = entity.NewPlayerSkills()
		}
		p.Skills.Points += lvl
	}
	runs, _ := ctx.Store.GetCounter(p.Fingerprint, store.MasteryCounterKey(p.Class.String()))
	if runs > 0 {
		reward := store.ClassMasteryRewards(p.Class.String(), int(runs))
		if reward.HPDelta > 0 {
			p.MaxHP += reward.HPDelta
			p.HP = p.MaxHP
		}
		if reward.ATKDelta > 0 {
			p.ATK += reward.ATKDelta
		}
		if reward.DEFDelta > 0 {
			p.DEF += reward.DEFDelta
		}
	}
}
