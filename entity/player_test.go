package entity

import "testing"

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
