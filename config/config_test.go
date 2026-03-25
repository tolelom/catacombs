package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.SSHPort != 2222 {
		t.Errorf("expected SSH port 2222, got %d", cfg.Server.SSHPort)
	}
	if cfg.Server.HTTPPort != 8080 {
		t.Errorf("expected HTTP port 8080, got %d", cfg.Server.HTTPPort)
	}
	if cfg.Game.TurnTimeoutSec != 5 {
		t.Errorf("expected turn timeout 5, got %d", cfg.Game.TurnTimeoutSec)
	}
	if cfg.Game.MaxPlayers != 4 {
		t.Errorf("expected max players 4, got %d", cfg.Game.MaxPlayers)
	}
	if cfg.Game.MaxFloors != 20 {
		t.Errorf("expected max floors 20, got %d", cfg.Game.MaxFloors)
	}
	if cfg.Game.CoopBonus != 0.10 {
		t.Errorf("expected coop bonus 0.10, got %f", cfg.Game.CoopBonus)
	}
	if cfg.Game.InventoryLimit != 10 {
		t.Errorf("expected inventory limit 10, got %d", cfg.Game.InventoryLimit)
	}
	if cfg.Combat.FleeChance != 0.50 {
		t.Errorf("expected flee chance 0.50, got %f", cfg.Combat.FleeChance)
	}
	if cfg.Combat.MonsterScaling != 1.15 {
		t.Errorf("expected monster scaling 1.15, got %f", cfg.Combat.MonsterScaling)
	}
}

func TestLoadFromFile(t *testing.T) {
	content := []byte(`
server:
  ssh_port: 3333
  http_port: 9090
game:
  turn_timeout_sec: 10
  max_players: 2
`)
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Write(content)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.SSHPort != 3333 {
		t.Errorf("expected SSH port 3333, got %d", cfg.Server.SSHPort)
	}
	if cfg.Server.HTTPPort != 9090 {
		t.Errorf("expected HTTP port 9090, got %d", cfg.Server.HTTPPort)
	}
	if cfg.Game.TurnTimeoutSec != 10 {
		t.Errorf("expected turn timeout 10, got %d", cfg.Game.TurnTimeoutSec)
	}
	if cfg.Game.MaxPlayers != 2 {
		t.Errorf("expected max players 2, got %d", cfg.Game.MaxPlayers)
	}
	// Unset fields should have defaults
	if cfg.Game.MaxFloors != 20 {
		t.Errorf("expected default max floors 20, got %d", cfg.Game.MaxFloors)
	}
	if cfg.Combat.FleeChance != 0.50 {
		t.Errorf("expected default flee chance 0.50, got %f", cfg.Combat.FleeChance)
	}
	if cfg.Dungeon.MapWidth != 60 {
		t.Errorf("expected default map width 60, got %d", cfg.Dungeon.MapWidth)
	}
}
