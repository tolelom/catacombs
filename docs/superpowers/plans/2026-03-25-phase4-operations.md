# Phase 4: Operational Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin dashboard, automatic DB backup, and graceful shutdown for production-ready operation.

**Architecture:** Three independent subsystems: (1) `/admin` HTTP endpoint with Basic Auth returning JSON stats, (2) periodic BoltDB file backup using existing `BackupConfig`, (3) signal-driven graceful shutdown that stops servers and backs up before exit. All hook into `main.go` orchestration.

**Tech Stack:** Go stdlib (`net/http`, `os/signal`, `io`, `time`), existing BoltDB (`go.etcd.io/bbolt`), Wish SSH server, config package.

**Spec:** `docs/superpowers/specs/2026-03-25-game-enhancement-design.md` (Phase 4, lines 302-325)

**Note on "restart session cleanup" (spec 4-2):** The spec mentions cleaning up incomplete sessions on restart. Since `game.Lobby` and all `GameSession` state are in-memory (not persisted to BoltDB), a fresh server start always begins with a clean slate. No explicit cleanup logic is needed.

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `config/config.go` | Modify | Add `AdminConfig` (username, password) to `Config` struct |
| `config.yaml` | Modify | Add `admin` section |
| `store/backup.go` | Create | `Backup(destDir)` method — copies DB file with timestamp |
| `store/backup_test.go` | Create | Tests for backup functionality |
| `store/stats.go` | Create | `GetTodayRunCount()` and `GetTodayAvgFloor()` query methods |
| `store/stats_test.go` | Create | Tests for stats queries |
| `web/admin.go` | Create | `/admin` handler with Basic Auth, JSON response |
| `web/admin_test.go` | Create | Tests for admin endpoint |
| `web/server.go` | Modify | Accept `*http.Server` pattern for graceful shutdown; add admin route; accept lobby+db params |
| `server/ssh.go` | Modify | Return `*wish.Server` for shutdown control |
| `main.go` | Modify | Signal handler, backup scheduler, graceful shutdown orchestration |

---

### Task 1: Admin Config

**Files:**
- Modify: `config/config.go:9-16` (Config struct)
- Modify: `config/config.go:56-68` (defaults func)
- Modify: `config.yaml:1-53`
- Test: `config/config_test.go` (existing)

- [ ] **Step 1: Add AdminConfig to config struct**

In `config/config.go`, add after `DifficultyConfig`:

```go
type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
```

Add `Admin AdminConfig \`yaml:"admin"\`` to the `Config` struct.

In `defaults()`, add:
```go
Admin: AdminConfig{Username: "admin", Password: "catacombs"},
```

- [ ] **Step 2: Add admin section to config.yaml**

Append to `config.yaml`:
```yaml

admin:
  # Basic auth credentials for /admin endpoint
  username: "admin"
  password: "catacombs"
```

- [ ] **Step 3: Run tests to verify config still loads**

Run: `go test ./config/ -v`
Expected: PASS (existing tests still pass with new fields)

- [ ] **Step 4: Commit**

```bash
git add config/config.go config.yaml
git commit -m "feat: add admin config for dashboard authentication"
```

---

### Task 2: DB Backup

**Files:**
- Create: `store/backup.go`
- Create: `store/backup_test.go`

- [ ] **Step 1: Write the failing test**

Create `store/backup_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackup(t *testing.T) {
	// Create a temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Write some data
	if err := db.SaveProfile("fp1", "player1"); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	backupDir := filepath.Join(tmpDir, "backups")

	// Run backup
	backupPath, err := db.Backup(backupDir)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("backup file does not exist")
	}

	// Verify backup file name contains timestamp pattern
	base := filepath.Base(backupPath)
	if !strings.HasPrefix(base, "catacombs-") || !strings.HasSuffix(base, ".db") {
		t.Fatalf("unexpected backup filename: %s", base)
	}

	// Verify backup is readable by opening it
	backupDB, err := Open(backupPath)
	if err != nil {
		t.Fatalf("failed to open backup: %v", err)
	}
	defer backupDB.Close()

	name, err := backupDB.GetProfile("fp1")
	if err != nil {
		t.Fatalf("failed to read from backup: %v", err)
	}
	if name != "player1" {
		t.Fatalf("expected player1, got %s", name)
	}
}

func TestBackupCreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	backupDir := filepath.Join(tmpDir, "nested", "backups")
	_, err = db.Backup(backupDir)
	if err != nil {
		t.Fatalf("backup with nested dir failed: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./store/ -run TestBackup -v`
