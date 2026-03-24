package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

func renderAchievements(playerName string, achievements []store.Achievement, width, height int) string {
	title := styleHeader.Render("── Achievements ──")

	var content string
	unlocked := 0
	for _, a := range achievements {
		icon := styleSystem.Render("  ○ ")
		nameStyle := styleSystem
		if a.Unlocked {
			icon = styleGold.Render("  ★ ")
			nameStyle = stylePlayer
			unlocked++
		}
		content += icon + nameStyle.Render(a.Name) + "\n"
		content += styleSystem.Render("    "+a.Description) + "\n"
	}

	progress := fmt.Sprintf("\n  %s", styleGold.Render(fmt.Sprintf("%d/%d Unlocked", unlocked, len(achievements))))

	footer := styleSystem.Render("\n[A] Back")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, progress, footer))
}
