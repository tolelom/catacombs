package game

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) MarkDisconnected(fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disconnectedAt[fingerprint] = s.clock()
}

func (s *GameSession) MarkConnected(fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.disconnectedAt, fingerprint)
}

func (s *GameSession) sweepDisconnectedLocked(now time.Time) bool {
	remaining := make([]*entity.Player, 0, len(s.state.Players))
	for _, p := range s.state.Players {
		if s.pastDisconnectGraceLocked(p, now) {
			slog.Warn("disconnected player removed", "fingerprint", p.Fingerprint, "name", p.Name)
			s.addLog(fmt.Sprintf("%s 제거됨 (접속 끊김)", p.Name))
			delete(s.disconnectedAt, p.Fingerprint)
			continue
		}
		remaining = append(remaining, p)
	}
	if len(remaining) == len(s.state.Players) {
		return false
	}
	s.state.Players = remaining
	if len(remaining) > 0 {
		s.recheckPhaseQuorumLocked()
	}
	return true
}

func (s *GameSession) pastDisconnectGraceLocked(p *entity.Player, now time.Time) bool {
	since, ok := s.disconnectedAt[p.Fingerprint]
	return ok && now.Sub(since) > disconnectGrace
}
