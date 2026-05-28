package game

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

// Boss target → ResolveAttacks skips the 5% miss / 5% glance rolls, so the
// smite landing is deterministic.
func bossDummy() *entity.Monster {
	return &entity.Monster{Name: "Dummy", HP: 500, MaxHP: 500, ATK: 1, DEF: 0, IsBoss: true}
}

func TestHealerSmitesWhenNoOneWounded(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassHealer) // solo healer, full HP
	healer := s.state.Players[0]
	healer.SkillUses = 3
	s.state.Monsters = []*entity.Monster{bossDummy()}
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionSkill, TargetIdx: 0},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false

	if s.state.Monsters[0].HP >= 500 {
		t.Errorf("healer should smite when no ally needs healing; monster HP=%d", s.state.Monsters[0].HP)
	}
	if healer.SkillUses != 2 {
		t.Errorf("smite should consume one skill use, got %d", healer.SkillUses)
	}
}

func TestHealerHealsWoundedAllyInsteadOfSmiting(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	healer.SkillUses = 3
	w := s.state.Players[1]
	w.HP = 10 // wounded
	mon := bossDummy()
	s.state.Monsters = []*entity.Monster{mon}
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionSkill, TargetIdx: 1},
		w.Fingerprint:      {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false

	if w.HP <= 10 {
		t.Errorf("wounded ally should be healed, HP=%d", w.HP)
	}
	if mon.HP != 500 {
		t.Errorf("monster must be untouched when the healer heals, HP=%d", mon.HP)
	}
}
