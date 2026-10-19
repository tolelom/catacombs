package ui

import tea "github.com/charmbracelet/bubbletea"

type Screen interface {
	Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd)
	View(ctx *Context) string
}
