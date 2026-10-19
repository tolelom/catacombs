package game

import (
	"fmt"

	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) checkBossPhaseTransitions() {
	threshold := s.bossPhaseThreshold()
	for _, m := range s.state.Monsters {
		if !m.CheckPhaseTransitionAt(threshold) {
			continue
		}
		s.state.BossPhase2Reached = true
		s.addLog(fmt.Sprintf("⚡ %s 광폭화! Phase 2 진입!", m.Name))
		s.applyBossPhaseTwoLocked(m)
	}
}

func (s *GameSession) bossPhaseThreshold() int {
	if s.ActiveCurse != nil && s.ActiveCurse.ID == "cruel" {
		return 70
	}
	return 50
}

func (s *GameSession) applyBossPhaseTwoLocked(m *entity.Monster) {
	switch s.state.FloorNum {
	case 5:
		s.enrageBossLocked(m)
	case 10:
		s.summonPhaseTwoSlimesLocked(m)
	case 15:
		s.exposeBossWeaknessLocked(m)
	case 20:
		s.curseBossFoesLocked(m)
	}
}

func (s *GameSession) enrageBossLocked(m *entity.Monster) {
	m.ATK = int(float64(m.ATK) * 1.5)
	if m.Pattern == entity.PatternHeal {
		m.Pattern = entity.PatternNone
	}
	s.addLog(fmt.Sprintf("  %s ATK +50%%!", m.Name))
}

func (s *GameSession) summonPhaseTwoSlimesLocked(m *entity.Monster) {
	floor := s.state.FloorNum
	scaling := s.cfg.Combat.MonsterScaling
	for i := 0; i < 2; i++ {
		summon := entity.NewMonster(entity.MonsterSlime, floor, scaling)
		summon.HP = summon.MaxHP * 2
		summon.MaxHP = summon.HP
		s.applyDifficultyMods(summon)
		s.state.Monsters = append(s.state.Monsters, summon)
	}
	s.addLog(fmt.Sprintf("  %s이(가) 슬라임 2마리를 소환!", m.Name))
}

func (s *GameSession) exposeBossWeaknessLocked(m *entity.Monster) {
	m.Vulnerable = true
	s.addLog(fmt.Sprintf("  %s의 약점이 노출됩니다! (받는 피해 1.5배)", m.Name))
}

func (s *GameSession) curseBossFoesLocked(m *entity.Monster) {
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