Expected: FAIL — `db.Backup` method does not exist

- [ ] **Step 3: Write minimal implementation**

Create `store/backup.go`:

```go
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Backup creates a consistent snapshot of the database in destDir.
// Returns the path to the backup file.
func (d *DB) Backup(destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("catacombs-%s.db", timestamp)
	destPath := filepath.Join(destDir, filename)

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	// BoltDB View transaction provides a consistent snapshot
	err = d.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(f)
		return err
	})
	if err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("backup write: %w", err)
	}

	return destPath, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./store/ -run TestBackup -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add store/backup.go store/backup_test.go
git commit -m "feat: add DB backup with consistent BoltDB snapshots"
```

---

### Task 3: Today's Stats Queries

**Files:**
- Create: `store/stats.go`
- Create: `store/stats_test.go`

- [ ] **Step 1: Write the failing test**

Create `store/stats_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGetTodayRunCount(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	today := time.Now().Format("2006-01-02")

	// No runs yet
	count, err := db.GetTodayRunCount()
	if err != nil {
		t.Fatalf("GetTodayRunCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	// Add some daily runs for today
	db.SaveDaily(DailyRecord{Date: today, Player: "fp1", PlayerName: "A", FloorReached: 10, GoldEarned: 100})
	db.SaveDaily(DailyRecord{Date: today, Player: "fp2", PlayerName: "B", FloorReached: 15, GoldEarned: 200})
	// Add a run for yesterday (should not count)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	db.SaveDaily(DailyRecord{Date: yesterday, Player: "fp3", PlayerName: "C", FloorReached: 5, GoldEarned: 50})

	count, err = db.GetTodayRunCount()
	if err != nil {
		t.Fatalf("GetTodayRunCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestGetTodayAvgFloor(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	today := time.Now().Format("2006-01-02")

	// No runs
	avg, err := db.GetTodayAvgFloor()
	if err != nil {
		t.Fatalf("GetTodayAvgFloor: %v", err)
	}
	if avg != 0 {
		t.Fatalf("expected 0, got %f", avg)
	}

	// Add runs: floor 10, floor 20 → avg 15
	db.SaveDaily(DailyRecord{Date: today, Player: "fp1", PlayerName: "A", FloorReached: 10, GoldEarned: 100})
	db.SaveDaily(DailyRecord{Date: today, Player: "fp2", PlayerName: "B", FloorReached: 20, GoldEarned: 200})

	avg, err = db.GetTodayAvgFloor()
	if err != nil {
		t.Fatalf("GetTodayAvgFloor: %v", err)
	}
	if avg != 15.0 {
		t.Fatalf("expected 15.0, got %f", avg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./store/ -run TestGetToday -v`
Expected: FAIL — methods do not exist

- [ ] **Step 3: Write minimal implementation**

Create `store/stats.go`:

```go
package store

import (
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

// GetTodayRunCount returns the number of daily challenge runs for today.
func (d *DB) GetTodayRunCount() (int, error) {
	today := time.Now().Format("2006-01-02")
	count := 0
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		c := b.Cursor()
		prefix := []byte(today + ":")
		for k, _ := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			count++
		}
		return nil
	})
	return count, err
}

// GetTodayAvgFloor returns the average floor reached in today's daily runs.
func (d *DB) GetTodayAvgFloor() (float64, error) {
	today := time.Now().Format("2006-01-02")
	total := 0
	count := 0
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		c := b.Cursor()
		prefix := []byte(today + ":")
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var r DailyRecord
			if json.Unmarshal(v, &r) == nil {
				total += r.FloorReached
				count++
			}
		}
		return nil
	})
	if count == 0 {
		return 0, err
	}
	return float64(total) / float64(count), err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./store/ -run TestGetToday -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add store/stats.go store/stats_test.go
git commit -m "feat: add today's run count and avg floor stat queries"
```

---

### Task 4: Admin HTTP Endpoint

**Files:**
- Create: `web/admin.go`
- Create: `web/admin_test.go`
- Modify: `web/server.go:30-43` (Start function signature and route setup)

- [ ] **Step 1: Write the failing test**

