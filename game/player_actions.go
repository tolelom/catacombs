package game

import (
	"fmt"

	"github.com/tolelom/catacombs/combat"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) performAction(p *entity.Player, action PlayerAction, plan *attackPlan) bool {
	switch action.Type {
	case ActionAttack:
		s.queueAttack(p, action, plan)
	case ActionSkill:
		s.useSkill(p, action, plan)
	case ActionItem:
		s.useItem(p)
	case ActionFlee:
		return s.attemptFlee(p)
	case ActionWait:
		s.addLog(fmt.Sprintf("%s 방어 중", p.Name))
		s.recordSoloAction("")
	case ActionCoop:
		coopOK := p.Class != entity.ClassRogue && !s.coopUsedThisCombat[p.Fingerprint]
		s.applyCoopAbility(p)
		if coopOK {
			s.recordSoloAction("coop")
		} else {
			s.recordSoloAction("")
		}
	case ActionRelic:
		s.applyActiveRelic(p, action.TargetIdx)
		s.recordSoloAction("")
	}
	return false
}

func (s *GameSession) queueAttack(p *entity.Player, action PlayerAction, plan *attackPlan) {
	multiplier := 1.0
	crit := false
	if s.critGuaranteedNext {
		multiplier = 2.0
		crit = true
		s.addLog(fmt.Sprintf("%s 급소 노출 치명타!", p.Name))
	} else if p.Skills != nil {
		if pct := p.Skills.GetCritChance(p.Class); pct > 0 && s.rng.Intn(100) < pct {
			multiplier = 2.0
			crit = true
			s.addLog(fmt.Sprintf("%s 치명타!", p.Name))
		}
	}
	bonusATK := 0
	if s.state.SoloMode {
		if chain := combat.DetectSoloChain(s.soloLastAction, s.soloAttackStreak); chain != nil {
			s.addLog(chain.Message)
			if chain.GuaranteeCrit && !crit {
				multiplier *= 2.0
			}
			if chain.DamageMultiplier > 0 {
				multiplier *= chain.DamageMultiplier
			}
			bonusATK = chain.BonusDamage
			if s.state.CombosFiredThisRun == nil {
				s.state.CombosFiredThisRun = make(map[string]bool)
			}
			s.state.CombosFiredThisRun[chain.Name] = true
		}
	}
	plan.add(combat.AttackIntent{
		PlayerATK:  p.EffectiveATK() + bonusATK,
		TargetIdx:  action.TargetIdx,
		Multiplier: multiplier,
		IsAoE:      false,
	}, p.Name)
	s.recordSoloAction("attack")
}

func (s *GameSession) useSkill(p *entity.Player, action PlayerAction, plan *attackPlan) {
	if p.SkillUses <= 0 {
		s.addLog(fmt.Sprintf("%s 스킬 사용 횟수가 없습니다!", p.Name))
		s.recordSoloAction("")
		return
	}
	p.SkillUses--
	skillExecuted := true
	switch p.Class {
	case entity.ClassWarrior:
		s.warriorTaunt(p)
	case entity.ClassMage:
		s.mageBlast(p, plan)
	case entity.ClassHealer:
		skillExecuted = s.healerSkill(p, action, plan)
	case entity.ClassRogue:
		s.rogueScout(p)
	}
	if skillExecuted {
		s.recordSoloAction("skill")
	} else {
		s.recordSoloAction("")
	}
}

func (s *GameSession) warriorTaunt(p *entity.Player) {
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			m.TauntTarget = true
			m.TauntTurns = 2
		}
	}
	s.addLog(fmt.Sprintf("%s Taunt 사용! 적들이 2턴간 %s를 집중 공격합니다", p.Name, p.Name))
}

func (s *GameSession) mageBlast(p *entity.Player, plan *attackPlan) {
	skillPower := 0
	if p.Skills != nil {
		skillPower = p.Skills.GetSkillPower(p.Class)
	}
	multiplier := 0.8 + float64(skillPower)/100.0
	plan.add(combat.AttackIntent{
		PlayerATK:  p.EffectiveATK(),
		TargetIdx:  -1,
		Multiplier: multiplier,
		IsAoE:      true,
	}, p.Name)
}

