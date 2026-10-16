package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"golang.org/x/crypto/ssh"
)

//go:embed static
var staticFiles embed.FS

const (
	wsWriteWait        = 10 * time.Second
	wsPongWait         = 60 * time.Second
	wsPingPeriod       = (wsPongWait * 9) / 10
	wsMaxMessageBytes  = 64 << 10
	defaultMaxConns    = 50
	defaultIdleTimeout = 30 * time.Minute
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	},
}

type Options struct {
	SSHPort     int
	Lobby       *game.Lobby
	DB          *store.DB
	StartTime   time.Time
	Admin       config.AdminConfig
	MaxConns    int
	IdleTimeout time.Duration
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

func Start(addr string, opts Options) *http.Server {
	srv := &http.Server{Addr: addr, Handler: NewHandler(opts)}
	slog.Info("starting web terminal", "addr", addr, "admin_enabled", opts.Admin.Enabled())
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("web server error", "error", err)
		}
	}()
	return srv
}

func NewHandler(opts Options) http.Handler {
	opts = withDefaults(opts)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(staticFiles)))
	mux.Handle("/ws", newBridge(opts))
	if opts.Admin.Enabled() {
		mux.Handle("/admin", AdminHandler(opts.Admin, opts.Lobby, opts.DB, opts.StartTime))
	}
	return mux
}

func withDefaults(opts Options) Options {
	if opts.MaxConns <= 0 {
		opts.MaxConns = defaultMaxConns
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = defaultIdleTimeout
	}
	return opts
}

type bridge struct {
	sshPort     int
	idleTimeout time.Duration
	slots       chan struct{}
}

func newBridge(opts Options) *bridge {
	return &bridge{
		sshPort:     opts.SSHPort,
		idleTimeout: opts.IdleTimeout,
		slots:       make(chan struct{}, opts.MaxConns),
	}
}

func (b *bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !b.acquireSlot() {
		http.Error(w, "server busy", http.StatusServiceUnavailable)
		return
	}
	defer b.releaseSlot()

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade error", "error", err)
		return
	}
	defer func() { _ = ws.Close() }()
	configureConn(ws)

	conn, err := dialGame(b.sshPort)
	if err != nil {
		slog.Error("game connection error", "error", err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to connect to game server: %v\r\n", err)))
		return
	}
	defer conn.close()

	b.relay(ws, conn)
}

func (b *bridge) acquireSlot() bool {
	select {
	case b.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (b *bridge) releaseSlot() {
	<-b.slots
}

func configureConn(ws *websocket.Conn) {
	ws.SetReadLimit(wsMaxMessageBytes)
	_ = ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(wsPongWait))
	})
}

type gameConn struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

func dialGame(sshPort int) (*gameConn, error) {
	client, err := ssh.Dial("tcp", fmt.Sprintf("localhost:%d", sshPort), &ssh.ClientConfig{
		User:            "web-player",
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	conn := &gameConn{client: client}
	if err := conn.openShell(); err != nil {
		conn.close()
		return nil, err
	}
	return conn, nil
}

func (c *gameConn) openShell() error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}
	c.session = session
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return fmt.Errorf("pty request: %w", err)
	}
	if c.stdin, err = session.StdinPipe(); err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	if c.stdout, err = session.StdoutPipe(); err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := session.Shell(); err != nil {
		return fmt.Errorf("shell: %w", err)
	}
	return nil
}

func (c *gameConn) close() {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.session != nil {
		_ = c.session.Close()
	}
	_ = c.client.Close()
}

type relaySession struct {
	ws      *websocket.Conn
	game    *gameConn
	writeMu sync.Mutex
	input   chan struct{}
	done    chan struct{}
	once    sync.Once
}

func (b *bridge) relay(ws *websocket.Conn, conn *gameConn) {
	rs := &relaySession{
		ws:    ws,
		game:  conn,
		input: make(chan struct{}, 1),
		done:  make(chan struct{}),
	}
	go rs.pumpGameOutput()
	go rs.pumpClientInput()
	go rs.keepAlive()
	go rs.watchIdle(b.idleTimeout)
	<-rs.done
}

func (rs *relaySession) stop() {
	rs.once.Do(func() { close(rs.done) })
}

func (rs *relaySession) write(messageType int, data []byte) error {
	rs.writeMu.Lock()
	defer rs.writeMu.Unlock()
	_ = rs.ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
	return rs.ws.WriteMessage(messageType, data)
}

func (rs *relaySession) pumpGameOutput() {
	defer rs.stop()
	buf := make([]byte, 4096)
	for {
		n, err := rs.game.stdout.Read(buf)
		if n > 0 {
			if writeErr := rs.write(websocket.BinaryMessage, buf[:n]); writeErr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (rs *relaySession) pumpClientInput() {
	defer rs.stop()
	for {
		msgType, data, err := rs.ws.ReadMessage()
		if err != nil {
			return
		}
		if err := rs.handleClientMessage(msgType, data); err != nil {
			return
		}
	}
}

func (rs *relaySession) handleClientMessage(msgType int, data []byte) error {
	switch msgType {
	case websocket.TextMessage:
		rs.noteInput()
		_, err := rs.game.stdin.Write(data)
		return err
	case websocket.BinaryMessage:
		var msg resizeMsg
		if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
			_ = rs.game.session.WindowChange(msg.Rows, msg.Cols)
		}
	}
	return nil
}

func (rs *relaySession) noteInput() {
	select {
	case rs.input <- struct{}{}:
	default:
	}
}

func (rs *relaySession) keepAlive() {
	defer rs.stop()
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := rs.write(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-rs.done:
			return
		}
	}
}

func (rs *relaySession) watchIdle(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-rs.input:
			timer.Reset(timeout)
		case <-timer.C:
			slog.Info("closing idle web session", "idle", timeout)
			rs.stop()
			return
		case <-rs.done:
			return
		}
	}
}
