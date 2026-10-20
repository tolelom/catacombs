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

// TestTimeGlassKillPaysBossReward pins that a boss finished by a Time Glass
// extra attack counts as a boss kill. If it breaks, the party clears the boss
// room without gold, relic, BossKilled or the per-run boss record.
func TestTimeGlassKillPaysBossReward(t *testing.T) {
	s, p := killRewardSession(t)
	boss := entity.NewMonster(entity.MonsterBoss5, 5, 1.15)
	boss.HP = 1
	s.state.Monsters = []*entity.Monster{boss}
	s.extraActionNext = map[string]bool{p.Fingerprint: true}

	s.resolveTurnLocked()

	if p.Gold == 0 {
		t.Errorf("Time Glass boss kill paid no gold; log %v", s.state.CombatLog)
	}
	if !s.state.BossKilled || !s.state.BossesKilledThisRun[boss.Name] {
		t.Errorf("BossKilled=%v BossesKilledThisRun=%v, want the boss recorded", s.state.BossKilled, s.state.BossesKilledThisRun)
	}
	if len(p.Relics) != 1 {
		t.Errorf("got %d relics, want the boss relic", len(p.Relics))
	}
}
