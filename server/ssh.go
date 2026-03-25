package server

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	tea "github.com/charmbracelet/bubbletea"
	gossh "golang.org/x/crypto/ssh"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/ui"
)

// NewServer creates the SSH server but does not start it.
// The caller is responsible for calling ListenAndServe() and Shutdown().
func NewServer(addr string, lobby *game.Lobby, db *store.DB) (*ssh.Server, error) {
	s, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(".ssh/catacombs_host_key"),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return true
		}),
		wish.WithPasswordAuth(func(_ ssh.Context, _ string) bool {
			return true
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				pty, _, _ := s.Pty()
				fingerprint := ""
				if s.PublicKey() != nil {
					fingerprint = gossh.FingerprintSHA256(s.PublicKey())
				}

				defer func() {
					if r := recover(); r != nil {
						slog.Error("session panic recovered", "error", r, "fingerprint", fingerprint)
					}
				}()

				slog.Info("new SSH session", "fingerprint", fingerprint, "width", pty.Window.Width, "height", pty.Window.Height)
				m := ui.NewModel(pty.Window.Width, pty.Window.Height, fingerprint, lobby, db)
				return m, []tea.ProgramOption{tea.WithAltScreen()}
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create server: %w", err)
	}
	return s, nil
}

// Start creates and starts the SSH server (blocking).
func Start(addr string, lobby *game.Lobby, db *store.DB) error {
	s, err := NewServer(addr, lobby, db)
	if err != nil {
		return err
	}
	slog.Info("starting SSH server", "addr", addr)
	return s.ListenAndServe()
}
