package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

// LeaderboardScreen shows the top runs.
type LeaderboardScreen struct {
	tab int // 0=all-time, 1=gold, 2=daily
}

func NewLeaderboardScreen() *LeaderboardScreen {
	return &LeaderboardScreen{}
}

func (s *LeaderboardScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "tab") || key.Type == tea.KeyTab {
			s.tab = (s.tab + 1) % 3
			return s, nil
		}
		if isKey(key, "l") || isEnter(key) || isQuit(key) {
			return NewTitleScreen(), nil
		}
	}
	return s, nil
}

func (s *LeaderboardScreen) View(ctx *Context) string {
	var byFloor, byGold []store.RunRecord
	var daily []store.DailyRecord
	if ctx.Store != nil {
		byFloor, _ = ctx.Store.TopRuns(10)
		byGold, _ = ctx.Store.TopRunsByGold(10)
		daily, _ = ctx.Store.GetDailyLeaderboard(time.Now().Format("2006-01-02"), 20)
	}
	return renderLeaderboard(byFloor, byGold, daily, s.tab, ctx.Width, ctx.Height)
}

func renderLeaderboard(byFloor, byGold []store.RunRecord, daily []store.DailyRecord, tab, width, height int) string {
	title := styleHeader.Render("── 리더보드 ──")

	// Tab header
	tabs := []string{"층수", "골드", "일일"}
	var tabLine string
	for i, t := range tabs {
		if i == tab {
			tabLine += styleHeader.Render(fmt.Sprintf(" [%s] ", t))
		} else {
			tabLine += styleSystem.Render(fmt.Sprintf("  %s  ", t))
		}
	}

	var content string

	switch tab {
	case 0: // By Floor
		content += styleCoop.Render("  층수 순위") + "\n"
		for i, r := range byFloor {
			if i >= 10 {
				break
			}
			medal := fmt.Sprintf("  %d.", i+1)
			cls := ""
			if r.Class != "" {
				cls = fmt.Sprintf(" [%s]", r.Class)
			}
			party := ""
			if len(r.Members) > 1 {
				party = styleSystem.Render(fmt.Sprintf(" (%s)", strings.Join(r.Members, ", ")))
			}
			content += fmt.Sprintf("  %s %s%s  B%d  %s%s\n",
				medal, stylePlayer.Render(r.Player), styleSystem.Render(cls),
				r.Floor, styleGold.Render(fmt.Sprintf("%dg", r.Score)), party)
		}
	case 1: // By Gold
		content += styleCoop.Render("  골드 순위") + "\n"
		for i, r := range byGold {
			if i >= 10 {
				break
			}
			medal := fmt.Sprintf("  %d.", i+1)
			cls := ""
			if r.Class != "" {
				cls = fmt.Sprintf(" [%s]", r.Class)
			}
			party := ""
			if len(r.Members) > 1 {
				party = styleSystem.Render(fmt.Sprintf(" (%s)", strings.Join(r.Members, ", ")))
			}
			content += fmt.Sprintf("  %s %s%s  B%d  %s%s\n",
				medal, stylePlayer.Render(r.Player), styleSystem.Render(cls),
				r.Floor, styleGold.Render(fmt.Sprintf("%dg", r.Score)), party)
		}
	case 2: // Daily
		content += styleCoop.Render(fmt.Sprintf("  일일 도전 — %s", time.Now().Format("2006-01-02"))) + "\n"
		if len(daily) == 0 {
			content += "  오늘 일일 도전 기록이 없습니다.\n"
		}
		for i, r := range daily {
			if i >= 20 {
				break
			}
			medal := fmt.Sprintf("  %d.", i+1)
			content += fmt.Sprintf("  %s %s  B%d  %s\n",
				medal, stylePlayer.Render(r.PlayerName),
				r.FloorReached, styleGold.Render(fmt.Sprintf("%dg", r.GoldEarned)))
		}
	}

	footer := styleSystem.Render("\n[Tab] 탭 전환  [L] 뒤로")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, tabLine, "", content, footer))
}
