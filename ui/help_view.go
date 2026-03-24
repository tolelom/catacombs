package ui

import (
	"github.com/charmbracelet/lipgloss"
)

func renderHelp(width, height int) string {
	title := styleHeader.Render("── Controls ──")

	sections := []struct{ header, body string }{
		{"Exploration", `  [Up/Down]  Select room
  [Enter]    Move to room
  [/]        Chat
  [Q]        Quit`},
		{"Combat", `  [1]  Attack      [2]  Skill
  [3]  Use Item    [4]  Flee
  [5]  Defend      [Tab] Switch Target
  [/]  Chat`},
		{"Shop", `  [1-3]  Buy item
  [Q]    Leave shop`},
		{"Classes", `  Warrior  120HP 12ATK 8DEF  Taunt (draw fire 2t)
  Mage      70HP 20ATK 3DEF  Fireball (AoE 0.8x)
  Healer    90HP  8ATK 5DEF  Heal (restore 30HP)
  Rogue     85HP 15ATK 4DEF  Scout (reveal rooms)`},
		{"Tips", `  • Skills have 3 uses per combat
  • Co-op bonus: 10% extra when 2+ attack same target
  • Items are limited to 10 per player
  • Dead players revive next floor at 30% HP`},
	}

	var content string
	headerStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(colorWhite)

	for _, s := range sections {
		content += headerStyle.Render(s.header) + "\n"
		content += bodyStyle.Render(s.body) + "\n\n"
	}

	footer := styleSystem.Render("[H] Back")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, footer))
}
