package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Game       GameConfig       `yaml:"game"`
	Combat     CombatConfig     `yaml:"combat"`
	Dungeon    DungeonConfig    `yaml:"dungeon"`
	Backup     BackupConfig     `yaml:"backup"`
	Difficulty DifficultyConfig `yaml:"difficulty"`
	Admin      AdminConfig      `yaml:"admin"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type DifficultyConfig struct {
	HardModeMonsterMult float64 `yaml:"hard_mode_monster_mult"`
	HardModeShopMult    float64 `yaml:"hard_mode_shop_mult"`
	HardModeHealMult    float64 `yaml:"hard_mode_heal_mult"`
}

type ServerConfig struct {
	SSHPort  int `yaml:"ssh_port"`
	HTTPPort int `yaml:"http_port"`
}

type GameConfig struct {
	TurnTimeoutSec int     `yaml:"turn_timeout_sec"`
	MaxPlayers     int     `yaml:"max_players"`
	MaxFloors      int     `yaml:"max_floors"`
	CoopBonus      float64 `yaml:"coop_bonus"`
	InventoryLimit int     `yaml:"inventory_limit"`
	SkillUses      int     `yaml:"skill_uses"`
}

type CombatConfig struct {
	FleeChance       float64 `yaml:"flee_chance"`
	MonsterScaling   float64 `yaml:"monster_scaling"`
	SoloHPReduction  float64 `yaml:"solo_hp_reduction"`
	SoloATKReduction float64 `yaml:"solo_atk_reduction"`
	EarlyThreatRamp  float64 `yaml:"early_threat_ramp"`
}

type DungeonConfig struct {
	MapWidth  int `yaml:"map_width"`
	MapHeight int `yaml:"map_height"`
	MinRooms  int `yaml:"min_rooms"`
	MaxRooms  int `yaml:"max_rooms"`
}

type BackupConfig struct {
	IntervalMin int    `yaml:"interval_min"`
	Dir         string `yaml:"dir"`
}

func defaults() Config {
	return Config{
		Server: ServerConfig{SSHPort: 2222, HTTPPort: 8080},
		Game: GameConfig{
			TurnTimeoutSec: 10, MaxPlayers: 4, MaxFloors: 20,
			CoopBonus: 0.10, InventoryLimit: 10, SkillUses: 3,
		},
		Combat: CombatConfig{FleeChance: 0.50, MonsterScaling: 1.15, SoloHPReduction: 0.50, SoloATKReduction: 0.75, EarlyThreatRamp: 0.08},
		Dungeon: DungeonConfig{MapWidth: 60, MapHeight: 20, MinRooms: 5, MaxRooms: 8},
		Backup:     BackupConfig{IntervalMin: 60, Dir: "./data/backup"},
		Difficulty: DifficultyConfig{HardModeMonsterMult: 1.35, HardModeShopMult: 1.75, HardModeHealMult: 0.6},
		Admin:      AdminConfig{Username: "admin", Password: "catacombs"},
	}
}

func Load(path string) (*Config, error) {
	cfg := defaults()
	if path == "" {
		return &cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.normalize()
	return &cfg, nil
}

// normalize replaces zero/negative values with defaults. Without this guard
// a YAML file that omits or zeroes a critical field — TurnTimeoutSec, MaxPlayers,
// MaxFloors, InventoryLimit — would put the game into an unrunnable state
// (turn timer firing instantly, JoinRoom always rejecting, immediate victory
// on floor 1, etc.). We only touch fields whose zero value is unsafe; flags
// like SkillUses=0 are legitimate (the no_skills mutation produces it via
// effectiveSkillUses).
func (c *Config) normalize() {
	d := defaults()
	if c.Server.SSHPort <= 0 {
		c.Server.SSHPort = d.Server.SSHPort
	}
	if c.Server.HTTPPort <= 0 {
		c.Server.HTTPPort = d.Server.HTTPPort
	}
	if c.Game.TurnTimeoutSec <= 0 {
		c.Game.TurnTimeoutSec = d.Game.TurnTimeoutSec
	}
	if c.Game.MaxPlayers <= 0 {
		c.Game.MaxPlayers = d.Game.MaxPlayers
	}
	if c.Game.MaxFloors <= 0 {
		c.Game.MaxFloors = d.Game.MaxFloors
	}
	if c.Game.InventoryLimit <= 0 {
		c.Game.InventoryLimit = d.Game.InventoryLimit
	}
	if c.Game.SkillUses < 0 {
		c.Game.SkillUses = d.Game.SkillUses
	}
	if c.Combat.MonsterScaling <= 0 {
		c.Combat.MonsterScaling = d.Combat.MonsterScaling
	}
	if c.Combat.SoloHPReduction <= 0 {
		c.Combat.SoloHPReduction = d.Combat.SoloHPReduction
	}
	if c.Combat.SoloATKReduction <= 0 {
		c.Combat.SoloATKReduction = d.Combat.SoloATKReduction
	}
	if c.Combat.EarlyThreatRamp < 0 {
		c.Combat.EarlyThreatRamp = d.Combat.EarlyThreatRamp
	}
	if c.Difficulty.HardModeMonsterMult <= 0 {
		c.Difficulty.HardModeMonsterMult = d.Difficulty.HardModeMonsterMult
	}
	if c.Difficulty.HardModeShopMult <= 0 {
		c.Difficulty.HardModeShopMult = d.Difficulty.HardModeShopMult
	}
	if c.Difficulty.HardModeHealMult <= 0 {
		c.Difficulty.HardModeHealMult = d.Difficulty.HardModeHealMult
	}
}
