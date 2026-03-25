package game

import "testing"

func TestParseEmote(t *testing.T) {
	tests := []struct {
		input    string
		isEmote  bool
		expected string
	}{
		{"/hi", true, "👋 waves hello!"},
		{"/gg", true, "🎉 says GG!"},
		{"/go", true, "⚔️ says Let's go!"},
		{"/wait", true, "✋ says Wait!"},
		{"/help", true, "🆘 calls for help!"},
		{"/unknown", false, ""},
		{"hello", false, ""},
		{"", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := ParseEmote(tt.input)
			if ok != tt.isEmote {
				t.Errorf("ParseEmote(%q) isEmote = %v, want %v", tt.input, ok, tt.isEmote)
			}
			if ok && result != tt.expected {
				t.Errorf("ParseEmote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
