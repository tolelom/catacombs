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
	FleeChance      float64 `yaml:"flee_chance"`
	MonsterScaling  float64 `yaml:"monster_scaling"`
	SoloHPReduction float64 `yaml:"solo_hp_reduction"`
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
		Combat: CombatConfig{FleeChance: 0.50, MonsterScaling: 1.15, SoloHPReduction: 0.50},
		Dungeon: DungeonConfig{MapWidth: 60, MapHeight: 20, MinRooms: 5, MaxRooms: 8},
		Backup:     BackupConfig{IntervalMin: 60, Dir: "./data/backup"},
		Difficulty: DifficultyConfig{HardModeMonsterMult: 1.5, HardModeShopMult: 2.0, HardModeHealMult: 0.5},
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
	return &cfg, nil
}
