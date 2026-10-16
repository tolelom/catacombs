package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/ui"
	gossh "golang.org/x/crypto/ssh"
)

func NewServer(addr, hostKeyPath string, lobby *game.Lobby, db *store.DB) (*ssh.Server, error) {
	s, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return true
		}),
		wish.WithPasswordAuth(func(_ ssh.Context, _ string) bool {
			return true
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				return newSessionModel(s, lobby, db)
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create server: %w", err)
	}
	return s, nil
}

func newSessionModel(s ssh.Session, lobby *game.Lobby, db *store.DB) (tea.Model, []tea.ProgramOption) {
	fingerprint, err := sessionFingerprint(s)
	if err != nil {
		slog.Error("anon fingerprint RNG failed; refusing session", "error", err)
		_ = s.Exit(1)
		return rejectModel{}, nil
	}
	pty, _, _ := s.Pty()
	slog.Info("new SSH session", "fingerprint", fingerprint, "width", pty.Window.Width, "height", pty.Window.Height)
	return ui.NewModel(pty.Window.Width, pty.Window.Height, fingerprint, lobby, db), []tea.ProgramOption{tea.WithAltScreen()}
}

func sessionFingerprint(s ssh.Session) (string, error) {
	if s.PublicKey() != nil {
		return gossh.FingerprintSHA256(s.PublicKey()), nil
	}
	tok, err := randomToken(8)
	if err != nil {
		return "", err
	}
	return "anon:" + tok, nil
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

type rejectModel struct{}

func (rejectModel) Init() tea.Cmd                       { return tea.Quit }
func (rejectModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return rejectModel{}, tea.Quit }
func (rejectModel) View() string                        { return "" }