Create `web/admin_test.go`:

```go
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"path/filepath"
)

func TestAdminEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := store.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		Admin: config.AdminConfig{Username: "admin", Password: "secret"},
	}
	lobby := game.NewLobby(cfg)

	handler := AdminHandler(lobby, db, time.Now())

	// Test without auth → 401
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Test with wrong auth → 401
	req = httptest.NewRequest("GET", "/admin", nil)
	req.SetBasicAuth("admin", "wrong")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Test with correct auth → 200 + JSON
	req = httptest.NewRequest("GET", "/admin", nil)
	req.SetBasicAuth("admin", "secret")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats AdminStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if stats.OnlinePlayers != 0 {
		t.Fatalf("expected 0 online, got %d", stats.OnlinePlayers)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/ -run TestAdmin -v`
Expected: FAIL — `AdminHandler` and `AdminStats` do not exist

- [ ] **Step 3: Write minimal implementation**

Create `web/admin.go`:

```go
package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// AdminStats is the JSON response for the /admin endpoint.
type AdminStats struct {
	OnlinePlayers int     `json:"online_players"`
	ActiveRooms   int     `json:"active_rooms"`
	TodayRuns     int     `json:"today_runs"`
	AvgFloorReach float64 `json:"avg_floor_reached"`
	UptimeSeconds int64   `json:"uptime_seconds"`
}

// AdminHandler returns an http.Handler for the /admin stats endpoint.
// It requires Basic Auth using credentials from config.
func AdminHandler(lobby *game.Lobby, db *store.DB, startTime time.Time) http.Handler {
	cfg := lobby.Cfg()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r, cfg.Admin) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Catacombs Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		todayRuns, _ := db.GetTodayRunCount()
		avgFloor, _ := db.GetTodayAvgFloor()

		stats := AdminStats{
			OnlinePlayers: len(lobby.ListOnline()),
			ActiveRooms:   len(lobby.ListRooms()),
			TodayRuns:     todayRuns,
			AvgFloorReach: avgFloor,
			UptimeSeconds: int64(time.Since(startTime).Seconds()),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})
}

func checkAuth(r *http.Request, cfg config.AdminConfig) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return username == cfg.Username && password == cfg.Password
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./web/ -run TestAdmin -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/admin.go web/admin_test.go
git commit -m "feat: add /admin endpoint with Basic Auth and JSON stats"
```

---

### Task 5: Refactor Web Server for Graceful Shutdown

**Files:**
- Modify: `web/server.go:29-43` (Start function)

The web server currently uses `http.ListenAndServe` which cannot be shut down gracefully. Refactor to return an `*http.Server` that the caller can `Shutdown()`.

- [ ] **Step 1: Refactor web.Start to return *http.Server**

Change `web/server.go` `Start` function:

```go
// Start launches the HTTP server for the web terminal.
// Returns the *http.Server for graceful shutdown control.
func Start(addr string, sshPort int, lobby *game.Lobby, db *store.DB, startTime time.Time) *http.Server {
	mux := http.NewServeMux()

	// Serve static files from embedded FS
	mux.Handle("/", http.FileServer(http.FS(staticFiles)))

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(w, r, sshPort)
	})

	// Admin dashboard endpoint
	mux.Handle("/admin", AdminHandler(lobby, db, startTime))

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		slog.Info("starting web terminal", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("web server error", "error", err)
		}
	}()

	return srv
}
```

Add `"time"` to imports. Add `game` and `store` imports:
```go
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
```

- [ ] **Step 2: Update main.go to use new Start signature**

In `main.go`, replace the web server goroutine:

```go
// Before:
go func() {
    if err := web.Start(webAddr, cfg.Server.SSHPort); err != nil {
        slog.Error("web server error", "error", err)
    }
}()

// After:
startTime := time.Now()
webServer := web.Start(webAddr, cfg.Server.SSHPort, lobby, db, startTime)
_ = webServer // used later for graceful shutdown
```

Add `"time"` to main.go imports.

