package game

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

// TestBossKilledFromFullHPSkipsPhaseTwo pins that a boss killed outright in
// one turn does not enter phase 2. If it breaks, the B10 boss summons two
// slimes from its corpse and the room does not clear.
func TestBossKilledFromFullHPSkipsPhaseTwo(t *testing.T) {
	s, p := killRewardSession(t)
	s.state.FloorNum = 10
	boss := entity.NewMonster(entity.MonsterBoss10, 10, 1.15)
	s.state.Monsters = []*entity.Monster{boss}
	p.ATK = 100000
	s.actions[p.Fingerprint] = PlayerAction{Type: ActionAttack, TargetIdx: 0}

	s.resolvePlayerActions()

	if s.state.BossPhase2Reached {
		t.Error("a dead boss entered phase 2")
	}
	if len(s.state.Monsters) != 0 {
		t.Errorf("%d monsters left after the boss died, want the room cleared", len(s.state.Monsters))
	}
}
