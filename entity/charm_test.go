package entity

import "testing"

func TestNewLuckyCharm(t *testing.T) {
	c := NewLuckyCharm(5)
	if c.Type != ItemCharm {
		t.Errorf("charm type should be ItemCharm, got %v", c.Type)
	}
	if c.Price != 35 {
		t.Errorf("charm price should be 30+floor=35, got %d", c.Price)
	}
	if c.Name == "" {
		t.Error("charm should have a name")
	}
}

func TestConsumeCharmRemovesOne(t *testing.T) {
	p := NewPlayer("A", ClassWarrior)
	p.Inventory = []Item{
		{Name: "sword", Type: ItemWeapon, Bonus: 5},
		NewLuckyCharm(1),
		NewLuckyCharm(1),
	}
	if !p.ConsumeCharm() {
		t.Fatal("ConsumeCharm should succeed with charms in inventory")
	}
	charms := 0
	for _, it := range p.Inventory {
		if it.Type == ItemCharm {
			charms++
		}
	}
	if charms != 1 {
		t.Errorf("exactly one charm should be consumed, %d remain", charms)
	}
	if len(p.Inventory) != 2 {
		t.Errorf("inventory should shrink to 2, got %d", len(p.Inventory))
	}
}

func TestConsumeCharmWithoutCharm(t *testing.T) {
	p := NewPlayer("A", ClassWarrior)
	p.Inventory = []Item{{Name: "sword", Type: ItemWeapon, Bonus: 5}}
	if p.ConsumeCharm() {
		t.Error("ConsumeCharm should fail without a charm")
	}
	if len(p.Inventory) != 1 {
		t.Error("inventory must be untouched on failed consume")
	}
}

func TestCharmDoesNotAffectCombatStats(t *testing.T) {
	p := NewPlayer("A", ClassWarrior)
	atk, def := p.EffectiveATK(), p.EffectiveDEF()
	p.Inventory = append(p.Inventory, NewLuckyCharm(10))
	if p.EffectiveATK() != atk || p.EffectiveDEF() != def {
		t.Error("charm must not change ATK/DEF")
	}
}
