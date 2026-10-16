package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	gossh "golang.org/x/crypto/ssh"
)

func startTestServer(t *testing.T, lobby *game.Lobby) string {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	srv, err := NewServer("127.0.0.1:0", filepath.Join(t.TempDir(), "host_key"), lobby, db)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

func newClientKey(t *testing.T) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return signer
}

// openTerminal connects with a public key, opens a pty shell and waits for the
// first frame, so the session is fully running before the test disconnects.
func openTerminal(t *testing.T, addr string, signer gossh.Signer) *gossh.Client {
	t.Helper()
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "player",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if err := session.RequestPty("xterm-256color", 24, 80, gossh.TerminalModes{}); err != nil {
		t.Fatalf("pty: %v", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	if err := session.Shell(); err != nil {
		t.Fatalf("shell: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := stdout.Read(buf); err != nil {
		t.Fatalf("no first frame: %v", err)
	}
	return client
}

func TestClosingTheConnectionReleasesThePlayer(t *testing.T) {
	cfg, _ := config.Load("")
	lobby := game.NewLobby(cfg)
	addr := startTestServer(t, lobby)
	signer := newClientKey(t)
	fp := gossh.FingerprintSHA256(signer.PublicKey())

	client := openTerminal(t, addr, signer)
	lobby.PlayerOnline(fp, "player")
	_ = client.Close()

	deadline := time.Now().Add(5 * time.Second)
	for len(lobby.ListOnline()) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("player still online after the SSH connection closed: %+v", lobby.ListOnline())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
