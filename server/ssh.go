package server

import (
	"crypto/rand"
	"encoding/hex"
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
//
// Auth policy: by design the server accepts any public key and any password.
// Catacombs uses SSH purely as a transport for an interactive TUI; the
// server treats the public-key fingerprint as a stable, self-asserted
// player identity. Web bridge clients have no key and instead authenticate
// via nickname+password through the in-game NicknameScreen flow. Treat
// the public TCP port as untrusted — anyone can open a session and reach
// the lobby.
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
				} else {
					// Web bridge connections don't carry a public key. Mint a
					// per-connection identifier so lobby/session bookkeeping
					// (dedup, activeSessions, RegisterSession) doesn't collide
					// across simultaneous web players.
					fingerprint = "anon:" + randomToken(8)
				}

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

func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// Fall back to a non-empty constant; collisions remain possible but
		// only when the OS RNG fails, which is itself a critical condition.
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
