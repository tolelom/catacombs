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

	// Collect actions with timeout
	turnTimeout := time.Duration(s.cfg.Game.TurnTimeoutSec) * time.Second
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
						s.addLog(fmt.Sprintf("  (%s theme: +%d damage)", theme.Name, bonus))
					}
				}
			}

			if p.IsDead() {
				s.addLog(fmt.Sprintf("☠ %s has fallen!", p.Name))
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
		action, ok := s.actions[p.Fingerprint]
		if !ok {
			continue
		}

		switch action.Type {
		case ActionAttack:
			intents = append(intents, combat.AttackIntent{
				PlayerATK:  p.EffectiveATK(),
				TargetIdx:  action.TargetIdx,
				Multiplier: 1.0,
				IsAoE:      false,
			})
			intentOwners = append(intentOwners, p.Name)
		case ActionSkill:
			if p.SkillUses <= 0 {
				s.addLog(fmt.Sprintf("%s has no skill uses left!", p.Name))
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
				s.addLog(fmt.Sprintf("%s used Taunt! Enemies focus on %s for 2 turns", p.Name, p.Name))
			case entity.ClassMage:
				intents = append(intents, combat.AttackIntent{
					PlayerATK:  p.EffectiveATK(),
					TargetIdx:  -1,
					Multiplier: 0.8,
					IsAoE:      true,
				})
				intentOwners = append(intentOwners, p.Name)
			case entity.ClassHealer:
				targetIdx := action.TargetIdx
				if targetIdx < 0 || targetIdx >= len(s.state.Players) {
					targetIdx = 0
				}
				target := s.state.Players[targetIdx]
				if target.IsDead() {
					// Find first alive player to heal instead
					for j, candidate := range s.state.Players {
						if !candidate.IsOut() {
							target = candidate
							targetIdx = j
							break
						}
					}
				}
				before := target.HP
				target.Heal(30)
				s.addLog(fmt.Sprintf("%s healed %s for %d HP", p.Name, target.Name, target.HP-before))
			case entity.ClassRogue:
				currentRoom := s.state.Floor.Rooms[s.state.Floor.CurrentRoom]
				for _, neighborIdx := range currentRoom.Neighbors {
					s.state.Floor.Rooms[neighborIdx].Visited = true
				}
				s.addLog(fmt.Sprintf("%s scouted nearby rooms!", p.Name))
			}
		case ActionItem:
			found := false
			for i, item := range p.Inventory {
				if item.Type == entity.ItemConsumable {
					before := p.HP
					p.Heal(item.Bonus)
					p.Inventory = append(p.Inventory[:i], p.Inventory[i+1:]...)
					s.addLog(fmt.Sprintf("%s used %s, restored %d HP", p.Name, item.Name, p.HP-before))
					found = true
					break
				}
			}
			if !found {
				s.addLog(fmt.Sprintf("%s has no items to use!", p.Name))
			}
		case ActionFlee:
			if combat.AttemptFlee(s.cfg.Combat.FleeChance) {
				s.addLog(fmt.Sprintf("%s fled from battle!", p.Name))
				s.state.FleeSucceeded = true
				if s.state.SoloMode {
					s.state.Phase = PhaseExploring
					return
				}
				p.Fled = true
			} else {
				s.addLog(fmt.Sprintf("%s failed to flee!", p.Name))
			}
		case ActionWait:
			s.addLog(fmt.Sprintf("%s is defending", p.Name))
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
		s.addLog("All players fled!")
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

	if len(intents) > 0 && len(s.state.Monsters) > 0 {
		results := combat.ResolveAttacks(intents, s.state.Monsters, s.cfg.Game.CoopBonus)
		for i, r := range results {
			owner := intentOwners[i]
			if r.IsAoE {
				coopStr := ""
				if r.CoopApplied {
					coopStr = " (co-op!)"
				}
				s.addLog(fmt.Sprintf("%s hit all enemies for %d total dmg%s", owner, r.Damage, coopStr))
			} else if r.TargetIdx >= 0 && r.TargetIdx < len(s.state.Monsters) {
				target := s.state.Monsters[r.TargetIdx]
				coopStr := ""
				if r.CoopApplied {
					coopStr = " (co-op!)"
				}
				s.addLog(fmt.Sprintf("%s hit %s for %d dmg%s", owner, target.Name, r.Damage, coopStr))
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
							s.addLog(fmt.Sprintf("%s's relic heals %d HP", p.Name, r.Value))
						}
					}
					p.Gold += goldReward + bonus
				}
			}
			s.addLog(fmt.Sprintf("%s defeated! +%d gold", m.Name, goldReward))
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
		s.addLog("Room cleared!")
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
		s.addLog("You conquered the Catacombs!")
		return
	}
	s.state.FloorNum++
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, rand.New(rand.NewSource(time.Now().UnixNano())))
	s.state.Phase = PhaseExploring
	s.state.CombatTurn = 0
	s.addLog(fmt.Sprintf("Descending to B%d...", s.state.FloorNum))
	for _, p := range s.state.Players {
		if p.IsDead() {
			p.Revive(0.30)
			s.addLog(fmt.Sprintf("✦ %s revived at %d HP!", p.Name, p.HP))
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
			s.addLog(fmt.Sprintf("%s obtained relic: %s", p.Name, r.Name))
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
					s.addLog(fmt.Sprintf("%s AoE hits %s for %d dmg", m.Name, p.Name, dmg))
					if p.IsDead() {
						s.addLog(fmt.Sprintf("☠ %s has fallen!", p.Name))
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
							s.addLog(fmt.Sprintf("%s poisons %s!", m.Name, p.Name))
						}
					}
				case entity.PatternBurn:
					for _, p := range s.state.Players {
						if !p.IsOut() {
							p.AddEffect(entity.ActiveEffect{Type: entity.StatusBurn, Duration: 2, Value: 8})
							s.addLog(fmt.Sprintf("%s burns %s!", m.Name, p.Name))
						}
					}
				case entity.PatternFreeze:
					for _, p := range s.state.Players {
						if !p.IsOut() {
							p.AddEffect(entity.ActiveEffect{Type: entity.StatusFreeze, Duration: 1, Value: 0})
							s.addLog(fmt.Sprintf("%s freezes %s!", m.Name, p.Name))
						}
					}
				case entity.PatternHeal:
					healAmt := m.MaxHP / 10
					m.HP += healAmt
					if m.HP > m.MaxHP {
						m.HP = m.MaxHP
					}
					s.addLog(fmt.Sprintf("%s regenerates %d HP!", m.Name, healAmt))
				}
			}
		} else {
			if targetIdx >= 0 && targetIdx < len(s.state.Players) {
				p := s.state.Players[targetIdx]
				if !p.IsOut() {
					dmg := combat.CalcDamage(m.ATK, p.EffectiveDEF(), 1.0)
					p.TakeDamage(dmg)
					s.addLog(fmt.Sprintf("%s attacks %s for %d dmg", m.Name, p.Name, dmg))
					if m.IsElite {
						def := entity.ElitePrefixDefs[m.ElitePrefix]
						if def.OnHit >= 0 {
							p.AddEffect(entity.ActiveEffect{Type: def.OnHit, Duration: 2, Value: 3})
							s.addLog(fmt.Sprintf("%s's %s effect afflicts %s!", m.Name, def.Name, p.Name))
						} else if m.ElitePrefix == entity.PrefixVampiric {
							heal := dmg / 4
							m.HP = min(m.HP+heal, m.MaxHP)
							s.addLog(fmt.Sprintf("%s drains life from %s! (+%d HP)", m.Name, p.Name, heal))
						}
					}
					if p.IsDead() {
						s.addLog(fmt.Sprintf("☠ %s has fallen!", p.Name))
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
		s.addLog("Party wiped!")
	}
}
