package game

import (
	"math/rand"
	"time"

	"github.com/tolelom/catacombs/combat"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

const TurnTimeout = 5 * time.Second

func (s *GameSession) RunTurn() {
	s.mu.Lock()
	s.state.TurnNum++
	s.state.CombatTurn++
	s.actions = make(map[string]PlayerAction)
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsDead() {
			aliveCount++
		}
	}
	s.mu.Unlock()

	// Collect actions with timeout
	timer := time.NewTimer(TurnTimeout)
	collected := 0
	for collected < aliveCount {
		select {
		case msg := <-s.actionCh:
			s.mu.Lock()
			s.actions[msg.PlayerName] = msg.Action
			s.mu.Unlock()
			collected++
		case <-timer.C:
			goto resolve
		}
	}
	timer.Stop()

resolve:
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default action for players who didn't submit: Wait
	for _, p := range s.state.Players {
		if !p.IsDead() {
			if _, ok := s.actions[p.Name]; !ok {
				s.actions[p.Name] = PlayerAction{Type: ActionWait}
			}
		}
	}

	s.resolvePlayerActions()
	s.resolveMonsterActions()
}

func (s *GameSession) resolvePlayerActions() {
	var intents []combat.AttackIntent

	// Track which monsters were alive before this turn (for gold awards)
	aliveBeforeTurn := make(map[int]bool)
	for i, m := range s.state.Monsters {
		if !m.IsDead() {
			aliveBeforeTurn[i] = true
		}
	}

	// Check if ALL alive players chose flee — only then the party flees
	fleeCount := 0
	aliveCount := 0
	for _, p := range s.state.Players {
		if p.IsDead() {
			continue
		}
		aliveCount++
		if action, ok := s.actions[p.Name]; ok && action.Type == ActionFlee {
			fleeCount++
		}
	}
	if fleeCount == aliveCount && aliveCount > 0 {
		if combat.AttemptFlee() {
			s.state.Phase = PhaseExploring
			return
		}
		// Flee failed — all fleeing players waste their turn, continue to monster phase
		return
	}

	for _, p := range s.state.Players {
		if p.IsDead() {
			continue
		}
		action, ok := s.actions[p.Name]
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
		case ActionSkill:
			switch p.Class {
			case entity.ClassWarrior:
				// Taunt: mark all monsters to target this warrior
				for _, m := range s.state.Monsters {
					if !m.IsDead() {
						m.TauntTarget = true
						m.TauntTurns = 2
					}
				}
			case entity.ClassMage:
				intents = append(intents, combat.AttackIntent{
					PlayerATK:  p.EffectiveATK(),
					TargetIdx:  -1,
					Multiplier: 0.8,
					IsAoE:      true,
				})
			case entity.ClassHealer:
				if action.TargetIdx >= 0 && action.TargetIdx < len(s.state.Players) {
					s.state.Players[action.TargetIdx].Heal(30)
				}
			case entity.ClassRogue:
				// Scout: reveal neighboring rooms
				currentRoom := s.state.Floor.Rooms[s.state.Floor.CurrentRoom]
				for _, neighborIdx := range currentRoom.Neighbors {
					s.state.Floor.Rooms[neighborIdx].Visited = true
				}
			}
		case ActionItem:
			// Use first consumable from inventory
			for i, item := range p.Inventory {
				if item.Type == entity.ItemConsumable {
					p.Heal(item.Bonus)
					p.Inventory = append(p.Inventory[:i], p.Inventory[i+1:]...)
					break
				}
			}
		case ActionFlee:
			// Individual flee does nothing if not unanimous (already handled above)
		case ActionWait:
			// Defensive stance — no action
		}
	}

	if len(intents) > 0 && len(s.state.Monsters) > 0 {
		combat.ResolveAttacks(intents, s.state.Monsters)
	}

	// Award gold only for monsters that JUST died this turn
	for i, m := range s.state.Monsters {
		if m.IsDead() && aliveBeforeTurn[i] {
			goldReward := 5 + s.state.FloorNum
			if goldReward > 15 {
				goldReward = 15
			}
			for _, p := range s.state.Players {
				if !p.IsDead() {
					p.Gold += goldReward
				}
			}
			// Boss kill: drop relic
			if m.IsBoss {
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
		// Check if this was the boss room -> advance floor
		if s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Type == dungeon.RoomBoss {
			s.advanceFloor()
		} else {
			s.state.Phase = PhaseExploring
		}
	}
}

func (s *GameSession) advanceFloor() {
	if s.state.FloorNum >= 20 {
		s.state.Phase = PhaseResult
		s.state.Victory = true
		s.state.GameOver = true
		return
	}
	s.state.FloorNum++
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum)
	s.state.Phase = PhaseExploring
	s.state.CombatTurn = 0
	// Revive dead players at 30% HP
	for _, p := range s.state.Players {
		if p.IsDead() {
			p.Revive(0.30)
		}
	}
}

func (s *GameSession) grantBossRelic() {
	relics := []entity.Relic{
		{Name: "Vampiric Ring", Effect: entity.RelicHealOnKill, Value: 5, Price: 100},
		{Name: "Power Amulet", Effect: entity.RelicATKBoost, Value: 3, Price: 120},
		{Name: "Iron Ward", Effect: entity.RelicDEFBoost, Value: 2, Price: 100},
		{Name: "Gold Charm", Effect: entity.RelicGoldBoost, Value: 5, Price: 150},
	}
	for _, p := range s.state.Players {
		if !p.IsDead() {
			r := relics[rand.Intn(len(relics))]
			p.Relics = append(p.Relics, r)
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
			// Boss AoE: 0.5x damage to all
			for _, p := range s.state.Players {
				if !p.IsDead() {
					dmg := combat.CalcDamage(m.ATK, p.EffectiveDEF(), 0.5)
					p.TakeDamage(dmg)
				}
			}
		} else {
			if targetIdx >= 0 && targetIdx < len(s.state.Players) {
				p := s.state.Players[targetIdx]
				if !p.IsDead() {
					dmg := combat.CalcDamage(m.ATK, p.EffectiveDEF(), 1.0)
					p.TakeDamage(dmg)
				}
			}
		}
		m.TickTaunt()
	}

	// Check party wipe
	allPlayersDead := true
	for _, p := range s.state.Players {
		if !p.IsDead() {
			allPlayersDead = false
			break
		}
	}
	if allPlayersDead {
		s.state.Phase = PhaseResult
	}
}
