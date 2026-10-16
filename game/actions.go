package game

import (
	"fmt"
)

func (s *GameSession) SubmitAction(playerID string, action PlayerAction) {
	s.mu.Lock()
	if !s.isAliveMemberLocked(playerID) {
		s.mu.Unlock()
		return
	}

	if _, already := s.state.SubmittedActions[playerID]; already {
		s.mu.Unlock()
		return
	}

	desc := ""
	switch action.Type {
	case ActionAttack:
		desc = "공격"
	case ActionSkill:
		desc = "스킬 사용"
	case ActionItem:
		desc = "아이템 사용"
	case ActionFlee:
		desc = "도주"
	case ActionWait:
		desc = "방어"
	}
	if s.state.SubmittedActions == nil {
		s.state.SubmittedActions = make(map[string]string)
	}
	s.state.SubmittedActions[playerID] = desc
	turn := s.state.TurnNum
	s.mu.Unlock()
	s.actionCh <- playerActionMsg{PlayerID: playerID, Action: action, Turn: turn}
}

func (s *GameSession) isAliveMemberLocked(fingerprint string) bool {
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			return !p.IsOut()
		}
	}
	return false
}

func (s *GameSession) RevealNextLog() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if len(s.state.PendingLogs) == 0 || now.Sub(s.lastReveal) < logRevealInterval {
		return false
	}
	s.lastReveal = now
	msg := s.state.PendingLogs[0]
	s.state.PendingLogs = s.state.PendingLogs[1:]
	s.state.CombatLog = append(s.state.CombatLog, msg)
	if len(s.state.CombatLog) > 8 {
		s.state.CombatLog = s.state.CombatLog[len(s.state.CombatLog)-8:]
	}
	return true
}

func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int, choice byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			if p.Skills == nil || p.Skills.Points <= p.Skills.Allocated {
				return fmt.Errorf("스킬 포인트가 없습니다")
			}
			before := p.Skills.GetMaxHPBonus(p.Class)
			if err := p.Skills.Allocate(branchIdx, p.Class, choice); err != nil {
				return err
			}

			delta := p.Skills.GetMaxHPBonus(p.Class) - before
			if delta > 0 {
				p.MaxHP += delta
				p.HP += delta
			}
			return nil
		}
	}
	return fmt.Errorf("플레이어를 찾을 수 없습니다")
}

func (s *GameSession) SendChat(playerName, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if emoteText, ok := ParseEmote(message); ok {
		s.addLog(fmt.Sprintf("✨ %s %s", playerName, emoteText))
	} else {
		s.addLog(fmt.Sprintf("[%s] %s", playerName, message))
	}
}
