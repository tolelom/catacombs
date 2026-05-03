package game

var emotes = map[string]string{
	"/hi":     "👋 인사합니다!",
	"/gg":     "🎉 GG!",
	"/go":     "⚔️ 가자!",
	"/wait":   "✋ 기다려!",
	"/help":   "🆘 도움 요청!",
	"/dance":  "💃 신나게 춤춘다!",
	"/cheer":  "📣 응원한다!",
	"/laugh":  "🤣 폭소!",
	"/cry":    "😭 흐엉...",
	"/think":  "🤔 음...",
	"/sorry":  "🙇 미안!",
	"/thanks": "🙏 고마워!",
	"/rage":   "🤬 분노!",
}

// EmoteList returns the list of available emote shortcuts (sorted-ish by usage).
func EmoteList() []string {
	return []string{
		"/hi", "/gg", "/go", "/wait", "/help",
		"/dance", "/cheer", "/laugh", "/cry", "/think",
		"/sorry", "/thanks", "/rage",
	}
}

func ParseEmote(input string) (string, bool) {
	if input == "" {
		return "", false
	}
	text, ok := emotes[input]
	return text, ok
}
