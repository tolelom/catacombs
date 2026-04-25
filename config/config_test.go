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
	if cfg.Game.TurnTimeoutSec != 10 {
		t.Errorf("expected turn timeout 10, got %d", cfg.Game.TurnTimeoutSec)
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
	if cfg.Difficulty.HardModeMonsterMult != 1.5 {
		t.Errorf("expected hard mode monster mult 1.5, got %f", cfg.Difficulty.HardModeMonsterMult)
	}
	if cfg.Difficulty.HardModeShopMult != 2.0 {
		t.Errorf("expected hard mode shop mult 2.0, got %f", cfg.Difficulty.HardModeShopMult)
	}
	if cfg.Difficulty.HardModeHealMult != 0.5 {
		t.Errorf("expected hard mode heal mult 0.5, got %f", cfg.Difficulty.HardModeHealMult)
	}
}

// TestLoadNormalizesZeroValues guards against a YAML supplying zero or
// negative critical fields that would put the game in an unrunnable state
// (TurnTimeoutSec=0 → instant turn timeout, MaxPlayers=0 → JoinRoom always
// rejects, MaxFloors=0 → immediate victory at floor 1).
func TestLoadNormalizesZeroValues(t *testing.T) {
	content := []byte(`
server:
  ssh_port: 0
  http_port: -1
game:
  turn_timeout_sec: 0
  max_players: 0
  max_floors: 0
  inventory_limit: 0
combat:
  monster_scaling: 0
  solo_hp_reduction: 0
difficulty:
  hard_mode_monster_mult: 0
  hard_mode_shop_mult: 0
  hard_mode_heal_mult: 0
`)
	f, err := os.CreateTemp("", "config-zero-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	d := defaults()
	if cfg.Server.SSHPort != d.Server.SSHPort {
		t.Errorf("zero SSHPort should normalize to default %d, got %d", d.Server.SSHPort, cfg.Server.SSHPort)
	}
	if cfg.Server.HTTPPort != d.Server.HTTPPort {
		t.Errorf("negative HTTPPort should normalize to default %d, got %d", d.Server.HTTPPort, cfg.Server.HTTPPort)
	}
	if cfg.Game.TurnTimeoutSec != d.Game.TurnTimeoutSec {
		t.Errorf("zero TurnTimeoutSec should normalize, got %d", cfg.Game.TurnTimeoutSec)
	}
	if cfg.Game.MaxPlayers != d.Game.MaxPlayers {
		t.Errorf("zero MaxPlayers should normalize, got %d", cfg.Game.MaxPlayers)
	}
	if cfg.Game.MaxFloors != d.Game.MaxFloors {
		t.Errorf("zero MaxFloors should normalize, got %d", cfg.Game.MaxFloors)
	}
	if cfg.Game.InventoryLimit != d.Game.InventoryLimit {
		t.Errorf("zero InventoryLimit should normalize, got %d", cfg.Game.InventoryLimit)
	}
	if cfg.Combat.MonsterScaling != d.Combat.MonsterScaling {
		t.Errorf("zero MonsterScaling should normalize, got %f", cfg.Combat.MonsterScaling)
	}
	if cfg.Difficulty.HardModeMonsterMult != d.Difficulty.HardModeMonsterMult {
		t.Errorf("zero HardModeMonsterMult should normalize, got %f", cfg.Difficulty.HardModeMonsterMult)
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
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

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
