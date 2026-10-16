package game

import (
	"fmt"
	"time"

	"github.com/tolelom/catacombs/combat"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) RunTurn() {
	s.mu.Lock()
	s.state.TurnNum++
	s.state.CombatTurn++
	s.clearLog()
	s.actions = make(map[string]PlayerAction)
	s.state.SubmittedActions = make(map[string]string)

	s.coopDamageReduction = make(map[string]float64)
	s.invincibleNextTurn = make(map[string]bool)
	s.dodgeBonusThisTurn = make(map[string]float64)
	s.mu.Unlock()

draining:
	for {
		select {
		case <-s.playerLeftCh:
		default:
			break draining
		}
	}

	turnTimeout := time.Duration(s.effectiveTurnTimeout()) * time.Second
	timer := time.NewTimer(turnTimeout)
	s.mu.Lock()
	s.state.TurnDeadline = time.Now().Add(turnTimeout)
	s.mu.Unlock()

collecting:
	for {

		s.mu.Lock()
		allActed := s.allAliveMembersActedLocked()
		s.mu.Unlock()
		if allActed {
			break collecting
		}

		select {
		case msg := <-s.actionCh:
			s.mu.Lock()
			if msg.Turn == s.state.TurnNum && s.isAliveMemberLocked(msg.PlayerID) {
				s.actions[msg.PlayerID] = msg.Action
			}
			s.mu.Unlock()
		case <-s.playerLeftCh:

		case <-timer.C:
			break collecting
		case <-s.done:
			timer.Stop()
			return
		}
	}
	timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.TurnDeadline = time.Time{}

	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.actions[p.Fingerprint]; !ok {
				s.actions[p.Fingerprint] = PlayerAction{Type: ActionWait}
			}
		}
	}

	s.state.TurnResolving = true
	s.state.PendingLogs = nil

	if len(s.extraActionNext) > 0 {
		s.runExtraActions()
		s.extraActionNext = make(map[string]bool)
	}
	s.resolvePlayerActions()
	s.resolveMonsterActions()
	s.state.TurnResolving = false

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
		s.tryReviveSoloHealerLocked(p)
	}
}

func (s *GameSession) tickMonsterEffectsLocked() {
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			for _, msg := range m.TickEffects() {
				s.addLog(msg)
			}
		}
	}
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

func (s *GameSession) killGoldMultiplier() float64 {
	goldMult := 1.0
	if s.NGPlusLevel > 0 {
		goldMult += 0.30 * float64(s.NGPlusLevel)
	}
	if s.ActiveCurse != nil && s.ActiveCurse.GoldMult > 0 {
		goldMult *= s.ActiveCurse.GoldMult
	}
	if s.DailyMod != nil && s.DailyMod.GoldMult > 0 {
		goldMult *= s.DailyMod.GoldMult
	}
	return goldMult
}

func (s *GameSession) awardKills(aliveBefore map[int]bool) {
	goldMult := s.killGoldMultiplier()
	for i, m := range s.state.Monsters {
		if m.IsDead() && aliveBefore[i] {
			s.awardKill(m, goldMult)
		}
	}
}

func (s *GameSession) awardKill(m *entity.Monster, goldMult float64) {
	if m.Name == "Mimic" {
		s.state.MimicKilled = true
	}
	goldReward := int(float64(5+s.state.FloorNum*2) * goldMult)
	for _, p := range s.state.Players {
		if !p.IsOut() {
			s.payKillReward(p, goldReward)
		}
	}
	s.addLog(fmt.Sprintf("%s 처치! +%d 골드", m.Name, goldReward))
	if m.IsBoss {
		s.state.BossKilled = true
		if s.state.BossesKilledThisRun == nil {
			s.state.BossesKilledThisRun = make(map[string]bool)
		}
		s.state.BossesKilledThisRun[m.Name] = true
		s.grantBossRelic()
	}
}

