package game

import (
	"fmt"
	"time"

	"github.com/tolelom/catacombs/combat"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) RunTurn() {
	s.beginTurn()
	s.drainPlayerLeft()
	if !s.collectActions() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolveTurnLocked()
}

func (s *GameSession) beginTurn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.TurnNum++
	s.state.CombatTurn++
	s.clearLog()
	s.actions = make(map[string]PlayerAction)
	s.state.SubmittedActions = make(map[string]string)
	s.coopDamageReduction = make(map[string]float64)
	s.invincibleNextTurn = make(map[string]bool)
	s.dodgeBonusThisTurn = make(map[string]float64)
}

func (s *GameSession) drainPlayerLeft() {
	for {
		select {
		case <-s.playerLeftCh:
		default:
			return
		}
	}
}

func (s *GameSession) collectActions() bool {
	turnTimeout := time.Duration(s.effectiveTurnTimeout()) * time.Second
	timer := time.NewTimer(turnTimeout)
	defer timer.Stop()
	s.mu.Lock()
	s.state.TurnDeadline = time.Now().Add(turnTimeout)
	s.mu.Unlock()

	for {
		s.mu.Lock()
		allActed := s.allAliveMembersActedLocked()
		s.mu.Unlock()
		if allActed {
			return true
		}

		select {
		case msg := <-s.actionCh:
			s.recordActionMessage(msg)
		case <-s.playerLeftCh:
		case <-timer.C:
			return true
		case <-s.done:
			return false
		}
	}
}

func (s *GameSession) recordActionMessage(msg playerActionMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if msg.Turn == s.state.TurnNum && s.isAliveMemberLocked(msg.PlayerID) {
		s.actions[msg.PlayerID] = msg.Action
	}
}

func (s *GameSession) resolveTurnLocked() {
	s.state.TurnDeadline = time.Time{}
	s.fillDefaultActionsLocked()

	s.state.TurnResolving = true
	s.state.PendingLogs = nil
	if len(s.extraActionNext) > 0 {
		s.runExtraActions()
		s.extraActionNext = make(map[string]bool)
	}
	s.resolvePlayerActions()
	s.resolveMonsterActions()
	s.state.TurnResolving = false

	s.endTurnLocked()
}

func (s *GameSession) fillDefaultActionsLocked() {
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		if _, ok := s.actions[p.Fingerprint]; !ok {
			s.actions[p.Fingerprint] = PlayerAction{Type: ActionWait}
		}
	}
}

func (s *GameSession) endTurnLocked() {
	s.critGuaranteedNext = false
	s.enemyMissNextTurn = false
	for _, m := range s.state.Monsters {
		if m.IsBoss && m.Phase == 2 && s.state.FloorNum == 15 && !m.IsDead() {
			m.Vulnerable = !m.Vulnerable
		}
	}
}

func (s *GameSession) resolvePlayerActions() {
	frozen := s.frozenPlayersLocked()
	s.tickPlayerEffectsLocked()
	s.tickMonsterEffectsLocked()
	aliveBefore := s.aliveMonsterIndicesLocked()
	plan := &attackPlan{}
	if s.collectPlayerActions(frozen, plan) {
		return
	}
	if s.endCombatIfEveryoneFled() {
		return
	}
	combos := s.applyCombos(plan)
	s.resolveAttacks(plan)
	s.applyComboHealing(combos)
	s.checkBossPhaseTransitions()
	s.awardKills(aliveBefore)
	s.removeDeadMonsters()
	s.finishRoomIfCleared()
}

type attackPlan struct {
	intents []combat.AttackIntent
	owners  []string
}

func (a *attackPlan) add(intent combat.AttackIntent, owner string) {
	a.intents = append(a.intents, intent)
	a.owners = append(a.owners, owner)
}

func (s *GameSession) frozenPlayersLocked() map[string]bool {
	frozen := make(map[string]bool)
	for _, p := range s.state.Players {
		if !p.IsOut() && p.HasEffect(entity.StatusFreeze) {
			frozen[p.Fingerprint] = true
		}
	}
	return frozen
}

func (s *GameSession) tickPlayerEffectsLocked() {
	theme := dungeon.GetTheme(s.state.FloorNum)
	for _, p := range s.state.Players {
		if !p.IsOut() {
			s.tickPlayerEffects(p, theme)
		}
	}
}