func (s *GameSession) healerSkill(p *entity.Player, action PlayerAction, plan *attackPlan) bool {
	healAmount := 30
	if p.Skills != nil {
		healAmount += p.Skills.GetSkillPower(p.Class) / 2
		healAmount += p.Skills.GetHealBoost(p.Class)
	}
	if s.HardMode {
		healAmount = int(float64(healAmount) * s.cfg.Difficulty.HardModeHealMult)
	}
	if target := s.pickHealTarget(action.TargetIdx); target != nil {
		before := target.HP
		target.Heal(healAmount)
		s.addLog(fmt.Sprintf("%s이(가) %s에게 HP %d 회복", p.Name, target.Name, target.HP-before))
		return true
	}
	if mIdx := s.firstAliveMonsterIdx(); mIdx >= 0 {
		plan.add(combat.AttackIntent{
			PlayerATK:  healAmount,
			TargetIdx:  mIdx,
			Multiplier: 1.0,
			IsAoE:      false,
		}, p.Name)
		s.addLog(fmt.Sprintf("✦ %s 치유할 대상이 없어 신성 일격을 가합니다!", p.Name))
		return true
	}
	p.SkillUses++
	s.addLog(fmt.Sprintf("%s 회복 대상을 찾을 수 없습니다!", p.Name))
	return false
}

func (s *GameSession) rogueScout(p *entity.Player) {
	currentRoom := s.state.Floor.Rooms[s.state.Floor.CurrentRoom]
	for _, neighborIdx := range currentRoom.Neighbors {
		s.state.Floor.Rooms[neighborIdx].Visited = true
	}
	s.addLog(fmt.Sprintf("%s 주변 방을 정찰했습니다!", p.Name))
}

func (s *GameSession) useItem(p *entity.Player) {
	for i, item := range p.Inventory {
		if item.Type == entity.ItemConsumable {
			before := p.HP
			healAmt := item.Bonus
			if p.Skills != nil {
				healAmt += p.Skills.GetHealBoost(p.Class)
			}
			if s.HardMode {
				healAmt = int(float64(healAmt) * s.cfg.Difficulty.HardModeHealMult)
			}
			p.Heal(healAmt)
			p.Inventory = append(p.Inventory[:i], p.Inventory[i+1:]...)
			s.addLog(fmt.Sprintf("%s %s 사용, HP %d 회복", p.Name, item.Name, p.HP-before))
			s.recordSoloAction("item")
			return
		}
	}
	s.addLog(fmt.Sprintf("%s 사용할 아이템이 없습니다!", p.Name))
	s.recordSoloAction("")
}

func (s *GameSession) attemptFlee(p *entity.Player) bool {
	if !combat.AttemptFlee(s.rng, s.cfg.Combat.FleeChance) {
		s.addLog(fmt.Sprintf("%s 도주에 실패했습니다!", p.Name))
		s.recordSoloAction("")
		return false
	}
	s.addLog(fmt.Sprintf("%s 전투에서 도주했습니다!", p.Name))
	s.state.FleeSucceeded = true
	if s.state.SoloMode {
		s.state.Phase = PhaseExploring
		return true
	}
	p.Fled = true
	s.recordSoloAction("")
	return false
}

func (s *GameSession) pickHealTarget(chosenIdx int) *entity.Player {
	if chosenIdx >= 0 && chosenIdx < len(s.state.Players) {
		c := s.state.Players[chosenIdx]
		if !c.IsOut() && c.HP < c.MaxHP {
			return c
		}
	}
	var best *entity.Player
	bestMissing := 0
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		if missing := p.MaxHP - p.HP; missing > bestMissing {
			bestMissing = missing
			best = p
		}
	}
	return best
}

func (s *GameSession) firstAliveMonsterIdx() int {
	for i, m := range s.state.Monsters {
		if !m.IsDead() {
			return i
		}
	}
	return -1
}