func (s *GameSession) payKillReward(p *entity.Player, goldReward int) {
	bonus := 0
	for _, r := range p.Relics {
		if r.Effect == entity.RelicGoldBoost {
			bonus += r.Value
		}
		if r.Effect == entity.RelicHealOnKill {
			p.Heal(r.Value)
			s.addLog(fmt.Sprintf("%s의 유물로 HP %d 회복", p.Name, r.Value))
		}
	}
	p.Gold += goldReward + bonus
}

func (s *GameSession) removeDeadMonsters() {
	alive := make([]*entity.Monster, 0)
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			alive = append(alive, m)
		}
	}
	s.state.Monsters = alive
}

func (s *GameSession) finishRoomIfCleared() {
	if len(s.state.Monsters) != 0 {
		return
	}
	s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
	s.addLog("방 클리어!")
	for _, p := range s.state.Players {
		p.Fled = false
	}
	if s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Type == dungeon.RoomBoss {
		s.advanceFloor()
	} else {
		s.state.Phase = PhaseExploring
	}
}

func (s *GameSession) advanceFloor() {
	if s.state.FloorNum >= s.cfg.Game.MaxFloors {
		s.state.Phase = PhaseResult
		s.state.Victory = true
		s.state.GameOver = true
		s.addLog("카타콤을 정복했습니다!")
		return
	}

	for _, p := range s.state.Players {
		if p.Skills == nil {
			p.Skills = &entity.PlayerSkills{BranchIndex: -1}
		}
		p.Skills.Points++
	}
	s.state.FloorNum++
	s.reseedForFloor()
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, s.rng)
	s.state.Phase = PhaseExploring
	s.state.CombatTurn = 0
	s.scoutUsedThisFloor = make(map[string]bool)
	s.nextRoomPreview = nil
	s.addLog(fmt.Sprintf("B%d층으로 내려갑니다...", s.state.FloorNum))
	for _, p := range s.state.Players {
		if p.IsDead() {
			p.Revive(0.30)
			s.addLog(fmt.Sprintf("✦ %s HP %d로 부활!", p.Name, p.HP))
		}
		p.Fled = false
	}
}

func (s *GameSession) grantBossRelic() {
	relics := []entity.Relic{
		{Name: "Vampiric Ring", Effect: entity.RelicHealOnKill, Value: 5, Price: 100},
		{Name: "Power Amulet", Effect: entity.RelicATKBoost, Value: 3, Price: 120},
		{Name: "Iron Ward", Effect: entity.RelicDEFBoost, Value: 2, Price: 100},
		{Name: "Gold Charm", Effect: entity.RelicGoldBoost, Value: 5, Price: 150},
		{Name: "Antidote Charm", Effect: entity.RelicPoisonImmunity, Value: 0, Price: 100},
		{Name: "Flame Guard", Effect: entity.RelicBurnResist, Value: 0, Price: 120},
		{Name: "Life Siphon", Effect: entity.RelicLifeSteal, Value: 10, Price: 150},
		{Name: "Smoke Bomb", Effect: entity.RelicActiveSmokeBomb, Value: 0, Price: 130},
		{Name: "Bloodstone", Effect: entity.RelicActiveBloodstone, Value: 30, Price: 130},
		{Name: "Time Glass", Effect: entity.RelicActiveTimeGlass, Value: 0, Price: 180},
	}
	for _, p := range s.state.Players {
		if !p.IsOut() {
			r := relics[s.rng.Intn(len(relics))]
			p.Relics = append(p.Relics, r)
			s.addLog(fmt.Sprintf("%s 유물 획득: %s", p.Name, r.Name))

			if len(p.Inventory) < s.cfg.Game.InventoryLimit {
				item := randomSetItem(s.rng, s.state.FloorNum)
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("%s 세트 아이템: %s", p.Name, item.Name))
			}
		}
	}
}

func randomSetItem(rng interface{ Intn(int) int }, floor int) entity.Item {
	sets := []entity.ItemSet{entity.SetFlame, entity.SetFrost, entity.SetHoly, entity.SetShadow, entity.SetNature}
	set := sets[rng.Intn(len(sets))]
	if rng.Intn(2) == 0 {
		bonus := 4 + rng.Intn(5) + floor/3
		return entity.Item{Name: entity.SetName(set) + " 무기", Type: entity.ItemWeapon, Bonus: bonus, Set: set}
	}
	bonus := 3 + rng.Intn(4) + floor/4
	return entity.Item{Name: entity.SetName(set) + " 갑옷", Type: entity.ItemArmor, Bonus: bonus, Set: set}
}

