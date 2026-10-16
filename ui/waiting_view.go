package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type WaitingScreen struct {
	ready bool
}

func NewWaitingScreen() *WaitingScreen {
	return &WaitingScreen{}
}

func (s *WaitingScreen) pollWaiting(ctx *Context) tea.Cmd {
	return scheduleTick(ctx, time.Second)
}

func (s *WaitingScreen) startGame(ctx *Context) (Screen, tea.Cmd) {
	snap, ok := ctx.Lobby.RoomSnapshot(ctx.RoomCode)
	if ok && snap.Session != nil {
		ctx.Session = snap.Session
		ctx.Session.StartGame()
		ctx.Lobby.StartRoom(ctx.RoomCode)
		gs := NewGameScreen()
		gs.gameState = ctx.Session.GetState()
		return gs, gs.pollState(ctx)
	}
	return s, s.pollWaiting(ctx)
}

func (s *WaitingScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:

		if ctx.Lobby != nil && ctx.Lobby.AllReady(ctx.RoomCode) {
			return s.startGame(ctx)
		}
		return s, s.pollWaiting(ctx)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		if isEnter(key) && !s.ready {
			s.ready = true
			if ctx.Lobby != nil {
				ctx.Lobby.SetPlayerReady(ctx.RoomCode, ctx.Fingerprint, true)

				if snap, ok := ctx.Lobby.RoomSnapshot(ctx.RoomCode); ok && len(snap.Players) == 1 {
					return s.startGame(ctx)
				}
			}
		} else if isKey(key, "esc") || key.Type == tea.KeyEsc {

			if ctx.Lobby != nil {
				ctx.Lobby.SetPlayerReady(ctx.RoomCode, ctx.Fingerprint, false)
				ctx.Lobby.LeaveRoom(ctx.RoomCode, ctx.Fingerprint)
				ctx.Lobby.LeaveSession(ctx.RoomCode, ctx.Fingerprint)
			}
			ctx.Session = nil
			ctx.RoomCode = ""
			ls := NewLobbyScreen()
			ls.refreshLobby(ctx)
			return ls, nil
		}
	}
	return s, nil
}

func (s *WaitingScreen) View(ctx *Context) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	readyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46"))

	notReadyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	header := headerStyle.Render(fmt.Sprintf("── 대기실 [%s] ──", ctx.RoomCode))

	playerList := ""
	if ctx.Lobby != nil {
		if snap, ok := ctx.Lobby.RoomSnapshot(ctx.RoomCode); ok {
			for _, p := range snap.Players {
				status := notReadyStyle.Render("...")
				if p.Ready {
					status = readyStyle.Render("준비 완료")
				}
				cls := p.Class
				if cls == "" {
					cls = "?"
				}
				playerList += fmt.Sprintf("  %s (%s) %s\n", p.Name, cls, status)
			}
		}
	}

	menu := "[Enter] 준비"
	if s.ready {
		menu = "다른 플레이어를 기다리는 중..."
	}
	menu += "  [Esc] 나가기"

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		playerList,
		"",
		menu,
	)
}