func (s *GameSession) applyActiveRelic(p *entity.Player, choice int) {
	if s.activeRelicUsed[p.Fingerprint] {
		s.addLog(fmt.Sprintf("%s 액티브 유물은 이번 전투에 이미 사용했습니다", p.Name))
		return
	}
	actives := make([]entity.Relic, 0, len(p.Relics))
	for _, r := range p.Relics {
		if r.Effect.IsActive() {
			actives = append(actives, r)
		}
	}
	if len(actives) == 0 {
		s.addLog(fmt.Sprintf("%s 발동 가능한 액티브 유물이 없습니다", p.Name))
		return
	}
	if choice < 0 || choice >= len(actives) {
		choice = 0
	}
	r := actives[choice]
	switch r.Effect {
	case entity.RelicActiveSmokeBomb:
		for _, ally := range s.state.Players {
			if !ally.IsOut() {
				s.dodgeBonusThisTurn[ally.Fingerprint] = 0.30
			}
		}
		s.addLog(fmt.Sprintf("%s Smoke Bomb! 1턴간 파티 회피 +30%%", p.Name))
	case entity.RelicActiveBloodstone:
		heal := p.MaxHP * 30 / 100
		before := p.HP
		p.Heal(heal)
		gained := p.HP - before
		if gained > 0 {
			s.addLog(fmt.Sprintf("%s Bloodstone! HP %d 회복", p.Name, gained))
		} else {
			s.addLog(fmt.Sprintf("%s Bloodstone — 이미 만피", p.Name))
		}
	case entity.RelicActiveTimeGlass:
		s.extraActionNext[p.Fingerprint] = true
		s.addLog(fmt.Sprintf("%s Time Glass! 다음 턴 행동 1회 추가 예약", p.Name))
	}
	s.activeRelicUsed[p.Fingerprint] = true
}

func (s *GameSession) applyCoopAbility(p *entity.Player) {
	if p.Class == entity.ClassRogue {
		s.addLog(fmt.Sprintf("%s 정찰은 전투 외에 사용해야 합니다", p.Name))
		return
	}
	if s.coopUsedThisCombat[p.Fingerprint] {
		s.addLog(fmt.Sprintf("%s 협동기는 이번 전투에 이미 사용했습니다", p.Name))
		return
	}
	switch p.Class {
	case entity.ClassWarrior:
		res := combat.ApplyWarriorCoop(p)
		s.mergeCoopReduction(res.DamageReduction)
		for _, msg := range res.Logs {
			s.addLog(msg)
		}
	case entity.ClassMage:
		res := combat.ApplyMageCoop(p, s.state.Players, s.state.SoloMode)
		s.mergeCoopReduction(res.DamageReduction)
		for _, msg := range res.Logs {
			s.addLog(msg)
		}
	case entity.ClassHealer:
		reviveAllowed := !s.healerReviveUsed[p.Fingerprint]
		res := combat.ApplyHealerCoop(p, s.state.Players, reviveAllowed)
		if res.ReviveFingerprint != "" {
			for _, target := range s.state.Players {
				if target.Fingerprint == res.ReviveFingerprint {
					target.Revive(res.ReviveHPPercent)
					s.healerReviveUsed[p.Fingerprint] = true
					s.state.RevivesThisRun++
					break
				}
			}
		} else {
			if s.state.SoloMode {
				p.Heal(p.MaxHP * 30 / 100)
			} else {
				for _, ally := range s.state.Players {
					if !ally.IsOut() {
						ally.Heal(ally.MaxHP * 30 / 100)
					}
				}
			}
		}
		for _, msg := range res.Logs {
			s.addLog(msg)
		}
	}
	s.coopUsedThisCombat[p.Fingerprint] = true
}

func (s *GameSession) recordSoloAction(actionType string) {
	if !s.state.SoloMode {
		return
	}
	if actionType == "attack" {
		s.soloAttackStreak++
	} else {
		s.soloAttackStreak = 0
	}
	s.soloLastAction = actionType
}

func (s *GameSession) mergeCoopReduction(in map[string]float64) {
	for fp, mult := range in {
		if cur := s.coopDamageReduction[fp]; mult > cur {
			s.coopDamageReduction[fp] = mult
		}
	}
}
