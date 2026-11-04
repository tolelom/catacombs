package ui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

type nicknamePhase int

const (
	phaseNickname nicknamePhase = iota
	phasePasswordLogin
	phasePasswordCreate
	phasePasswordConfirm
)

type NicknameScreen struct {
	input    string
	password string
	confirm  string
	phase    nicknamePhase
	error    string
}

func NewNicknameScreen() *NicknameScreen {
	return &NicknameScreen{}
}

func isWebUser(ctx *Context) bool {
	return ctx.Fingerprint == "" ||
		strings.HasPrefix(ctx.Fingerprint, "anon:")
}

func (s *NicknameScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}

	if isKey(key, "esc") || key.Type == tea.KeyEsc {
		switch s.phase {
		case phaseNickname:
			s.input = ""
			return NewTitleScreen(), nil
		case phasePasswordLogin, phasePasswordCreate:
			s.phase = phaseNickname
			s.password = ""
			s.error = ""
			return s, nil
		case phasePasswordConfirm:
			s.phase = phasePasswordCreate
			s.confirm = ""
			s.error = ""
			return s, nil
		}
		return s, nil
	}

	switch s.phase {
	case phaseNickname:
		return s.updateNickname(key, ctx)
	case phasePasswordLogin:
		return s.updatePasswordLogin(key, ctx)
	case phasePasswordCreate:
		return s.updatePasswordCreate(key, ctx)
	case phasePasswordConfirm:
		return s.updatePasswordConfirm(key, ctx)
	}
	return s, nil
}

func (s *NicknameScreen) updateNickname(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
	if isEnter(key) && len(s.input) > 0 {

		if !isWebUser(ctx) {
			return s.finishLogin(ctx)
		}

		if ctx.Store != nil && ctx.Store.HasPassword(s.input) {
			s.phase = phasePasswordLogin
			s.error = ""
		} else {
			s.phase = phasePasswordCreate
			s.error = ""
		}
		return s, nil
	}
	if key.Type == tea.KeyBackspace && len(s.input) > 0 {
		_, size := utf8.DecodeLastRuneInString(s.input)
		s.input = s.input[:len(s.input)-size]
	} else if len(key.Runes) == 1 && utf8.RuneCountInString(s.input) < 12 {
		r := key.Runes[0]
		if unicode.IsPrint(r) {
			s.input += string(r)
		}
	}
	return s, nil
}

func (s *NicknameScreen) updatePasswordLogin(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
	if isEnter(key) {
		if ctx.Store == nil {
			return s.finishLogin(ctx)
		}
		if ctx.LoginLimiter != nil {
			if blocked, _ := ctx.LoginLimiter.Blocked(s.input); blocked {
				s.error = "로그인 시도가 너무 많습니다. 잠시 후 다시 시도하세요"
				s.password = ""
				return s, nil
			}
		}
		ok, err := ctx.Store.CheckPassword(s.input, s.password)
		if err != nil {
			s.error = "오류가 발생했습니다"
			slog.Error("password check failed", "error", err)
			return s, nil
		}
		if !ok {
			if ctx.LoginLimiter != nil {
				ctx.LoginLimiter.Fail(s.input)
			}
			s.error = "비밀번호가 틀렸습니다"
			s.password = ""
			return s, nil
		}
		if ctx.LoginLimiter != nil {
			ctx.LoginLimiter.Reset(s.input)
		}

		ctx.Fingerprint = webFingerprint(s.input)
		return s.finishLogin(ctx)
	}
	s.password = handlePasswordInput(key, s.password)
	return s, nil
}

const minPasswordLen = 6

func (s *NicknameScreen) updatePasswordCreate(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
	if isEnter(key) {
		if len(s.password) < minPasswordLen {
			s.error = fmt.Sprintf("비밀번호는 %d자 이상이어야 합니다", minPasswordLen)
			return s, nil
		}
		s.phase = phasePasswordConfirm
		s.error = ""
		return s, nil
	}
	s.password = handlePasswordInput(key, s.password)
	return s, nil
}

