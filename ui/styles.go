package ui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	colorRed     = lipgloss.Color("196")
	colorGreen   = lipgloss.Color("46")
	colorYellow  = lipgloss.Color("226")
	colorCyan    = lipgloss.Color("51")
	colorMagenta = lipgloss.Color("201")
	colorWhite   = lipgloss.Color("255")
	colorGray    = lipgloss.Color("240")
	colorPink    = lipgloss.Color("205")
)

// Text styles
var (
	styleDamage = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleHeal   = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleCoop   = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleFlee   = lipgloss.NewStyle().Foreground(colorCyan)
	styleStatus = lipgloss.NewStyle().Foreground(colorMagenta)
	styleGold   = lipgloss.NewStyle().Foreground(colorYellow)
	styleSystem = lipgloss.NewStyle().Foreground(colorGray).Italic(true)
	styleEnemy  = lipgloss.NewStyle().Foreground(colorRed)
	stylePlayer = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	styleHeader = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	styleAction = lipgloss.NewStyle().Bold(true)
	styleTimer  = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleCombo  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
)
