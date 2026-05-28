package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/game"
)

// tagIcon maps a ChoiceTag to a short display token used in event hints.
func tagIcon(t game.ChoiceTag) string {
	switch t {
	case game.TagHeal:
		return "❤"
	case game.TagHarm:
		return "🔥"
	case game.TagGoldGain:
		return "💰+"
	case game.TagGoldCost:
		return "💰-"
	case game.TagItem:
		return "✨"
	case game.TagGamble:
		return "🎲"
	case game.TagSkip:
		return "⤳"
	}
	return "?"
}

// EventScreen handles the random-event vote phase. Mirrors ShopScreen's
// poll-based update pattern so other players' votes show up in real time.
type EventScreen struct {
	gameState game.GameState
	cursor    int
	statusMsg string
}

func NewEventScreen(state game.GameState) *EventScreen {
	return &EventScreen{gameState: state}
}

func (e *EventScreen) pollState() tea.Cmd {
	return tea.Tick(time.Millisecond*300, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (e *EventScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if _, ok := msg.(tickMsg); ok {
		if ctx.Session != nil {
			st := ctx.Session.GetState()
			e.gameState = st
			if st.Phase != game.PhaseEvent {
				gs := NewGameScreen()
				gs.gameState = st
				return gs, nil
			}
		}
		return e, e.pollState()
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "1", "2", "3":
			idx := int(key.String()[0] - '1')
			if e.gameState.PendingEvent != nil && idx >= 0 && idx < len(e.gameState.PendingEvent.Choices) {
				e.submitVote(ctx, idx)
			}
		case "up", "k":
			if e.cursor > 0 {
				e.cursor--
			}
		case "down", "j":
			if e.gameState.PendingEvent != nil && e.cursor < len(e.gameState.PendingEvent.Choices)-1 {
				e.cursor++
			}
		case "enter":
			if e.gameState.PendingEvent != nil {
				e.submitVote(ctx, e.cursor)
			}
		}
	}
	return e, e.pollState()
}

func (e *EventScreen) submitVote(ctx *Context, idx int) {
	if ctx.Session == nil {
		return
	}
	if ctx.Session.SubmitEventVote(ctx.Fingerprint, idx) {
		// Vote triggered resolution; the next tick will follow phase out.
		e.statusMsg = "결정되었습니다..."
	} else {
		e.statusMsg = fmt.Sprintf("선택지 %d에 투표 — 다른 플레이어 대기 중", idx+1)
	}
	e.gameState = ctx.Session.GetState()
}

func (e *EventScreen) View(ctx *Context) string {
	if e.gameState.PendingEvent == nil {
		return "이벤트 정보 없음"
	}
	ev := e.gameState.PendingEvent

	header := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("이벤트: %s", ev.Name))
	desc := ev.Description

	var lines []string
	lines = append(lines, header)
	lines = append(lines, desc)
	lines = append(lines, "")

	for i, c := range ev.Choices {
		var tags []string
		for _, t := range c.Tags {
			tags = append(tags, tagIcon(t))
		}
		prefix := "  "
		if i == e.cursor {
			prefix = "▶ "
		}
		lines = append(lines, fmt.Sprintf("%s%d. %s  %s", prefix, i+1, strings.Join(tags, ""), c.Label))
	}

	lines = append(lines, "")
	lines = append(lines, "투표 현황:")
	for _, p := range e.gameState.Players {
		if p.IsOut() {
			continue
		}
		if idx, ok := e.gameState.EventVotes[p.Fingerprint]; ok {
			lines = append(lines, fmt.Sprintf("  %s → #%d", p.Name, idx+1))
		} else {
			lines = append(lines, fmt.Sprintf("  %s → 대기 중...", p.Name))
		}
	}

	if e.statusMsg != "" {
		lines = append(lines, "")
		lines = append(lines, e.statusMsg)
	}
	lines = append(lines, "")
	lines = append(lines, "[1/2/3] 즉시 선택   [↑/↓] 커서   [Enter] 투표")

	return strings.Join(lines, "\n")
}
