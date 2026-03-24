package ui

import (
	"fmt"
	"strings"
	"time"

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
	session      *game.GameSession
	roomCode     string
	gameState    game.GameState
	lobbyState   lobbyState
	classState   classSelectState
	inputBuffer  string
	targetCursor int
	moveCursor   int // selected neighbor index during exploration
	rankingSaved bool
}

func NewModel(width, height int, fingerprint string, lobby *game.Lobby, db *store.DB) Model {
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
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
		if m.width == 0 {
			m.width = 80
		}
		if m.height == 0 {
			m.height = 24
		}
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
		return renderGame(m.gameState, m.width, m.height, m.targetCursor, m.moveCursor)
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

func (m Model) updateTitle(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isEnter(key) {
			if m.store != nil {
				name, err := m.store.GetProfile(m.fingerprint)
				if err != nil {
					m.playerName = "Adventurer"
					if m.store != nil && m.fingerprint != "" {
						m.store.SaveProfile(m.fingerprint, m.playerName)
					}
				} else {
					m.playerName = name
				}
			} else {
				m.playerName = "Adventurer"
			}
			m.screen = screenLobby
			m = m.withRefreshedLobby()
		} else if isQuit(key) {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateLobby(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// Join-by-code input mode
		if m.lobbyState.joining {
			if isEnter(key) && len(m.lobbyState.codeInput) == 4 {
				if m.lobby != nil {
					if err := m.lobby.JoinRoom(m.lobbyState.codeInput, m.playerName); err == nil {
						m.roomCode = m.lobbyState.codeInput
						m.screen = screenClassSelect
					}
				}
				m.lobbyState.joining = false
				m.lobbyState.codeInput = ""
			} else if isKey(key, "esc") || key.Type == tea.KeyEsc {
				m.lobbyState.joining = false
				m.lobbyState.codeInput = ""
			} else if key.Type == tea.KeyBackspace && len(m.lobbyState.codeInput) > 0 {
				m.lobbyState.codeInput = m.lobbyState.codeInput[:len(m.lobbyState.codeInput)-1]
			} else if len(key.Runes) == 1 && len(m.lobbyState.codeInput) < 4 {
				ch := strings.ToUpper(string(key.Runes))
				m.lobbyState.codeInput += ch
			}
			return m, nil
		}
		// Normal lobby key handling
		if isKey(key, "c") {
			if m.lobby != nil {
				code := m.lobby.CreateRoom(m.playerName + "'s Room")
				m.lobby.JoinRoom(code, m.playerName)
				m.roomCode = code
				m.screen = screenClassSelect
			}
		} else if isKey(key, "j") {
			m.lobbyState.joining = true
			m.lobbyState.codeInput = ""
		} else if isUp(key) {
			if m.lobbyState.cursor > 0 {
				m.lobbyState.cursor--
			}
		} else if isDown(key) {
			if m.lobbyState.cursor < len(m.lobbyState.rooms)-1 {
				m.lobbyState.cursor++
			}
		} else if isEnter(key) {
			if m.lobby != nil && len(m.lobbyState.rooms) > 0 {
				r := m.lobbyState.rooms[m.lobbyState.cursor]
				if err := m.lobby.JoinRoom(r.Code, m.playerName); err == nil {
					m.roomCode = r.Code
					m.screen = screenClassSelect
				}
			}
		} else if isKey(key, "q") {
			m.screen = screenTitle
		}
	}
	return m, nil
}

func (m Model) updateClassSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isUp(key) {
			if m.classState.cursor > 0 {
				m.classState.cursor--
			}
		} else if isDown(key) {
			if m.classState.cursor < len(classOptions)-1 {
				m.classState.cursor++
			}
		} else if isEnter(key) {
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

// pollState returns a Cmd that waits briefly then refreshes game state
func (m Model) pollState() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

type tickMsg struct{}

func (m Model) updateGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Refresh state on every update
	if m.session != nil {
		m.gameState = m.session.GetState()
	}

	if m.gameState.GameOver {
		if m.store != nil && !m.rankingSaved {
			score := 0
			for _, p := range m.gameState.Players {
				score += p.Gold
			}
			m.store.SaveRun(m.playerName, m.gameState.FloorNum, score)
			m.rankingSaved = true
		}
		m.screen = screenResult
		return m, nil
	}
	if m.gameState.Phase == game.PhaseShop {
		m.screen = screenShop
		return m, nil
	}

	switch msg.(type) {
	case tickMsg:
		// State already refreshed above, just keep polling during combat
		if m.gameState.Phase == game.PhaseCombat {
			return m, m.pollState()
		}
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch m.gameState.Phase {
		case game.PhaseExploring:
			neighbors := m.getNeighbors()
			if isUp(key) {
				if m.moveCursor > 0 {
					m.moveCursor--
				}
			} else if isDown(key) {
				if m.moveCursor < len(neighbors)-1 {
					m.moveCursor++
				}
			} else if isEnter(key) {
				if m.session != nil && len(neighbors) > 0 {
					roomIdx := neighbors[m.moveCursor]
					m.session.EnterRoom(roomIdx)
					m.gameState = m.session.GetState()
					m.moveCursor = 0
					if m.gameState.Phase == game.PhaseCombat {
						return m, m.pollState()
					}
				}
			} else if isQuit(key) {
				return m, tea.Quit
			}
		case game.PhaseCombat:
			isPlayerDead := false
			for _, p := range m.gameState.Players {
				if p.Name == m.playerName && p.IsDead() {
					isPlayerDead = true
					break
				}
			}
			if isPlayerDead {
				return m, m.pollState()
			}
			if isKey(key, "tab") || key.Type == tea.KeyTab {
				if len(m.gameState.Monsters) > 0 {
					m.targetCursor = (m.targetCursor + 1) % len(m.gameState.Monsters)
				}
				return m, m.pollState()
			}
			if m.session != nil {
				switch key.String() {
				case "1":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionAttack, TargetIdx: m.targetCursor})
				case "2":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionSkill, TargetIdx: m.targetCursor})
				case "3":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionItem})
				case "4":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionFlee})
				case "5":
					m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionWait})
				}
				// After submitting, poll for turn resolution
				return m, m.pollState()
			}
		}
	}
	return m, nil
}

func (m Model) getNeighbors() []int {
	if m.gameState.Floor == nil {
		return nil
	}
	cur := m.gameState.Floor.CurrentRoom
	if cur < 0 || cur >= len(m.gameState.Floor.Rooms) {
		return nil
	}
	return m.gameState.Floor.Rooms[cur].Neighbors
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
		if isEnter(key) {
			m.screen = screenLobby
			m.rankingSaved = false
			m = m.withRefreshedLobby()
		} else if isQuit(key) {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) withRefreshedLobby() Model {
	if m.lobby == nil {
		return m
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
	return m
}
