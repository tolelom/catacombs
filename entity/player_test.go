package entity

import (
	"strings"
	"testing"
)

func TestNewPlayer(t *testing.T) {
	p := NewPlayer("testuser", ClassWarrior)
	if p.HP != 120 || p.MaxHP != 120 {
		t.Errorf("Warrior HP: got %d, want 120", p.HP)
	}
	if p.ATK != 12 {
		t.Errorf("Warrior ATK: got %d, want 12", p.ATK)
	}
	if p.DEF != 8 {
		t.Errorf("Warrior DEF: got %d, want 8", p.DEF)
	}
	if p.Gold != 0 {
		t.Errorf("Initial gold: got %d, want 0", p.Gold)
	}
}

func TestAllClasses(t *testing.T) {
	tests := []struct {
		class    Class
		hp, atk, def int
	}{
		{ClassWarrior, 120, 12, 8},
		{ClassMage, 70, 20, 3},
		{ClassHealer, 90, 8, 5},
		{ClassRogue, 85, 15, 4},
	}
	for _, tt := range tests {
		p := NewPlayer("test", tt.class)
		if p.HP != tt.hp || p.ATK != tt.atk || p.DEF != tt.def {
			t.Errorf("Class %v: got HP=%d ATK=%d DEF=%d, want HP=%d ATK=%d DEF=%d",
				tt.class, p.HP, p.ATK, p.DEF, tt.hp, tt.atk, tt.def)
		}
	}
}

func TestRelicEffects(t *testing.T) {
	p := NewPlayer("test", ClassWarrior)
	p.Relics = append(p.Relics, Relic{Name: "Power Amulet", Effect: RelicATKBoost, Value: 3})
	if p.EffectiveATK() != 15 {
		t.Errorf("ATK with relic: got %d, want 15", p.EffectiveATK())
	}
	p.Relics = append(p.Relics, Relic{Name: "Iron Ward", Effect: RelicDEFBoost, Value: 2})
	if p.EffectiveDEF() != 10 {
		t.Errorf("DEF with relic: got %d, want 10", p.EffectiveDEF())
	}
}

func TestPlayerTakeDamage(t *testing.T) {
	p := NewPlayer("test", ClassWarrior)
	p.TakeDamage(30)
	if p.HP != 90 {
		t.Errorf("HP after 30 dmg: got %d, want 90", p.HP)
	}
	p.TakeDamage(200)
	if p.HP != 0 {
		t.Errorf("HP should not go below 0: got %d", p.HP)
	}
	if !p.IsDead() {
		t.Error("Player should be dead")
	}
}

func TestIsOut(t *testing.T) {
	p := NewPlayer("test", ClassWarrior)
	if p.IsOut() {
		t.Error("alive player should not be out")
	}
	p.Dead = true
	if !p.IsOut() {
		t.Error("dead player should be out")
	}
	p.Dead = false
	p.Fled = true
	if !p.IsOut() {
		t.Error("fled player should be out")
	}
}

func TestRevive(t *testing.T) {
	p := NewPlayer("test", ClassWarrior) // 120 MaxHP
	p.TakeDamage(200)
	if !p.IsDead() {
		t.Error("should be dead")
	}
	p.Revive(0.30)
	if p.IsDead() {
		t.Error("should be alive after revive")
	}
	if p.HP != 36 { // 120 * 0.30
		t.Errorf("HP should be 36, got %d", p.HP)
	}
}

func TestHealCap(t *testing.T) {
	p := NewPlayer("test", ClassWarrior) // 120 HP
	p.HP = 100
	p.Heal(50) // should cap at 120
	if p.HP != 120 {
		t.Errorf("HP should cap at 120, got %d", p.HP)
	}
}

func TestEffectiveATKWithItems(t *testing.T) {
	p := NewPlayer("test", ClassWarrior) // base ATK 12
	p.Inventory = append(p.Inventory, Item{Name: "Sword", Type: ItemWeapon, Bonus: 5})
	p.Inventory = append(p.Inventory, Item{Name: "Sword2", Type: ItemWeapon, Bonus: 3})
	if p.EffectiveATK() != 20 { // 12 + 5 + 3
		t.Errorf("ATK should be 20, got %d", p.EffectiveATK())
	}
}

func TestEffectiveDEFWithItems(t *testing.T) {
	p := NewPlayer("test", ClassWarrior) // base DEF 8
	p.Inventory = append(p.Inventory, Item{Name: "Shield", Type: ItemArmor, Bonus: 4})
	if p.EffectiveDEF() != 12 { // 8 + 4
		t.Errorf("DEF should be 12, got %d", p.EffectiveDEF())
	}
}

