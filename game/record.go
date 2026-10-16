package game

import "time"

type RunResult struct {
	State     GameState
	DailyMode bool
	DailyDate string
}

type RunRecorder interface {
	RecordRun(RunResult)
}

func (s *GameSession) setRecorder(r RunRecorder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorder = r
}

func (s *GameSession) recordRun() {
	s.recordOnce.Do(func() {
		defer close(s.recorded)
		s.mu.Lock()
		rec := s.recorder
		s.mu.Unlock()
		if rec == nil {
			return
		}
		rec.RecordRun(RunResult{State: s.GetState(), DailyMode: s.DailyMode, DailyDate: s.DailyDate})
	})
}

func (s *GameSession) WaitRecorded(timeout time.Duration) bool {
	select {
	case <-s.recorded:
		return true
	case <-time.After(timeout):
		return false
	}
}
