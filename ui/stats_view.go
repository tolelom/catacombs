package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

func renderStats(playerName string, stats store.PlayerStats, width, height int) string {
	title := styleHeader.Render("── Player Statistics ──")

	var content string
	content += stylePlayer.Render(fmt.Sprintf("  %s", playerName)) + "\n\n"
	content += fmt.Sprintf("  Total Runs:   %s\n", styleGold.Render(fmt.Sprintf("%d", stats.TotalRuns)))
	content += fmt.Sprintf("  Best Floor:   %s\n", styleGold.Render(fmt.Sprintf("B%d", stats.BestFloor)))
	content += fmt.Sprintf("  Total Gold:   %s\n", styleGold.Render(fmt.Sprintf("%d", stats.TotalGold)))
	content += fmt.Sprintf("  Victories:    %s\n", styleHeal.Render(fmt.Sprintf("%d", stats.Victories)))

	winRate := 0.0
	if stats.TotalRuns > 0 {
		winRate = float64(stats.Victories) / float64(stats.TotalRuns) * 100
	}
	content += fmt.Sprintf("  Win Rate:     %s\n", styleSystem.Render(fmt.Sprintf("%.1f%%", winRate)))

	footer := styleSystem.Render("[S] Back")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, "", footer))
}
