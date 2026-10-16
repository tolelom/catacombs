package game

import (
	"fmt"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) recheckPhaseQuorumLocked() {
	switch s.state.Phase {
	case PhaseExploring:
		if winner, met := s.tallyMoveQuorumLocked(); met {
			s.moveVotes = nil
			s.state.MoveVotes = nil
			s.enterRoomLocked(winner)
		}
	case PhaseShop:
		if !s.tallyShopLeaversLocked() {
			return
		}
		s.shopLeavers = nil
		s.state.Phase = PhaseExploring
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
	case PhaseEvent:

		alive := make(map[string]bool, len(s.state.Players))
		for _, p := range s.state.Players {
			if !p.IsOut() && p.Fingerprint != "" {
				alive[p.Fingerprint] = true
			}
		}
		for fp := range s.eventVotes {
			if !alive[fp] {
				delete(s.eventVotes, fp)
				delete(s.state.EventVotes, fp)
			}
		}
		if len(alive) == 0 {
			s.clearPendingEventLocked(true)
			return
		}
		if s.tallyEventQuorumLocked() {
			s.resolveEventVoteLocked()
		}
	}
}

func (s *GameSession) SubmitForesight(fingerprint string, roomIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseExploring {
		return fmt.Errorf("정찰은 탐험 중에만 사용 가능")
	}
	if s.scoutUsedThisFloor[fingerprint] {
		return fmt.Errorf("정찰은 층당 1회만 사용 가능")
	}
	var rogue *entity.Player
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			rogue = p
			break
		}
	}
	if rogue == nil || rogue.IsOut() || rogue.Class != entity.ClassRogue {
		return fmt.Errorf("정찰은 살아있는 도적만 사용 가능")
	}
	if s.state.Floor == nil || roomIdx < 0 || roomIdx >= len(s.state.Floor.Rooms) {
		return fmt.Errorf("유효하지 않은 방")
	}
	room := s.state.Floor.Rooms[roomIdx]
	preview := &RoomPreview{RoomIdx: roomIdx, RoomTypeLabel: room.Type.String()}
	switch room.Type {
	case dungeon.RoomCombat:
		preview.MonsterNames = []string{"전투방"}
	case dungeon.RoomBoss:
		preview.HideMonsters = true
	}
	s.nextRoomPreview = preview
	s.scoutUsedThisFloor[fingerprint] = true
	s.addLog(fmt.Sprintf("%s 정찰 사용! 다음 방: %s", rogue.Name, room.Type.String()))
	return nil
}

func (s *GameSession) tallyMoveQuorumLocked() (int, bool) {
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		return -1, false
	}
	voteCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.moveVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}
	if voteCount < aliveCount {
		return -1, false
	}
	tally := make(map[int]int)
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if room, ok := s.moveVotes[p.Fingerprint]; ok {
				tally[room]++
			}
		}
	}
	bestRoom := -1
	bestCount := 0
	for room, count := range tally {
		if count > bestCount || (count == bestCount && (bestRoom < 0 || room < bestRoom)) {
			bestRoom = room
			bestCount = count
		}
	}
	if bestRoom < 0 {
		return -1, false
	}
	return bestRoom, true
}

func (s *GameSession) SubmitMoveVote(fingerprint string, roomIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Phase != PhaseExploring {
		return false
	}

	if s.moveVotes == nil {
		s.moveVotes = make(map[string]int)
	}
	s.moveVotes[fingerprint] = roomIdx

	s.state.MoveVotes = make(map[string]int, len(s.moveVotes))
	for k, v := range s.moveVotes {
		s.state.MoveVotes[k] = v
	}

	winner, met := s.tallyMoveQuorumLocked()
	if !met {
		return false
	}

	s.moveVotes = nil
	s.state.MoveVotes = nil
	s.enterRoomLocked(winner)
	return true
}

func (s *GameSession) tallyEventQuorumLocked() bool {
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		return false
	}
	voteCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.eventVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}
	return voteCount >= aliveCount
}

func (s *GameSession) SubmitEventVote(fingerprint string, choiceIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Phase != PhaseEvent || s.pendingEvent == nil {
		return false
	}
	if choiceIdx < 0 || choiceIdx >= len(s.pendingEvent.Choices) {
		return false
	}
	if s.eventVotes == nil {
		s.eventVotes = make(map[string]int)
	}
	if fingerprint != "" {
		s.eventVotes[fingerprint] = choiceIdx
	}

	s.state.EventVotes = make(map[string]int, len(s.eventVotes))
	for k, v := range s.eventVotes {
		s.state.EventVotes[k] = v
	}

	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		s.clearPendingEventLocked(true)
		return true
	}

	if aliveCount == 1 && fingerprint == "" && len(s.eventVotes) == 0 {
		s.resolveEventVoteLocked()
		return true
	}
	if !s.tallyEventQuorumLocked() {
		return false
	}
	s.resolveEventVoteLocked()
	return true
}

func (s *GameSession) clearPendingEventLocked(markRoomCleared bool) {
	s.pendingEvent = nil
	s.eventPerPlayer = false
	s.state.PendingEvent = nil
	s.state.EventVotes = nil
	s.eventVotes = nil
	if markRoomCleared && s.state.Floor != nil {
		idx := s.state.Floor.CurrentRoom
		if idx >= 0 && idx < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[idx].Cleared = true
		}
	}
	s.state.Phase = PhaseExploring
}
