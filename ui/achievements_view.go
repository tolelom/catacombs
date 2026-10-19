package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

type AchievementsScreen struct{}

func NewAchievementsScreen() *AchievementsScreen {
	return &AchievementsScreen{}
}

func (s *AchievementsScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "a") || isEnter(key) || isQuit(key) {
			return NewTitleScreen(), nil
		}
	}
	return s, nil
}

func (s *AchievementsScreen) View(ctx *Context) string {
	var achievements []store.Achievement
	if ctx.Store != nil {
		achievements, _ = ctx.Store.GetAchievements(ctx.Fingerprint)
	}
	return renderAchievements(ctx.PlayerName, achievements, ctx.Width, ctx.Height)
}

func renderAchievements(playerName string, achievements []store.Achievement, width, height int) string {
	title := styleHeader.Render("── 업적 ──")

	var content string
	unlocked := 0
	for _, a := range achievements {
		icon := styleSystem.Render("  ○ ")
		nameStyle := styleSystem
		if a.Unlocked {
			icon = styleGold.Render("  ★ ")
			nameStyle = stylePlayer
			unlocked++
		}
		content += icon + nameStyle.Render(a.Name) + "\n"
		content += styleSystem.Render("    "+a.Description) + "\n"
	}

	const barWidth = 20
	filled := 0
	if len(achievements) > 0 {
		filled = unlocked * barWidth / len(achievements)
		if unlocked > 0 && filled == 0 {
			filled = 1
		}
	}
	bar := lipgloss.NewStyle().Foreground(colorYellow).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(colorGray).Render(strings.Repeat("░", barWidth-filled))
	progress := fmt.Sprintf("\n  %s  %s", bar,
		styleGold.Render(fmt.Sprintf("%d/%d 해금", unlocked, len(achievements))))

	footer := styleSystem.Render("\n[A] 뒤로")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, progress, footer))
}
