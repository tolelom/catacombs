package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TitleScreen is the main menu screen.
type TitleScreen struct{}

func NewTitleScreen() *TitleScreen {
	return &TitleScreen{}
}

func (s *TitleScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isEnter(key) {
			if ctx.Fingerprint == "" {
				// Defensive only — server/ssh.go always supplies a real
				// SHA256:... or anon:<rand> fingerprint in production. Match
				// the production "anon:" prefix so downstream isWebUser() and
				// lobby bookkeeping see one consistent shape.
				ctx.Fingerprint = fmt.Sprintf("anon:%d", time.Now().UnixNano())
			}
			if ctx.Store != nil {
				name, err := ctx.Store.GetProfile(ctx.Fingerprint)
				if err != nil {
					// First time player — show nickname input
					return NewNicknameScreen(), nil
				}
				ctx.PlayerName = name
			} else {
				ctx.PlayerName = "Adventurer"
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
					ctx.Session.MarkConnected(ctx.Fingerprint)
					ctx.Session.SendChat("System", ctx.PlayerName+" 재접속!")
					return gs, gs.pollState(ctx)
				}
			}
			ls := NewLobbyScreen()
			ls.refreshLobby(ctx)
			return ls, ls.pollLobby(ctx)
		} else if isKey(key, "h") {
			return NewHelpScreen(), nil
		} else if isKey(key, "s") {
			return NewStatsScreen(), nil
		} else if isKey(key, "a") {
			return NewAchievementsScreen(), nil
		} else if isKey(key, "l") {
			return NewLeaderboardScreen(), nil
		} else if isKey(key, "c") {
			return NewCodexScreen(ctx), nil
		} else if isQuit(key) {
			return s, tea.Quit
		}
	}
	return s, nil
}

func (s *TitleScreen) View(ctx *Context) string {
	return renderTitle(ctx.Width, ctx.Height)
}

var titleLines = []string{
	` ██████╗ █████╗ ████████╗ █████╗  ██████╗ ██████╗ ███╗   ███╗██████╗ ███████╗`,
	`██╔════╝██╔══██╗╚══██╔══╝██╔══██╗██╔════╝██╔═══██╗████╗ ████║██╔══██╗██╔════╝`,
	`██║     ███████║   ██║   ███████║██║     ██║   ██║██╔████╔██║██████╔╝███████╗`,
	`██║     ██╔══██║   ██║   ██╔══██║██║     ██║   ██║██║╚██╔╝██║██╔══██╗╚════██║`,
	`╚██████╗██║  ██║   ██║   ██║  ██║╚██████╗╚██████╔╝██║ ╚═╝ ██║██████╔╝███████║`,
	` ╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚══════╝`,
}

var titleColors = []lipgloss.Color{
	lipgloss.Color("196"),
	lipgloss.Color("202"),
	lipgloss.Color("208"),
	lipgloss.Color("214"),
	lipgloss.Color("220"),
	lipgloss.Color("226"),
}

func renderTitle(width, height int) string {
	var logoLines []string
	for i, line := range titleLines {
		color := titleColors[i%len(titleColors)]
		style := lipgloss.NewStyle().Foreground(color).Bold(true)
		logoLines = append(logoLines, style.Render(line))
	}
	logo := strings.Join(logoLines, "\n")

	subtitle := lipgloss.NewStyle().
		Foreground(colorGray).
		Render("⚔  협동 던전 크롤러  ⚔")

	server := lipgloss.NewStyle().
		Foreground(colorCyan).
		Render("ssh catacombs.tolelom.xyz")

	menu := lipgloss.NewStyle().
		Foreground(colorWhite).
		Bold(true).
		Render("[Enter] 시작  [H] 도움말  [S] 통계  [A] 업적  [L] 리더보드  [C] 도감  [Q] 종료")

	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		subtitle,
		server,
		"",
		"",
		menu,
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}
