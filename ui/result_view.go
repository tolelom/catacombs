package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

func renderResult(won bool, floorReached int, rankings []store.RunRecord) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	var title string
	if won {
		title = titleStyle.Render("VICTORY! You escaped the Catacombs!")
	} else {
		title = titleStyle.Render("GAME OVER")
	}

	floorInfo := fmt.Sprintf("Floor Reached: B%d", floorReached)

	rankHeader := lipgloss.NewStyle().Bold(true).Render("── Rankings ──")
	rankList := ""
	for i, r := range rankings {
		rankList += fmt.Sprintf("  %d. %s — B%d (Score: %d)\n", i+1, r.Player, r.Floor, r.Score)
	}

	menu := "[Enter] Return to Lobby  [Q] Quit"

	return lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		floorInfo,
		"",
		rankHeader,
		rankList,
		"",
		menu,
	)
}
