package game

import (
	"fmt"
	"math/rand"
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
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	s.mu.Unlock()

	// Drain stale actions from previous turn
draining:
	for {
		select {
		case <-s.actionCh:
		default:
			break draining
		}
	}

	// Collect actions with timeout
	turnTimeout := time.Duration(s.effectiveTurnTimeout()) * time.Second
	timer := time.NewTimer(turnTimeout)
	s.mu.Lock()
	s.state.TurnDeadline = time.Now().Add(turnTimeout)
	s.mu.Unlock()
	collected := 0

collecting:
	for collected < aliveCount {
		select {
		case msg := <-s.actionCh:
			s.mu.Lock()
			s.actions[msg.PlayerID] = msg.Action
			s.mu.Unlock()
			collected++
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

	// Default action for players who didn't submit: Wait
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.actions[p.Fingerprint]; !ok {
				s.actions[p.Fingerprint] = PlayerAction{Type: ActionWait}
			}
		}
	}

	s.state.TurnResolving = true
	s.state.PendingLogs = nil
	s.resolvePlayerActions()
	s.resolveMonsterActions()
	s.state.TurnResolving = false
	// PendingLogs now contains all turn results — UI will reveal them one by one via RevealNextLog
}

func (s *GameSession) resolvePlayerActions() {
	// Record frozen players BEFORE ticking effects (freeze expires on tick)
	frozenPlayers := make(map[string]bool)
	for _, p := range s.state.Players {
		if !p.IsOut() && p.HasEffect(entity.StatusFreeze) {
			frozenPlayers[p.Fingerprint] = true
		}
	}

	// Tick status effects with floor theme damage bonus
	theme := dungeon.GetTheme(s.state.FloorNum)
	for _, p := range s.state.Players {
		if !p.IsOut() {
			// Snapshot effects before tick to compute theme bonus
			effectsBefore := make([]entity.ActiveEffect, len(p.Effects))
			copy(effectsBefore, p.Effects)

			msgs := p.TickEffects()
			for _, msg := range msgs {
				s.addLog(msg)
			}

			// Apply theme damage bonus for matching status effects
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
				s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
			}
		}
	}

	var intents []combat.AttackIntent
	var intentOwners []string // track who owns each intent

	// Track which monsters were alive before this turn (for gold awards)
	aliveBeforeTurn := make(map[int]bool)
	for i, m := range s.state.Monsters {
		if !m.IsDead() {
			aliveBeforeTurn[i] = true
		}
	}

	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		// Frozen players skip their action
		if frozenPlayers[p.Fingerprint] {
			s.addLog(fmt.Sprintf("%s 동결되어 행동할 수 없습니다!", p.Name))
			continue
		}
		action, ok := s.actions[p.Fingerprint]
		if !ok {
			continue
		}

		switch action.Type {
		case ActionAttack:
			multiplier := 1.0
			if p.Skills != nil {
				if pct := p.Skills.GetCritChance(p.Class); pct > 0 && rand.Intn(100) < pct {
					multiplier = 2.0
					s.addLog(fmt.Sprintf("%s 치명타!", p.Name))
				}
			}
			intents = append(intents, combat.AttackIntent{
				PlayerATK:  p.EffectiveATK(),
				TargetIdx:  action.TargetIdx,
				Multiplier: multiplier,
				IsAoE:      false,
			})
			intentOwners = append(intentOwners, p.Name)
		case ActionSkill:
			if p.SkillUses <= 0 {
				s.addLog(fmt.Sprintf("%s 스킬 사용 횟수가 없습니다!", p.Name))
				break
			}
			p.SkillUses--
			switch p.Class {
			case entity.ClassWarrior:
				for _, m := range s.state.Monsters {
					if !m.IsDead() {
						m.TauntTarget = true
						m.TauntTurns = 2
					}
				}
				s.addLog(fmt.Sprintf("%s Taunt 사용! 적들이 2턴간 %s를 집중 공격합니다", p.Name, p.Name))
			case entity.ClassMage:
				skillPower := 0
				if p.Skills != nil {
					skillPower = p.Skills.GetSkillPower(p.Class)
				}
				multiplier := 0.8 + float64(skillPower)/100.0
				intents = append(intents, combat.AttackIntent{
					PlayerATK:  p.EffectiveATK(),
					TargetIdx:  -1,
					Multiplier: multiplier,
					IsAoE:      true,
				})
				intentOwners = append(intentOwners, p.Name)
			case entity.ClassHealer:
				targetIdx := action.TargetIdx
				if targetIdx < 0 || targetIdx >= len(s.state.Players) {
					targetIdx = 0
				}
				target := s.state.Players[targetIdx]
				if target.IsOut() {
					// Find first alive player to heal instead
					found := false
					for _, candidate := range s.state.Players {
						if !candidate.IsOut() {
							target = candidate
							found = true
							break
						}
					}
					if !found {
						// No valid target — refund skill use
						p.SkillUses++
						s.addLog(fmt.Sprintf("%s 회복 대상을 찾을 수 없습니다!", p.Name))
						break
					}
				}
				healAmount := 30
				if p.Skills != nil {
					healAmount += p.Skills.GetSkillPower(p.Class) / 2
					healAmount += p.Skills.GetHealBoost(p.Class)
				}
				if s.HardMode {
					healAmount = int(float64(healAmount) * s.cfg.Difficulty.HardModeHealMult)
				}
				before := target.HP
				target.Heal(healAmount)
				s.addLog(fmt.Sprintf("%s이(가) %s에게 HP %d 회복", p.Name, target.Name, target.HP-before))
			case entity.ClassRogue:
				currentRoom := s.state.Floor.Rooms[s.state.Floor.CurrentRoom]
				for _, neighborIdx := range currentRoom.Neighbors {
					s.state.Floor.Rooms[neighborIdx].Visited = true
				}
				s.addLog(fmt.Sprintf("%s 주변 방을 정찰했습니다!", p.Name))
			}
		case ActionItem:
			found := false
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
					found = true
					break
				}
			}
			if !found {
				s.addLog(fmt.Sprintf("%s 사용할 아이템이 없습니다!", p.Name))
			}
		case ActionFlee:
			if combat.AttemptFlee(s.cfg.Combat.FleeChance) {
				s.addLog(fmt.Sprintf("%s 전투에서 도주했습니다!", p.Name))
				s.state.FleeSucceeded = true
				if s.state.SoloMode {
					s.state.Phase = PhaseExploring
					return
				}
				p.Fled = true
			} else {
				s.addLog(fmt.Sprintf("%s 도주에 실패했습니다!", p.Name))
			}
		case ActionWait:
			s.addLog(fmt.Sprintf("%s 방어 중", p.Name))
		}
	}

	// Check if all alive players have fled
	allFled := true
	for _, p := range s.state.Players {
		if !p.IsDead() && !p.Fled {
			allFled = false
			break
		}
	}
	if allFled && !s.state.SoloMode {
		s.state.Phase = PhaseExploring
		s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		s.addLog("모든 플레이어가 도주했습니다!")
		for _, p := range s.state.Players {
			p.Fled = false
		}
		return
	}

	// Combo detection: build action map and apply combo effects before resolving attacks
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
		default:
			continue
		}
		comboActions[p.Fingerprint] = combat.ComboAction{Class: p.Class, ActionType: actionType}
	}

	combos := combat.DetectCombos(comboActions)
	for _, combo := range combos {
		s.addLog(combo.Effect.Message)
		for i := range intents {
			if combo.Effect.DamageMultiplier > 0 {
				intents[i].Multiplier *= combo.Effect.DamageMultiplier
			}
			intents[i].PlayerATK += combo.Effect.BonusDamage
		}
	}

	// Build name→player map for relic effects
	playerByName := make(map[string]*entity.Player)
	for _, p := range s.state.Players {
		playerByName[p.Name] = p
	}

	if len(intents) > 0 && len(s.state.Monsters) > 0 {
		results := combat.ResolveAttacks(intents, s.state.Monsters, s.cfg.Game.CoopBonus)
		for i, r := range results {
			owner := intentOwners[i]
			if r.IsAoE {
				coopStr := ""
				if r.CoopApplied {
					coopStr = " (협동!)"
				}
				s.addLog(fmt.Sprintf("%s 전체 적에게 총 %d 피해%s", owner, r.Damage, coopStr))
			} else if r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters) {
				target := s.state.Monsters[r.TargetIdx]
				coopStr := ""
				if r.CoopApplied {
					coopStr = " (협동!)"
				}
				s.addLog(fmt.Sprintf("%s이(가) %s에게 %d 피해%s", owner, target.Name, r.Damage, coopStr))
			}
			// Apply Life Siphon relic: heal percentage of damage dealt
			if r.Damage > 0 {
				if p := playerByName[owner]; p != nil && !p.IsOut() {
					for _, rel := range p.Relics {
						if rel.Effect == entity.RelicLifeSteal {
							heal := r.Damage * rel.Value / 100
							if heal > 0 {
								p.Heal(heal)
								s.addLog(fmt.Sprintf("  %s의 Life Siphon으로 HP %d 회복", p.Name, heal))
							}
						}
					}
				}
			}
		}
	}

	// Apply combo HealAll effects after attack resolution
	for _, combo := range combos {
		if combo.Effect.HealAll > 0 {
			for _, p := range s.state.Players {
				if !p.IsOut() {
					p.Heal(combo.Effect.HealAll)
				}
			}
		}
	}

	// Award gold only for monsters that JUST died this turn
	for i, m := range s.state.Monsters {
		if m.IsDead() && aliveBeforeTurn[i] {
			goldReward := 5 + s.state.FloorNum*2
			for _, p := range s.state.Players {
				if !p.IsOut() {
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
			}
			s.addLog(fmt.Sprintf("%s 처치! +%d 골드", m.Name, goldReward))
			if m.IsBoss {
				s.state.BossKilled = true
				s.grantBossRelic()
			}
		}
	}

	// Filter out dead monsters
	alive := make([]*entity.Monster, 0)
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			alive = append(alive, m)
		}
	}
	s.state.Monsters = alive

	// Check if combat is over
	if len(s.state.Monsters) == 0 {
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
}

