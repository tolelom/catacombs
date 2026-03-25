package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

// AchievementsScreen shows the player's achievements.
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
		achievements, _ = ctx.Store.GetAchievements(ctx.PlayerName)
	}
	return renderAchievements(ctx.PlayerName, achievements, ctx.Width, ctx.Height)
}

func renderAchievements(playerName string, achievements []store.Achievement, width, height int) string {
	title := styleHeader.Render("── Achievements ──")

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

	progress := fmt.Sprintf("\n  %s", styleGold.Render(fmt.Sprintf("%d/%d Unlocked", unlocked, len(achievements))))

	footer := styleSystem.Render("\n[A] Back")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, progress, footer))
}