- [ ] **Step 3: Run build to verify compilation**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add web/server.go main.go
git commit -m "refactor: web server returns *http.Server for shutdown control"
```

---

### Task 6: Refactor SSH Server for Graceful Shutdown

**Files:**
- Modify: `server/ssh.go:17-53`

Return the `*ssh.Server` so the caller can shut it down.

- [ ] **Step 1: Refactor server.Start to return wish Server**

Change `server/ssh.go`:

```go
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
```

Keep the old `Start` function for backwards compatibility during transition:

```go
// Start creates and starts the SSH server (blocking).
func Start(addr string, lobby *game.Lobby, db *store.DB) error {
	s, err := NewServer(addr, lobby, db)
	if err != nil {
		return err
	}
	slog.Info("starting SSH server", "addr", addr)
	return s.ListenAndServe()
}
```

- [ ] **Step 2: Run build to verify compilation**

Run: `go build ./...`
Expected: SUCCESS (Start still works, NewServer is additive)

- [ ] **Step 3: Commit**

```bash
git add server/ssh.go
git commit -m "refactor: extract NewServer for SSH shutdown control"
```

---

### Task 7: Graceful Shutdown + Backup Scheduler in main.go

**Files:**
- Modify: `main.go`

This is the orchestration task: signal handling, backup scheduler, and graceful shutdown.

- [ ] **Step 1: Rewrite main.go with full orchestration**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/web"
)

func main() {
	os.MkdirAll("data", 0755)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			cfg, _ = config.Load("")
		} else {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	db, err := store.Open("data/catacombs.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	lobby := game.NewLobby(cfg)
	startTime := time.Now()

	sshAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.SSHPort)
	webAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)

	// Start web server (non-blocking, returns *http.Server)
	webServer := web.Start(webAddr, cfg.Server.SSHPort, lobby, db, startTime)

	// Create SSH server
	sshServer, err := server.NewServer(sshAddr, lobby, db)
	if err != nil {
		log.Fatalf("Failed to create SSH server: %v", err)
	}

	// Start backup scheduler
	backupDone := make(chan struct{})
	go backupScheduler(db, cfg.Backup, backupDone)

	// Start SSH server in background
	sshErrCh := make(chan error, 1)
	go func() {
		slog.Info("starting SSH server", "addr", sshAddr)
		sshErrCh <- sshServer.ListenAndServe()
	}()

	slog.Info("server starting", "ssh_port", cfg.Server.SSHPort, "http_port", cfg.Server.HTTPPort)

	// Wait for shutdown signal or SSH server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("shutdown signal received", "signal", sig)
	case err := <-sshErrCh:
		if err != nil {
			slog.Error("SSH server error", "error", err)
		}
	}

	// Graceful shutdown
	slog.Info("starting graceful shutdown")

	// Stop backup scheduler
	close(backupDone)

	// Shutdown web server (5s timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := webServer.Shutdown(ctx); err != nil {
		slog.Error("web server shutdown error", "error", err)
	}

	// Shutdown SSH server (10s timeout for active sessions to finish)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	if err := sshServer.Shutdown(ctx2); err != nil {
		slog.Error("SSH server shutdown error", "error", err)
	}

	// Final backup before exit
	if path, err := db.Backup(cfg.Backup.Dir); err != nil {
		slog.Error("final backup failed", "error", err)
	} else {
		slog.Info("final backup completed", "path", path)
	}

	slog.Info("server stopped")
}

func backupScheduler(db *store.DB, cfg config.BackupConfig, done chan struct{}) {
	if cfg.IntervalMin <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(cfg.IntervalMin) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if path, err := db.Backup(cfg.Dir); err != nil {
				slog.Error("scheduled backup failed", "error", err)
			} else {
				slog.Info("scheduled backup completed", "path", path)
			}
		}
	}
}
```

Note: `syscall.SIGTERM` works on Windows as a no-op but `SIGINT` (Ctrl+C) works. On Linux both work. This is acceptable.

- [ ] **Step 2: Run build to verify compilation**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: graceful shutdown with signal handling and backup scheduler"
```

---

### Task 8: Integration Verification

**Files:** None (verification only)

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Run vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Build binary**

Run: `go build -o catacombs .`
Expected: Binary builds successfully

- [ ] **Step 4: Verify admin endpoint manually (optional)**

Start the server and test:
```bash
curl -u admin:catacombs http://localhost:8080/admin
```
Expected: JSON response with `online_players`, `active_rooms`, `today_runs`, `avg_floor_reached`, `uptime_seconds`

- [ ] **Step 5: Final commit if any fixes needed**

Only if previous steps revealed issues that were fixed.
