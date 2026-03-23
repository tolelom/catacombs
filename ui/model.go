package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenTitle screen = iota
)

type Model struct {
	width       int
	height      int
	fingerprint string
	screen      screen
}

func NewModel(width, height int, fingerprint string) Model {
	return Model{
		width:       width,
		height:      height,
		fingerprint: fingerprint,
		screen:      screenTitle,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	return "Welcome to Catacombs!\n\nPress q to quit."
}
