package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var titleArt = `
 ██████╗ █████╗ ████████╗ █████╗  ██████╗ ██████╗ ███╗   ███╗██████╗ ███████╗
██╔════╝██╔══██╗╚══██╔══╝██╔══██╗██╔════╝██╔═══██╗████╗ ████║██╔══██╗██╔════╝
██║     ███████║   ██║   ███████║██║     ██║   ██║██╔████╔██║██████╔╝███████╗
██║     ██╔══██║   ██║   ██╔══██║██║     ██║   ██║██║╚██╔╝██║██╔══██╗╚════██║
╚██████╗██║  ██║   ██║   ██║  ██║╚██████╗╚██████╔╝██║ ╚═╝ ██║██████╔╝███████║
 ╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚══════╝
`

func renderTitle(width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Align(lipgloss.Center)

	menuStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render(titleArt),
		"",
		subtitleStyle.Render("A Co-op Roguelike Adventure"),
		"",
		menuStyle.Render("[Enter] Start  [Q] Quit"),
	)
}
