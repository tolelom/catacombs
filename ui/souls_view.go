package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

// SoulsScreen lets the player spend accumulated souls on permanent upgrades.
type SoulsScreen struct {
	cursor    int
	souls     int64
	levels    map[store.PermaUpgrade]int
	upgrades  []store.UpgradeDef
	statusMsg string
}

// NewSoulsScreen builds a fresh souls/upgrades menu.
func NewSoulsScreen() *SoulsScreen {
	return &SoulsScreen{upgrades: store.AllUpgrades(), levels: map[store.PermaUpgrade]int{}}
}

func (s *SoulsScreen) refresh(ctx *Context) {
	if ctx.Store == nil || ctx.Fingerprint == "" {
		return
	}
	if n, err := ctx.Store.GetSouls(ctx.Fingerprint); err == nil {
		s.souls = n
	}
	if levels, err := ctx.Store.GetUpgrades(ctx.Fingerprint); err == nil {
		s.levels = levels
	}
}

// Update handles input on the souls screen.
func (s *SoulsScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isUp(key) {
			if s.cursor > 0 {
				s.cursor--
			}
		} else if isDown(key) {
			if s.cursor < len(s.upgrades)-1 {
				s.cursor++
			}
		} else if isEnter(key) {
			if ctx.Store != nil && ctx.Fingerprint != "" && s.cursor < len(s.upgrades) {
				def := s.upgrades[s.cursor]
				newLvl, err := ctx.Store.BuyUpgrade(ctx.Fingerprint, def.ID)
				switch {
				case errors.Is(err, store.ErrNotEnoughSouls):
					s.statusMsg = "영혼이 부족합니다"
				case errors.Is(err, store.ErrAtMaxLevel):
					s.statusMsg = "이미 최대 레벨"
				case err != nil:
					s.statusMsg = err.Error()
				default:
					s.statusMsg = fmt.Sprintf("%s 레벨 %d 구매!", def.Name, newLvl)
				}
				s.refresh(ctx)
			}
		} else if isKey(key, "q") || isKey(key, "esc") || key.Type == tea.KeyEsc {
			ls := NewLobbyScreen()
			ls.refreshLobby(ctx)
			return ls, ls.pollLobby(ctx)
		}
	}
	return s, nil
}

// View renders the souls screen.
func (s *SoulsScreen) View(ctx *Context) string {
	s.refresh(ctx)
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).
		Render(fmt.Sprintf("── 영혼의 제단 ── 잔액: %d 영혼 ──", s.souls))

	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	maxedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString(descStyle.Render("  런 종료시 보유 골드의 10%가 영혼으로 적립됩니다."))
	sb.WriteString("\n\n")

	for i, def := range s.upgrades {
		marker := "  "
		style := normalStyle
		if i == s.cursor {
			marker = "> "
			style = selectedStyle
		}
		lvl := s.levels[def.ID]
		costStr := fmt.Sprintf("%d 영혼", store.UpgradeCost(lvl, def))
		statusStr := fmt.Sprintf("Lv %d/%d", lvl, def.MaxLevel)
		if lvl >= def.MaxLevel {
			statusStr = maxedStyle.Render("MAX")
			costStr = "—"
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%s — %s [%s] (%s)", marker, def.Name, def.Desc, statusStr, costStr)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	if s.statusMsg != "" {
		sb.WriteString(descStyle.Render("  " + s.statusMsg))
		sb.WriteString("\n")
	}
	sb.WriteString("\n[Up/Down] 선택  [Enter] 구매  [Q/Esc] 로비로")
	return sb.String()
}
