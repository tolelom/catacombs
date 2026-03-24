package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

func renderLeaderboard(byFloor, byGold []store.RunRecord, width, height int) string {
	title := styleHeader.Render("── Leaderboard ──")

	// By Floor
	var floorSection string
	floorSection += styleCoop.Render("  Top by Floor") + "\n"
	for i, r := range byFloor {
		if i >= 5 {
			break
		}
		medal := fmt.Sprintf("  %d.", i+1)
		cls := ""
		if r.Class != "" {
			cls = fmt.Sprintf(" [%s]", r.Class)
		}
		floorSection += fmt.Sprintf("  %s %s%s  B%d  %s\n",
			medal, stylePlayer.Render(r.Player), styleSystem.Render(cls),
			r.Floor, styleGold.Render(fmt.Sprintf("%dg", r.Score)))
	}

	// By Gold
	var goldSection string
	goldSection += styleCoop.Render("\n  Top by Gold") + "\n"
	for i, r := range byGold {
		if i >= 5 {
			break
		}
		medal := fmt.Sprintf("  %d.", i+1)
		cls := ""
		if r.Class != "" {
			cls = fmt.Sprintf(" [%s]", r.Class)
		}
		goldSection += fmt.Sprintf("  %s %s%s  B%d  %s\n",
			medal, stylePlayer.Render(r.Player), styleSystem.Render(cls),
			r.Floor, styleGold.Render(fmt.Sprintf("%dg", r.Score)))
	}

	footer := styleSystem.Render("\n[L] Back")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", floorSection, goldSection, footer))
}