func TestStatusEffectPoison(t *testing.T) {
	p := NewPlayer("test", ClassWarrior) // 120 HP
	p.AddEffect(ActiveEffect{Type: StatusPoison, Duration: 2, Value: 10})
	if !p.HasEffect(StatusPoison) {
		t.Error("should have poison")
	}
	msgs := p.TickEffects()
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
	if p.HP != 110 {
		t.Errorf("HP should be 110 after poison tick, got %d", p.HP)
	}
	// Poison can't kill
	p.HP = 5
	p.TickEffects() // duration expires after this tick
	if p.HP != 1 {
		t.Errorf("poison should leave at 1 HP, got %d", p.HP)
	}
	if p.IsDead() {
		t.Error("poison should not kill")
	}
	if p.HasEffect(StatusPoison) {
		t.Error("poison should have expired")
	}
}

func TestStatusEffectBurn(t *testing.T) {
	p := NewPlayer("test", ClassMage) // 70 HP
	p.AddEffect(ActiveEffect{Type: StatusBurn, Duration: 1, Value: 100})
	p.TickEffects()
	if !p.IsDead() {
		t.Error("burn should be able to kill")
	}
}

func TestRelicPoisonImmunity(t *testing.T) {
	p := NewPlayer("test", ClassWarrior)
	p.Relics = append(p.Relics, Relic{Name: "Antidote", Effect: RelicPoisonImmunity})
	p.AddEffect(ActiveEffect{Type: StatusPoison, Duration: 3, Value: 10})
	if p.HasEffect(StatusPoison) {
		t.Error("should be immune to poison")
	}
}

func TestRelicBurnResist(t *testing.T) {
	p := NewPlayer("test", ClassWarrior)
	p.Relics = append(p.Relics, Relic{Name: "Flame Guard", Effect: RelicBurnResist})
	p.AddEffect(ActiveEffect{Type: StatusBurn, Duration: 2, Value: 10})
	// Burn value should be halved to 5
	if len(p.Effects) == 0 {
		t.Fatal("should have burn effect (resisted, not immune)")
	}
	if p.Effects[0].Value != 5 {
		t.Errorf("burn value should be halved to 5, got %d", p.Effects[0].Value)
	}
}

func TestEffectOverwrite(t *testing.T) {
	p := NewPlayer("test", ClassWarrior)
	p.AddEffect(ActiveEffect{Type: StatusPoison, Duration: 1, Value: 5})
	p.AddEffect(ActiveEffect{Type: StatusPoison, Duration: 3, Value: 10}) // should overwrite
	if len(p.Effects) != 1 {
		t.Errorf("should have 1 effect, got %d", len(p.Effects))
	}
	if p.Effects[0].Duration != 3 || p.Effects[0].Value != 10 {
		t.Error("should have overwritten with new values")
	}
}

func TestBleedEffect(t *testing.T) {
	p := NewPlayer("Test", ClassWarrior)
	startHP := p.HP
	p.AddEffect(ActiveEffect{Type: StatusBleed, Duration: 3, Value: 2})

	msgs := p.TickEffects()
	if len(msgs) == 0 || !strings.Contains(msgs[0], "bleed") {
		t.Error("expected bleed damage message")
	}
	if p.HP != startHP-2 {
		t.Errorf("expected HP %d, got %d", startHP-2, p.HP)
	}
	// After tick, remaining bleed should have value 3 (increased by 1)
	if len(p.Effects) == 0 || p.Effects[0].Value != 3 {
		t.Error("expected bleed value to increase to 3")
	}
}

func TestCurseReducesHealing(t *testing.T) {
	p := NewPlayer("Test", ClassHealer)
	p.HP = 50
	p.AddEffect(ActiveEffect{Type: StatusCurse, Duration: 3, Value: 50})
	p.Heal(100)
	// Curse reduces by 50%, so heal 50 from HP 50 -> 100, capped at MaxHP
	expected := p.MaxHP
	if 50+50 < p.MaxHP {
		expected = 50 + 50
	}
	if p.HP != expected {
		t.Errorf("expected HP %d, got %d", expected, p.HP)
	}
}

func TestFreezeTickMessage(t *testing.T) {
	p := NewPlayer("Test", ClassMage)
	p.AddEffect(ActiveEffect{Type: StatusFreeze, Duration: 1, Value: 0})
	msgs := p.TickEffects()
	if len(msgs) == 0 || !strings.Contains(msgs[0], "frozen") {
		t.Error("expected freeze message")
	}
	// Freeze duration 1 -> removed after tick
	if len(p.Effects) != 0 {
		t.Error("expected freeze to be removed after 1 tick")
	}
}
