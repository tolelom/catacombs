package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

//go:embed static
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// Start launches the HTTP server for the web terminal and returns the server handle.
func Start(addr string, sshPort int, lobby *game.Lobby, db *store.DB, startTime time.Time) *http.Server {
	mux := http.NewServeMux()

	// Serve static files from embedded FS
	mux.Handle("/", http.FileServer(http.FS(staticFiles)))

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(w, r, sshPort)
	})

	// Admin endpoint
	mux.Handle("/admin", AdminHandler(lobby, db, startTime))

	srv := &http.Server{Addr: addr, Handler: mux}

	slog.Info("starting web terminal", "addr", addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("web server error", "error", err)
		}
	}()

	return srv
}

func handleWS(w http.ResponseWriter, r *http.Request, sshPort int) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade error", "error", err)
		return
	}
	defer ws.Close()

	// Connect to local SSH server
	sshConfig := &ssh.ClientConfig{
		User: "web-player",
		Auth: []ssh.AuthMethod{
			ssh.Password(""),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshAddr := fmt.Sprintf("localhost:%d", sshPort)
	client, err := ssh.Dial("tcp", sshAddr, sshConfig)
	if err != nil {
		slog.Error("SSH dial error", "error", err)
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to connect to game server: %v\r\n", err)))
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		slog.Error("SSH session error", "error", err)
		return
	}
	defer session.Close()

	// Request PTY
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		slog.Error("PTY request error", "error", err)
		return
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		slog.Error("stdin pipe error", "error", err)
		return
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		slog.Error("stdout pipe error", "error", err)
		return
	}

	if err := session.Shell(); err != nil {
		slog.Error("shell error", "error", err)
		return
	}

	var once sync.Once
	done := make(chan struct{})
	cleanup := func() {
		once.Do(func() {
			close(done)
		})
	}

	// SSH stdout → WebSocket
	go func() {
		defer cleanup()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if writeErr := ws.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket → SSH stdin (text frames) or resize (binary frames)
	go func() {
		defer cleanup()
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			switch msgType {
			case websocket.TextMessage:
				if _, err := stdin.Write(data); err != nil {
					return
				}
			case websocket.BinaryMessage:
				var msg resizeMsg
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
					session.WindowChange(msg.Rows, msg.Cols)
				}
			}
		}
	}()

	// Wait for either side to close
	select {
	case <-done:
	}

	// Ensure SSH session ends
	_ = session.Close()
	_ = client.Close()
	_ = io.WriteCloser(stdin).Close()
}
