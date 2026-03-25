package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
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

	sshAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.SSHPort)
	webAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)

	startTime := time.Now()
	webServer := web.Start(webAddr, cfg.Server.SSHPort, lobby, db, startTime)
	_ = webServer // used later for graceful shutdown

	slog.Info("server starting", "ssh_port", cfg.Server.SSHPort, "http_port", cfg.Server.HTTPPort)
	if err := server.Start(sshAddr, lobby, db); err != nil {
		log.Fatal(err)
	}
}