func (s *NicknameScreen) updatePasswordConfirm(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
	if isEnter(key) {
		if s.confirm != s.password {
			s.error = "비밀번호가 일치하지 않습니다"
			s.confirm = ""
			return s, nil
		}
		if ctx.Store != nil {
			err := ctx.Store.CreatePassword(s.input, s.password)
			if errors.Is(err, store.ErrPasswordExists) {
				s.phase = phaseNickname
				s.password, s.confirm = "", ""
				s.error = "이미 사용 중인 닉네임입니다"
				return s, nil
			}
			if err != nil {
				s.error = "저장 오류가 발생했습니다"
				slog.Error("failed to save password", "error", err)
				return s, nil
			}
		}
		ctx.Fingerprint = webFingerprint(s.input)
		return s.finishLogin(ctx)
	}
	s.confirm = handlePasswordInput(key, s.confirm)
	return s, nil
}

func (s *NicknameScreen) finishLogin(ctx *Context) (Screen, tea.Cmd) {
	ctx.PlayerName = s.input
	if ctx.Store != nil && ctx.Fingerprint != "" {
		if err := ctx.Store.SaveProfile(ctx.Fingerprint, ctx.PlayerName); err != nil {
			slog.Error("failed to save profile", "error", err)
		}
	}
	if ctx.Lobby != nil {
		ctx.Lobby.PlayerOnline(ctx.Fingerprint, ctx.PlayerName)
	}

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
}

func webFingerprint(nickname string) string {
	h := sha256.Sum256([]byte("web:" + nickname))
	return fmt.Sprintf("SHA256:%x", h)
}

func handlePasswordInput(key tea.KeyMsg, current string) string {
	if key.Type == tea.KeyBackspace && len(current) > 0 {
		return current[:len(current)-1]
	}
	if len(key.Runes) == 1 && len(current) < 32 {
		ch := string(key.Runes)
		if len(ch) == 1 && ch[0] >= 32 && ch[0] < 127 {
			return current + ch
		}
	}
	return current
}

func (s *NicknameScreen) View(ctx *Context) string {
	return renderNicknameLogin(s, ctx.Width, ctx.Height)
}

func renderNicknameLogin(s *NicknameScreen, width, height int) string {
	var sections []string

	switch s.phase {
	case phaseNickname:
		sections = renderNicknamePhase(s.input)
	case phasePasswordLogin:
		sections = renderPasswordPhase(s.input, s.password, "비밀번호를 입력하세요", s.error)
	case phasePasswordCreate:
		sections = renderPasswordPhase(s.input, s.password, "비밀번호를 설정하세요 (4자 이상)", s.error)
	case phasePasswordConfirm:
		sections = renderPasswordPhase(s.input, s.confirm, "비밀번호를 다시 입력하세요", s.error)
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, sections...))
}

func renderNicknamePhase(input string) []string {
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

	hint := styleSystem.Render(fmt.Sprintf("(%d/12 글자)", utf8.RuneCountInString(input)))
	footer := styleAction.Render("[Enter] 확인    [Esc] 취소")

	return []string{title, "", inputBox, hint, "", footer}
}

func renderPasswordPhase(nickname, password, prompt, errMsg string) []string {
	title := styleHeader.Render("── " + prompt + " ──")

	nameDisplay := stylePlayer.Render("이름: " + nickname)

	masked := strings.Repeat("*", len(password))
	if masked == "" {
		masked = strings.Repeat("_", 8)
	} else {
		masked += "_"
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(0, 2).
		Render(stylePlayer.Render(masked))

	sections := []string{title, "", nameDisplay, "", inputBox}

	if errMsg != "" {
		sections = append(sections, "",
			lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render(errMsg))
	}

	footer := styleAction.Render("[Enter] 확인    [Esc] 뒤로")
	sections = append(sections, "", footer)

	return sections
}
