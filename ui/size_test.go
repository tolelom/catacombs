package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
)

func newSizeTestModel(width, height int) Model {
	cfg, _ := config.Load("")
	return NewModel(width, height, "", game.NewLobby(cfg), nil)
}

// assertViewWithin fails when the rendered frame exceeds the terminal size cap.
func assertViewWithin(t *testing.T, view string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > maxTerminalHeight {
		t.Fatalf("view has %d lines, cap is %d", len(lines), maxTerminalHeight)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > maxTerminalWidth {
			t.Fatalf("line %d is %d cells wide, cap is %d", i, w, maxTerminalWidth)
		}
	}
}

func TestHugeResizeIsCappedBeforeRendering(t *testing.T) {
	m := newSizeTestModel(80, 24)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 30000, Height: 30000})

	assertViewWithin(t, next.(Model).View())
}

func TestHugeInitialPtyIsCappedBeforeRendering(t *testing.T) {
	m := newSizeTestModel(30000, 30000)

	assertViewWithin(t, m.View())
}

func TestNormalSizeIsKept(t *testing.T) {
	m := newSizeTestModel(80, 24)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	got := next.(Model).ctx
	if got.Width != 120 || got.Height != 40 {
		t.Fatalf("size changed to %dx%d, want 120x40", got.Width, got.Height)
	}
}
