# Phase 3: Retention Systems Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add retention mechanics (daily challenges, meta progression, difficulty modes) that give players reasons to keep coming back.

**Architecture:** Extend the BoltDB store with 4 new buckets (daily_runs, unlocks, titles, codex). Daily challenges use the Phase 2 seed-based generator with a date-derived seed. Unlock/title/codex systems follow the existing achievements pattern (key-value pairs per player). Hard mode and weekly mutations modify game config at session creation. New UI screens (codex_view) follow the Phase 1 Screen interface pattern.

**Tech Stack:** Go 1.25.1, BoltDB, `math/rand`, `crypto/sha256` for date→seed hashing

**Module path:** `github.com/tolelom/catacombs`

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `store/daily.go` | DailyRecord struct; SaveDaily, GetDailyLeaderboard, GetStreak methods |
| `store/daily_test.go` | Daily record persistence tests |
| `store/unlocks.go` | Unlock definitions; CheckUnlock, GetUnlocks methods |
| `store/unlocks_test.go` | Unlock condition tests |
| `store/titles.go` | Title definitions; EarnTitle, SetActiveTitle, GetTitles methods |
| `store/titles_test.go` | Title persistence tests |
| `store/codex.go` | Codex struct; RecordMonster/Item/Event, GetCodex methods |
| `store/codex_test.go` | Codex tracking tests |
| `game/daily.go` | DailySeed generation; CreateDailySession helper |
| `game/daily_test.go` | Seed determinism tests |
| `game/mutation.go` | Mutation definitions; GetWeeklyMutation, ApplyMutation |
| `game/mutation_test.go` | Mutation application tests |
| `ui/codex_view.go` | CodexScreen implementing Screen interface |

### Modified Files
| File | Changes |
|------|---------|
| `store/db.go` | Create 4 new buckets in Open(); use cfg.Game.MaxFloors in GetStats victory check |
| `config/config.go` | Add DifficultyConfig (HardMode multipliers) |
| `config.yaml` | Add difficulty section |
| `game/session.go` | Add DailyMode/HardMode/Mutation fields to GameSession; seed-based generation for daily; codex recording hooks |
| `game/event.go` | Record codex entries on monster spawn, treasure, events |
| `game/turn.go` | Record codex on monster kills; apply hard mode multipliers |
| `ui/lobby_view.go` | Add daily challenge button; show hard mode option if unlocked |
| `ui/game_view.go` | Show daily/hard mode indicators in HUD; save daily record on game over |
| `ui/title.go` | Show active title next to player name on title screen |
| `ui/leaderboard_view.go` | Add daily leaderboard tab |

---

## Task 1: Daily Challenge Seed System

**Files:**
- Create: `game/daily.go`
- Create: `game/daily_test.go`

- [ ] **Step 1: Write determinism test**

```go
// game/daily_test.go
package game

import (
	"math/rand"
	"testing"

	"github.com/tolelom/catacombs/dungeon"
)

func TestDailySeed(t *testing.T) {
	seed1 := DailySeed("2026-03-25")
	seed2 := DailySeed("2026-03-25")
	if seed1 != seed2 {
		t.Errorf("same date should produce same seed: %d vs %d", seed1, seed2)
	}
	seed3 := DailySeed("2026-03-26")
	if seed1 == seed3 {
		t.Error("different dates should produce different seeds")
	}
}

func TestDailyFloorDeterminism(t *testing.T) {
	seed := DailySeed("2026-03-25")
	rng1 := rand.New(rand.NewSource(seed))
	rng2 := rand.New(rand.NewSource(seed))
	f1 := dungeon.GenerateFloor(1, rng1)
	f2 := dungeon.GenerateFloor(1, rng2)
	if len(f1.Rooms) != len(f2.Rooms) {
		t.Fatal("daily floors should be identical")
	}
	for i := range f1.Rooms {
		if f1.Rooms[i].Type != f2.Rooms[i].Type {
			t.Errorf("room %d type differs", i)
		}
	}
}
```

- [ ] **Step 2: Implement DailySeed**

```go
// game/daily.go
package game

import (
	"crypto/sha256"
	"encoding/binary"
)

// DailySeed returns a deterministic seed for the given date string (YYYY-MM-DD).
func DailySeed(date string) int64 {
	h := sha256.Sum256([]byte("catacombs:" + date))
	return int64(binary.BigEndian.Uint64(h[:8]))
}
```

