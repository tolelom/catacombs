package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

type StatsScreen struct {
	stats  store.PlayerStats
	loaded bool
}

func NewStatsScreen() *StatsScreen {
	return &StatsScreen{}
}

func (s *StatsScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "s") || isEnter(key) || isQuit(key) {
			return NewTitleScreen(), nil
		}
	}
	return s, nil
}

func (s *StatsScreen) View(ctx *Context) string {
	s.loadOnce(ctx)
	return renderStats(ctx.PlayerName, s.stats, ctx.Width, ctx.Height)
}

func (s *StatsScreen) loadOnce(ctx *Context) {
	if s.loaded || ctx.Store == nil {
		return
	}
	s.loaded = true
	s.stats, _ = ctx.Store.GetStats(ctx.PlayerName)
}

func renderStats(playerName string, stats store.PlayerStats, width, height int) string {
	title := styleHeader.Render("── 플레이어 통계 ──")

	var content string
	content += stylePlayer.Render(fmt.Sprintf("  %s", playerName)) + "\n\n"
	content += fmt.Sprintf("  총 플레이:    %s\n", styleGold.Render(fmt.Sprintf("%d", stats.TotalRuns)))
	content += fmt.Sprintf("  최고 층:      %s\n", styleGold.Render(fmt.Sprintf("B%d", stats.BestFloor)))
	content += fmt.Sprintf("  총 골드:      %s\n", styleGold.Render(fmt.Sprintf("%d", stats.TotalGold)))
	content += fmt.Sprintf("  승리 횟수:    %s\n", styleHeal.Render(fmt.Sprintf("%d", stats.Victories)))

	winRate := 0.0
	if stats.TotalRuns > 0 {
		winRate = float64(stats.Victories) / float64(stats.TotalRuns) * 100
	}
	content += fmt.Sprintf("  승률:         %s\n", styleSystem.Render(fmt.Sprintf("%.1f%%", winRate)))

	footer := styleSystem.Render("[S] 뒤로")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, "", footer))
}
