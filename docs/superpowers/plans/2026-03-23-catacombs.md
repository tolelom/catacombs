# Catacombs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Catacombs, a multiplayer co-op roguelike terminal game accessible via SSH, using Go and the Charm stack.

**Architecture:** Single Go binary monolith. Wish handles SSH connections, each spawning a Bubble Tea TUI program. A central GameSession goroutine per party owns all game state and broadcasts updates to connected players via channels. BoltDB stores rankings and player identities.

**Tech Stack:** Go 1.22+, charmbracelet/wish, charmbracelet/bubbletea, charmbracelet/lipgloss, go.etcd.io/bbolt

**Spec:** `docs/superpowers/specs/2026-03-23-catacombs-design.md`

---

## File Structure

```
catacombs/
├── main.go                  # Entrypoint: starts SSH server
├── server/
│   └── ssh.go               # Wish SSH config, session creation, key-based identity
├── game/
│   ├── lobby.go             # Room listing, creation (4-char code), join, start
│   ├── session.go           # GameSession goroutine: state ownership, broadcast, turn loop
│   ├── turn.go              # Turn timer (5s), input collection, action resolution
│   └── event.go             # Room event dispatch: combat, shop, treasure, random events
├── dungeon/
│   ├── generator.go         # BSP dungeon generation per floor
│   ├── room.go              # Room types, content placement, probability table
│   └── fov.go               # Fog of war: visited/visible/hidden states
├── entity/
│   ├── player.go            # Player struct, class stats, inventory, gold
│   ├── monster.go           # Monster definitions, stat scaling, AI targeting
│   └── item.go              # Items: weapons, armor, consumables, relics
├── combat/
│   └── combat.go            # Damage calc, co-op bonus, flee, AoE, boss patterns
├── ui/
│   ├── model.go             # Root Bubble Tea model: state machine, input routing
│   ├── title.go             # Title screen view
│   ├── lobby_view.go        # Lobby view: room list, create/join
│   ├── game_view.go         # Dungeon map + HUD + combat UI rendering
│   └── result_view.go       # Game over / victory screen, ranking display
├── store/
│   └── db.go                # BoltDB: player profiles (key fingerprint → nickname), rankings
├── go.mod
├── go.sum
├── Dockerfile
└── docker-compose.yml
```

---

### Task 1: Project Scaffold & SSH Server

**Files:**
- Create: `main.go`, `server/ssh.go`, `go.mod`, `Dockerfile`, `docker-compose.yml`

- [ ] **Step 1: Initialize Go module**

```bash
cd E:/projects/catacombs
go mod init github.com/tolelom/catacombs
```

- [ ] **Step 2: Install dependencies**

```bash
go get github.com/charmbracelet/wish@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get go.etcd.io/bbolt@latest
```

- [ ] **Step 3: Write SSH server**

`server/ssh.go` — Wish SSH server that accepts all connections, extracts public key fingerprint, creates a Bubble Tea program per session.

```go
package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/tolelom/catacombs/ui"
)

func Start(host string, port int) error {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
		wish.WithHostKeyPath(".ssh/catacombs_host_key"),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return true // accept all keys
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(func(s ssh.Session) (bubbletea.Model, []bubbletea.ProgramOption) {
				pty, _, _ := s.Pty()
				fingerprint := ""
				if s.PublicKey() != nil {
					fingerprint = ssh.FingerprintSHA256(s.PublicKey())
				}
				m := ui.NewModel(pty.Window.Width, pty.Window.Height, fingerprint)
				return m, []bubbletea.ProgramOption{bubbletea.WithAltScreen()}
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("could not create server: %w", err)
	}

	log.Printf("Starting SSH server on %s:%d", host, port)
	return s.ListenAndServe()
}
```

- [ ] **Step 4: Write entrypoint**

`main.go`:

```go
package main

import (
	"log"

	"github.com/tolelom/catacombs/server"
)

func main() {
	if err := server.Start("0.0.0.0", 2222); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Write minimal UI model (placeholder)**

`ui/model.go` — just enough to verify SSH works:

```go
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenTitle screen = iota
)

type Model struct {
	width       int
	height      int
	fingerprint string
	screen      screen
}