- [ ] **Step 3: Run tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./game/ -run TestDaily -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add game/daily.go game/daily_test.go
git commit -m "feat: add daily challenge seed generation"
```

---

## Task 2: Daily Record Store

**Files:**
- Create: `store/daily.go`
- Create: `store/daily_test.go`
- Modify: `store/db.go` (add bucket creation)

- [ ] **Step 1: Write tests**

```go
// store/daily_test.go
package store

import (
	"os"
	"testing"
)

func TestSaveAndGetDaily(t *testing.T) {
	f, _ := os.CreateTemp("", "daily-*.db")
	f.Close()
	defer os.Remove(f.Name())

	db, err := Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.SaveDaily(DailyRecord{
		Date: "2026-03-25", Player: "Alice", PlayerName: "Alice",
		FloorReached: 10, GoldEarned: 200,
	})
	if err != nil {
		t.Fatal(err)
	}

	records, err := db.GetDailyLeaderboard("2026-03-25", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].FloorReached != 10 {
		t.Errorf("expected floor 10, got %d", records[0].FloorReached)
	}
}

func TestDailyStreak(t *testing.T) {
	f, _ := os.CreateTemp("", "streak-*.db")
	f.Close()
	defer os.Remove(f.Name())

	db, _ := Open(f.Name())
	defer db.Close()

	db.SaveDaily(DailyRecord{Date: "2026-03-23", Player: "fp1", PlayerName: "A", FloorReached: 5, GoldEarned: 50})
	db.SaveDaily(DailyRecord{Date: "2026-03-24", Player: "fp1", PlayerName: "A", FloorReached: 8, GoldEarned: 100})
	db.SaveDaily(DailyRecord{Date: "2026-03-25", Player: "fp1", PlayerName: "A", FloorReached: 12, GoldEarned: 150})

	streak := db.GetStreak("fp1", "2026-03-25")
	if streak != 3 {
		t.Errorf("expected streak 3, got %d", streak)
	}
}
```

- [ ] **Step 2: Implement daily store**

```go
// store/daily.go
package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucketDailyRuns = []byte("daily_runs")

type DailyRecord struct {
	Date         string `json:"date"`
	Player       string `json:"player"`       // fingerprint
	PlayerName   string `json:"player_name"`
	FloorReached int    `json:"floor_reached"`
	GoldEarned   int    `json:"gold_earned"`
}

func (d *DB) SaveDaily(record DailyRecord) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		key := []byte(record.Date + ":" + record.Player)
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func (d *DB) GetDailyLeaderboard(date string, limit int) ([]DailyRecord, error) {
	var records []DailyRecord
	prefix := []byte(date + ":")
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var r DailyRecord
			if json.Unmarshal(v, &r) == nil {
				records = append(records, r)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].FloorReached != records[j].FloorReached {
			return records[i].FloorReached > records[j].FloorReached
		}
		return records[i].GoldEarned > records[j].GoldEarned
	})
	if len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func (d *DB) GetStreak(fingerprint, currentDate string) int {
	streak := 0
	date, err := time.Parse("2006-01-02", currentDate)
	if err != nil {
		return 0
	}
	for i := 0; i < 365; i++ {
		checkDate := date.AddDate(0, 0, -i).Format("2006-01-02")
		key := []byte(checkDate + ":" + fingerprint)
		found := false
		d.db.View(func(tx *bolt.Tx) error {
			if tx.Bucket(bucketDailyRuns).Get(key) != nil {
				found = true
			}
			return nil
		})
		if found {
			streak++
		} else {
			break
		}
	}
	return streak
}
```

In `store/db.go` `Open()`, add bucket creation:
```go
if _, err := tx.CreateBucketIfNotExists(bucketDailyRuns); err != nil {
	return err
}
```

- [ ] **Step 3: Run tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./store/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add store/daily.go store/daily_test.go store/db.go
git commit -m "feat: add daily challenge record storage and leaderboard"
```

---

## Task 3: Unlock System

**Files:**
- Create: `store/unlocks.go`
- Create: `store/unlocks_test.go`
- Modify: `store/db.go` (add bucket)

- [ ] **Step 1: Implement unlock system**

Follow the `achievements.go` pattern. Define 3 unlocks:
- `"fifth_class"`: "Clear floor 10+" → unlocks 5th class (future)
- `"hard_mode"`: "Clear with 3+ players" → unlocks hard mode
- `"mutations"`: "Clear floor 20" → unlocks weekly mutations

