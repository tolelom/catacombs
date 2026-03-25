# Phase 1: Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the foundational infrastructure (UI architecture, config system, structured logging, emote system) that all subsequent phases build upon.

**Architecture:** Extract each screen's Update logic from the central `ui/model.go` into independent screen models with a shared Context struct. Add a `config/` package for YAML-based configuration, `log/slog` structured logging in server/game packages, and a chat emote system on top of the existing `SendChat()`.

**Tech Stack:** Go 1.25.1, Bubble Tea, BoltDB, `log/slog`, `gopkg.in/yaml.v3`

**Module path:** `github.com/tolelom/catacombs` (all imports must use this prefix)

**Note on spec deviation:** The spec defines `TurnTimeout` as `time.Duration`, but this plan uses `TurnTimeoutSec int` because `time.Duration` does not unmarshal cleanly from YAML integers.

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `config/config.go` | YAML config parsing, default values, `Config` struct |
| `config/config_test.go` | Config loading and default tests |
| `config.yaml` | Default configuration file |
| `game/emote.go` | Emote definitions, parsing `/command` → emote text |
| `game/emote_test.go` | Emote parsing tests |
| `ui/context.go` | Shared `Context` struct used by all screen models |
| `ui/screen.go` | `Screen` interface definition (`Update`, `View`) |

### Modified Files
| File | Changes |
|------|---------|
| `ui/model.go` | Extract Update/View dispatch to use Screen interface; remove per-screen update logic |
| `ui/game_view.go` | Add `GameScreen` struct implementing `Screen`; move `updateGame()` here |
| `ui/lobby_view.go` | Add `LobbyScreen` struct implementing `Screen`; move `updateLobby()` here |
| `ui/class_view.go` | Add `ClassSelectScreen` struct implementing `Screen`; move `updateClassSelect()` here |
| `ui/shop_view.go` | Add `ShopScreen` struct implementing `Screen`; move `updateShop()` here |
| `ui/result_view.go` | Add `ResultScreen` struct implementing `Screen`; move `updateResult()` here |
| `ui/help_view.go` | Add `HelpScreen` struct implementing `Screen`; move `updateHelp()` here |
| `ui/stats_view.go` | Add `StatsScreen` struct implementing `Screen`; move `updateStats()` here |
| `ui/achievements_view.go` | Add `AchievementsScreen` struct implementing `Screen`; move `updateAchievements()` here |
| `ui/leaderboard_view.go` | Add `LeaderboardScreen` struct implementing `Screen`; move `updateLeaderboard()` here |
| `ui/nickname_view.go` | Add `NicknameScreen` struct implementing `Screen`; move `updateNickname()` here |
| `ui/title.go` | Add `TitleScreen` struct implementing `Screen`; move `updateTitle()` here |
| `ui/model_test.go` | Update tests to work with new Screen interface |
| `game/session.go` | Store `*config.Config`; update `SendChat` to handle emote parsing |
| `game/turn.go` | Replace hardcoded `TurnTimeout` with config value |
| `game/event.go` | Replace hardcoded constants (skill uses, solo reduction, inventory limit) with config |
| `game/lobby.go` | Store `*config.Config`; replace hardcoded max players |
| `entity/monster.go` | Accept scaling param instead of hardcoded 1.15 |
| `combat/combat.go` | Accept flee chance and coop bonus params from config |
| `main.go` | Load config, initialize slog, pass config to server/lobby |
| `server/ssh.go` | Change `Start(host, port, lobby, db)` → `Start(addr string, lobby *game.Lobby, db *store.DB)`; add slog + panic recovery |
| `web/server.go` | Replace `log.Printf` with `slog.Info`/`slog.Error` (signature unchanged) |
| `go.mod` | Add `gopkg.in/yaml.v3` dependency |

---

