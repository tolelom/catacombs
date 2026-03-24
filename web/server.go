package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
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

// Start launches the HTTP server for the web terminal.
func Start(addr string, sshPort int) error {
	mux := http.NewServeMux()

	// Serve static files from embedded FS
	mux.Handle("/", http.FileServer(http.FS(staticFiles)))

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(w, r, sshPort)
	})

	log.Printf("Starting web terminal on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func handleWS(w http.ResponseWriter, r *http.Request, sshPort int) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
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
		log.Printf("SSH dial error: %v", err)
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to connect to game server: %v\r\n", err)))
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("SSH session error: %v", err)
		return
	}
	defer session.Close()

	// Request PTY
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		log.Printf("PTY request error: %v", err)
		return
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Printf("stdin pipe error: %v", err)
		return
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("stdout pipe error: %v", err)
		return
	}

	if err := session.Shell(); err != nil {
		log.Printf("shell error: %v", err)
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
