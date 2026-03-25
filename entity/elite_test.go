package entity

import "testing"

func TestApplyPrefix(t *testing.T) {
	m := &Monster{
		Name:  "Slime",
		HP:    100,
		MaxHP: 100,
		ATK:   10,
		DEF:   5,
	}
	ApplyPrefix(m, PrefixVenomous)

	if !m.IsElite {
		t.Fatal("expected IsElite to be true")
	}
	if m.ElitePrefix != PrefixVenomous {
		t.Fatalf("expected PrefixVenomous, got %d", m.ElitePrefix)
	}
	if m.Name != "Venomous Slime" {
		t.Fatalf("expected 'Venomous Slime', got %q", m.Name)
	}
	// HP should be multiplied by 1.3 => 130
	if m.HP != 130 {
		t.Fatalf("expected HP=130, got %d", m.HP)
	}
	if m.MaxHP != 130 {
		t.Fatalf("expected MaxHP=130, got %d", m.MaxHP)
	}
	// ATK mult is 1.0 for Venomous, so ATK stays 10
	if m.ATK != 10 {
		t.Fatalf("expected ATK=10, got %d", m.ATK)
	}
}

func TestApplyPrefixVampiric(t *testing.T) {
	m := &Monster{
		Name:  "Orc",
		HP:    100,
		MaxHP: 100,
		ATK:   20,
		DEF:   5,
	}
	ApplyPrefix(m, PrefixVampiric)

	if !m.IsElite {
		t.Fatal("expected IsElite to be true")
	}
	if m.Name != "Vampiric Orc" {
		t.Fatalf("expected 'Vampiric Orc', got %q", m.Name)
	}
	// HP * 1.4 = 140
	if m.HP != 140 {
		t.Fatalf("expected HP=140, got %d", m.HP)
	}
	// ATK * 1.1 = 22
	if m.ATK != 22 {
		t.Fatalf("expected ATK=22, got %d", m.ATK)
	}
}

func TestRandomPrefix(t *testing.T) {
	seen := make(map[ElitePrefixType]bool)
	for i := 0; i < 200; i++ {
		p := RandomPrefix()
		if p < 0 || p > 4 {
			t.Fatalf("RandomPrefix returned out-of-range value: %d", p)
		}
		seen[p] = true
	}
	// With 200 tries, all 5 prefixes should appear
	if len(seen) != 5 {
		t.Fatalf("expected all 5 prefixes to appear, got %d distinct values", len(seen))
	}
}

func TestElitePrefixString(t *testing.T) {
	if PrefixVenomous.String() != "Venomous" {
		t.Fatalf("expected 'Venomous', got %q", PrefixVenomous.String())
	}
	if PrefixBurning.String() != "Burning" {
		t.Fatalf("expected 'Burning', got %q", PrefixBurning.String())
	}
	if PrefixFreezing.String() != "Freezing" {
		t.Fatalf("expected 'Freezing', got %q", PrefixFreezing.String())
	}
	if PrefixBleeding.String() != "Bleeding" {
		t.Fatalf("expected 'Bleeding', got %q", PrefixBleeding.String())
	}
	if PrefixVampiric.String() != "Vampiric" {
		t.Fatalf("expected 'Vampiric', got %q", PrefixVampiric.String())
	}
}
