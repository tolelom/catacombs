package ui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NicknameScreen handles first-time player name input.
type NicknameScreen struct {
	input string
}

func NewNicknameScreen() *NicknameScreen {
	return &NicknameScreen{}
}

func (s *NicknameScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isEnter(key) && len(s.input) > 0 {
			ctx.PlayerName = s.input
			if ctx.Store != nil && ctx.Fingerprint != "" {
				if err := ctx.Store.SaveProfile(ctx.Fingerprint, ctx.PlayerName); err != nil {
					slog.Error("failed to save profile", "error", err)
				}
			}
			if ctx.Lobby != nil {
				ctx.Lobby.PlayerOnline(ctx.Fingerprint, ctx.PlayerName)
			}
			// Check for active session to reconnect
			if ctx.Lobby != nil {
				code, session := ctx.Lobby.GetActiveSession(ctx.Fingerprint)
				if session != nil {
					ctx.RoomCode = code
					ctx.Session = session
					gs := NewGameScreen()
					gs.gameState = ctx.Session.GetState()
					ctx.Session.TouchActivity(ctx.Fingerprint)
					ctx.Session.SendChat("System", ctx.PlayerName+" 재접속!")
					return gs, gs.pollState()
				}
			}
			ls := NewLobbyScreen()
			ls.refreshLobby(ctx)
			return ls, ls.pollLobby()
		} else if isKey(key, "esc") || key.Type == tea.KeyEsc {
			s.input = ""
			return NewTitleScreen(), nil
		} else if key.Type == tea.KeyBackspace && len(s.input) > 0 {
			s.input = s.input[:len(s.input)-1]
		} else if len(key.Runes) == 1 && len(s.input) < 12 {
			ch := string(key.Runes)
			if len(ch) == 1 && ch[0] >= 32 && ch[0] < 127 {
				s.input += ch
			}
		}
	}
	return s, nil
}

func (s *NicknameScreen) View(ctx *Context) string {
	return renderNickname(s.input, ctx.Width, ctx.Height)
}

func renderNickname(input string, width, height int) string {
	title := styleHeader.Render("── 이름을 입력하세요 ──")

	display := input
	if display == "" {
		display = strings.Repeat("_", 12)
	} else {
		display = input + "_"
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(0, 2).
		Render(stylePlayer.Render(display))

	hint := styleSystem.Render(fmt.Sprintf("(%d/12 글자)", len(input)))
	footer := styleAction.Render("[Enter] 확인    [Esc] 취소")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", inputBox, hint, "", footer))
}
