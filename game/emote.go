package game

var emotes = map[string]string{
	"/hi":   "👋 waves hello!",
	"/gg":   "🎉 says GG!",
	"/go":   "⚔️ says Let's go!",
	"/wait": "✋ says Wait!",
	"/help": "🆘 calls for help!",
}

func ParseEmote(input string) (string, bool) {
	if input == "" {
		return "", false
	}
	text, ok := emotes[input]
	return text, ok
}
