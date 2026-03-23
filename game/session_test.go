package game

import (
	"testing"
	"time"

	"github.com/tolelom/catacombs/entity"
)

func TestSessionTurnTimeout(t *testing.T) {
	s := NewGameSession()
	p := entity.NewPlayer("test", entity.ClassWarrior)
	s.AddPlayer(p)
	s.StartFloor()

	// Don't submit any action, wait for timeout
	done := make(chan struct{})
	go func() {
		s.RunTurn()
		close(done)
	}()

	select {
	case <-done:
		// Turn completed via timeout
	case <-time.After(7 * time.Second):
		t.Error("Turn did not timeout within 7 seconds")
	}
}
