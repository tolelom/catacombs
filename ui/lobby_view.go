package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/game"
)

type roomInfo struct {
	Code    string
	Name    string
	Players []playerInfo
	Status  string
}

type playerInfo struct {
	Name  string
	Class string
	Ready bool
}

// LobbyScreen shows available rooms and lets players create/join.
type LobbyScreen struct {
	rooms     []roomInfo
	input     string
	cursor    int
	creating  bool
	roomName  string
	joining   bool
	codeInput string
	online    int
}

func NewLobbyScreen() *LobbyScreen {
	return &LobbyScreen{}
}

func (s *LobbyScreen) refreshLobby(ctx *Context) {
	if ctx.Lobby == nil {
		return
	}
	rooms := ctx.Lobby.ListRooms()
	s.rooms = make([]roomInfo, len(rooms))
	for i, r := range rooms {
		status := "Waiting"
		if r.Status == game.RoomPlaying {
			status = "Playing"
		}
		players := make([]playerInfo, len(r.Players))
		for j, p := range r.Players {
			players[j] = playerInfo{Name: p.Name, Class: p.Class, Ready: p.Ready}
		}
		s.rooms[i] = roomInfo{
			Code:    r.Code,
			Name:    r.Name,
			Players: players,
			Status:  status,
		}
	}
	s.online = len(ctx.Lobby.ListOnline())
	s.cursor = 0
}

func (s *LobbyScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		// Join-by-code input mode
		if s.joining {
			if isEnter(key) && len(s.codeInput) == 4 {
				if ctx.Lobby != nil {
					if err := ctx.Lobby.JoinRoom(s.codeInput, ctx.PlayerName, ctx.Fingerprint); err == nil {
						ctx.RoomCode = s.codeInput
						return NewClassSelectScreen(), nil
					}
				}
				s.joining = false
				s.codeInput = ""
			} else if isKey(key, "esc") || key.Type == tea.KeyEsc {
				s.joining = false
				s.codeInput = ""
			} else if key.Type == tea.KeyBackspace && len(s.codeInput) > 0 {
				s.codeInput = s.codeInput[:len(s.codeInput)-1]
			} else if len(key.Runes) == 1 && len(s.codeInput) < 4 {
				ch := strings.ToUpper(string(key.Runes))
				s.codeInput += ch
			}
			return s, nil
		}
		// Normal lobby key handling
		if isKey(key, "c") {
			if ctx.Lobby != nil {
				code := ctx.Lobby.CreateRoom(ctx.PlayerName + "'s Room")
				ctx.Lobby.JoinRoom(code, ctx.PlayerName, ctx.Fingerprint)
				ctx.RoomCode = code
				return NewClassSelectScreen(), nil
			}
		} else if isKey(key, "j") {
			s.joining = true
			s.codeInput = ""
		} else if isUp(key) {
			if s.cursor > 0 {
				s.cursor--
			}
		} else if isDown(key) {
			if s.cursor < len(s.rooms)-1 {
				s.cursor++
			}
		} else if isEnter(key) {
			if ctx.Lobby != nil && len(s.rooms) > 0 {
				r := s.rooms[s.cursor]
				if err := ctx.Lobby.JoinRoom(r.Code, ctx.PlayerName, ctx.Fingerprint); err == nil {
					ctx.RoomCode = r.Code
					return NewClassSelectScreen(), nil
				}
			}
		} else if isKey(key, "q") {
			if ctx.Lobby != nil {
				ctx.Lobby.PlayerOffline(ctx.Fingerprint)
			}
			return NewTitleScreen(), nil
		}
	}
	return s, nil
}

func (s *LobbyScreen) View(ctx *Context) string {
	state := lobbyState{
		rooms:     s.rooms,
		input:     s.input,
		cursor:    s.cursor,
		creating:  s.creating,
		roomName:  s.roomName,
		joining:   s.joining,
		codeInput: s.codeInput,
		online:    s.online,
	}
	return renderLobby(state, ctx.Width, ctx.Height)
}

type lobbyState struct {
	rooms     []roomInfo
	input     string
	cursor    int
	creating  bool
	roomName  string
	joining   bool
	codeInput string
	online    int
}

func renderLobby(state lobbyState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	roomStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	header := headerStyle.Render(fmt.Sprintf("── Lobby ── %d online ──", state.online))
	menu := "[C] Create Room  [J] Join by Code  [Up/Down] Select  [Enter] Join  [Q] Back"

	roomList := ""
	for i, r := range state.rooms {
		marker := "  "
		if i == state.cursor {
			marker = "> "
		}
		roomList += fmt.Sprintf("%s%s [%s] (%d/4) %s\n",
			marker, r.Name, r.Code, len(r.Players), r.Status)
		// Show players in selected room
		if i == state.cursor {
			for _, p := range r.Players {
				cls := p.Class
				if cls == "" {
					cls = "..."
				}
				readyMark := "  "
				if p.Ready {
					readyMark = "✓ "
				}
				roomList += fmt.Sprintf("    %s%s (%s)\n", readyMark, p.Name, cls)
			}
		}
	}
	if roomList == "" {
		roomList = "  No rooms available. Create one!"
	}
	if state.joining {
		inputStr := state.codeInput + strings.Repeat("_", 4-len(state.codeInput))
		roomList += fmt.Sprintf("\n  Enter room code: [%s]  (Esc to cancel)\n", inputStr)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		roomStyle.Render(roomList),
		"",
		menu,
	)
}
