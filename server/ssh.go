package server

import (
	"fmt"
	"log"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	tea "github.com/charmbracelet/bubbletea"
	gossh "golang.org/x/crypto/ssh"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/ui"
)

func Start(host string, port int, lobby *game.Lobby, db *store.DB) error {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
		wish.WithHostKeyPath(".ssh/catacombs_host_key"),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return true // accept all keys
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				pty, _, _ := s.Pty()
				fingerprint := ""
				if s.PublicKey() != nil {
					fingerprint = gossh.FingerprintSHA256(s.PublicKey())
				}
				m := ui.NewModel(pty.Window.Width, pty.Window.Height, fingerprint, lobby, db)
				return m, []tea.ProgramOption{tea.WithAltScreen()}
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("could not create server: %w", err)
	}

	log.Printf("Starting SSH server on %s:%d", host, port)
	return s.ListenAndServe()
}
