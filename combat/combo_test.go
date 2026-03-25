package combat

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func TestDetectCombo_IceShatter(t *testing.T) {
	actions := map[string]ComboAction{
		"mage":    {Class: entity.ClassMage, ActionType: "skill"},
		"warrior": {Class: entity.ClassWarrior, ActionType: "attack"},
	}
	combos := DetectCombos(actions)
	if len(combos) == 0 {
		t.Fatal("expected Ice Shatter combo")
	}
	if combos[0].Name != "Ice Shatter" {
		t.Errorf("expected 'Ice Shatter', got %q", combos[0].Name)
	}
}

func TestDetectCombo_NoMatch(t *testing.T) {
	actions := map[string]ComboAction{
		"w1": {Class: entity.ClassWarrior, ActionType: "attack"},
		"w2": {Class: entity.ClassWarrior, ActionType: "attack"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 0 {
		t.Errorf("expected no combos, got %d", len(combos))
	}
}

func TestDetectCombo_Multiple(t *testing.T) {
	// Healer skill + Warrior attack + Rogue item → Holy Assault + Restoration
	actions := map[string]ComboAction{
		"healer":  {Class: entity.ClassHealer, ActionType: "skill"},
		"warrior": {Class: entity.ClassWarrior, ActionType: "attack"},
		"rogue":   {Class: entity.ClassRogue, ActionType: "item"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 2 {
		t.Errorf("expected 2 combos, got %d", len(combos))
	}
}
