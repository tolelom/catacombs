package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/game"
)

func renderShop(state game.GameState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)

	header := headerStyle.Render("── Shop ──")
	items := ""
	for i, item := range state.ShopItems {
		items += fmt.Sprintf("  [%d] %s (+%d) — %d gold\n", i+1, item.Name, item.Bonus, item.Price)
	}

	menu := "[1-3] Buy  [Q] Leave Shop"

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		items,
		"",
		menu,
	)
}