func (s *GameSession) resolveMonsterActions() {
	if s.state.Phase != PhaseCombat {
		return
	}
	for _, m := range s.state.Monsters {
		if m.IsDead() {
			continue
		}
		targetIdx, isAoE := combat.MonsterAI(s.rng, m, s.state.Players, s.state.CombatTurn)
		if isAoE {
			for _, p := range s.state.Players {
				if !p.IsOut() {
					if s.enemyMissNextTurn && s.rng.Float64() < 0.50 {
						s.addLog(fmt.Sprintf("%s 환영진으로 %s 광역 공격 회피!", p.Name, m.Name))
						continue
					}
					dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), 0.5)
					dmg = s.applyDamageMods(p, dmg)
					p.TakeDamage(dmg)
					s.addLog(fmt.Sprintf("%s 광역 공격으로 %s에게 %d 피해", m.Name, p.Name, dmg))
					if p.IsDead() {
						s.tryReviveSoloHealerLocked(p)
					}
				}
			}
			if m.IsBoss || m.IsMiniBoss {

				switch m.Pattern {
				case entity.PatternPoison:
					for _, p := range s.state.Players {
						if !p.IsOut() {
							p.AddEffect(entity.ActiveEffect{Type: entity.StatusPoison, Duration: 3, Value: 5})
							s.addLog(fmt.Sprintf("%s이(가) %s에게 독을 걸었습니다!", m.Name, p.Name))
						}
					}
				case entity.PatternBurn:
					for _, p := range s.state.Players {
						if !p.IsOut() {
							p.AddEffect(entity.ActiveEffect{Type: entity.StatusBurn, Duration: 2, Value: 8})
							s.addLog(fmt.Sprintf("%s이(가) %s을(를) 불태웠습니다!", m.Name, p.Name))
						}
					}
				case entity.PatternFreeze:
					for _, p := range s.state.Players {
						if !p.IsOut() {
							p.AddEffect(entity.ActiveEffect{Type: entity.StatusFreeze, Duration: 1, Value: 0})
							s.addLog(fmt.Sprintf("%s이(가) %s을(를) 동결시켰습니다!", m.Name, p.Name))
						}
					}
				case entity.PatternHeal:
					healAmt := m.MaxHP / 10
					m.HP += healAmt
					if m.HP > m.MaxHP {
						m.HP = m.MaxHP
					}
					s.addLog(fmt.Sprintf("%s HP %d 재생!", m.Name, healAmt))
				}
			}
		} else {
			if targetIdx >= 0 && targetIdx < len(s.state.Players) {
				p := s.state.Players[targetIdx]
				if !p.IsOut() {
					if s.enemyMissNextTurn && s.rng.Float64() < 0.50 {
						s.addLog(fmt.Sprintf("%s 환영진으로 %s의 공격 회피!", p.Name, m.Name))
						m.TickTaunt()
						continue
					}
					dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), 1.0)
					dmg = s.applyDamageMods(p, dmg)
					p.TakeDamage(dmg)
					s.addLog(fmt.Sprintf("%s이(가) %s을(를) 공격하여 %d 피해", m.Name, p.Name, dmg))

					if m.IsElite && dmg > 0 {
						def := entity.ElitePrefixDefs[m.ElitePrefix]
						if def.OnHit >= 0 {
							p.AddEffect(entity.ActiveEffect{Type: def.OnHit, Duration: 2, Value: 3})
							s.addLog(fmt.Sprintf("%s의 %s 효과가 %s에게 적용!", m.Name, def.Name, p.Name))
						} else if m.ElitePrefix == entity.PrefixVampiric {
							heal := dmg / 4
							m.HP = min(m.HP+heal, m.MaxHP)
							s.addLog(fmt.Sprintf("%s이(가) %s의 생명력을 흡수! (+%d HP)", m.Name, p.Name, heal))
						}
					}
					if p.IsDead() {
						s.tryReviveSoloHealerLocked(p)
					}
				}
			}
		}
		m.TickTaunt()
	}

	s.endRunIfPartyWipedLocked()
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

