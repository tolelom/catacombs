package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
)

// startGameServer runs the real SSH game server on a free port and returns the port.
func startGameServer(t *testing.T, lobby *game.Lobby, db *store.DB) int {
	t.Helper()
	srv, err := server.NewServer("127.0.0.1:0", filepath.Join(t.TempDir(), "host_key"), lobby, db)
	if err != nil {
		t.Fatalf("new ssh server: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().(*net.TCPAddr).Port
}

// newTestSite serves the web handler, wired to a live game server, over httptest.
func newTestSite(t *testing.T, mutate func(*Options)) *httptest.Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg, _ := config.Load("")
	lobby := game.NewLobby(cfg)
	opts := Options{
		SSHPort:   startGameServer(t, lobby, db),
		Lobby:     lobby,
		DB:        db,
		StartTime: time.Now(),
	}
	if mutate != nil {
		mutate(&opts)
	}
	site := httptest.NewServer(NewHandler(opts))
	t.Cleanup(site.Close)
	return site
}

func dialWS(t *testing.T, site *httptest.Server) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(site.URL, "http") + "/ws"
	return websocket.DefaultDialer.Dial(url, nil)
}

// waitClosed keeps reading until the server drops the connection; true if it did within the window.
func waitClosed(ws *websocket.Conn, within time.Duration) bool {
	_ = ws.SetReadDeadline(time.Now().Add(within))
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			var netErr net.Error
			if ne, ok := err.(net.Error); ok {
				netErr = ne
			}
			return netErr == nil || !netErr.Timeout()
		}
	}
}

func TestOversizedFrameClosesConnectionAndServerStaysUp(t *testing.T) {
	site := newTestSite(t, nil)
	ws, _, err := dialWS(t, site)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = ws.Close() }()

	huge := strings.Repeat("a", 1<<20)
	_ = ws.WriteMessage(websocket.TextMessage, []byte(huge))

	if !waitClosed(ws, 5*time.Second) {
		t.Fatal("server kept the connection open after a 1MB frame")
	}

	again, _, err := dialWS(t, site)
	if err != nil {
		t.Fatalf("new connection failed after the oversized frame: %v", err)
	}
	_ = again.Close()
}

func TestConnectionsBeyondLimitAreRejectedBeforeUpgrade(t *testing.T) {
	site := newTestSite(t, func(o *Options) { o.MaxConns = 1 })
	first, _, err := dialWS(t, site)
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	defer func() { _ = first.Close() }()

	_, resp, err := dialWS(t, site)
	if err == nil {
		t.Fatal("connection beyond the limit was accepted")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for connection beyond the limit, got %v", resp)
	}
}

func TestSlotIsReleasedWhenConnectionEnds(t *testing.T) {
	site := newTestSite(t, func(o *Options) { o.MaxConns = 1 })
	first, _, err := dialWS(t, site)
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	_ = first.Close()

	deadline := time.Now().Add(3 * time.Second)
	for {
		again, _, err := dialWS(t, site)
		if err == nil {
			_ = again.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("slot was not released after the connection ended: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestIdleClientWithoutInputIsDisconnected(t *testing.T) {
	site := newTestSite(t, func(o *Options) { o.IdleTimeout = 300 * time.Millisecond })
	ws, _, err := dialWS(t, site)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = ws.Close() }()

	if !waitClosed(ws, 5*time.Second) {
		t.Fatal("connection without input stayed open past the idle timeout")
	}
}

func TestAdminIsNotServedWithoutCredentials(t *testing.T) {
	site := newTestSite(t, nil)
	resp, err := http.Get(site.URL + "/admin")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for /admin without credentials, got %d", resp.StatusCode)
	}
}

func TestAdminRequiresAuthWhenCredentialsAreSet(t *testing.T) {
	site := newTestSite(t, func(o *Options) {
		o.Admin = config.AdminConfig{Username: "ops", Password: "s3cret-pass"}
	})
	resp, err := http.Get(site.URL + "/admin")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated /admin, got %d", resp.StatusCode)
	}
}
