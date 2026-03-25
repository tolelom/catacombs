package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpScreen shows controls and tips.
type HelpScreen struct{}

func NewHelpScreen() *HelpScreen {
	return &HelpScreen{}
}

func (s *HelpScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "h") || isEnter(key) || isQuit(key) {
			return NewTitleScreen(), nil
		}
	}
	return s, nil
}

func (s *HelpScreen) View(ctx *Context) string {
	return renderHelp(ctx.Width, ctx.Height)
}

func renderHelp(width, height int) string {
	title := styleHeader.Render("── Controls ──")

	sections := []struct{ header, body string }{
		{"Lobby", `  [C]  Create room    [J]  Join by code
  [Enter] Join selected room
  [D]  Daily Challenge  [H]  Hard Mode toggle
  [Q]  Back to title`},
		{"Exploration", `  [Up/Down]  Select room
  [Enter]    Move to room
  [[]  / []]  Allocate skill point (branch 1/2)
  [/]        Chat
  [Q]        Quit`},
		{"Combat (10s per turn)", `  [1]  Attack      [2]  Skill
  [3]  Use Item    [4]  Flee
  [5]  Defend      [Tab] Switch Target
  [/]  Chat`},
		{"Shop", `  [1-3]  Buy item
  [Q]    Leave shop`},
		{"Classes", `  Warrior  120HP 12ATK 8DEF  Taunt (draw fire 2t)
  Mage      70HP 20ATK 3DEF  Fireball (AoE 0.8x)
  Healer    90HP  8ATK 5DEF  Heal (restore 30HP)
  Rogue     85HP 15ATK 4DEF  Scout (reveal rooms)`},
		{"Multiplayer", `  • Up to 4 players per room
  • Co-op bonus: +10% dmg when 2+ target same enemy
  • Class combos trigger bonus effects
  • All players ready → game starts`},
		{"Tips", `  • Skills have 3 uses per combat
  • Items are limited to 10 per player
  • Dead players revive next floor at 30% HP
  • Bosses appear at floors 5, 10, 15, 20
  • Skill points: 1 per floor clear (max 3)
  • Weekly mutations rotate gameplay modifiers`},
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
