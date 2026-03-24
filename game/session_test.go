package game

import (
	"testing"
	"time"

	"github.com/tolelom/catacombs/entity"
)

func TestGetStateNoRace(t *testing.T) {
	s := NewGameSession()
	p := entity.NewPlayer("Racer", entity.ClassWarrior)
	s.AddPlayer(p)
	s.StartGame()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			st := s.GetState()
			for _, p := range st.Players {
				_ = p.HP
				_ = p.Gold
			}
			for _, m := range st.Monsters {
				_ = m.HP
			}
		}
	}()

	for i := 0; i < 10; i++ {
		select {
		case s.actionCh <- playerActionMsg{PlayerName: "Racer", Action: PlayerAction{Type: ActionWait}}:
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	<-done
}

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