## Task 1: Configuration Package

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`
- Create: `config.yaml`
- Modify: `go.mod`

- [ ] **Step 1: Write failing test for config loading**

```go
// config/config_test.go
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
	// Unset sub-structs should have defaults
	if cfg.Combat.FleeChance != 0.50 {
		t.Errorf("expected default flee chance 0.50, got %f", cfg.Combat.FleeChance)
	}
	if cfg.Dungeon.MapWidth != 60 {
		t.Errorf("expected default map width 60, got %d", cfg.Dungeon.MapWidth)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./config/ -v`
Expected: FAIL — package `config` not found

- [ ] **Step 3: Implement config package**

```go
// config/config.go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Game    GameConfig    `yaml:"game"`
	Combat  CombatConfig  `yaml:"combat"`
	Dungeon DungeonConfig `yaml:"dungeon"`
	Backup  BackupConfig  `yaml:"backup"`
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
		Server: ServerConfig{
			SSHPort:  2222,
			HTTPPort: 8080,
		},
		Game: GameConfig{
			TurnTimeoutSec: 5,
			MaxPlayers:     4,
			MaxFloors:      20,
			CoopBonus:      0.10,
			InventoryLimit: 10,
			SkillUses:      3,
		},
		Combat: CombatConfig{
			FleeChance:      0.50,
			MonsterScaling:  1.15,
			SoloHPReduction: 0.50,
		},
		Dungeon: DungeonConfig{
			MapWidth:  60,
			MapHeight: 20,
			MinRooms:  5,
			MaxRooms:  8,
		},
		Backup: BackupConfig{
			IntervalMin: 60,
			Dir:         "./data/backup",
		},
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
```

- [ ] **Step 4: Add yaml dependency**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go get gopkg.in/yaml.v3`

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./config/ -v`
Expected: PASS (both TestLoadDefaults and TestLoadFromFile)

- [ ] **Step 6: Create default config.yaml**

```yaml
# config.yaml — Catacombs server configuration
server:
  ssh_port: 2222
  http_port: 8080

game:
  turn_timeout_sec: 5
  max_players: 4
  max_floors: 20
  coop_bonus: 0.10
  inventory_limit: 10
  skill_uses: 3

combat:
  flee_chance: 0.50
  monster_scaling: 1.15
  solo_hp_reduction: 0.50

dungeon:
  map_width: 60
  map_height: 20
  min_rooms: 5
  max_rooms: 8

backup:
  interval_min: 60
  dir: "./data/backup"
```

- [ ] **Step 7: Commit**

```bash
git add config/ config.yaml go.mod go.sum
git commit -m "feat: add config package with YAML loading and defaults"
```

---

## Task 2: Wire Config Into main.go and Server

**Files:**
- Modify: `main.go`
- Modify: `server/ssh.go`
- Modify: `game/lobby.go`

**Note:** `web/server.go`'s `Start(addr string, sshPort int)` signature does NOT need changing — the plan passes `cfg.Server.SSHPort` as the sshPort argument.

- [ ] **Step 1: Update main.go to load config and pass to server**

```go
// main.go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/web"
)

