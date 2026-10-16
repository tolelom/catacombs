package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/web"
)

func main() {
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

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

	db, err := store.Open("data/catacombs.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("db close failed", "error", err)
		}
	}()

	lobby := game.NewLobby(cfg)
	startTime := time.Now()

	sshAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.SSHPort)
	webAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	webServer := web.Start(webAddr, web.Options{
		SSHPort:   cfg.Server.SSHPort,
		Lobby:     lobby,
		DB:        db,
		StartTime: startTime,
		Admin:     cfg.Admin,
	})
	sshServer, err := server.NewServer(sshAddr, "data/ssh_host_key", lobby, db)
	if err != nil {
		log.Fatalf("Failed to create SSH server: %v", err)
	}
	backupDone := make(chan struct{})
	var backupWG sync.WaitGroup
	backupWG.Add(1)
	go func() {
		defer backupWG.Done()
		backupScheduler(db, cfg.Backup, backupDone)
	}()
	sshErrCh := make(chan error, 1)
	go func() {
		slog.Info("starting SSH server", "addr", sshAddr)
		sshErrCh <- sshServer.ListenAndServe()
	}()

	slog.Info("server starting", "ssh_port", cfg.Server.SSHPort, "http_port", cfg.Server.HTTPPort)
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
	slog.Info("starting graceful shutdown")
	close(backupDone)
	backupWG.Wait()
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
	lobby.StopAllSessions()
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
			if pruned, err := store.PruneBackups(cfg.Dir, 24); err != nil {
				slog.Warn("backup prune failed", "error", err)
			} else if pruned > 0 {
				slog.Info("backup prune completed", "deleted", pruned)
			}
		}
	}
}
