package game

var emotes = map[string]string{
	"/hi":   "👋 인사합니다!",
	"/gg":   "🎉 GG!",
	"/go":   "⚔️ 가자!",
	"/wait": "✋ 기다려!",
	"/help": "🆘 도움 요청!",
}

func ParseEmote(input string) (string, bool) {
	if input == "" {
		return "", false
	}
	text, ok := emotes[input]
	return text, ok
}