```go
// store/unlocks.go
package store

import bolt "go.etcd.io/bbolt"

var bucketUnlocks = []byte("unlocks")

type UnlockDef struct {
	ID          string
	Name        string
	Description string
}

var UnlockDefs = []UnlockDef{
	{ID: "fifth_class", Name: "New Recruit", Description: "Clear floor 10 to unlock a new class"},
	{ID: "hard_mode", Name: "Hardened", Description: "Complete a run with 3+ players to unlock Hard Mode"},
	{ID: "mutations", Name: "Chaos Unleashed", Description: "Conquer floor 20 to unlock Weekly Mutations"},
}

func (d *DB) UnlockContent(fingerprint, unlockID string) (bool, error) {
	key := []byte(fingerprint + ":" + unlockID)
	alreadyUnlocked := false
	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUnlocks)
		if b.Get(key) != nil {
			alreadyUnlocked = true
			return nil
		}
		return b.Put(key, []byte("1"))
	})
	return !alreadyUnlocked, err
}

func (d *DB) IsUnlocked(fingerprint, unlockID string) bool {
	key := []byte(fingerprint + ":" + unlockID)
	found := false
	d.db.View(func(tx *bolt.Tx) error {
		if tx.Bucket(bucketUnlocks).Get(key) != nil {
			found = true
		}
		return nil
	})
	return found
}

func (d *DB) GetUnlocks(fingerprint string) []UnlockDef {
	result := make([]UnlockDef, len(UnlockDefs))
	copy(result, UnlockDefs)
	// Mark unlocked ones (reuse the struct, no Unlocked field — caller checks IsUnlocked)
	return result
}
```

Add `bucketUnlocks` creation in `store/db.go` `Open()`.

- [ ] **Step 2: Write tests and verify**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./store/ -v`

- [ ] **Step 3: Commit**

```bash
git add store/unlocks.go store/unlocks_test.go store/db.go
git commit -m "feat: add unlock system with 3 unlockable contents"
```

---

## Task 4: Player Titles

**Files:**
- Create: `store/titles.go`
- Create: `store/titles_test.go`
- Modify: `store/db.go` (add bucket)

- [ ] **Step 1: Implement titles**

```go
// store/titles.go
package store

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

var bucketTitles = []byte("titles")

type TitleDef struct {
	ID   string
	Name string
}

var TitleDefs = []TitleDef{
	{ID: "novice", Name: "Novice"},
	{ID: "explorer", Name: "Explorer"},          // reach floor 5
	{ID: "veteran", Name: "Veteran"},             // reach floor 10
	{ID: "champion", Name: "Champion"},           // reach floor 20
	{ID: "gold_king", Name: "Gold King"},          // earn 500+ gold in one run
	{ID: "team_player", Name: "Team Player"},      // complete 5 multiplayer runs
	{ID: "survivor", Name: "Survivor"},            // complete a run without dying
}

type PlayerTitleData struct {
	ActiveTitle string   `json:"active_title"`
	Earned      []string `json:"earned"`
}

func (d *DB) EarnTitle(fingerprint, titleID string) (bool, error) {
	data := d.loadTitleData(fingerprint)
	for _, t := range data.Earned {
		if t == titleID {
			return false, nil // already earned
		}
	}
	data.Earned = append(data.Earned, titleID)
	if data.ActiveTitle == "" {
		data.ActiveTitle = titleID
	}
	return true, d.saveTitleData(fingerprint, data)
}

func (d *DB) SetActiveTitle(fingerprint, titleID string) error {
	data := d.loadTitleData(fingerprint)
	data.ActiveTitle = titleID
	return d.saveTitleData(fingerprint, data)
}

func (d *DB) GetTitleData(fingerprint string) PlayerTitleData {
	return d.loadTitleData(fingerprint)
}

func (d *DB) loadTitleData(fingerprint string) PlayerTitleData {
	var data PlayerTitleData
	d.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketTitles).Get([]byte(fingerprint))
		if v != nil {
			json.Unmarshal(v, &data)
		}
		return nil
	})
	return data
}

func (d *DB) saveTitleData(fingerprint string, data PlayerTitleData) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		raw, _ := json.Marshal(data)
		return tx.Bucket(bucketTitles).Put([]byte(fingerprint), raw)
	})
}
```

Add `bucketTitles` to `store/db.go` `Open()`.

- [ ] **Step 2: Write tests and verify**
- [ ] **Step 3: Commit**

```bash
git add store/titles.go store/titles_test.go store/db.go
git commit -m "feat: add player title system with 7 titles"
```

---

## Task 5: Codex System

**Files:**
- Create: `store/codex.go`
- Create: `store/codex_test.go`
- Modify: `store/db.go` (add bucket)
- Modify: `game/event.go` (record monster/item/event encounters)

- [ ] **Step 1: Implement codex store**

```go
// store/codex.go
package store

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

