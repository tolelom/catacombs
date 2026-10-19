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

	dailyDate string
	dailyMod  game.DailyModifier
}

func NewLobbyScreen() *LobbyScreen {
	return &LobbyScreen{}
}

func (s *LobbyScreen) pollLobby(ctx *Context) tea.Cmd {
	return scheduleTick(ctx, time.Second*2)
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
		return s, s.pollLobby(ctx)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.joining {
		return s.handleCodeEntryKey(key, ctx)
	}
	return s.handleLobbyKey(key, ctx)
}

func (s *LobbyScreen) handleCodeEntryKey(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
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

func (s *LobbyScreen) handleLobbyKey(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
	if isKey(key, "c") {
		return s.createAndJoinRoom(ctx)
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
		return s.joinSelectedRoom(ctx)
	} else if isKey(key, "d") {
		return s.startDailyChallenge(ctx)
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
	return s, nil
}

func (s *LobbyScreen) createAndJoinRoom(ctx *Context) (Screen, tea.Cmd) {
	if ctx.Lobby == nil {
		return s, nil
	}
	code, err := ctx.Lobby.CreateRoom(ctx.Fingerprint, ctx.PlayerName+"의 방")
	if err != nil {
		s.errMsg = err.Error()
		return s, nil
	}
	if err := ctx.Lobby.JoinRoom(code, ctx.PlayerName, ctx.Fingerprint); err != nil {
		s.errMsg = "방 참가 실패: " + err.Error()
		return s, nil
	}
	ctx.RoomCode = code
	return NewClassSelectScreen(), nil
}

func (s *LobbyScreen) joinSelectedRoom(ctx *Context) (Screen, tea.Cmd) {
	if ctx.Lobby == nil || len(s.rooms) == 0 {
		return s, nil
	}
	r := s.rooms[s.cursor]
	if err := ctx.Lobby.JoinRoom(r.Code, ctx.PlayerName, ctx.Fingerprint); err != nil {
		s.errMsg = err.Error()
		return s, nil
	}
	ctx.RoomCode = r.Code
	s.errMsg = ""
	return NewClassSelectScreen(), nil
}

func (s *LobbyScreen) startDailyChallenge(ctx *Context) (Screen, tea.Cmd) {
	if ctx.Lobby == nil {
		return s, nil
	}
	if s.dailyDate == "" {
		s.dailyDate = time.Now().Format("2006-01-02")
		s.dailyMod = game.PickDailyModifier(s.dailyDate)
	}
	dailyDate, dailyMod := s.dailyDate, s.dailyMod
	code, err := ctx.Lobby.CreatePrivateRoom(ctx.Fingerprint, ctx.PlayerName+"의 일일 도전")
	if err != nil {
		s.errMsg = err.Error()
		return s, nil
	}
	if err := ctx.Lobby.JoinRoom(code, ctx.PlayerName, ctx.Fingerprint); err != nil {
		return s, nil
	}
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

	parts := []string{
		header,
		"",
		renderDailyHint(state.dailyMod),
		"",
		roomStyle.Render(renderRoomList(state)),
		"",
		lobbyMenu(state),
	}
	if state.errMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		parts = append(parts, "", errStyle.Render("! "+state.errMsg))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func renderDailyHint(dailyMod game.DailyModifier) string {
	if dailyMod.Name == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(fmt.Sprintf("오늘의 일일 변이: %s — %s", dailyMod.Name, dailyMod.Desc))
}

func lobbyMenu(state lobbyState) string {
	menu := "[C] 방 만들기  [J] 코드로 참가  [D] 일일 도전  [S] 영혼의 제단  [Up/Down] 선택  [Enter] 참가  [Q] 뒤로"
	if state.hardUnlocked {
		hardStatus := "OFF"
		if state.hardMode {
			hardStatus = "ON"
		}
		menu += fmt.Sprintf("  [H] 하드 모드: %s", hardStatus)
	}
	return menu
}

func renderRoomList(state lobbyState) string {
	roomList := ""
	for i, r := range state.rooms {
		marker := "  "
		if i == state.cursor {
			marker = "> "
		}
		roomList += fmt.Sprintf("%s%s [%s] (%d/4) %s\n",
			marker, r.Name, r.Code, len(r.Players), r.Status)

		if i == state.cursor {
			roomList += renderRoomMembers(r.Players)
		}
	}
	if roomList == "" {
		roomList = "  방이 없습니다. 새로 만드세요!"
	}
	if state.joining {
		inputStr := state.codeInput + strings.Repeat("_", 4-len(state.codeInput))
		roomList += fmt.Sprintf("\n  방 코드 입력: [%s]  (Esc로 취소)\n", inputStr)
	}
	return roomList
}

func renderRoomMembers(players []playerInfo) string {
	var members string
	for _, p := range players {
		cls := p.Class
		if cls == "" {
			cls = "..."
		}
		readyMark := "  "
		if p.Ready {
			readyMark = "✓ "
		}
		members += fmt.Sprintf("    %s%s (%s)\n", readyMark, p.Name, cls)
	}
	return members
}
