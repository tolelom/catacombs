package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderNickname(input string, width, height int) string {
	title := styleHeader.Render("── Enter Your Name ──")

	display := input
	if display == "" {
		display = strings.Repeat("_", 12)
	} else {
		display = input + "_"
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(0, 2).
		Render(stylePlayer.Render(display))

	hint := styleSystem.Render(fmt.Sprintf("(%d/12 characters)", len(input)))
	footer := styleAction.Render("[Enter] Confirm    [Esc] Cancel")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", inputBox, hint, "", footer))
}