func (s *GameSession) advanceFloor() {
	if s.state.FloorNum >= s.cfg.Game.MaxFloors {
		s.state.Phase = PhaseResult
		s.state.Victory = true
		s.state.GameOver = true
		s.addLog("카타콤을 정복했습니다!")
		return
	}
	// Grant 1 skill point per floor clear
	for _, p := range s.state.Players {
		if p.Skills == nil {
			p.Skills = &entity.PlayerSkills{BranchIndex: -1}
		}
		p.Skills.Points++
	}
	s.state.FloorNum++
	if s.DailyMode {
		seed := DailySeed(s.DailyDate) + int64(s.state.FloorNum)
		s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, rand.New(rand.NewSource(seed)))
	} else {
		s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, rand.New(rand.NewSource(time.Now().UnixNano())))
	}
	s.state.Phase = PhaseExploring
	s.state.CombatTurn = 0
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
	}
	for _, p := range s.state.Players {
		if !p.IsOut() {
			r := relics[rand.Intn(len(relics))]
			p.Relics = append(p.Relics, r)
			s.addLog(fmt.Sprintf("%s 유물 획득: %s", p.Name, r.Name))
		}
	}
}

func (s *GameSession) resolveMonsterActions() {
	if s.state.Phase != PhaseCombat {
		return
	}
	for _, m := range s.state.Monsters {
		if m.IsDead() {
			continue
		}
		targetIdx, isAoE := combat.MonsterAI(m, s.state.Players, s.state.CombatTurn)
		if isAoE {
			for _, p := range s.state.Players {
				if !p.IsOut() {
					dmg := combat.CalcDamage(m.ATK, p.EffectiveDEF(), 0.5)
					p.TakeDamage(dmg)
					s.addLog(fmt.Sprintf("%s 광역 공격으로 %s에게 %d 피해", m.Name, p.Name, dmg))
					if p.IsDead() {
						s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
					}
				}
			}
			if m.IsBoss || m.IsMiniBoss {
				// Boss/mini-boss special pattern
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
					dmg := combat.CalcDamage(m.ATK, p.EffectiveDEF(), 1.0)
					p.TakeDamage(dmg)
					s.addLog(fmt.Sprintf("%s이(가) %s을(를) 공격하여 %d 피해", m.Name, p.Name, dmg))
					if m.IsElite {
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
						s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
					}
				}
			}
		}
		m.TickTaunt()
	}

	allPlayersDead := true
	for _, p := range s.state.Players {
		if !p.IsOut() {
			allPlayersDead = false
			break
		}
	}
	if allPlayersDead {
		s.state.Phase = PhaseResult
		s.state.GameOver = true
		s.addLog("파티가 전멸했습니다!")
	}
}
