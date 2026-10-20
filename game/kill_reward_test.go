package game

import (
	"strings"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func killRewardSession(t *testing.T) (*GameSession, *entity.Player) {
	t.Helper()
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Killer", entity.ClassWarrior)
	p.Fingerprint = "fp-killer"
	s.AddPlayer(p)
	s.StartFloor()
	s.mu.Lock()
	t.Cleanup(s.mu.Unlock)
	s.state.Phase = PhaseCombat
	s.state.Floor.CurrentRoom = 1
	s.actions = map[string]PlayerAction{p.Fingerprint: {Type: ActionWait}}
	return s, p
}

func logContains(log []string, part string) bool {
	for _, line := range log {
		if strings.Contains(line, part) {
			return true
		}
	}
	return false
}

// TestPoisonKillPaysKillReward pins that a monster finished by a damage-over-
// time tick pays the same kill reward as one killed by an attack. If it
// breaks, the party clears the room with poison and gets no gold.
func TestPoisonKillPaysKillReward(t *testing.T) {
	s, p := killRewardSession(t)
	slime := entity.NewMonster(entity.MonsterSlime, 1, 1.15)
	slime.HP = 1
	slime.AddEffect(entity.ActiveEffect{Type: entity.StatusPoison, Duration: 2, Value: 5})
	s.state.Monsters = []*entity.Monster{slime}

	s.resolvePlayerActions()

	if p.Gold == 0 {
		t.Errorf("poison kill paid no gold; log %v", s.state.CombatLog)
	}
	if !logContains(s.state.CombatLog, "Slime 처치!") {
		t.Errorf("poison kill was not logged as a kill; log %v", s.state.CombatLog)
	}
}
