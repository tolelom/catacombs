package game

import (
	"fmt"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

// recheckPhaseQuorumLocked runs the per-phase quorum tally and resolves
// the phase if quorum is met. Used by RemovePlayer and the combatLoop
// inactivity sweep so a player leaving (explicit Q or 60s timeout) cannot
// strand the party waiting for a vote they will never cast. Caller holds
// s.mu.
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
		// Mirror the sweep behavior: prune any stale votes for fingerprints
		// no longer in the alive set (RemovePlayer already deletes from the
		// maps but defense-in-depth keeps the tally correct if a future
		// caller forgets).
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

// SubmitForesight is the Rogue's coop ability. Available once per floor while
// exploring. Reveals the next room's type and exposes monster room context for
// informed party voting. Boss rooms hide monster names but show "보스" type.
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

// tallyMoveQuorumLocked returns the winning room index and true if every
// alive player has cast a move vote. Returns (-1, false) when quorum is
// not met or no alive players remain. Tie-break: lowest room index.
// Caller holds s.mu.
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

// SubmitMoveVote records a player's room choice during exploration.
// When all alive players have voted, the majority choice wins and the party moves.
// Returns true if the vote triggered a move (all votes collected).
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

	// Mirror to snapshot map for UI display.
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

// ClearMoveVotes resets any pending move votes (e.g. when phase changes).
func (s *GameSession) ClearMoveVotes() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveVotes = nil
	s.state.MoveVotes = nil
}

// tallyEventQuorumLocked returns true if every alive player has cast an
// event vote. Does NOT compute a winner — winner selection lives in
// resolveEventVoteLocked. Returns false when no alive players remain.
// The anonymous-solo special case stays in SubmitEventVote because it
// depends on the caller's fingerprint context. Caller holds s.mu.
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

// SubmitEventVote records a player's event choice during PhaseEvent. When
// all alive players have voted, the majority choice (ties broken by lowest
// index) is resolved and applied to the party. Returns true if this call
// triggered resolution.
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

	// Mirror to snapshot map
	s.state.EventVotes = make(map[string]int, len(s.eventVotes))
	for k, v := range s.eventVotes {
		s.state.EventVotes[k] = v
	}

	// Defensive: nobody alive to receive the outcome. Clear and bail.
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
	// Anonymous single-player test path: the caller passed no fingerprint so
	// the vote did not land in s.eventVotes — treat this call as the sole
	// alive vote.
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

// clearPendingEventLocked clears all pending-event state and optionally marks
// the current event room as cleared. Caller holds s.mu.
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
