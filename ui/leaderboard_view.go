package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

// LeaderboardScreen shows the top runs.
type LeaderboardScreen struct{}

func NewLeaderboardScreen() *LeaderboardScreen {
	return &LeaderboardScreen{}
}

func (s *LeaderboardScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "l") || isEnter(key) || isQuit(key) {
			return NewTitleScreen(), nil
		}
	}
	return s, nil
}

func (s *LeaderboardScreen) View(ctx *Context) string {
	var byFloor, byGold []store.RunRecord
	if ctx.Store != nil {
		byFloor, _ = ctx.Store.TopRuns(10)
		byGold, _ = ctx.Store.TopRunsByGold(10)
	}
	return renderLeaderboard(byFloor, byGold, ctx.Width, ctx.Height)
}

func renderLeaderboard(byFloor, byGold []store.RunRecord, width, height int) string {
	title := styleHeader.Render("── Leaderboard ──")

	// By Floor
	var floorSection string
	floorSection += styleCoop.Render("  Top by Floor") + "\n"
	for i, r := range byFloor {
		if i >= 5 {
			break
		}
		medal := fmt.Sprintf("  %d.", i+1)
		cls := ""
		if r.Class != "" {
			cls = fmt.Sprintf(" [%s]", r.Class)
		}
		floorSection += fmt.Sprintf("  %s %s%s  B%d  %s\n",
			medal, stylePlayer.Render(r.Player), styleSystem.Render(cls),
			r.Floor, styleGold.Render(fmt.Sprintf("%dg", r.Score)))
	}

	// By Gold
	var goldSection string
	goldSection += styleCoop.Render("\n  Top by Gold") + "\n"
	for i, r := range byGold {
		if i >= 5 {
			break
		}
		medal := fmt.Sprintf("  %d.", i+1)
		cls := ""
		if r.Class != "" {
			cls = fmt.Sprintf(" [%s]", r.Class)
		}
		goldSection += fmt.Sprintf("  %s %s%s  B%d  %s\n",
			medal, stylePlayer.Render(r.Player), styleSystem.Render(cls),
			r.Floor, styleGold.Render(fmt.Sprintf("%dg", r.Score)))
	}

	footer := styleSystem.Render("\n[L] Back")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", floorSection, goldSection, footer))
}
