package game

import (
	"fmt"

	"github.com/tolelom/catacombs/combat"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) resolveMonsterActions() {
	if s.state.Phase != PhaseCombat {
		return
	}
	for _, m := range s.state.Monsters {
		if m.IsDead() {
			continue
		}
		s.runMonsterTurnLocked(m)
		m.TickTaunt()
	}
	s.endRunIfPartyWipedLocked()
}

func (s *GameSession) runMonsterTurnLocked(m *entity.Monster) {
	targetIdx, isAoE := combat.MonsterAI(s.rng, m, s.state.Players, s.state.CombatTurn)
	if !isAoE {
		s.monsterSingleAttackLocked(m, targetIdx)
		return
	}
	s.monsterAreaAttackLocked(m)
	if m.IsBoss || m.IsMiniBoss {
		s.applyBossPatternLocked(m)
	}
}

func (s *GameSession) monsterAreaAttackLocked(m *entity.Monster) {
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		if s.enemyMissNextTurn && s.rng.Float64() < 0.50 {
			s.addLog(fmt.Sprintf("%s 환영진으로 %s 광역 공격 회피!", p.Name, m.Name))
			continue
		}
		dmg := s.monsterDamageLocked(m, p, 0.5)
		s.addLog(fmt.Sprintf("%s 광역 공격으로 %s에게 %d 피해", m.Name, p.Name, dmg))
		if p.IsDead() {
			s.handlePlayerDeathLocked(p)
		}
	}
}

func (s *GameSession) monsterSingleAttackLocked(m *entity.Monster, targetIdx int) {
	if targetIdx < 0 || targetIdx >= len(s.state.Players) {
		return
	}
	p := s.state.Players[targetIdx]
	if p.IsOut() {
		return
	}
	if s.enemyMissNextTurn && s.rng.Float64() < 0.50 {
		s.addLog(fmt.Sprintf("%s 환영진으로 %s의 공격 회피!", p.Name, m.Name))
		return
	}
	dmg := s.monsterDamageLocked(m, p, 1.0)
	s.addLog(fmt.Sprintf("%s이(가) %s을(를) 공격하여 %d 피해", m.Name, p.Name, dmg))
	if m.IsElite && dmg > 0 {
		s.applyEliteOnHitLocked(m, p, dmg)
	}
	if p.IsDead() {
		s.handlePlayerDeathLocked(p)
	}
}

func (s *GameSession) monsterDamageLocked(m *entity.Monster, p *entity.Player, multiplier float64) int {
	dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), multiplier)
	dmg = s.applyDamageMods(p, dmg)
	p.TakeDamage(dmg)
	return dmg
}

func (s *GameSession) applyEliteOnHitLocked(m *entity.Monster, p *entity.Player, dmg int) {
	def := entity.ElitePrefixDefs[m.ElitePrefix]
	if def.OnHit >= 0 {
		p.AddEffect(entity.ActiveEffect{Type: def.OnHit, Duration: 2, Value: 3})
		s.addLog(fmt.Sprintf("%s의 %s 효과가 %s에게 적용!", m.Name, def.Name, p.Name))
		return
	}
	if m.ElitePrefix == entity.PrefixVampiric {
		heal := dmg / 4
		m.HP = min(m.HP+heal, m.MaxHP)
		s.addLog(fmt.Sprintf("%s이(가) %s의 생명력을 흡수! (+%d HP)", m.Name, p.Name, heal))
	}
}

func (s *GameSession) applyBossPatternLocked(m *entity.Monster) {
	switch m.Pattern {
	case entity.PatternPoison:
		s.afflictPartyLocked(entity.ActiveEffect{Type: entity.StatusPoison, Duration: 3, Value: 5}, "%s이(가) %s에게 독을 걸었습니다!", m.Name)
	case entity.PatternBurn:
		s.afflictPartyLocked(entity.ActiveEffect{Type: entity.StatusBurn, Duration: 2, Value: 8}, "%s이(가) %s을(를) 불태웠습니다!", m.Name)
	case entity.PatternFreeze:
		s.afflictPartyLocked(entity.ActiveEffect{Type: entity.StatusFreeze, Duration: 1, Value: 0}, "%s이(가) %s을(를) 동결시켰습니다!", m.Name)
	case entity.PatternHeal:
		healAmt := m.MaxHP / 10
		m.HP = min(m.HP+healAmt, m.MaxHP)
		s.addLog(fmt.Sprintf("%s HP %d 재생!", m.Name, healAmt))
	}
}

func (s *GameSession) afflictPartyLocked(effect entity.ActiveEffect, logFormat, sourceName string) {
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		p.AddEffect(effect)
		s.addLog(fmt.Sprintf(logFormat, sourceName, p.Name))
	}
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