var bucketCodex = []byte("codex")

type Codex struct {
	Monsters map[string]bool `json:"monsters"`
	Items    map[string]bool `json:"items"`
	Events   map[string]bool `json:"events"`
}

func (d *DB) RecordCodexEntry(fingerprint, category, id string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCodex)
		var codex Codex
		if v := b.Get([]byte(fingerprint)); v != nil {
			json.Unmarshal(v, &codex)
		}
		if codex.Monsters == nil {
			codex.Monsters = make(map[string]bool)
		}
		if codex.Items == nil {
			codex.Items = make(map[string]bool)
		}
		if codex.Events == nil {
			codex.Events = make(map[string]bool)
		}
		switch category {
		case "monster":
			codex.Monsters[id] = true
		case "item":
			codex.Items[id] = true
		case "event":
			codex.Events[id] = true
		}
		raw, _ := json.Marshal(codex)
		return b.Put([]byte(fingerprint), raw)
	})
}

func (d *DB) GetCodex(fingerprint string) Codex {
	var codex Codex
	d.db.View(func(tx *bolt.Tx) error {
		if v := tx.Bucket(bucketCodex).Get([]byte(fingerprint)); v != nil {
			json.Unmarshal(v, &codex)
		}
		return nil
	})
	if codex.Monsters == nil {
		codex.Monsters = make(map[string]bool)
	}
	if codex.Items == nil {
		codex.Items = make(map[string]bool)
	}
	if codex.Events == nil {
		codex.Events = make(map[string]bool)
	}
	return codex
}
```

Add `bucketCodex` to `store/db.go`. The codex recording hooks in `game/event.go` will be added — but since GameSession doesn't have a DB reference, recording will happen from the UI layer (GameScreen) which has access to `ctx.Store`. Add codex recording in `ui/game_view.go` when the gameState changes (new monsters, items, events).

- [ ] **Step 2: Write tests and verify**
- [ ] **Step 3: Commit**

```bash
git add store/codex.go store/codex_test.go store/db.go
git commit -m "feat: add codex system for monster/item/event tracking"
```

---

## Task 6: Codex UI Screen

**Files:**
- Create: `ui/codex_view.go`
- Modify: `ui/lobby_view.go` or `ui/title.go` (add navigation to codex)

- [ ] **Step 1: Create CodexScreen**

```go
// ui/codex_view.go
package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/store"
)

type CodexScreen struct {
	codex store.Codex
	tab   int // 0=monsters, 1=items, 2=events
}

func NewCodexScreen(ctx *Context) *CodexScreen {
	codex := ctx.Store.GetCodex(ctx.Fingerprint)
	return &CodexScreen{codex: codex}
}
```

Implement `Update` (Tab to switch tabs, Esc to go back) and `View` (show discovered entries with completion percentage per category).

Add a keybinding in the title screen or lobby to open the codex (e.g., `c` key).

- [ ] **Step 2: Run tests**
- [ ] **Step 3: Commit**

```bash
git add ui/codex_view.go ui/title.go
git commit -m "feat: add codex UI screen with completion tracking"
```

---

## Task 7: Hard Mode and Weekly Mutations

**Files:**
- Create: `game/mutation.go`
- Create: `game/mutation_test.go`
- Modify: `config/config.go` (add DifficultyConfig)
- Modify: `game/session.go` (add HardMode/Mutation fields, apply at session start)
- Modify: `ui/lobby_view.go` (show hard mode toggle if unlocked)

- [ ] **Step 1: Add DifficultyConfig**

```go
// Add to config/config.go
type DifficultyConfig struct {
	HardModeMonsterMult float64 `yaml:"hard_mode_monster_mult"` // default: 1.5
	HardModeShopMult    float64 `yaml:"hard_mode_shop_mult"`    // default: 2.0
	HardModeHealMult    float64 `yaml:"hard_mode_heal_mult"`    // default: 0.5
}
```

Add to Config struct and defaults.

- [ ] **Step 2: Implement mutations**

```go
// game/mutation.go
package game

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/tolelom/catacombs/config"
)

type Mutation struct {
	ID          string
	Name        string
	Description string
	Apply       func(cfg *config.GameConfig)
}

