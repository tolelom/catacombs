package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

type screen int

const (
	screenTitle screen = iota
	screenLobby
	screenClassSelect
	screenGame
	screenShop
	screenResult
)

// StateUpdateMsg is sent by GameSession to update the view
type StateUpdateMsg struct {
	State game.GameState
}

type Model struct {
	width       int
	height      int
	fingerprint string
	playerName  string
	screen      screen

	// Shared references (set by server)
	lobby *game.Lobby
	store *store.DB

	// Per-session state
	session     *game.GameSession
	roomCode    string
	gameState   game.GameState
	lobbyState  lobbyState
	classState  classSelectState
	inputBuffer string
}

func NewModel(width, height int, fingerprint string, lobby *game.Lobby, db *store.DB) Model {
	return Model{
		width:       width,
		height:      height,
		fingerprint: fingerprint,
		screen:      screenTitle,
		lobby:       lobby,
		store:       db,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case StateUpdateMsg:
		m.gameState = msg.State
		return m, nil
	}

	switch m.screen {
	case screenTitle:
		return m.updateTitle(msg)
	case screenLobby:
		return m.updateLobby(msg)
	case screenClassSelect:
		return m.updateClassSelect(msg)
	case screenGame:
		return m.updateGame(msg)
	case screenShop:
		return m.updateShop(msg)
	case screenResult:
		return m.updateResult(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.width < 80 || m.height < 24 {
		return fmt.Sprintf("Terminal too small (%dx%d). Minimum: 80x24.", m.width, m.height)
	}
	switch m.screen {
	case screenTitle:
		return renderTitle(m.width, m.height)
	case screenLobby:
		return renderLobby(m.lobbyState, m.width, m.height)
	case screenClassSelect:
		return renderClassSelect(m.classState, m.width, m.height)
	case screenGame:
		return renderGame(m.gameState, m.width, m.height)
	case screenShop:
		return renderShop(m.gameState, m.width, m.height)
	case screenResult:
		var rankings []store.RunRecord
		if m.store != nil {
			rankings, _ = m.store.TopRuns(10)
		}
		return renderResult(m.gameState.Victory, m.gameState.FloorNum, rankings)
	}
	return ""
}

func (m Model) updateTitle(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if m.store != nil {
				name, err := m.store.GetProfile(m.fingerprint)
				if err != nil {
					m.playerName = "Adventurer"
				} else {
					m.playerName = name
				}
			} else {
				m.playerName = "Adventurer"
			}
			m.screen = screenLobby
			m.refreshLobbyState()
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateLobby(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "c":
			if m.lobby != nil {
				code := m.lobby.CreateRoom(m.playerName + "'s Room")
				m.lobby.JoinRoom(code, m.playerName)
				m.roomCode = code
				m.screen = screenClassSelect
			}
		case "up":
			if m.lobbyState.cursor > 0 {
				m.lobbyState.cursor--
			}
		case "down":
			if m.lobbyState.cursor < len(m.lobbyState.rooms)-1 {
				m.lobbyState.cursor++
			}
		case "enter":
			if m.lobby != nil && len(m.lobbyState.rooms) > 0 {
				r := m.lobbyState.rooms[m.lobbyState.cursor]
				if err := m.lobby.JoinRoom(r.Code, m.playerName); err == nil {
					m.roomCode = r.Code
					m.screen = screenClassSelect
				}
			}
		case "q":
			m.screen = screenTitle
		}
	}
	return m, nil
}

func (m Model) updateClassSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up":
			if m.classState.cursor > 0 {
				m.classState.cursor--
			}
		case "down":
			if m.classState.cursor < len(classOptions)-1 {
				m.classState.cursor++
			}
		case "enter":
			if m.lobby != nil {
				selectedClass := classOptions[m.classState.cursor].class
				room := m.lobby.GetRoom(m.roomCode)
				if room != nil {
					if room.Session == nil {
						room.Session = game.NewGameSession()
					}
					m.session = room.Session
					player := entity.NewPlayer(m.playerName, selectedClass)
					player.Fingerprint = m.fingerprint
					m.session.AddPlayer(player)
					m.session.StartGame()
					m.gameState = m.session.GetState()
					m.screen = screenGame
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.gameState.GameOver {
		m.screen = screenResult
		return m, nil
	}
	if m.gameState.Phase == game.PhaseShop {
		m.screen = screenShop
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch m.gameState.Phase {
		case game.PhaseExploring:
			if key.String() >= "0" && key.String() <= "9" {
				idx := int(key.String()[0] - '0')
				if m.session != nil {
					m.session.EnterRoom(idx)
					m.gameState = m.session.GetState()
				}
			}
		case game.PhaseCombat:
			if m.session != nil {
				switch key.String() {
				case "1":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionAttack, TargetIdx: 0})
				case "2":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionSkill, TargetIdx: 0})
				case "3":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionItem})
				case "4":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionFlee})
				case "5":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionWait})
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateShop(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "1", "2", "3":
			if m.session != nil {
				idx := int(key.String()[0] - '1')
				m.session.BuyItem(m.playerName, idx)
				m.gameState = m.session.GetState()
			}
		case "q":
			if m.session != nil {
				m.session.LeaveShop()
				m.gameState = m.session.GetState()
				m.screen = screenGame
			}
		}
	}
	return m, nil
}

func (m Model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.screen = screenLobby
			m.refreshLobbyState()
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) refreshLobbyState() {
	if m.lobby == nil {
		return
	}
	rooms := m.lobby.ListRooms()
	m.lobbyState.rooms = make([]roomInfo, len(rooms))
	for i, r := range rooms {
		status := "Waiting"
		if r.Status == game.RoomPlaying {
			status = "Playing"
		}
		m.lobbyState.rooms[i] = roomInfo{
			Code:    r.Code,
			Name:    r.Name,
			Players: len(r.Players),
			Status:  status,
		}
	}
	m.lobbyState.cursor = 0
}
