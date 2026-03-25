package game

import "testing"

func TestParseEmote(t *testing.T) {
	tests := []struct {
		input    string
		isEmote  bool
		expected string
	}{
		{"/hi", true, "👋 인사합니다!"},
		{"/gg", true, "🎉 GG!"},
		{"/go", true, "⚔️ 가자!"},
		{"/wait", true, "✋ 기다려!"},
		{"/help", true, "🆘 도움 요청!"},
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