var mutations = []Mutation{
	{ID: "no_skills", Name: "Skill Lockout", Description: "Class skills are disabled",
		Apply: func(cfg *config.GameConfig) { cfg.SkillUses = 0 }},
	{ID: "elite_flood", Name: "Elite Flood", Description: "All monsters are elite",
		Apply: func(cfg *config.GameConfig) {}}, // handled in event.go spawn logic
	{ID: "no_shop", Name: "Shop Closed", Description: "Shops are unavailable",
		Apply: func(cfg *config.GameConfig) {}}, // handled in event.go room generation
	{ID: "glass_cannon", Name: "Glass Cannon", Description: "Double ATK, half HP",
		Apply: func(cfg *config.GameConfig) {}}, // handled at player creation
	{ID: "speed_run", Name: "Speed Run", Description: "Turn timeout halved",
		Apply: func(cfg *config.GameConfig) { cfg.TurnTimeoutSec = cfg.TurnTimeoutSec / 2 }},
}

func GetWeeklyMutation() Mutation {
	_, week := time.Now().ISOWeek()
	year, _ := time.Now().ISOWeek()
	h := sha256.Sum256([]byte(fmt.Sprintf("mutation:%d:%d", year, week)))
	idx := int(binary.BigEndian.Uint64(h[:8])) % len(mutations)
	return mutations[idx]
}
```

- [ ] **Step 3: Wire into session**

Add `HardMode bool` and `ActiveMutation *Mutation` fields to `GameSession`. When creating a session for hard mode, apply difficulty multipliers to a cloned config. When creating a mutation session, call `mutation.Apply(&cfg.Game)`.

- [ ] **Step 4: Add hard mode toggle in lobby UI**

In `LobbyScreen`, if `ctx.Store.IsUnlocked(ctx.Fingerprint, "hard_mode")`, show a `[H] Hard Mode` toggle. Store the selection and pass to session creation.

- [ ] **Step 5: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`

- [ ] **Step 6: Commit**

```bash
git add game/mutation.go game/mutation_test.go config/ game/session.go ui/lobby_view.go config.yaml
git commit -m "feat: add hard mode and weekly mutation system"
```

---

## Task 8: Integration — Daily Challenge Flow + Codex Recording

**Files:**
- Modify: `ui/lobby_view.go` (add daily challenge button)
- Modify: `ui/game_view.go` (save daily record on game over; record codex entries)
- Modify: `ui/leaderboard_view.go` (add daily tab)
- Modify: `game/session.go` (add DailyMode field, use daily seed)

- [ ] **Step 1: Add DailyMode to GameSession**

Add `DailyMode bool` and `DailyDate string` fields. When DailyMode, use `DailySeed(date)` to create the RNG for `GenerateFloor`.

- [ ] **Step 2: Add daily challenge in lobby**

In `LobbyScreen`, add `[D] Daily Challenge` button. Creates a solo session with DailyMode=true using today's date.

- [ ] **Step 3: Save daily record on game over**

In `GameScreen`, when game ends and `ctx.Session.DailyMode`, call `ctx.Store.SaveDaily(...)`.

- [ ] **Step 4: Record codex entries**

In `GameScreen.Update()`, when gameState shows new monsters (entering combat), record them to codex. When getting items (treasure/shop), record items. When events trigger, record event IDs.

- [ ] **Step 5: Add daily leaderboard tab**

In `LeaderboardScreen`, add a tab for daily leaderboard using `ctx.Store.GetDailyLeaderboard(today, 20)`.

- [ ] **Step 6: Trigger unlock checks**

In `GameScreen`, after game over:
- Floor >= 10 → unlock "fifth_class"
- Players >= 3 && floor >= 5 → unlock "hard_mode"
- Victory (floor >= maxFloors) → unlock "mutations"

Trigger title earning based on conditions.

- [ ] **Step 7: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`

- [ ] **Step 8: Commit**

```bash
git add ui/ game/session.go
git commit -m "feat: integrate daily challenges, codex recording, and unlock triggers"
```

---

## Task 9: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Run go vet**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go vet ./...`
Expected: No issues

- [ ] **Step 3: Build**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go build -o catacombs.exe .`
Expected: Success

- [ ] **Step 4: Verify new BoltDB buckets**

Confirm `store/db.go` `Open()` creates all 7 buckets: profiles, rankings, achievements, daily_runs, unlocks, titles, codex.

- [ ] **Step 5: Commit if cleanup needed**

```bash
git add -A
git commit -m "chore: phase 3 complete — retention systems verified"
```
