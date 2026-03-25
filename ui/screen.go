package ui

import tea "github.com/charmbracelet/bubbletea"

// Screen represents an independent screen with its own Update and View logic.
// Update returns the next Screen (can return itself or a different screen for transitions)
// plus a tea.Cmd for async operations.
type Screen interface {
	Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd)
	View(ctx *Context) string
}
