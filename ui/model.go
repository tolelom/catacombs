package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

type tickMsg struct{}

const (
	defaultTerminalWidth  = 80
	defaultTerminalHeight = 24
	maxTerminalWidth      = 300
	maxTerminalHeight     = 100
)

func normalizeTerminalSize(width, height int) (int, int) {
	return clampDimension(width, defaultTerminalWidth, maxTerminalWidth),
		clampDimension(height, defaultTerminalHeight, maxTerminalHeight)
}

func clampDimension(v, fallback, max int) int {
	switch {
	case v <= 0:
		return fallback
	case v > max:
		return max
	default:
		return v
	}
}

type Model struct {
	currentScreen Screen
	ctx           *Context
}

func NewModel(width, height int, fingerprint string, lobby *game.Lobby, db *store.DB) Model {
	width, height = normalizeTerminalSize(width, height)
	ctx := &Context{
		Width:       width,
		Height:      height,
		Fingerprint: fingerprint,
		Lobby:       lobby,
		Store:       db,
	}
	var initialScreen Screen
	if fingerprint != "" && db != nil {
		if name, err := db.GetProfile(fingerprint); err == nil {
			ctx.PlayerName = name
		}
	}
	initialScreen = NewTitleScreen()

	return Model{
		currentScreen: initialScreen,
		ctx:           ctx,
	}
}

func (m Model) Fingerprint() string {
	return m.ctx.Fingerprint
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ctx.Width, m.ctx.Height = normalizeTerminalSize(msg.Width, msg.Height)
		return m, nil
	}

	next, cmd := m.currentScreen.Update(msg, m.ctx)
	m.currentScreen = next
	return m, cmd
}

func (m Model) View() string {
	if m.ctx.Width < 80 || m.ctx.Height < 24 {
		return fmt.Sprintf("Terminal too small (%dx%d). Minimum: 80x24.", m.ctx.Width, m.ctx.Height)
	}
	return m.currentScreen.View(m.ctx)
}
func isKey(key tea.KeyMsg, names ...string) bool {
	s := key.String()
	for _, n := range names {
		if s == n {
			return true
		}
	}
	return false
}

func isEnter(key tea.KeyMsg) bool {
	return isKey(key, "enter") || key.Type == tea.KeyEnter
}

func isQuit(key tea.KeyMsg) bool {
	return isKey(key, "q", "ctrl+c") || key.Type == tea.KeyCtrlC
}

func isForceQuit(key tea.KeyMsg) bool {
	return isKey(key, "ctrl+c") || key.Type == tea.KeyCtrlC
}

func isUp(key tea.KeyMsg) bool {
	return isKey(key, "up") || key.Type == tea.KeyUp
}

func isDown(key tea.KeyMsg) bool {
	return isKey(key, "down") || key.Type == tea.KeyDown
}

type screen int

const (
	screenTitle screen = iota
	screenLobby
	screenClassSelect
	screenWaiting
	screenGame
	screenShop
	screenResult
	screenHelp
	screenStats
	screenAchievements
	screenLeaderboard
	screenNickname
)

func (m Model) screenType() screen {
	switch m.currentScreen.(type) {
	case *TitleScreen:
		return screenTitle
	case *LobbyScreen:
		return screenLobby
	case *ClassSelectScreen:
		return screenClassSelect
	case *WaitingScreen:
		return screenWaiting
	case *GameScreen:
		return screenGame
	case *ShopScreen:
		return screenShop
	case *ResultScreen:
		return screenResult
	case *HelpScreen:
		return screenHelp
	case *StatsScreen:
		return screenStats
	case *AchievementsScreen:
		return screenAchievements
	case *LeaderboardScreen:
		return screenLeaderboard
	case *NicknameScreen:
		return screenNickname
	}
	return screenTitle
}
func (m Model) playerName() string {
	return m.ctx.PlayerName
}

func (m Model) roomCode() string {
	return m.ctx.RoomCode
}

func (m Model) session() *game.GameSession {
	return m.ctx.Session
}
