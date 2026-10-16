package ui

import (
	"fmt"
	"strings"
	"time"

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
	rooms        []roomInfo
	input        string
	cursor       int
	creating     bool
	roomName     string
	joining      bool
	codeInput    string
	online       int
	hardMode     bool
	hardUnlocked bool
	errMsg       string
	// dailyDate is captured on lobby refresh so the header banner and the
	// daily-challenge session created via [D] use the same date — avoids a
	// midnight crossover where header shows yesterday's modifier but the run
	// is created with today's.
	dailyDate string
	dailyMod  game.DailyModifier
}

func NewLobbyScreen() *LobbyScreen {
	return &LobbyScreen{}
}

func (s *LobbyScreen) pollLobby() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (s *LobbyScreen) refreshLobby(ctx *Context) {
	if ctx.Lobby == nil {
		return
	}
	rooms := ctx.Lobby.ListRoomSnapshots()
	s.rooms = make([]roomInfo, len(rooms))
	for i, r := range rooms {
		status := "대기중"
		if r.Status == game.RoomPlaying {
			status = "진행중"
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
	if ctx.Store != nil {
		s.hardUnlocked = ctx.Store.IsUnlocked(ctx.Fingerprint, "hard_mode")
	}
	s.dailyDate = time.Now().Format("2006-01-02")
	s.dailyMod = game.PickDailyModifier(s.dailyDate)
}

func (s *LobbyScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		s.refreshLobby(ctx)
		return s, s.pollLobby()
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		// Join-by-code input mode
		if s.joining {
			if isEnter(key) && len(s.codeInput) == 4 {
				if ctx.Lobby != nil {
					if err := ctx.Lobby.JoinRoom(s.codeInput, ctx.PlayerName, ctx.Fingerprint); err == nil {
						ctx.RoomCode = s.codeInput
						s.errMsg = ""
						return NewClassSelectScreen(), nil
					} else {
						s.errMsg = err.Error()
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
				code, err := ctx.Lobby.CreateRoom(ctx.Fingerprint, ctx.PlayerName+"의 방")
				if err != nil {
					s.errMsg = err.Error()
					return s, nil
				}
				if err := ctx.Lobby.JoinRoom(code, ctx.PlayerName, ctx.Fingerprint); err != nil {
					// Joining a room we just created shouldn't fail, but if it
					// does the player must see why instead of a ghost lobby.
					s.errMsg = "방 참가 실패: " + err.Error()
					return s, nil
				}
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
					s.errMsg = ""
					return NewClassSelectScreen(), nil
				} else {
					s.errMsg = err.Error()
				}
			}
		} else if isKey(key, "d") {
			// Daily Challenge: create a private solo daily session
			if ctx.Lobby != nil {
				if s.dailyDate == "" {
					s.dailyDate = time.Now().Format("2006-01-02")
					s.dailyMod = game.PickDailyModifier(s.dailyDate)
				}
				dailyDate, dailyMod := s.dailyDate, s.dailyMod
				code, err := ctx.Lobby.CreateRoom(ctx.Fingerprint, ctx.PlayerName+"의 일일 도전")
				if err != nil {
					s.errMsg = err.Error()
					return s, nil
				}
				if err := ctx.Lobby.JoinRoom(code, ctx.PlayerName, ctx.Fingerprint); err == nil {
					ctx.RoomCode = code
					session := ctx.Lobby.EnsureSession(code, func() *game.GameSession {
						gs := game.NewGameSession(ctx.Lobby.Cfg())
						gs.DailyMode = true
						gs.DailyDate = dailyDate
						gs.DailyMod = &dailyMod
						gs.ApplyWeeklyMutation()
						return gs
					})
					if session != nil {
						ctx.Session = session
					}
					return NewClassSelectScreen(), nil
				}
			}
		} else if isKey(key, "h") && s.hardUnlocked {
			s.hardMode = !s.hardMode
			ctx.HardMode = s.hardMode
		} else if isKey(key, "s") {
			return NewSoulsScreen(), nil
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
		rooms:        s.rooms,
		input:        s.input,
		cursor:       s.cursor,
		creating:     s.creating,
		roomName:     s.roomName,
		joining:      s.joining,
		codeInput:    s.codeInput,
		online:       s.online,
		hardMode:     s.hardMode,
		hardUnlocked: s.hardUnlocked,
		errMsg:       s.errMsg,
		dailyMod:     s.dailyMod,
	}
	return renderLobby(state, ctx.Width, ctx.Height)
}

type lobbyState struct {
	rooms        []roomInfo
	input        string
	cursor       int
	creating     bool
	roomName     string
	joining      bool
	codeInput    string
	online       int
	hardMode     bool
	hardUnlocked bool
	errMsg       string
	dailyMod     game.DailyModifier
}

func renderLobby(state lobbyState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	roomStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	header := headerStyle.Render(fmt.Sprintf("── 로비 ── %d명 접속중 ──", state.online))
	dailyHint := ""
	if state.dailyMod.Name != "" {
		dailyHint = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(fmt.Sprintf("오늘의 일일 변이: %s — %s", state.dailyMod.Name, state.dailyMod.Desc))
	}
	menu := "[C] 방 만들기  [J] 코드로 참가  [D] 일일 도전  [S] 영혼의 제단  [Up/Down] 선택  [Enter] 참가  [Q] 뒤로"
	if state.hardUnlocked {
		hardStatus := "OFF"
		if state.hardMode {
			hardStatus = "ON"
		}
		menu += fmt.Sprintf("  [H] 하드 모드: %s", hardStatus)
	}

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
		roomList = "  방이 없습니다. 새로 만드세요!"
	}
	if state.joining {
		inputStr := state.codeInput + strings.Repeat("_", 4-len(state.codeInput))
		roomList += fmt.Sprintf("\n  방 코드 입력: [%s]  (Esc로 취소)\n", inputStr)
	}

	parts := []string{
		header,
		"",
		dailyHint,
		"",
		roomStyle.Render(roomList),
		"",
		menu,
	}
	if state.errMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		parts = append(parts, "", errStyle.Render("! "+state.errMsg))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
