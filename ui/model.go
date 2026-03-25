package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// StateUpdateMsg is sent by GameSession to update the view
type StateUpdateMsg struct {
	State game.GameState
}

type tickMsg struct{}

type Model struct {
	currentScreen Screen
	ctx           *Context
}

func NewModel(width, height int, fingerprint string, lobby *game.Lobby, db *store.DB) Model {
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	ctx := &Context{
		Width:       width,
		Height:      height,
		Fingerprint: fingerprint,
		Lobby:       lobby,
		Store:       db,
	}

	// Determine initial screen
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

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ctx.Width = msg.Width
		m.ctx.Height = msg.Height
		if m.ctx.Width == 0 {
			m.ctx.Width = 80
		}
		if m.ctx.Height == 0 {
			m.ctx.Height = 24
		}
		return m, nil
	case StateUpdateMsg:
		if gs, ok := m.currentScreen.(*GameScreen); ok {
			gs.gameState = msg.State
		}
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

// Key helper functions used by all screens.
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

func isUp(key tea.KeyMsg) bool {
	return isKey(key, "up") || key.Type == tea.KeyUp
}

func isDown(key tea.KeyMsg) bool {
	return isKey(key, "down") || key.Type == tea.KeyDown
}

// Keep these for backward compatibility with tests
// screen enum kept temporarily for test compatibility
type screen int

const (
	screenTitle screen = iota
	screenLobby
	screenClassSelect
	screenGame
	screenShop
	screenResult
	screenHelp
	screenStats
	screenAchievements
	screenLeaderboard
	screenNickname
)

// screenType returns the screen enum for the current screen (for test compatibility).
func (m Model) screenType() screen {
	switch m.currentScreen.(type) {
	case *TitleScreen:
		return screenTitle
	case *LobbyScreen:
		return screenLobby
	case *ClassSelectScreen:
		return screenClassSelect
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

// Convenience accessors for test compatibility
func (m Model) playerName() string {
	return m.ctx.PlayerName
}

func (m Model) roomCode() string {
	return m.ctx.RoomCode
}

func (m Model) session() *game.GameSession {
	return m.ctx.Session
}