func NewModel(width, height int, fingerprint string) Model {
	return Model{
		width:       width,
		height:      height,
		fingerprint: fingerprint,
		screen:      screenTitle,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	return "Welcome to Catacombs!\n\nPress q to quit."
}
```

- [ ] **Step 6: Write Dockerfile**

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o catacombs .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/catacombs .
EXPOSE 2222
CMD ["./catacombs"]
```

- [ ] **Step 7: Write docker-compose.yml**

```yaml
services:
  catacombs:
    build: .
    ports:
      - "2222:2222"
    volumes:
      - catacombs-data:/app/data
    restart: unless-stopped

volumes:
  catacombs-data:
```

- [ ] **Step 8: Build and verify**

```bash
go build -o catacombs .
```

Expected: binary compiles without errors.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat: project scaffold with SSH server and placeholder TUI"
```

---

### Task 2: Entity Definitions (Player, Monster, Item)

**Files:**
- Create: `entity/player.go`, `entity/monster.go`, `entity/item.go`
- Test: `entity/player_test.go`, `entity/monster_test.go`, `entity/item_test.go`

- [ ] **Step 1: Write item types first** (player depends on Item/Relic types)

`entity/item.go`:

```go
package entity

type ItemType int

const (
	ItemWeapon ItemType = iota
	ItemArmor
	ItemConsumable
)

type Item struct {
	Name  string
	Type  ItemType
	Bonus int   // ATK bonus for weapons, DEF bonus for armor, HP restore for consumables
	Price int
}

type RelicEffect int

const (
	RelicHealOnKill RelicEffect = iota
	RelicATKBoost
	RelicDEFBoost
	RelicGoldBoost
)

type Relic struct {
	Name   string
	Effect RelicEffect
	Value  int
	Price  int
}

func NewHPPotion() Item {
	return Item{Name: "HP Potion", Type: ItemConsumable, Bonus: 30, Price: 20}
}
```

- [ ] **Step 2: Write player test**

`entity/player_test.go`:

```go
package entity

import "testing"

func TestNewPlayer(t *testing.T) {
	p := NewPlayer("testuser", ClassWarrior)
	if p.HP != 120 || p.MaxHP != 120 {
		t.Errorf("Warrior HP: got %d, want 120", p.HP)
	}
	if p.ATK != 12 {
		t.Errorf("Warrior ATK: got %d, want 12", p.ATK)
	}
	if p.DEF != 8 {
		t.Errorf("Warrior DEF: got %d, want 8", p.DEF)
	}
	if p.Gold != 0 {
		t.Errorf("Initial gold: got %d, want 0", p.Gold)
	}
}

func TestAllClasses(t *testing.T) {
	tests := []struct {
		class    Class
		hp, atk, def int
	}{
		{ClassWarrior, 120, 12, 8},
		{ClassMage, 70, 20, 3},
		{ClassHealer, 90, 8, 5},
		{ClassRogue, 85, 15, 4},
	}
	for _, tt := range tests {
		p := NewPlayer("test", tt.class)
		if p.HP != tt.hp || p.ATK != tt.atk || p.DEF != tt.def {
			t.Errorf("Class %v: got HP=%d ATK=%d DEF=%d, want HP=%d ATK=%d DEF=%d",
				tt.class, p.HP, p.ATK, p.DEF, tt.hp, tt.atk, tt.def)
		}
	}
}

func TestPlayerTakeDamage(t *testing.T) {
	p := NewPlayer("test", ClassWarrior)
	p.TakeDamage(30)
	if p.HP != 90 {
		t.Errorf("HP after 30 dmg: got %d, want 90", p.HP)
	}
	p.TakeDamage(200)
	if p.HP != 0 {
		t.Errorf("HP should not go below 0: got %d", p.HP)
	}
	if !p.IsDead() {
		t.Error("Player should be dead")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd E:/projects/catacombs && go test ./entity/ -v
```

Expected: FAIL — Player type not defined.

- [ ] **Step 4: Implement player**

`entity/player.go`:

```go
package entity

type Class int

const (
	ClassWarrior Class = iota
	ClassMage
	ClassHealer
	ClassRogue
)

func (c Class) String() string {
	return [...]string{"Warrior", "Mage", "Healer", "Rogue"}[c]
}

type classStats struct {
	HP, ATK, DEF int
}

var classBaseStats = map[Class]classStats{
	ClassWarrior: {120, 12, 8},
	ClassMage:    {70, 20, 3},
	ClassHealer:  {90, 8, 5},
	ClassRogue:   {85, 15, 4},
}

type Player struct {
	Name        string
	Fingerprint string
	Class       Class
	HP, MaxHP   int
	ATK, DEF    int
	Gold        int
	Inventory   []Item
	Relics      []Relic
	Dead        bool
}

func NewPlayer(name string, class Class) *Player {
	stats := classBaseStats[class]
	return &Player{
		Name:  name,
		Class: class,
		HP:    stats.HP,
		MaxHP: stats.HP,
		ATK:   stats.ATK,
		DEF:   stats.DEF,
	}
}

func (p *Player) TakeDamage(dmg int) {
	p.HP -= dmg
	if p.HP <= 0 {
		p.HP = 0
		p.Dead = true
	}
}

func (p *Player) Heal(amount int) {
	p.HP += amount
	if p.HP > p.MaxHP {
		p.HP = p.MaxHP
	}
}

func (p *Player) IsDead() bool {
	return p.Dead
}

func (p *Player) Revive(hpPercent float64) {
	p.Dead = false
	p.HP = int(float64(p.MaxHP) * hpPercent)
	if p.HP < 1 {
		p.HP = 1
	}
}

func (p *Player) EffectiveATK() int {
	atk := p.ATK
	for _, item := range p.Inventory {
		if item.Type == ItemWeapon {
			atk += item.Bonus
		}
	}
	return atk
}

func (p *Player) EffectiveDEF() int {
	def := p.DEF
	for _, item := range p.Inventory {
		if item.Type == ItemArmor {
			def += item.Bonus
		}
	}
	return def
}
```

- [ ] **Step 5: Run player tests**

```bash
go test ./entity/ -run TestNewPlayer -v && go test ./entity/ -run TestAllClasses -v && go test ./entity/ -run TestPlayerTakeDamage -v
```

Expected: all PASS.

- [ ] **Step 6: Write monster test**

`entity/monster_test.go`:

```go
package entity

import (
	"testing"
	"math"
)

func TestMonsterScaling(t *testing.T) {
	slime := NewMonster(MonsterSlime, 1)
	if slime.HP != 20 || slime.ATK != 5 {
		t.Errorf("Slime floor 1: got HP=%d ATK=%d, want HP=20 ATK=5", slime.HP, slime.ATK)
	}

	slimeF3 := NewMonster(MonsterSlime, 3)
	expectedHP := int(math.Round(20 * math.Pow(1.15, 2)))
	if slimeF3.HP != expectedHP {
		t.Errorf("Slime floor 3: got HP=%d, want %d", slimeF3.HP, expectedHP)
	}
}

func TestBossStats(t *testing.T) {
	boss := NewMonster(MonsterBoss5, 5)
	if boss.HP != 150 || boss.ATK != 15 || boss.DEF != 8 {
		t.Errorf("Boss5: got HP=%d ATK=%d DEF=%d, want 150/15/8", boss.HP, boss.ATK, boss.DEF)
	}
}
```

- [ ] **Step 7: Implement monster**

`entity/monster.go`:

```go
package entity

import "math"

type MonsterType int

const (
	MonsterSlime MonsterType = iota
	MonsterSkeleton
	MonsterOrc
	MonsterDarkKnight
	MonsterBoss5
	MonsterBoss10
	MonsterBoss15
	MonsterBoss20
)

type monsterBase struct {
	Name     string
	HP, ATK, DEF int
	MinFloor int
	IsBoss   bool
}

var monsterDefs = map[MonsterType]monsterBase{
	MonsterSlime:      {"Slime", 20, 5, 1, 1, false},
	MonsterSkeleton:   {"Skeleton", 35, 10, 4, 3, false},
	MonsterOrc:        {"Orc", 55, 14, 6, 6, false},
	MonsterDarkKnight: {"Dark Knight", 80, 18, 10, 12, false},
	MonsterBoss5:      {"Guardian", 150, 15, 8, 5, true},
	MonsterBoss10:     {"Warden", 250, 22, 12, 10, true},
	MonsterBoss15:     {"Overlord", 400, 30, 16, 15, true},
	MonsterBoss20:     {"Archlich", 600, 40, 20, 20, true},
}

type Monster struct {
	Name        string
	Type        MonsterType
	HP, MaxHP   int
	ATK, DEF    int
	IsBoss      bool
	TauntTarget bool // is being taunted
	TauntTurns  int
}

func NewMonster(mt MonsterType, floor int) *Monster {
	base := monsterDefs[mt]
	scale := 1.0
	if !base.IsBoss && floor > base.MinFloor {
		scale = math.Pow(1.15, float64(floor-base.MinFloor))
	}
	hp := int(math.Round(float64(base.HP) * scale))
	atk := int(math.Round(float64(base.ATK) * scale))
	return &Monster{
		Name:   base.Name,
		Type:   mt,
		HP:     hp,
		MaxHP:  hp,
		ATK:    atk,
		DEF:    base.DEF,
		IsBoss: base.IsBoss,
	}
}

func (m *Monster) TakeDamage(dmg int) {
	m.HP -= dmg
	if m.HP < 0 {
		m.HP = 0
	}
}

func (m *Monster) IsDead() bool {
	return m.HP <= 0
}

func (m *Monster) TickTaunt() {
	if m.TauntTurns > 0 {
		m.TauntTurns--
		if m.TauntTurns == 0 {
			m.TauntTarget = false
		}
	}
}
```

- [ ] **Step 8: Run all entity tests**

```bash
go test ./entity/ -v
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add entity/
git commit -m "feat: entity definitions — player classes, monsters, items"
```

---

### Task 3: Combat System

**Files:**
- Create: `combat/combat.go`
- Test: `combat/combat_test.go`

- [ ] **Step 1: Write combat test**

`combat/combat_test.go`:

```go
package combat

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func TestCalcDamage(t *testing.T) {
	// Warrior ATK=12, skill multiplier 1.0, vs Slime DEF=1
	// Expected: max(1, 12*1.0 - 1) * rand(0.85~1.15) = 11 * (0.85~1.15)
	// Range: 9~12 (with rounding)
	dmg := CalcDamage(12, 1, 1.0)
	if dmg < 9 || dmg > 13 {
		t.Errorf("Damage out of expected range: got %d, want 9~13", dmg)
	}
}

func TestCalcDamageMinimum(t *testing.T) {
	// ATK=1 vs DEF=100 → max(1, 1-100) = 1 * rand = 1
	dmg := CalcDamage(1, 100, 1.0)
	if dmg != 1 {
		t.Errorf("Minimum damage: got %d, want 1", dmg)
	}
}

func TestCoopBonus(t *testing.T) {
	// Two players attacking same target: 2nd gets 10% bonus
	attackers := []AttackIntent{
		{PlayerATK: 12, TargetIdx: 0, Multiplier: 1.0, IsAoE: false},
		{PlayerATK: 15, TargetIdx: 0, Multiplier: 1.0, IsAoE: false},
	}
	results := ResolveAttacks(attackers, []*entity.Monster{entity.NewMonster(entity.MonsterSlime, 1)})
	// Second attacker should have co-op bonus
	if !results[1].CoopApplied {
		t.Error("Second attacker should get co-op bonus")
	}
}

func TestAoENoCoopBonus(t *testing.T) {
	attackers := []AttackIntent{
		{PlayerATK: 12, TargetIdx: 0, Multiplier: 1.0, IsAoE: false},
		{PlayerATK: 20, TargetIdx: -1, Multiplier: 0.8, IsAoE: true}, // fireball
	}
	monsters := []*entity.Monster{
		entity.NewMonster(entity.MonsterSlime, 1),
		entity.NewMonster(entity.MonsterSlime, 1),
	}
	results := ResolveAttacks(attackers, monsters)
	// First attacker should NOT get co-op from AoE
	if results[0].CoopApplied {
		t.Error("AoE should not trigger co-op bonus")
	}
}

func TestFleeChance(t *testing.T) {
	// Run 100 flee attempts, expect roughly 50% success
	successes := 0
	for i := 0; i < 100; i++ {
		if AttemptFlee() {
			successes++
		}
	}
	if successes < 20 || successes > 80 {
		t.Errorf("Flee success rate suspicious: %d/100", successes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./combat/ -v
```

Expected: FAIL — package not defined.

- [ ] **Step 3: Implement combat**

`combat/combat.go`:

```go
package combat

import (
	"math"
	"math/rand"

	"github.com/tolelom/catacombs/entity"
)

// CalcDamage: max(1, ATK * multiplier - DEF) * random(0.85~1.15)
func CalcDamage(atk, def int, multiplier float64) int {
	base := float64(atk)*multiplier - float64(def)
	if base < 1 {
		base = 1
	}
	randomFactor := 0.85 + rand.Float64()*0.30 // 0.85 ~ 1.15
	return int(math.Round(base * randomFactor))
}

type AttackIntent struct {
	PlayerATK  int
	TargetIdx  int     // -1 for AoE
	Multiplier float64
	IsAoE      bool
}

type AttackResult struct {
	TargetIdx   int
	Damage      int
	CoopApplied bool
	IsAoE       bool
}

func ResolveAttacks(intents []AttackIntent, monsters []*entity.Monster) []AttackResult {
	// Count single-target attacks per target (for co-op bonus)
	targetCount := make(map[int]int)
	targetOrder := make(map[int]int) // which attacker index was first
	for i, intent := range intents {
		if !intent.IsAoE {
			targetCount[intent.TargetIdx]++
			if _, ok := targetOrder[intent.TargetIdx]; !ok {
				targetOrder[intent.TargetIdx] = i
			}
		}
	}

	results := make([]AttackResult, len(intents))
	for i, intent := range intents {
		if intent.IsAoE {
			// AoE hits all monsters, no co-op bonus
			totalDmg := 0
			for _, m := range monsters {
				if !m.IsDead() {
					dmg := CalcDamage(intent.PlayerATK, m.DEF, intent.Multiplier)
					m.TakeDamage(dmg)
					totalDmg += dmg
				}
			}
			results[i] = AttackResult{TargetIdx: -1, Damage: totalDmg, IsAoE: true}
		} else {
			if intent.TargetIdx < 0 || intent.TargetIdx >= len(monsters) {
				continue
			}
			m := monsters[intent.TargetIdx]
			dmg := CalcDamage(intent.PlayerATK, m.DEF, intent.Multiplier)

			// Co-op bonus: 2+ single-target attackers on same target, 2nd+ gets 10%
			coopApplied := false
			if targetCount[intent.TargetIdx] >= 2 && targetOrder[intent.TargetIdx] != i {
				dmg = int(math.Round(float64(dmg) * 1.10))
				coopApplied = true
			}

			m.TakeDamage(dmg)
			results[i] = AttackResult{
				TargetIdx:   intent.TargetIdx,
				Damage:      dmg,
				CoopApplied: coopApplied,
			}
		}
	}
	return results
}

func AttemptFlee() bool {
	return rand.Float64() < 0.5
}

// MonsterAI picks a target for a monster
func MonsterAI(m *entity.Monster, players []*entity.Player, turnNumber int) (targetIdx int, isAoE bool) {
	// Boss AoE every 3 turns
	if m.IsBoss && turnNumber%3 == 0 {
		return -1, true
	}

	// If taunted, attack the taunter
	if m.TauntTarget {
		for i, p := range players {
			if !p.IsDead() && p.Class == entity.ClassWarrior {
				return i, false
			}
		}
	}

	// 30% chance to target lowest HP player
	if rand.Float64() < 0.3 {
		minHP := int(^uint(0) >> 1)
		minIdx := 0
		for i, p := range players {
			if !p.IsDead() && p.HP < minHP {
				minHP = p.HP
				minIdx = i
			}
		}
		return minIdx, false
	}

	// Default: first alive player (closest)
	for i, p := range players {
		if !p.IsDead() {
			return i, false
		}
	}
	return 0, false
}
```

- [ ] **Step 4: Run combat tests**

```bash
go test ./combat/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add combat/
git commit -m "feat: combat system — damage calc, co-op bonus, flee, monster AI"
```

---

### Task 4: Dungeon Generation

**Files:**
- Create: `dungeon/generator.go`, `dungeon/room.go`, `dungeon/fov.go`
- Test: `dungeon/generator_test.go`, `dungeon/room_test.go`

- [ ] **Step 1: Write room types**

`dungeon/room.go`:

```go
package dungeon

import "math/rand"

type RoomType int

const (
	RoomCombat RoomType = iota
	RoomTreasure
	RoomShop
	RoomEvent
	RoomEmpty
	RoomBoss
)

func (r RoomType) String() string {
	return [...]string{"Combat", "Treasure", "Shop", "Event", "Empty", "Boss"}[r]
}

type Room struct {
	Type      RoomType
	X, Y      int // position in grid
	Width, Height int
	Visited   bool
	Cleared   bool
	Neighbors []int // indices of connected rooms
}

// RandomRoomType returns a room type based on probability table
// Combat: 45%, Treasure: 15%, Shop: 10%, Event: 15%, Empty: 15%
func RandomRoomType() RoomType {
	r := rand.Float64() * 100
	switch {
	case r < 45:
		return RoomCombat
	case r < 60:
		return RoomTreasure
	case r < 70:
		return RoomShop
	case r < 85:
		return RoomEvent
	default:
		return RoomEmpty
	}
}
```

- [ ] **Step 2: Write generator test**

`dungeon/generator_test.go`:

```go
package dungeon

import "testing"

func TestGenerateFloor(t *testing.T) {
	floor := GenerateFloor(1)
	if len(floor.Rooms) < 5 || len(floor.Rooms) > 8 {
		t.Errorf("Room count: got %d, want 5~8", len(floor.Rooms))
	}

	// Must have exactly one boss room
	bossCount := 0
	for _, r := range floor.Rooms {
		if r.Type == RoomBoss {
			bossCount++
		}
	}
	if bossCount != 1 {
		t.Errorf("Boss rooms: got %d, want 1", bossCount)
	}

	// All rooms must be connected (reachable from room 0)
	visited := make(map[int]bool)
	var dfs func(int)
	dfs = func(idx int) {
		if visited[idx] {
			return
		}
		visited[idx] = true
		for _, n := range floor.Rooms[idx].Neighbors {
			dfs(n)
		}
	}
	dfs(0)
	if len(visited) != len(floor.Rooms) {
		t.Errorf("Not all rooms connected: reachable %d / %d", len(visited), len(floor.Rooms))
	}
}

func TestRoomTypeProbability(t *testing.T) {
	counts := make(map[RoomType]int)
	n := 10000
	for i := 0; i < n; i++ {
		counts[RandomRoomType()]++
	}
	// Combat should be ~45% (allow 40-50%)
	combatPct := float64(counts[RoomCombat]) / float64(n) * 100
	if combatPct < 40 || combatPct > 50 {
		t.Errorf("Combat room probability: got %.1f%%, want ~45%%", combatPct)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./dungeon/ -v
```

Expected: FAIL.

- [ ] **Step 4: Implement BSP generator**

`dungeon/generator.go`:

```go
package dungeon

import "math/rand"

type Floor struct {
	Number    int
	Rooms     []*Room
	CurrentRoom int
}

func GenerateFloor(floorNum int) *Floor {
	numRooms := 5 + rand.Intn(4) // 5~8 rooms

	rooms := make([]*Room, numRooms)

	// Place rooms in a simple grid layout for BSP-like connectivity
	for i := 0; i < numRooms; i++ {
		rt := RandomRoomType()
		rooms[i] = &Room{
			Type:      rt,
			X:         (i % 3) * 20,
			Y:         (i / 3) * 10,
			Width:     12 + rand.Intn(6),
			Height:    6 + rand.Intn(4),
			Neighbors: []int{},
		}
	}

	// Last room is always the boss room
	rooms[numRooms-1].Type = RoomBoss

	// Connect rooms linearly, then add some extra edges for variety
	for i := 0; i < numRooms-1; i++ {
		rooms[i].Neighbors = append(rooms[i].Neighbors, i+1)
		rooms[i+1].Neighbors = append(rooms[i+1].Neighbors, i)
	}

	// Add 1-2 extra connections for loops
	extras := 1 + rand.Intn(2)
	for e := 0; e < extras; e++ {
		a := rand.Intn(numRooms)
		b := rand.Intn(numRooms)
		if a != b && !hasNeighbor(rooms[a], b) {
			rooms[a].Neighbors = append(rooms[a].Neighbors, b)
			rooms[b].Neighbors = append(rooms[b].Neighbors, a)
		}
	}

	return &Floor{
		Number:      floorNum,
		Rooms:       rooms,
		CurrentRoom: 0,
	}
}

func hasNeighbor(r *Room, idx int) bool {
	for _, n := range r.Neighbors {
		if n == idx {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Implement FOV**

`dungeon/fov.go`:

```go
package dungeon

type Visibility int

const (
	Hidden  Visibility = iota
	Visited            // dimmed
	Visible            // fully visible
)

func UpdateVisibility(floor *Floor) {
	for i, room := range floor.Rooms {
		if i == floor.CurrentRoom {
			room.Visited = true
		}
	}
}

func GetRoomVisibility(floor *Floor, roomIdx int) Visibility {
	if roomIdx == floor.CurrentRoom {
		return Visible
	}
	if floor.Rooms[roomIdx].Visited {
		return Visited
	}
	return Hidden
}
```

- [ ] **Step 6: Run dungeon tests**

```bash
go test ./dungeon/ -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add dungeon/
git commit -m "feat: dungeon generation — BSP rooms, room types, fog of war"
```

---

### Task 5: Game Session & Turn System

**Files:**
- Create: `game/session.go`, `game/turn.go`, `game/event.go`, `game/lobby.go`
- Test: `game/session_test.go`, `game/turn_test.go`, `game/lobby_test.go`

- [ ] **Step 1: Write lobby test**

`game/lobby_test.go`:

```go
package game

import "testing"

func TestCreateRoom(t *testing.T) {
	lobby := NewLobby()
	code := lobby.CreateRoom("Test Room")
	if len(code) != 4 {
		t.Errorf("Room code length: got %d, want 4", len(code))
	}
	rooms := lobby.ListRooms()
	if len(rooms) != 1 {
		t.Errorf("Room count: got %d, want 1", len(rooms))
	}
}

func TestJoinRoom(t *testing.T) {
	lobby := NewLobby()
	code := lobby.CreateRoom("Test Room")
	err := lobby.JoinRoom(code, "player1")
	if err != nil {
		t.Errorf("Join failed: %v", err)
	}
	room := lobby.GetRoom(code)
	if len(room.Players) != 1 {
		t.Errorf("Player count: got %d, want 1", len(room.Players))
	}
}

func TestJoinRoomFull(t *testing.T) {
	lobby := NewLobby()
	code := lobby.CreateRoom("Test Room")
	for i := 0; i < 4; i++ {
		lobby.JoinRoom(code, "player")
	}
	err := lobby.JoinRoom(code, "player5")
	if err == nil {
		t.Error("Should reject 5th player")
	}
}
```

- [ ] **Step 2: Implement lobby**

`game/lobby.go`:

```go
package game

import (
	"fmt"
	"math/rand"
	"sync"
)

type RoomStatus int

const (
	RoomWaiting RoomStatus = iota
	RoomPlaying
)

type LobbyRoom struct {
	Code    string
	Name    string
	Players []string
	Status  RoomStatus
	Session *GameSession
}

type Lobby struct {
	mu    sync.RWMutex
	rooms map[string]*LobbyRoom
}

func NewLobby() *Lobby {
	return &Lobby{rooms: make(map[string]*LobbyRoom)}
}

func (l *Lobby) CreateRoom(name string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	code := generateCode()
	for l.rooms[code] != nil {
		code = generateCode()
	}
	l.rooms[code] = &LobbyRoom{
		Code:   code,
		Name:   name,
		Status: RoomWaiting,
	}
	return code
}

func (l *Lobby) JoinRoom(code, playerName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	room, ok := l.rooms[code]
	if !ok {
		return fmt.Errorf("room %s not found", code)
	}
	if len(room.Players) >= 4 {
		return fmt.Errorf("room %s is full", code)
	}
	if room.Status != RoomWaiting {
		return fmt.Errorf("room %s already in progress", code)
	}
	room.Players = append(room.Players, playerName)
	return nil
}

func (l *Lobby) GetRoom(code string) *LobbyRoom {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rooms[code]
}

func (l *Lobby) ListRooms() []*LobbyRoom {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*LobbyRoom, 0, len(l.rooms))
	for _, r := range l.rooms {
		result = append(result, r)
	}
	return result
}

func (l *Lobby) RemoveRoom(code string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.rooms, code)
}

func generateCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 4)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
```

- [ ] **Step 3: Write session/turn test**

`game/session_test.go`:

```go
package game

import (
	"testing"
	"time"

	"github.com/tolelom/catacombs/entity"
)

func TestSessionTurnTimeout(t *testing.T) {
	s := NewGameSession()
	p := entity.NewPlayer("test", entity.ClassWarrior)
	s.AddPlayer(p)
	s.StartFloor()

	// Don't submit any action, wait for timeout
	done := make(chan struct{})
	go func() {
		s.RunTurn()
		close(done)
	}()

	select {
	case <-done:
		// Turn completed via timeout
	case <-time.After(7 * time.Second):
		t.Error("Turn did not timeout within 7 seconds")
	}
}
```

- [ ] **Step 4: Implement session and turn**

`game/session.go`:

```go
package game

import (
	"sync"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

type GamePhase int

const (
	PhaseExploring GamePhase = iota
	PhaseCombat
	PhaseShop
	PhaseResult
)

type PlayerAction struct {
	Type      ActionType
	TargetIdx int
}

type ActionType int

const (
	ActionAttack ActionType = iota
	ActionSkill
	ActionItem
	ActionFlee
	ActionWait
)

type GameState struct {
	Floor       *dungeon.Floor
	Players     []*entity.Player
	Monsters    []*entity.Monster
	Phase       GamePhase
	FloorNum    int
	TurnNum     int
	CombatTurn  int  // reset per combat encounter
	SoloMode    bool
	GameOver    bool
	Victory     bool
	ShopItems   []entity.Item
}

type GameSession struct {
	mu       sync.Mutex
	state    GameState
	actions  map[string]PlayerAction // playerName -> action
	actionCh chan playerActionMsg
}

type playerActionMsg struct {
	PlayerName string
	Action     PlayerAction
}

func NewGameSession() *GameSession {
	return &GameSession{
		state: GameState{
			FloorNum: 1,
		},
		actions:  make(map[string]PlayerAction),
		actionCh: make(chan playerActionMsg, 4),
	}
}

// StartGame determines solo mode from actual player count at game start
func (s *GameSession) StartGame() {
	s.mu.Lock()
	s.state.SoloMode = len(s.state.Players) == 1
	s.mu.Unlock()
	s.StartFloor()
}

func (s *GameSession) AddPlayer(p *entity.Player) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Players = append(s.state.Players, p)
}

func (s *GameSession) StartFloor() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum)
	s.state.Phase = PhaseExploring
	s.state.TurnNum = 0

	// Revive dead players at 30% HP
	for _, p := range s.state.Players {
		if p.IsDead() {
			p.Revive(0.30)
		}
	}
}

func (s *GameSession) GetState() GameState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *GameSession) SubmitAction(playerName string, action PlayerAction) {
	s.actionCh <- playerActionMsg{PlayerName: playerName, Action: action}
}
```

`game/turn.go`:

```go
package game

import (
	"math/rand"
	"time"

	"github.com/tolelom/catacombs/combat"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

const TurnTimeout = 5 * time.Second

func (s *GameSession) RunTurn() {
	s.mu.Lock()
	s.state.TurnNum++
	s.state.CombatTurn++
	s.actions = make(map[string]PlayerAction)
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsDead() {
			aliveCount++
		}
	}
	s.mu.Unlock()

	// Collect actions with timeout
	timer := time.NewTimer(TurnTimeout)
	collected := 0
	for collected < aliveCount {
		select {
		case msg := <-s.actionCh:
			s.mu.Lock()
			s.actions[msg.PlayerName] = msg.Action
			s.mu.Unlock()
			collected++
		case <-timer.C:
			goto resolve
		}
	}
	timer.Stop()

resolve:
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default action for players who didn't submit: Wait
	for _, p := range s.state.Players {
		if !p.IsDead() {
			if _, ok := s.actions[p.Name]; !ok {
				s.actions[p.Name] = PlayerAction{Type: ActionWait}
			}
		}
	}

	s.resolvePlayerActions()
	s.resolveMonsterActions()
}

func (s *GameSession) resolvePlayerActions() {
	var intents []combat.AttackIntent

	// Track which monsters were alive before this turn (for gold awards)
	aliveBeforeTurn := make(map[int]bool)
	for i, m := range s.state.Monsters {
		if !m.IsDead() {
			aliveBeforeTurn[i] = true
		}
	}

	// Check if ALL alive players chose flee — only then the party flees
	fleeCount := 0
	aliveCount := 0
	for _, p := range s.state.Players {
		if p.IsDead() {
			continue
		}
		aliveCount++
		if action, ok := s.actions[p.Name]; ok && action.Type == ActionFlee {
			fleeCount++
		}
	}
	if fleeCount == aliveCount && aliveCount > 0 {
		if combat.AttemptFlee() {
			s.state.Phase = PhaseExploring
			return
		}
		// Flee failed — all fleeing players waste their turn, continue to monster phase
		return
	}

	for _, p := range s.state.Players {
		if p.IsDead() {
			continue
		}
		action, ok := s.actions[p.Name]
		if !ok {
			continue
		}

		switch action.Type {
		case ActionAttack:
			intents = append(intents, combat.AttackIntent{
				PlayerATK:  p.EffectiveATK(),
				TargetIdx:  action.TargetIdx,
				Multiplier: 1.0,
				IsAoE:      false,
			})
		case ActionSkill:
			switch p.Class {
			case entity.ClassWarrior:
				// Taunt: mark all monsters to target this warrior
				for _, m := range s.state.Monsters {
					if !m.IsDead() {
						m.TauntTarget = true
						m.TauntTurns = 2
					}
				}
			case entity.ClassMage:
				intents = append(intents, combat.AttackIntent{
					PlayerATK:  p.EffectiveATK(),
					TargetIdx:  -1,
					Multiplier: 0.8,
					IsAoE:      true,
				})
			case entity.ClassHealer:
				if action.TargetIdx >= 0 && action.TargetIdx < len(s.state.Players) {
					s.state.Players[action.TargetIdx].Heal(30)
				}
			case entity.ClassRogue:
				// Scout: reveal neighboring rooms
				currentRoom := s.state.Floor.Rooms[s.state.Floor.CurrentRoom]
				for _, neighborIdx := range currentRoom.Neighbors {
					s.state.Floor.Rooms[neighborIdx].Visited = true
				}
			}
		case ActionItem:
			// Use first consumable from inventory
			for i, item := range p.Inventory {
				if item.Type == entity.ItemConsumable {
					p.Heal(item.Bonus)
					p.Inventory = append(p.Inventory[:i], p.Inventory[i+1:]...)
					break
				}
			}
		case ActionFlee:
			// Individual flee does nothing if not unanimous (already handled above)
		case ActionWait:
			// Defensive stance — no action
		}
	}

	if len(intents) > 0 && len(s.state.Monsters) > 0 {
		combat.ResolveAttacks(intents, s.state.Monsters)
	}

	// Award gold only for monsters that JUST died this turn
	for i, m := range s.state.Monsters {
		if m.IsDead() && aliveBeforeTurn[i] {
			goldReward := 5 + s.state.FloorNum
			if goldReward > 15 {
				goldReward = 15
			}
			for _, p := range s.state.Players {
				if !p.IsDead() {
					p.Gold += goldReward
				}
			}
			// Boss kill: drop relic
			if m.IsBoss {
				s.grantBossRelic()
			}
		}
	}

	// Filter out dead monsters
	alive := make([]*entity.Monster, 0)
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			alive = append(alive, m)
		}
	}
	s.state.Monsters = alive

	// Check if combat is over
	if len(s.state.Monsters) == 0 {
		s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		// Check if this was the boss room -> advance floor
		if s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Type == dungeon.RoomBoss {
			s.advanceFloor()
		} else {
			s.state.Phase = PhaseExploring
		}
	}
}

func (s *GameSession) advanceFloor() {
	if s.state.FloorNum >= 20 {
		s.state.Phase = PhaseResult
		s.state.Victory = true
		s.state.GameOver = true
		return
	}
	s.state.FloorNum++
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum)
	s.state.Phase = PhaseExploring
	s.state.CombatTurn = 0
	// Revive dead players at 30% HP
	for _, p := range s.state.Players {
		if p.IsDead() {
			p.Revive(0.30)
		}
	}
}

func (s *GameSession) grantBossRelic() {
	relics := []entity.Relic{
		{Name: "Vampiric Ring", Effect: entity.RelicHealOnKill, Value: 5, Price: 100},
		{Name: "Power Amulet", Effect: entity.RelicATKBoost, Value: 3, Price: 120},
		{Name: "Iron Ward", Effect: entity.RelicDEFBoost, Value: 2, Price: 100},
		{Name: "Gold Charm", Effect: entity.RelicGoldBoost, Value: 5, Price: 150},
	}
	for _, p := range s.state.Players {
		if !p.IsDead() {
			r := relics[rand.Intn(len(relics))]
			p.Relics = append(p.Relics, r)
		}
	}
}

// BuyItem handles shop purchases
func (s *GameSession) BuyItem(playerName string, itemIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop || itemIdx < 0 || itemIdx >= len(s.state.ShopItems) {
		return false
	}
	item := s.state.ShopItems[itemIdx]
	for _, p := range s.state.Players {
		if p.Name == playerName && p.Gold >= item.Price {
			p.Gold -= item.Price
			p.Inventory = append(p.Inventory, item)
			return true
		}
	}
	return false
}

// LeaveShop exits the shop phase
func (s *GameSession) LeaveShop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Phase = PhaseExploring
	s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
}

func (s *GameSession) resolveMonsterActions() {
	if s.state.Phase != PhaseCombat {
		return
	}
	for _, m := range s.state.Monsters {
		if m.IsDead() {
			continue
		}
		targetIdx, isAoE := combat.MonsterAI(m, s.state.Players, s.state.CombatTurn)
		if isAoE {
			// Boss AoE: 0.5x damage to all
			for _, p := range s.state.Players {
				if !p.IsDead() {
					dmg := combat.CalcDamage(m.ATK, p.EffectiveDEF(), 0.5)
					p.TakeDamage(dmg)
				}
			}
		} else {
			if targetIdx >= 0 && targetIdx < len(s.state.Players) {
				p := s.state.Players[targetIdx]
				if !p.IsDead() {
					dmg := combat.CalcDamage(m.ATK, p.EffectiveDEF(), 1.0)
					p.TakeDamage(dmg)
				}
			}
		}
		m.TickTaunt()
	}

	// Check party wipe
	allPlayersDead := true
	for _, p := range s.state.Players {
		if !p.IsDead() {
			allPlayersDead = false
			break
		}
	}
	if allPlayersDead {
		s.state.Phase = PhaseResult
	}
}
```

`game/event.go`:

```go
package game

import (
	"math/rand"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) EnterRoom(roomIdx int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Floor.CurrentRoom = roomIdx
	dungeon.UpdateVisibility(s.state.Floor)
	room := s.state.Floor.Rooms[roomIdx]

	if room.Cleared {
		return
	}

	switch room.Type {
	case dungeon.RoomCombat:
		s.spawnMonsters()
		s.state.Phase = PhaseCombat
		s.state.CombatTurn = 0
	case dungeon.RoomBoss:
		s.spawnBoss()
		s.state.Phase = PhaseCombat
		s.state.CombatTurn = 0
	case dungeon.RoomShop:
		s.generateShopItems()
		s.state.Phase = PhaseShop
	case dungeon.RoomTreasure:
		s.grantTreasure()
		room.Cleared = true
	case dungeon.RoomEvent:
		s.triggerEvent()
		room.Cleared = true
	case dungeon.RoomEmpty:
		room.Cleared = true
	}
}

func (s *GameSession) spawnMonsters() {
	count := 1 + rand.Intn(5) // 1~5 monsters
	floor := s.state.FloorNum
	s.state.Monsters = make([]*entity.Monster, count)

	// Pick appropriate monster type for floor
	var mt entity.MonsterType
	switch {
	case floor <= 5:
		mt = entity.MonsterSlime
	case floor <= 10:
		mt = entity.MonsterSkeleton
	case floor <= 14:
		mt = entity.MonsterOrc
	default:
		mt = entity.MonsterDarkKnight
	}

	// Solo mode: 50% HP
	for i := 0; i < count; i++ {
		m := entity.NewMonster(mt, floor)
		if s.state.SoloMode {
			m.HP = m.HP / 2
			if m.HP < 1 {
				m.HP = 1
			}
			m.MaxHP = m.HP
		}
		s.state.Monsters[i] = m
	}
}

func (s *GameSession) spawnBoss() {
	var mt entity.MonsterType
	switch s.state.FloorNum {
	case 5:
		mt = entity.MonsterBoss5
	case 10:
		mt = entity.MonsterBoss10
	case 15:
		mt = entity.MonsterBoss15
	case 20:
		mt = entity.MonsterBoss20
	default:
		mt = entity.MonsterBoss5
	}
	boss := entity.NewMonster(mt, s.state.FloorNum)
	if s.state.SoloMode {
		boss.HP = boss.HP / 2
		boss.MaxHP = boss.HP
	}
	s.state.Monsters = []*entity.Monster{boss}
}

func (s *GameSession) grantTreasure() {
	// Random item for each player
	for _, p := range s.state.Players {
		if rand.Float64() < 0.5 {
			p.Inventory = append(p.Inventory, entity.Item{
				Name: "Iron Sword", Type: entity.ItemWeapon, Bonus: 3 + rand.Intn(6),
			})
		} else {
			p.Inventory = append(p.Inventory, entity.Item{
				Name: "Iron Shield", Type: entity.ItemArmor, Bonus: 2 + rand.Intn(4),
			})
		}
	}
}

func (s *GameSession) generateShopItems() {
	s.state.ShopItems = []entity.Item{
		{Name: "HP Potion", Type: entity.ItemConsumable, Bonus: 30, Price: 20},
		{Name: "Iron Sword", Type: entity.ItemWeapon, Bonus: 3 + rand.Intn(6), Price: 40 + rand.Intn(41)},
		{Name: "Iron Shield", Type: entity.ItemArmor, Bonus: 2 + rand.Intn(4), Price: 30 + rand.Intn(31)},
	}
}

func (s *GameSession) triggerEvent() {
	// Random event: 50% trap, 50% blessing
	for _, p := range s.state.Players {
		if p.IsDead() {
			continue
		}
		if rand.Float64() < 0.5 {
			// Trap: 10~20 damage
			dmg := 10 + rand.Intn(11)
			p.TakeDamage(dmg)
		} else {
			// Blessing: heal 15~25
			heal := 15 + rand.Intn(11)
			p.Heal(heal)
		}
	}
}
```

- [ ] **Step 5: Run lobby tests**

```bash
go test ./game/ -run TestCreate -v && go test ./game/ -run TestJoin -v
```

Expected: PASS.

- [ ] **Step 6: Run session/turn tests**

```bash
go test ./game/ -run TestSession -v -timeout 10s
```

Expected: PASS (turn resolves within 5s + buffer).

- [ ] **Step 7: Commit**

```bash
git add game/
git commit -m "feat: game session, turn system, lobby, and room events"
```

---

### Task 6: BoltDB Store (Rankings & Player Identity)

**Files:**
- Create: `store/db.go`
- Test: `store/db_test.go`

- [ ] **Step 1: Write store test**

`store/db_test.go`:

```go
package store

import (
	"os"
	"testing"
)

func TestPlayerProfile(t *testing.T) {
	db, err := Open("test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Close()
		os.Remove("test.db")
	}()

	err = db.SaveProfile("SHA256:abc123", "TestPlayer")
	if err != nil {
		t.Fatal(err)
	}
	name, err := db.GetProfile("SHA256:abc123")
	if err != nil {
		t.Fatal(err)
	}
	if name != "TestPlayer" {
		t.Errorf("Name: got %q, want %q", name, "TestPlayer")
	}
}

func TestRanking(t *testing.T) {
	db, err := Open("test_rank.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Close()
		os.Remove("test_rank.db")
	}()

	db.SaveRun("Alice", 20, 1500)
	db.SaveRun("Bob", 15, 1000)
	db.SaveRun("Charlie", 20, 2000)

	rankings, err := db.TopRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rankings) != 3 {
		t.Errorf("Rankings: got %d, want 3", len(rankings))
	}
	if rankings[0].Player != "Charlie" {
		t.Errorf("Top player: got %q, want Charlie", rankings[0].Player)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./store/ -v
```

Expected: FAIL.

- [ ] **Step 3: Implement store**

`store/db.go`:

```go
package store

import (
	"encoding/json"
	"fmt"
	"sort"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketProfiles = []byte("profiles")
	bucketRankings = []byte("rankings")
)

type DB struct {
	db *bolt.DB
}

type RunRecord struct {
	Player   string `json:"player"`
	Floor    int    `json:"floor"`
	Score    int    `json:"score"`
}

func Open(path string) (*DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketProfiles); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketRankings); err != nil {
			return err
		}
		return nil
	})
	return &DB{db: db}, err
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) SaveProfile(fingerprint, name string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProfiles).Put([]byte(fingerprint), []byte(name))
	})
}

func (d *DB) GetProfile(fingerprint string) (string, error) {
	var name string
	err := d.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketProfiles).Get([]byte(fingerprint))
		if v == nil {
			return fmt.Errorf("profile not found")
		}
		name = string(v)
		return nil
	})
	return name, err
}

func (d *DB) SaveRun(player string, floor, score int) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRankings)
		id, _ := b.NextSequence()
		record := RunRecord{Player: player, Floor: floor, Score: score}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put([]byte(fmt.Sprintf("%010d", id)), data)
	})
}

func (d *DB) TopRuns(limit int) ([]RunRecord, error) {
	var runs []RunRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRankings)
		return b.ForEach(func(k, v []byte) error {
			var r RunRecord
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			runs = append(runs, r)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Floor != runs[j].Floor {
			return runs[i].Floor > runs[j].Floor
		}
		return runs[i].Score > runs[j].Score
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}
```

- [ ] **Step 4: Run store tests**

```bash
go test ./store/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add store/
git commit -m "feat: BoltDB store — player profiles and run rankings"
```

---

### Task 7: TUI Views (Title, Lobby, Game, Result)

**Files:**
- Modify: `ui/model.go`
- Create: `ui/title.go`, `ui/lobby_view.go`, `ui/game_view.go`, `ui/result_view.go`

- [ ] **Step 1: Write title screen**

`ui/title.go`:

```go
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var titleArt = `
 ██████╗ █████╗ ████████╗ █████╗  ██████╗ ██████╗ ███╗   ███╗██████╗ ███████╗
██╔════╝██╔══██╗╚══██╔══╝██╔══██╗██╔════╝██╔═══██╗████╗ ████║██╔══██╗██╔════╝
██║     ███████║   ██║   ███████║██║     ██║   ██║██╔████╔██║██████╔╝███████╗
██║     ██╔══██║   ██║   ██╔══██║██║     ██║   ██║██║╚██╔╝██║██╔══██╗╚════██║
╚██████╗██║  ██║   ██║   ██║  ██║╚██████╗╚██████╔╝██║ ╚═╝ ██║██████╔╝███████║
 ╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚══════╝
`

func renderTitle(width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Align(lipgloss.Center)

	menuStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render(titleArt),
		"",
		subtitleStyle.Render("A Co-op Roguelike Adventure"),
		"",
		menuStyle.Render("[Enter] Start  [Q] Quit"),
	)
}
```

- [ ] **Step 2: Write lobby view**

`ui/lobby_view.go`:

```go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type lobbyState struct {
	rooms      []roomInfo
	input      string
	cursor     int
	creating   bool
	roomName   string
}

type roomInfo struct {
	Code    string
	Name    string
	Players int
	Status  string
}

func renderLobby(state lobbyState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	roomStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	header := headerStyle.Render("── Lobby ──")
	menu := "[C] Create Room  [J] Join by Code  [Q] Back"

	roomList := ""
	for i, r := range state.rooms {
		marker := "  "
		if i == state.cursor {
			marker = "> "
		}
		roomList += fmt.Sprintf("%s%s [%s] (%d/4) %s\n",
			marker, r.Name, r.Code, r.Players, r.Status)
	}
	if roomList == "" {
		roomList = "  No rooms available. Create one!"
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		roomStyle.Render(roomList),
		"",
		menu,
	)
}
```

- [ ] **Step 3: Write game view**

`ui/game_view.go`:

```go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

func renderGame(state game.GameState, width, height int) string {
	mapView := renderMap(state.Floor)
	hudView := renderHUD(state)

	return lipgloss.JoinVertical(lipgloss.Left,
		mapView,
		hudView,
	)
}

func renderMap(floor *dungeon.Floor) string {
	if floor == nil {
		return ""
	}

	var sb strings.Builder
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	sb.WriteString(headerStyle.Render(fmt.Sprintf("── Catacombs B%d ──", floor.Number)))
	sb.WriteString("\n\n")

	roomStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	hiddenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	for i, room := range floor.Rooms {
		vis := dungeon.GetRoomVisibility(floor, i)
		symbol := roomTypeSymbol(room.Type)
		label := fmt.Sprintf("[%d] %s %s", i, symbol, room.Type.String())

		if i == floor.CurrentRoom {
			label = ">> " + label + " <<"
		}

		switch vis {
		case dungeon.Visible:
			sb.WriteString(roomStyle.Render(label))
		case dungeon.Visited:
			sb.WriteString(dimStyle.Render(label))
		case dungeon.Hidden:
			sb.WriteString(hiddenStyle.Render("[?] ???"))
		}

		// Show connections
		for _, n := range room.Neighbors {
			if n > i {
				sb.WriteString(" ─── ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderHUD(state game.GameState) string {
	var sb strings.Builder
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1)

	for _, p := range state.Players {
		hpBar := renderHPBar(p.HP, p.MaxHP, 20)
		status := ""
		if p.IsDead() {
			status = " [DEAD]"
		}
		sb.WriteString(fmt.Sprintf("%s (%s) %s %d/%d%s  Gold: %d\n",
			p.Name, p.Class, hpBar, p.HP, p.MaxHP, status, p.Gold))
	}

	if state.Phase == game.PhaseCombat {
		sb.WriteString("\n")
		for i, m := range state.Monsters {
			if !m.IsDead() {
				mhpBar := renderHPBar(m.HP, m.MaxHP, 15)
				sb.WriteString(fmt.Sprintf("  [%d] %s %s %d/%d\n", i, m.Name, mhpBar, m.HP, m.MaxHP))
			}
		}
		sb.WriteString("\n[1]Attack [2]Skill [3]Item [4]Flee [5]Wait")
	} else if state.Phase == game.PhaseExploring {
		sb.WriteString("\nChoose a room to enter (number) or [Q] quit")
	}

	return border.Render(sb.String())
}

func renderHPBar(current, max, width int) string {
	if max == 0 {
		return ""
	}
	filled := current * width / max
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	bar := greenStyle.Render(strings.Repeat("█", filled)) +
		redStyle.Render(strings.Repeat("░", empty))
	return bar
}

func roomTypeSymbol(rt dungeon.RoomType) string {
	switch rt {
	case dungeon.RoomCombat:
		return "D"
	case dungeon.RoomTreasure:
		return "$"
	case dungeon.RoomShop:
		return "S"
	case dungeon.RoomEvent:
		return "?"
	case dungeon.RoomEmpty:
		return "."
	case dungeon.RoomBoss:
		return "B"
	default:
		return " "
	}
}
```

- [ ] **Step 4: Write result view**

`ui/result_view.go`:

```go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

func renderResult(won bool, floorReached int, rankings []store.RunRecord) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	var title string
	if won {
		title = titleStyle.Render("VICTORY! You escaped the Catacombs!")
	} else {
		title = titleStyle.Render("GAME OVER")
	}

	floorInfo := fmt.Sprintf("Floor Reached: B%d", floorReached)

	rankHeader := lipgloss.NewStyle().Bold(true).Render("── Rankings ──")
	rankList := ""
	for i, r := range rankings {
		rankList += fmt.Sprintf("  %d. %s — B%d (Score: %d)\n", i+1, r.Player, r.Floor, r.Score)
	}

	menu := "[Enter] Return to Lobby  [Q] Quit"

	return lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		floorInfo,
		"",
		rankHeader,
		rankList,
		"",
		menu,
	)
}
```

- [ ] **Step 5: Write class selection screen**

`ui/class_view.go`:

```go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/entity"
)

type classSelectState struct {
	cursor int
}

var classOptions = []struct {
	class entity.Class
	name  string
	desc  string
}{
	{entity.ClassWarrior, "Warrior", "HP:120 ATK:12 DEF:8  Skill: Taunt (draw enemy fire)"},
	{entity.ClassMage, "Mage", "HP:70  ATK:20 DEF:3  Skill: Fireball (AoE damage)"},
	{entity.ClassHealer, "Healer", "HP:90  ATK:8  DEF:5  Skill: Heal (restore 30 HP)"},
	{entity.ClassRogue, "Rogue", "HP:85  ATK:15 DEF:4  Skill: Scout (reveal rooms)"},
}

func renderClassSelect(state classSelectState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	header := headerStyle.Render("── Choose Your Class ──")
	list := ""
	for i, opt := range classOptions {
		marker := "  "
		style := normalStyle
		if i == state.cursor {
			marker = "> "
			style = selectedStyle
		}
		list += fmt.Sprintf("%s%s\n   %s\n\n",
			marker, style.Render(opt.name), descStyle.Render(opt.desc))
	}

	menu := "[Up/Down] Select  [Enter] Confirm"

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		list,
		menu,
	)
}
```

- [ ] **Step 6: Update model.go with full state machine**

`ui/model.go` — complete rewrite with all screens and input routing:

```go
package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

type screen int

const (
	screenTitle screen = iota
	screenLobby
	screenClassSelect
	screenGame
	screenShop
	screenResult
)

// StateUpdateMsg is sent by GameSession to update the view
type StateUpdateMsg struct {
	State game.GameState
}

type Model struct {
	width       int
	height      int
	fingerprint string
	playerName  string
	screen      screen

	// Shared references (set by server)
	lobby *game.Lobby
	store *store.DB

	// Per-session state
	session     *game.GameSession
	roomCode    string
	gameState   game.GameState
	lobbyState  lobbyState
	classState  classSelectState
	inputBuffer string
}

func NewModel(width, height int, fingerprint string, lobby *game.Lobby, db *store.DB) Model {
	return Model{
		width:       width,
		height:      height,
		fingerprint: fingerprint,
		screen:      screenTitle,
		lobby:       lobby,
		store:       db,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case StateUpdateMsg:
		m.gameState = msg.State
		return m, nil
	}

	switch m.screen {
	case screenTitle:
		return m.updateTitle(msg)
	case screenLobby:
		return m.updateLobby(msg)
	case screenClassSelect:
		return m.updateClassSelect(msg)
	case screenGame:
		return m.updateGame(msg)
	case screenShop:
		return m.updateShop(msg)
	case screenResult:
		return m.updateResult(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.width < 80 || m.height < 24 {
		return fmt.Sprintf("Terminal too small (%dx%d). Minimum: 80x24.", m.width, m.height)
	}
	switch m.screen {
	case screenTitle:
		return renderTitle(m.width, m.height)
	case screenLobby:
		return renderLobby(m.lobbyState, m.width, m.height)
	case screenClassSelect:
		return renderClassSelect(m.classState, m.width, m.height)
	case screenGame:
		return renderGame(m.gameState, m.width, m.height)
	case screenShop:
		return renderShop(m.gameState, m.width, m.height)
	case screenResult:
		rankings, _ := m.store.TopRuns(10)
		return renderResult(m.gameState.Victory, m.gameState.FloorNum, rankings)
	}
	return ""
}

func (m Model) updateTitle(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			// Check if player has a name, if not prompt for one
			name, err := m.store.GetProfile(m.fingerprint)
			if err != nil {
				m.playerName = "Adventurer"
			} else {
				m.playerName = name
			}
			m.screen = screenLobby
			m.refreshLobbyState()
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateLobby(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "c":
			code := m.lobby.CreateRoom(m.playerName + "'s Room")
			m.lobby.JoinRoom(code, m.playerName)
			m.roomCode = code
			m.screen = screenClassSelect
		case "j":
			// Join by code — simplified: use input buffer
		case "up":
			if m.lobbyState.cursor > 0 {
				m.lobbyState.cursor--
			}
		case "down":
			if m.lobbyState.cursor < len(m.lobbyState.rooms)-1 {
				m.lobbyState.cursor++
			}
		case "enter":
			if len(m.lobbyState.rooms) > 0 {
				r := m.lobbyState.rooms[m.lobbyState.cursor]
				if err := m.lobby.JoinRoom(r.Code, m.playerName); err == nil {
					m.roomCode = r.Code
					m.screen = screenClassSelect
				}
			}
		case "q":
			m.screen = screenTitle
		}
	}
	return m, nil
}

func (m Model) updateClassSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up":
			if m.classState.cursor > 0 {
				m.classState.cursor--
			}
		case "down":
			if m.classState.cursor < len(classOptions)-1 {
				m.classState.cursor++
			}
		case "enter":
			selectedClass := classOptions[m.classState.cursor].class
			room := m.lobby.GetRoom(m.roomCode)
			if room.Session == nil {
				room.Session = game.NewGameSession()
			}
			m.session = room.Session
			player := entity.NewPlayer(m.playerName, selectedClass)
			player.Fingerprint = m.fingerprint
			m.session.AddPlayer(player)
			m.session.StartGame()
			m.gameState = m.session.GetState()
			m.screen = screenGame
			return m, m.listenForUpdates()
		}
	}
	return m, nil
}

func (m Model) updateGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.gameState.GameOver {
		m.screen = screenResult
		return m, nil
	}
	if m.gameState.Phase == game.PhaseShop {
		m.screen = screenShop
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch m.gameState.Phase {
		case game.PhaseExploring:
			// Number keys to select room
			if key.String() >= "0" && key.String() <= "9" {
				idx := int(key.String()[0] - '0')
				m.session.EnterRoom(idx)
				m.gameState = m.session.GetState()
			}
		case game.PhaseCombat:
			switch key.String() {
			case "1":
				m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionAttack, TargetIdx: 0})
			case "2":
				m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionSkill, TargetIdx: 0})
			case "3":
				m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionItem})
			case "4":
				m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionFlee})
			case "5":
				m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionWait})
			}
		}
	}
	return m, nil
}

func (m Model) updateShop(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "1", "2", "3":
			idx := int(key.String()[0] - '1')
			m.session.BuyItem(m.playerName, idx)
			m.gameState = m.session.GetState()
		case "q":
			m.session.LeaveShop()
			m.gameState = m.session.GetState()
			m.screen = screenGame
		}
	}
	return m, nil
}

func (m Model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.screen = screenLobby
			m.refreshLobbyState()
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) refreshLobbyState() {
	rooms := m.lobby.ListRooms()
	m.lobbyState.rooms = make([]roomInfo, len(rooms))
	for i, r := range rooms {
		status := "Waiting"
		if r.Status == game.RoomPlaying {
			status = "Playing"
		}
		m.lobbyState.rooms[i] = roomInfo{
			Code:    r.Code,
			Name:    r.Name,
			Players: len(r.Players),
			Status:  status,
		}
	}
	m.lobbyState.cursor = 0
}

// listenForUpdates returns a tea.Cmd that waits for state updates from GameSession
func (m Model) listenForUpdates() tea.Cmd {
	// This will be connected to GameSession's broadcast channel in Task 8
	return nil
}
```

- [ ] **Step 7: Write shop view**

`ui/shop_view.go`:

```go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/game"
)

func renderShop(state game.GameState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)

	header := headerStyle.Render("── Shop ──")
	items := ""
	for i, item := range state.ShopItems {
		items += fmt.Sprintf("  [%d] %s (+%d) — %d gold\n", i+1, item.Name, item.Bonus, item.Price)
	}

	menu := "[1-3] Buy  [Q] Leave Shop"

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		items,
		"",
		menu,
	)
}
```

- [ ] **Step 8: Build and verify compilation**

```bash
go build ./...
```

Expected: compiles without errors.

- [ ] **Step 7: Commit**

```bash
git add ui/
git commit -m "feat: TUI views — title, lobby, game map, HUD, result screen"
```

---

### Task 8: Integration — Wire Everything Together

**Files:**
- Modify: `main.go`, `server/ssh.go`

- [ ] **Step 1: Update main.go with store and lobby initialization**

```go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
)

func main() {
	db, err := store.Open("data/catacombs.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	lobby := game.NewLobby()

	go func() {
		if err := server.Start("0.0.0.0", 2222, lobby, db); err != nil {
			log.Fatal(err)
		}
	}()

	log.Println("Catacombs server running on :2222")

	// Wait for interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down...")
}
```

- [ ] **Step 2: Update server/ssh.go to pass lobby and store**

```go
package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/ui"
)

func Start(host string, port int, lobby *game.Lobby, db *store.DB) error {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
		wish.WithHostKeyPath(".ssh/catacombs_host_key"),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return true
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(func(s ssh.Session) (bubbletea.Model, []bubbletea.ProgramOption) {
				pty, _, _ := s.Pty()
				fingerprint := ""
				if s.PublicKey() != nil {
					fingerprint = ssh.FingerprintSHA256(s.PublicKey())
				}
				m := ui.NewModel(pty.Window.Width, pty.Window.Height, fingerprint, lobby, db)
				return m, []bubbletea.ProgramOption{bubbletea.WithAltScreen()}
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("could not create server: %w", err)
	}

	log.Printf("Starting SSH server on %s:%d", host, port)
	return s.ListenAndServe()
}
```

- [ ] **Step 3: Create data directory and verify build**

```bash
mkdir -p E:/projects/catacombs/data
echo "data/" >> E:/projects/catacombs/.gitignore
go build ./...
```

Expected: compiles without errors.

- [ ] **Step 4: Manual test — single player flow**

```bash
go build -o catacombs . && ./catacombs &
ssh -p 2222 -o StrictHostKeyChecking=no localhost
```

Verify: title screen → lobby → create room → select class → enter dungeon → encounter combat → complete a few turns.

- [ ] **Step 5: Manual test — two player co-op**

Open two terminals, both SSH to localhost:2222. One creates a room, the other joins by code. Start game and verify both see the same dungeon state.

- [ ] **Step 6: Commit**

```bash
git add main.go server/ .gitignore
git commit -m "feat: wire up SSH server with lobby, store, and graceful shutdown"
```

---

### Task 9: Docker & Deployment

**Files:**
- Modify: `Dockerfile`, `docker-compose.yml`

- [ ] **Step 1: Test Docker build**

```bash
docker build -t catacombs .
```

Expected: image builds successfully.

- [ ] **Step 2: Test Docker run**

```bash
docker run -p 2222:2222 --name catacombs-test catacombs &
ssh -p 2222 -o StrictHostKeyChecking=no localhost
```

Expected: game loads in terminal.

- [ ] **Step 3: Stop and clean up test container**

```bash
docker stop catacombs-test && docker rm catacombs-test
```

- [ ] **Step 4: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: Docker build and compose for deployment"
```

---

### Task 10: Polish & Final Testing

- [ ] **Step 1: Terminal size check**

Add terminal size validation on connect. If width < 80 or height < 24, display a warning message instead of the game.

- [ ] **Step 2: Chat system**

Add `/` key handler in combat view to open a one-line chat input. Messages broadcast to all party members via `GameSession`.

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v
```

Expected: all PASS.

- [ ] **Step 4: Full playthrough test**

Start server, connect 2 players via SSH, play through at least 5 floors of co-op. Verify:
- Class selection works
- Combat resolves correctly
- Turns time out properly
- Dead player sees spectator view
- Disconnect/reconnect works
- Gold is distributed correctly

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: polish — terminal size check, chat system, final testing"
```
