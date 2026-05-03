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

func TestDetectCombo_IronAegis(t *testing.T) {
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"m": {Class: entity.ClassMage, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Iron Aegis" {
		t.Errorf("expected Iron Aegis, got %+v", combos)
	}
	if combos[0].Effect.WarriorBulwarkOverride != 0.75 {
		t.Errorf("Iron Aegis bulwark override = %v, want 0.75", combos[0].Effect.WarriorBulwarkOverride)
	}
}

func TestDetectCombo_Crusader(t *testing.T) {
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Crusader" {
		t.Errorf("expected Crusader, got %+v", combos)
	}
}

func TestDetectCombo_ExposeWeakness(t *testing.T) {
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"r": {Class: entity.ClassRogue, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Expose Weakness" {
		t.Errorf("expected Expose Weakness, got %+v", combos)
	}
	if !combos[0].Effect.GuaranteeCritNextTurn {
		t.Error("Expose Weakness should set GuaranteeCritNextTurn")
	}
}

func TestDetectCombo_Sanctuary(t *testing.T) {
	actions := map[string]ComboAction{
		"m": {Class: entity.ClassMage, ActionType: "coop"},
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Sanctuary" {
		t.Errorf("expected Sanctuary, got %+v", combos)
	}
}

func TestDetectCombo_Mirage(t *testing.T) {
	actions := map[string]ComboAction{
		"m": {Class: entity.ClassMage, ActionType: "coop"},
		"r": {Class: entity.ClassRogue, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Mirage" {
		t.Errorf("expected Mirage, got %+v", combos)
	}
}

func TestDetectCombo_VenomousEdge(t *testing.T) {
	actions := map[string]ComboAction{
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
		"r": {Class: entity.ClassRogue, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Venomous Edge" {
		t.Errorf("expected Venomous Edge, got %+v", combos)
	}
}

func TestDetectCombo_TripleCoopStacksAllPairs(t *testing.T) {
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"m": {Class: entity.ClassMage, ActionType: "coop"},
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	names := make(map[string]bool)
	for _, c := range combos {
		names[c.Name] = true
	}
	for _, want := range []string{"Iron Aegis", "Crusader", "Sanctuary"} {
		if !names[want] {
			t.Errorf("expected %s in triple-coop, got %+v", want, names)
		}
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