func (s *GameSession) tickPlayerEffects(p *entity.Player, theme dungeon.ThemeModifier) {
	effectsBefore := make([]entity.ActiveEffect, len(p.Effects))
	copy(effectsBefore, p.Effects)
	for _, msg := range p.TickEffects() {
		s.addLog(msg)
	}
	for _, e := range effectsBefore {
		if e.Value > 0 && (theme.StatusBoost == entity.StatusEffect(-1) || e.Type == theme.StatusBoost) {
			bonus := int(float64(e.Value) * (theme.DamageMult - 1.0))
			if bonus > 0 {
				p.TakeDamage(bonus)
				s.addLog(fmt.Sprintf("  (%s 테마: +%d 피해)", theme.Name, bonus))
			}
		}
	}
	if p.IsDead() {
		s.handlePlayerDeathLocked(p)
	}
}

func (s *GameSession) tickMonsterEffectsLocked() {
	aliveBefore := s.aliveMonsterIndicesLocked()
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			for _, msg := range m.TickEffects() {
				s.addLog(msg)
			}
		}
	}
	s.awardKills(aliveBefore)
}

func (s *GameSession) aliveMonsterIndicesLocked() map[int]bool {
	alive := make(map[int]bool)
	for i, m := range s.state.Monsters {
		if !m.IsDead() {
			alive[i] = true
		}
	}
	return alive
}

func (s *GameSession) collectPlayerActions(frozen map[string]bool, plan *attackPlan) bool {
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		if frozen[p.Fingerprint] {
			s.addLog(fmt.Sprintf("%s 동결되어 행동할 수 없습니다!", p.Name))
			s.recordSoloAction("")
			continue
		}
		action, ok := s.actions[p.Fingerprint]
		if !ok {
			s.recordSoloAction("")
			continue
		}
		if s.performAction(p, action, plan) {
			return true
		}
	}
	return false
}

func (s *GameSession) endCombatIfEveryoneFled() bool {
	for _, p := range s.state.Players {
		if !p.IsOut() {
			return false
		}
	}
	if s.state.SoloMode {
		return false
	}
	s.state.Phase = PhaseExploring
	s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
	s.addLog("모든 플레이어가 도주했습니다!")
	for _, p := range s.state.Players {
		p.Fled = false
	}
	return true
}

func (s *GameSession) applyCombos(plan *attackPlan) []combat.ComboDef {
	combos := combat.DetectCombos(s.comboActionsLocked())
	for _, combo := range combos {
		s.addLog(combo.Effect.Message)
		for i := range plan.intents {
			if combo.Effect.DamageMultiplier > 0 {
				plan.intents[i].Multiplier *= combo.Effect.DamageMultiplier
			}
			plan.intents[i].PlayerATK += combo.Effect.BonusDamage
		}
		s.applyCoopComboEffect(combo.Effect)
		if s.state.CombosFiredThisRun == nil {
			s.state.CombosFiredThisRun = make(map[string]bool)
		}
		s.state.CombosFiredThisRun[combo.Name] = true
	}
	return combos
}

func (s *GameSession) comboActionsLocked() map[string]combat.ComboAction {
	comboActions := make(map[string]combat.ComboAction)
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		action, ok := s.actions[p.Fingerprint]
		if !ok {
			continue
		}
		var actionType string
		switch action.Type {
		case ActionAttack:
			actionType = "attack"
		case ActionSkill:
			actionType = "skill"
		case ActionItem:
			actionType = "item"
		case ActionCoop:
			actionType = "coop"
		default:
			continue
		}
		comboActions[p.Fingerprint] = combat.ComboAction{Class: p.Class, ActionType: actionType}
	}
	return comboActions
}

func (s *GameSession) resolveAttacks(plan *attackPlan) {
	if len(plan.intents) == 0 || len(s.state.Monsters) == 0 {
		return
	}
	playerByName := make(map[string]*entity.Player)
	for _, p := range s.state.Players {
		playerByName[p.Name] = p
	}
	results := combat.ResolveAttacks(s.rng, plan.intents, s.state.Monsters, s.cfg.Game.CoopBonus)
	for i, r := range results {
		owner := plan.owners[i]
		s.logAttackResult(owner, r)
		if r.Damage > 0 {
			s.applyLifeSiphon(playerByName[owner], r.Damage)
		}
	}
}

func (s *GameSession) logAttackResult(owner string, r combat.AttackResult) {
	switch {
	case r.IsAoE:
		coopStr := ""
		if r.CoopApplied {
			coopStr = " (협동!)"
		}
		s.addLog(fmt.Sprintf("%s 전체 적에게 총 %d 피해%s", owner, r.Damage, coopStr))
	case r.Missed && r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters):
		s.addLog(fmt.Sprintf("%s의 공격을 %s가 회피!", owner, s.state.Monsters[r.TargetIdx].Name))
	case r.Glanced && r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters):
		target := s.state.Monsters[r.TargetIdx]
		s.addLog(fmt.Sprintf("%s이(가) %s에게 %d 피해 (글랜스)", owner, target.Name, r.Damage))
	case r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters):
		target := s.state.Monsters[r.TargetIdx]
		coopStr := ""
		if r.CoopApplied {
			coopStr = " (협동!)"
		}
		s.addLog(fmt.Sprintf("%s이(가) %s에게 %d 피해%s", owner, target.Name, r.Damage, coopStr))
	}
}

