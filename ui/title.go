package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var titleLines = []string{
	` ██████╗ █████╗ ████████╗ █████╗  ██████╗ ██████╗ ███╗   ███╗██████╗ ███████╗`,
	`██╔════╝██╔══██╗╚══██╔══╝██╔══██╗██╔════╝██╔═══██╗████╗ ████║██╔══██╗██╔════╝`,
	`██║     ███████║   ██║   ███████║██║     ██║   ██║██╔████╔██║██████╔╝███████╗`,
	`██║     ██╔══██║   ██║   ██╔══██║██║     ██║   ██║██║╚██╔╝██║██╔══██╗╚════██║`,
	`╚██████╗██║  ██║   ██║   ██║  ██║╚██████╗╚██████╔╝██║ ╚═╝ ██║██████╔╝███████║`,
	` ╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚══════╝`,
}

var titleColors = []lipgloss.Color{
	lipgloss.Color("196"),
	lipgloss.Color("202"),
	lipgloss.Color("208"),
	lipgloss.Color("214"),
	lipgloss.Color("220"),
	lipgloss.Color("226"),
}

func renderTitle(width, height int) string {
	var logoLines []string
	for i, line := range titleLines {
		color := titleColors[i%len(titleColors)]
		style := lipgloss.NewStyle().Foreground(color).Bold(true)
		logoLines = append(logoLines, style.Render(line))
	}
	logo := strings.Join(logoLines, "\n")

	subtitle := lipgloss.NewStyle().
		Foreground(colorGray).
		Render("⚔  A Cooperative Dungeon Crawler  ⚔")

	server := lipgloss.NewStyle().
		Foreground(colorCyan).
		Render("ssh catacombs.tolelom.xyz")

	menu := lipgloss.NewStyle().
		Foreground(colorWhite).
		Bold(true).
		Render("[Enter] Start    [H] Help    [S] Stats    [Q] Quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		subtitle,
		server,
		"",
		"",
		menu,
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}
