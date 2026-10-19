package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/record"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/web"
)

func main() {
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	setupLogger()
	cfg := loadConfig()

	db := openDatabase()
	defer closeDatabase(db)

	lobby := game.NewLobby(cfg)
	lobby.SetRecorder(record.New(db))

	webServer := startWebServer(cfg, lobby, db, time.Now())
	sshServer, sshAddr := newSSHServer(cfg, lobby, db)
	stopBackups := startBackupScheduler(db, cfg.Backup)
	sshErrCh := serveSSH(sshServer, sshAddr)

	slog.Info("server starting", "ssh_port", cfg.Server.SSHPort, "http_port", cfg.Server.HTTPPort)
	waitForShutdown(sshErrCh)

	slog.Info("starting graceful shutdown")
	stopBackups()
	shutdownServers(webServer, sshServer)
	lobby.StopAllSessions()
	runFinalBackup(db, cfg.Backup.Dir)

	slog.Info("server stopped")
}

func setupLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func loadConfig() *config.Config {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			cfg, err = config.Load("")
			if err != nil {
				log.Fatalf("Failed to load default config: %v", err)
			}
		} else {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	config.ApplyEnv(cfg, os.LookupEnv)
	return cfg
}

func openDatabase() *store.DB {
	db, err := store.Open("data/catacombs.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	return db
}

func closeDatabase(db *store.DB) {
	if err := db.Close(); err != nil {
		slog.Error("db close failed", "error", err)
	}
}

func startWebServer(cfg *config.Config, lobby *game.Lobby, db *store.DB, startTime time.Time) *http.Server {
	webAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	return web.Start(webAddr, web.Options{
		SSHPort:   cfg.Server.SSHPort,
		Lobby:     lobby,
		DB:        db,
		StartTime: startTime,
		Admin:     cfg.Admin,
	})
}

func newSSHServer(cfg *config.Config, lobby *game.Lobby, db *store.DB) (*ssh.Server, string) {
	sshAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.SSHPort)
	sshServer, err := server.NewServer(sshAddr, "data/ssh_host_key", lobby, db)
	if err != nil {
		log.Fatalf("Failed to create SSH server: %v", err)
	}
	return sshServer, sshAddr
}

func startBackupScheduler(db *store.DB, cfg config.BackupConfig) (stop func()) {
	backupDone := make(chan struct{})
	var backupWG sync.WaitGroup
	backupWG.Add(1)
	go func() {
		defer backupWG.Done()
		backupScheduler(db, cfg, backupDone)
	}()
	return func() {
		close(backupDone)
		backupWG.Wait()
	}
}

func serveSSH(sshServer *ssh.Server, sshAddr string) <-chan error {
	sshErrCh := make(chan error, 1)
	go func() {
		slog.Info("starting SSH server", "addr", sshAddr)
		sshErrCh <- sshServer.ListenAndServe()
	}()
	return sshErrCh
}

func waitForShutdown(sshErrCh <-chan error) {
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
}

func shutdownServers(webServer *http.Server, sshServer *ssh.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := webServer.Shutdown(ctx); err != nil {
		slog.Error("web server shutdown error", "error", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	if err := sshServer.Shutdown(ctx2); err != nil {
		slog.Error("SSH server shutdown error", "error", err)
	}
}

func runFinalBackup(db *store.DB, dir string) {
	if path, err := db.Backup(dir); err != nil {
		slog.Error("final backup failed", "error", err)
	} else {
		slog.Info("final backup completed", "path", path)
	}
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
			if pruned, err := store.PruneBackups(cfg.Dir, 24); err != nil {
				slog.Warn("backup prune failed", "error", err)
			} else if pruned > 0 {
				slog.Info("backup prune completed", "deleted", pruned)
			}
		}
	}
}
