package dungeon

import "testing"

func TestGetTheme(t *testing.T) {
	tests := []struct {
		floor int
		name  string
	}{
		{1, "Swamp"}, {5, "Swamp"},
		{6, "Volcano"}, {10, "Volcano"},
		{11, "Glacier"}, {15, "Glacier"},
		{16, "Inferno"}, {20, "Inferno"},
	}
	for _, tt := range tests {
		theme := GetTheme(tt.floor)
		if theme.Name != tt.name {
			t.Errorf("floor %d: expected %q, got %q", tt.floor, tt.name, theme.Name)
		}
	}
}

func TestThemeDamageMult(t *testing.T) {
	theme := GetTheme(1) // Swamp
	if theme.DamageMult != 1.5 {
		t.Errorf("Swamp DamageMult: expected 1.5, got %f", theme.DamageMult)
	}
	theme = GetTheme(20) // Inferno
	if theme.DamageMult != 1.3 {
		t.Errorf("Inferno DamageMult: expected 1.3, got %f", theme.DamageMult)
	}
}