func main() {
	os.MkdirAll("data", 0755)

	cfg, err := config.Load("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			cfg, _ = config.Load("")
			log.Println("No config.yaml found, using defaults")
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

	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
		if err := web.Start(addr, cfg.Server.SSHPort); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	log.Printf("Catacombs server starting — SSH :%d, Web :%d", cfg.Server.SSHPort, cfg.Server.HTTPPort)
	sshAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.SSHPort)
	if err := server.Start(sshAddr, lobby, db); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Update server.Start() signature**

Read `server/ssh.go` first. Change signature from:
```go
func Start(host string, port int, lobby *game.Lobby, db *store.DB) error
```
to:
```go
func Start(addr string, lobby *game.Lobby, db *store.DB) error
```
Replace internal `fmt.Sprintf("%s:%d", host, port)` with direct use of `addr`.

- [ ] **Step 3: Update game.NewLobby() to accept config**

Read `game/lobby.go` first. Add `cfg *config.Config` field to `Lobby` struct. Change `NewLobby()` to `NewLobby(cfg *config.Config)`. The config will propagate: `Lobby.cfg` → `GameSession.cfg` when sessions are created.

Find where `NewGameSession` is called (likely in `ui/model.go` during class select). The config is accessible via `ctx.Lobby.Cfg()` or a public field.

- [ ] **Step 4: Run build and all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go build . && go test ./...`
Expected: Build succeeds, all tests pass. Fix any tests that call `NewLobby()` without config by passing `defaults()` or a test config.

- [ ] **Step 5: Commit**

```bash
git add main.go server/ssh.go game/lobby.go game/lobby_test.go
git commit -m "feat: wire config into main, server, and lobby"
```

---

## Task 3: Replace All Hardcoded Constants with Config

**Files:**
- Modify: `game/session.go` (add cfg field)
- Modify: `game/turn.go` (TurnTimeout → cfg, MaxFloors → cfg)
- Modify: `game/event.go` (SkillUses, solo reduction, inventory limit)
- Modify: `game/lobby.go` (max players in JoinRoom)
- Modify: `combat/combat.go` (flee chance, coop bonus)
- Modify: `entity/monster.go` (monster scaling)

- [ ] **Step 1: Add config to GameSession**

In `game/session.go`, add `cfg *config.Config` to the `GameSession` struct. Update the constructor to accept and store it. The lobby should pass its config when creating sessions.

- [ ] **Step 2: Replace TurnTimeout in turn.go**

Remove `const TurnTimeout = 5 * time.Second` from `game/turn.go:13`. In `RunTurn()`, compute timeout from config:
```go
timeout := time.Duration(s.cfg.Game.TurnTimeoutSec) * time.Second
```

- [ ] **Step 3: Replace MaxFloors in turn.go**

In `game/turn.go` `advanceFloor()` (~line 290), replace `if s.state.FloorNum >= 20` with `if s.state.FloorNum >= s.cfg.Game.MaxFloors`.

- [ ] **Step 4: Replace skill uses in event.go**

In `game/event.go`, `spawnMonsters()` and `spawnBoss()` set `p.SkillUses = 3`. Replace with `p.SkillUses = s.cfg.Game.SkillUses`.

- [ ] **Step 5: Replace solo reduction in event.go**

In `game/event.go`, solo mode does `m.HP /= 2` and `m.DEF /= 2`. Replace with:
```go
reduction := s.cfg.Combat.SoloHPReduction
m.HP = int(float64(m.HP) * reduction)
m.DEF = int(float64(m.DEF) * reduction)
```

- [ ] **Step 6: Replace inventory limit in session.go and event.go**

In `game/session.go` `BuyItem()` (~line 343): replace `len(p.Inventory) >= 10` with `len(p.Inventory) >= s.cfg.Game.InventoryLimit`.

In `game/event.go` `grantTreasure()` (~line 143): replace `len(p.Inventory) >= 10` with `len(p.Inventory) >= s.cfg.Game.InventoryLimit`.

- [ ] **Step 7: Replace max players in lobby.go**

In `game/lobby.go` `JoinRoom()` (~line 143): replace `len(room.Players) >= 4` with `len(room.Players) >= l.cfg.Game.MaxPlayers`.

- [ ] **Step 8: Replace flee chance in combat.go**

Change `combat.AttemptFlee()` signature to accept chance:
```go
func AttemptFlee(fleeChance float64) bool {
	return rand.Float64() < fleeChance
}
```
Update all callers to pass `s.cfg.Combat.FleeChance`.

- [ ] **Step 9: Replace coop bonus in combat.go**

In `combat.ResolveAttacks()`, the coop multiplier is `* 1.10` (~line 66). Change the function to accept `coopBonus float64` parameter:
```go
func ResolveAttacks(intents []AttackIntent, monsters []*entity.Monster, coopBonus float64) []AttackResult
```
Replace `* 1.10` with `* (1.0 + coopBonus)`. Update all callers to pass `s.cfg.Game.CoopBonus`.

- [ ] **Step 10: Replace monster scaling in entity/monster.go**

In `entity/monster.go` `NewMonster()` (~line 61), change `math.Pow(1.15, ...)` to accept a scaling parameter. Change signature:
```go
func NewMonster(mt MonsterType, floor int, scaling float64) *Monster
```
Update all callers in `game/event.go` to pass `s.cfg.Combat.MonsterScaling`.

- [ ] **Step 11: Run all tests and fix breakages**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go build . && go test ./...`

Expected: Some tests will break because function signatures changed. Fix:
- `entity/monster_test.go`: pass `1.15` as scaling param to `NewMonster`
- `combat/combat_test.go`: pass `0.10` as coopBonus to `ResolveAttacks`
- `game/session_test.go`: create sessions with test config
- `game/lobby_test.go`: create lobby with test config
- `ui/model_test.go`: create lobby/session with test config

- [ ] **Step 12: Commit**

```bash
git add game/ entity/ combat/ ui/model_test.go
git commit -m "feat: replace all hardcoded constants with config values"
```

---

## Task 4: Structured Logging

**Files:**
- Modify: `main.go` (initialize slog)
- Modify: `server/ssh.go` (slog + panic recovery)
- Modify: `web/server.go` (slog)
- Modify: `game/session.go` (game event logging)
- Modify: `game/lobby.go` (room event logging)

- [ ] **Step 1: Initialize slog in main.go**

Add after config loading in `main.go`:

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))
slog.SetDefault(logger)
```

Replace `log.Printf`/`log.Println`/`log.Fatalf` in `main.go` with `slog.Info`/`slog.Error`. Keep `log.Fatalf` for fatal errors (slog doesn't have Fatal).

- [ ] **Step 2: Add panic recovery to SSH session handler**

Read `server/ssh.go` to find the session handler function. Add at the top of the handler:

```go
defer func() {
	if r := recover(); r != nil {
		slog.Error("session panic recovered", "error", r, "fingerprint", fingerprint)
	}
}()
```

Replace all `log.Printf` in `server/ssh.go` with `slog.Info`/`slog.Error`.

- [ ] **Step 3: Add logging to game/lobby.go**

Add slog calls at key points:
- Room created: `slog.Info("room created", "code", code)`
- Player joined: `slog.Info("player joined", "room", code, "player", name)`
- Game started: `slog.Info("game started", "room", code, "players", playerCount)`

- [ ] **Step 4: Add logging to game/session.go**

Add slog calls:
- Game over: `slog.Info("game over", "floor", s.state.FloorNum, "victory", s.state.Victory)`
- Player removed for inactivity: `slog.Warn("player inactive removed", "fingerprint", fp)`

- [ ] **Step 5: Replace log calls in web/server.go**

Replace `log.Printf` with `slog.Info`/`slog.Error` for WebSocket connection events.

- [ ] **Step 6: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go build . && go test ./...`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add main.go server/ web/ game/
git commit -m "feat: add structured logging with log/slog and panic recovery"
```

---

## Task 5: UI Screen Interface and Context

**Files:**
- Create: `ui/context.go`
- Create: `ui/screen.go`

- [ ] **Step 1: Define Screen interface**

```go
// ui/screen.go
package ui