func (s *GameSession) applyLifeSiphon(p *entity.Player, damage int) {
	if p == nil || p.IsOut() {
		return
	}
	for _, rel := range p.Relics {
		if rel.Effect == entity.RelicLifeSteal {
			heal := damage * rel.Value / 100
			if heal > 0 {
				p.Heal(heal)
				s.addLog(fmt.Sprintf("  %s의 Life Siphon으로 HP %d 회복", p.Name, heal))
			}
		}
	}
}

func (s *GameSession) applyComboHealing(combos []combat.ComboDef) {
	for _, combo := range combos {
		if combo.Effect.HealAll > 0 {
			for _, p := range s.state.Players {
				if !p.IsOut() {
					p.Heal(combo.Effect.HealAll)
				}
			}
		}
	}
}

func (s *GameSession) allAliveMembersActedLocked() bool {
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		if _, ok := s.actions[p.Fingerprint]; !ok {
			return false
		}
	}
	return true
}

func (s *GameSession) runExtraActions() {
	if len(s.state.Monsters) == 0 {
		return
	}
	var extras []combat.AttackIntent
	var owners []string
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		if !s.extraActionNext[p.Fingerprint] {
			continue
		}

		targetIdx := -1
		for i, m := range s.state.Monsters {
			if !m.IsDead() {
				targetIdx = i
				break
			}
		}
		if targetIdx < 0 {
			continue
		}
		extras = append(extras, combat.AttackIntent{PlayerATK: p.EffectiveATK(), TargetIdx: targetIdx, Multiplier: 1.0, IsAoE: false})
		owners = append(owners, p.Name)
		s.addLog(fmt.Sprintf("⏳ %s Time Glass 추가 행동!", p.Name))
	}
	if len(extras) == 0 {
		return
	}
	results := combat.ResolveAttacks(s.rng, extras, s.state.Monsters, s.cfg.Game.CoopBonus)
	for i, r := range results {
		owner := owners[i]
		if r.Missed && r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters) {
			s.addLog(fmt.Sprintf("%s의 추가 공격을 %s가 회피!", owner, s.state.Monsters[r.TargetIdx].Name))
		} else if r.Glanced && r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters) {
			s.addLog(fmt.Sprintf("%s 추가 공격: %s에게 %d 피해 (글랜스)", owner, s.state.Monsters[r.TargetIdx].Name, r.Damage))
		} else if r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters) {
			s.addLog(fmt.Sprintf("%s 추가 공격: %s에게 %d 피해", owner, s.state.Monsters[r.TargetIdx].Name, r.Damage))
		}
	}
}

func (s *GameSession) applyCoopComboEffect(eff combat.ComboEffect) {
	if eff.WarriorBulwarkOverride > 0 {
		for _, p := range s.state.Players {
			if p.Class == entity.ClassWarrior && !p.IsOut() {
				if cur := s.coopDamageReduction[p.Fingerprint]; eff.WarriorBulwarkOverride > cur {
					s.coopDamageReduction[p.Fingerprint] = eff.WarriorBulwarkOverride
				}
			}
		}
	}
	if eff.HealMultiplier > 1 {
		bonus := eff.HealMultiplier - 1.0
		for _, p := range s.state.Players {
			if !p.IsOut() {
				p.Heal(int(float64(p.MaxHP) * 0.30 * bonus))
			}
		}
	}
	if eff.GuaranteeCritNextTurn {
		s.critGuaranteedNext = true
	}
	if eff.InvincibleHealTarget {
		var minHPP *entity.Player
		minHP := int(^uint(0) >> 1)
		for _, p := range s.state.Players {
			if !p.IsOut() && p.HP < minHP {
				minHP = p.HP
				minHPP = p
			}
		}
		if minHPP != nil {
			s.invincibleNextTurn[minHPP.Fingerprint] = true
		}
	}
	if eff.EnemyMissNextTurn {
		s.enemyMissNextTurn = true
	}
	if eff.PoisonNextCombat {
		s.poisonNextCombat = true
	}
}

func (s *GameSession) handlePlayerDeathLocked(p *entity.Player) {
	if s.state.SoloMode && p.Class == entity.ClassHealer && !s.soloHealerRevived[p.Fingerprint] {
		p.Revive(0.30)
		s.soloHealerRevived[p.Fingerprint] = true
		s.addLog(fmt.Sprintf("✦ %s 신성 가호로 30%%HP로 부활!", p.Name))
		return
	}
	s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
	s.state.AnyPlayerDied = true
}