func (s *GameSession) applyDamageMods(p *entity.Player, dmg int) int {
	if s.invincibleNextTurn[p.Fingerprint] {
		return 0
	}
	dodgeChance := 0.05
	blockChance := 0.05
	switch p.Class {
	case entity.ClassRogue:
		dodgeChance += 0.10
	case entity.ClassWarrior:
		blockChance += 0.10
	}
	if bonus, ok := s.dodgeBonusThisTurn[p.Fingerprint]; ok {
		dodgeChance += bonus
	}
	roll := s.rng.Float64()
	if roll < dodgeChance {
		s.addLog(fmt.Sprintf("%s 회피!", p.Name))
		return 0
	}
	if roll < dodgeChance+blockChance {
		s.addLog(fmt.Sprintf("%s 블록!", p.Name))
		dmg = dmg / 2
	}
	if mult, ok := s.coopDamageReduction[p.Fingerprint]; ok && mult > 0 {
		reduced := float64(dmg) * (1.0 - mult)
		if reduced < 0 {
			reduced = 0
		}
		return int(reduced)
	}
	return dmg
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

func (s *GameSession) checkBossPhaseTransitions() {
	floor := s.state.FloorNum
	threshold := 50
	if s.ActiveCurse != nil && s.ActiveCurse.ID == "cruel" {
		threshold = 70
	}
	for _, m := range s.state.Monsters {
		if !m.CheckPhaseTransitionAt(threshold) {
			continue
		}
		s.state.BossPhase2Reached = true
		s.addLog(fmt.Sprintf("⚡ %s 광폭화! Phase 2 진입!", m.Name))
		switch floor {
		case 5:

			m.ATK = int(float64(m.ATK) * 1.5)
			if m.Pattern == entity.PatternHeal {
				m.Pattern = entity.PatternNone
			}
			s.addLog(fmt.Sprintf("  %s ATK +50%%!", m.Name))
		case 10:

			scaling := s.cfg.Combat.MonsterScaling
			for i := 0; i < 2; i++ {
				summon := entity.NewMonster(entity.MonsterSlime, floor, scaling)
				summon.HP = summon.MaxHP * 2
				summon.MaxHP = summon.HP
				s.applyDifficultyMods(summon)
				s.state.Monsters = append(s.state.Monsters, summon)
			}
			s.addLog(fmt.Sprintf("  %s이(가) 슬라임 2마리를 소환!", m.Name))
		case 15:

			m.Vulnerable = true
			s.addLog(fmt.Sprintf("  %s의 약점이 노출됩니다! (받는 피해 1.5배)", m.Name))
		case 20:

			m.ATK = int(float64(m.ATK) * 1.3)
			m.DEF = int(float64(m.DEF) * 1.2)
			for _, p := range s.state.Players {
				if !p.IsOut() {
					p.AddEffect(entity.ActiveEffect{Type: entity.StatusPoison, Duration: 3, Value: 5})
					p.AddEffect(entity.ActiveEffect{Type: entity.StatusBurn, Duration: 2, Value: 6})
				}
			}
			s.addLog(fmt.Sprintf("  %s이(가) 모든 적을 저주! 독+화상!", m.Name))
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

func (s *GameSession) tryReviveSoloHealerLocked(p *entity.Player) bool {
	if s.state.SoloMode && p.Class == entity.ClassHealer && !s.soloHealerRevived[p.Fingerprint] {
		p.Revive(0.30)
		s.soloHealerRevived[p.Fingerprint] = true
		s.addLog(fmt.Sprintf("✦ %s 신성 가호로 30%%HP로 부활!", p.Name))
		return true
	}
	s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
	s.state.AnyPlayerDied = true
	return false
}