import tea "github.com/charmbracelet/bubbletea"

// Screen represents an independent screen with its own Update and View logic.
// Update returns the next Screen (can return itself or a different screen for transitions)
// plus a tea.Cmd for async operations.
type Screen interface {
	Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd)
	View(ctx *Context) string
}
```

- [ ] **Step 2: Define Context struct**

Read `ui/model.go` to identify all shared state. Create:

```go
// ui/context.go
package ui

import (
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// Context holds shared state accessible to all screens.
type Context struct {
	Width       int
	Height      int
	Fingerprint string
	PlayerName  string

	Lobby    *game.Lobby
	Store    *store.DB
	Session  *game.GameSession
	RoomCode string
}
```

Note: Screen transitions are handled by returning a new `Screen` from `Update()`, not via context flags. This is cleaner and more idiomatic Bubble Tea.

- [ ] **Step 3: Run build to verify compilation**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go build ./ui/`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add ui/context.go ui/screen.go
git commit -m "feat: add Screen interface and Context for UI architecture"
```

---

## Task 6: Extract Title Screen

**Files:**
- Modify: `ui/title.go`
- Modify: `ui/model.go`

- [ ] **Step 1: Read current title update logic**

Read `ui/model.go` — find `updateTitle()` method. Note all `m.xxx` fields it reads/writes.

- [ ] **Step 2: Create TitleScreen in title.go**

Add to `ui/title.go`:

```go
type TitleScreen struct {
	cursor int
}

func NewTitleScreen() *TitleScreen {
	return &TitleScreen{}
}

func (s *TitleScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	// Move logic from updateTitle() here
	// For screen transitions: return NewXxxScreen(), cmd
	// For staying on same screen: return s, cmd
}

func (s *TitleScreen) View(ctx *Context) string {
	// Move existing titleView() logic, replace m.xxx with ctx.xxx and s.cursor
}
```

- [ ] **Step 3: Add currentScreen field to Model and delegate**

In `model.go`:
1. Add `currentScreen Screen` and `ctx *Context` fields to `Model`
2. Initialize in `Init()` or constructor: `m.currentScreen = NewTitleScreen()`, `m.ctx = &Context{...}`
3. In `Update()`: handle `tea.WindowSizeMsg` to update ctx, then delegate `m.currentScreen.Update(msg, m.ctx)`
4. If returned Screen differs from current, swap it
5. In `View()`: `return m.currentScreen.View(m.ctx)`

Keep the old switch-based code for screens not yet extracted (will be removed incrementally).

- [ ] **Step 4: Run tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./ui/ -v`
Expected: All existing tests pass

- [ ] **Step 5: Commit**

```bash
git add ui/title.go ui/model.go
git commit -m "refactor: extract TitleScreen from model.go"
```

---

## Task 7: Extract Nickname Screen

**Files:**
- Modify: `ui/nickname_view.go`
- Modify: `ui/model.go`

- [ ] **Step 1: Read updateNickname() in model.go**

Read `ui/model.go` — find `updateNickname()`. Note fields: `m.nicknameInput`, `m.playerName`, `m.fingerprint`, `m.store`.

- [ ] **Step 2: Create NicknameScreen**

```go
type NicknameScreen struct {
	input string
}
```

Move `updateNickname()` logic into `Update()`. Move nickname view rendering into `View()`. When nickname is confirmed, set `ctx.PlayerName` and return `NewLobbyScreen()` (or the next appropriate screen).

- [ ] **Step 3: Delegate in model.go, remove old code**

- [ ] **Step 4: Run tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./ui/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add ui/nickname_view.go ui/model.go
git commit -m "refactor: extract NicknameScreen from model.go"
```

---

## Task 8: Extract Lobby Screen

**Files:**
- Modify: `ui/lobby_view.go`
- Modify: `ui/model.go`

- [ ] **Step 1: Read updateLobby() and lobbyState**

Read `ui/model.go` for `updateLobby()` and `ui/lobby_view.go` for the existing `lobbyState` struct (10 fields).

- [ ] **Step 2: Create LobbyScreen**

Embed or replace `lobbyState` fields into `LobbyScreen` struct. Move `updateLobby()` logic into `Update()`. The view rendering functions already in `lobby_view.go` should reference `LobbyScreen` fields.

- [ ] **Step 3: Delegate in model.go, remove old code**

- [ ] **Step 4: Run tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./ui/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add ui/lobby_view.go ui/model.go
git commit -m "refactor: extract LobbyScreen from model.go"
```

---

## Task 9: Extract Class Select Screen

**Files:**
- Modify: `ui/class_view.go`
- Modify: `ui/model.go`

- [ ] **Step 1: Read and extract**

Read `ui/model.go` for `updateClassSelect()`. Create `ClassSelectScreen` with `cursor int` field. Move logic. This is where `NewGameSession` is called — use `ctx.Lobby.Cfg` to get config for session creation.

- [ ] **Step 2: Delegate and test**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./ui/ -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add ui/class_view.go ui/model.go
git commit -m "refactor: extract ClassSelectScreen from model.go"
```

---

## Task 10: Extract Game Screen (Largest)

**Files:**
- Modify: `ui/game_view.go`
- Modify: `ui/model.go`

- [ ] **Step 1: Read updateGame() thoroughly**

Read `ui/model.go` for `updateGame()` — the most complex screen. Note: exploration navigation (Up/Down/Enter), combat actions (1-5 keys, Tab), chat mode (`/` toggle), pollState, getNeighbors, achievement checking (lines with `m.store.UnlockAchievement`).

- [ ] **Step 2: Create GameScreen struct**

```go
type GameScreen struct {
	gameState    game.GameState
	targetCursor int
	moveCursor   int
	chatting     bool
	chatInput    string
}
```

Move `updateGame()` logic into `Update()`. Move `pollState()` and `getNeighbors()` as methods on `GameScreen`. Move achievement checking logic into `GameScreen.Update()`.

- [ ] **Step 3: Delegate in model.go, remove old code**

- [ ] **Step 4: Run tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./ui/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add ui/game_view.go ui/model.go
git commit -m "refactor: extract GameScreen from model.go"
```

---

## Task 11: Extract Remaining Screens (Shop, Result, Help, Stats, Achievements, Leaderboard)

**Files:**
- Modify: `ui/shop_view.go`, `ui/result_view.go`, `ui/help_view.go`, `ui/stats_view.go`, `ui/achievements_view.go`, `ui/leaderboard_view.go`
- Modify: `ui/model.go`

- [ ] **Step 1: Extract ShopScreen**

Read `ui/model.go` for `updateShop()`. Create `ShopScreen` with `shopMsg string` field. Move logic.

- [ ] **Step 2: Extract ResultScreen**

Read `ui/model.go` for `updateResult()`. Create `ResultScreen` with `rankingSaved bool` field. Move logic.

- [ ] **Step 3: Extract HelpScreen, StatsScreen, AchievementsScreen, LeaderboardScreen**

These are simple screens that handle Esc to go back. Create minimal structs for each.

- [ ] **Step 4: Clean up model.go**

After all screens extracted, `model.go` should only:
1. Define `Model` struct with `currentScreen Screen`, `ctx *Context`
2. `Init()`: create initial screen and context
3. `Update()`: handle `tea.WindowSizeMsg`, delegate to `m.currentScreen.Update(msg, m.ctx)`, swap screen if needed
4. `View()`: `return m.currentScreen.View(m.ctx)`

Remove all `updateXxx()` methods, `screen` enum (if no longer needed), and per-screen state fields from `Model`.

- [ ] **Step 5: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add ui/
git commit -m "refactor: extract all remaining screens, model.go is now a thin router"
```

---

## Task 12: Emote System

**Files:**
- Create: `game/emote.go`
- Create: `game/emote_test.go`
- Modify: `game/session.go` (SendChat emote integration)

- [ ] **Step 1: Write failing test for emote parsing**

```go
// game/emote_test.go
package game

import "testing"

func TestParseEmote(t *testing.T) {
	tests := []struct {
		input    string
		isEmote  bool
		expected string
	}{
		{"/hi", true, "👋 waves hello!"},
		{"/gg", true, "🎉 says GG!"},
		{"/go", true, "⚔️ says Let's go!"},
		{"/wait", true, "✋ says Wait!"},
		{"/help", true, "🆘 calls for help!"},
		{"/unknown", false, ""},
		{"hello", false, ""},
		{"", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := ParseEmote(tt.input)
			if ok != tt.isEmote {
				t.Errorf("ParseEmote(%q) isEmote = %v, want %v", tt.input, ok, tt.isEmote)
			}
			if ok && result != tt.expected {
				t.Errorf("ParseEmote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./game/ -run TestParseEmote -v`
Expected: FAIL — `ParseEmote` undefined

- [ ] **Step 3: Implement emote system**

```go
// game/emote.go
package game

var emotes = map[string]string{
	"/hi":   "👋 waves hello!",
	"/gg":   "🎉 says GG!",
	"/go":   "⚔️ says Let's go!",
	"/wait": "✋ says Wait!",
	"/help": "🆘 calls for help!",
}

func ParseEmote(input string) (string, bool) {
	if input == "" {
		return "", false
	}
	text, ok := emotes[input]
	return text, ok
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./game/ -run TestParseEmote -v`
Expected: PASS

- [ ] **Step 5: Write integration test for SendChat with emotes**

Add to `game/emote_test.go` or the existing `game/session_test.go`:

```go
func TestSendChatEmote(t *testing.T) {
	// Create a test session (use test config)
	// Call SendChat("Alice", "/hi")
	// Verify the log contains "✨ Alice 👋 waves hello!"
	// Call SendChat("Bob", "normal message")
	// Verify the log contains "[Bob] normal message"
}
```

This confirms the existing `TestSendChat` still passes (regression) and emotes work end-to-end.

- [ ] **Step 6: Integrate emote into SendChat**

Modify `game/session.go` `SendChat()`:

```go
func (s *GameSession) SendChat(playerName, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if emoteText, ok := ParseEmote(message); ok {
		s.addLog(fmt.Sprintf("✨ %s %s", playerName, emoteText))
	} else {
		s.addLog(fmt.Sprintf("[%s] %s", playerName, message))
	}
}
```

- [ ] **Step 7: Run all tests (including existing TestSendChat as regression)**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./game/ -v`
Expected: ALL PASS (TestParseEmote, TestSendChat, TestSendChatEmote)

- [ ] **Step 8: Commit**

```bash
git add game/emote.go game/emote_test.go game/session.go
git commit -m "feat: add chat emote system (/hi, /gg, /go, /wait, /help)"
```

---

## Task 13: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Run go vet**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go vet ./...`
Expected: No issues

- [ ] **Step 3: Build and verify**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go build -o catacombs .`
Expected: Build succeeds

- [ ] **Step 4: Verify model.go is now a thin router**

Read `ui/model.go` and confirm:
- `Update()` only handles window resize and delegates to `currentScreen.Update()`
- `View()` only delegates to `currentScreen.View()`
- All per-screen update functions (`updateTitle`, `updateLobby`, etc.) are removed
- Per-screen state fields are removed from `Model`

- [ ] **Step 5: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore: phase 1 complete — foundation verified"
```
