# Catacombs Bug Fixes & Missing Features Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix critical bugs and implement missing features identified in the spec-vs-implementation review.

**Architecture:** Incremental fixes to existing codebase. No major restructuring — patch what's broken, add what's missing. Each task is independently shippable.

**Tech Stack:** Go 1.22+, charmbracelet/wish, charmbracelet/bubbletea, charmbracelet/lipgloss, go.etcd.io/bbolt

**Spec:** `docs/superpowers/specs/2026-03-23-catacombs-design.md`
**Review:** See "Critical Issues Summary" in conversation history.

**Go binary path:** `/c/Users/98kim/sdk/go1.25.1/bin/go.exe` (not in PATH)

---

## File Map (files touched in this plan)

```
ui/model.go          — remove debug, add nickname input, target selection, dead player guard, save ranking
ui/game_view.go      — turn timer display, target selection indicator
game/session.go      — fix StartGame race, add TurnTimeLeft to GameState, relic effects
game/turn.go         — relic effect application on kill, individual flee
game/event.go        — overlapping monster floor ranges
entity/player.go     — relic-aware EffectiveATK/EffectiveDEF
dungeon/generator.go — BSP dungeon generation with 2D tile map
dungeon/room.go      — tile-based room rendering data
dungeon/render.go    — NEW: ASCII tile map renderer
store/db.go          — no changes needed (SaveRun already exists)
```

---

### Task 1: Quick Fixes (debug removal, ranking save, nickname, dead player guard)

**Files:**
- Modify: `ui/model.go`

- [ ] **Step 1: Remove debug lines from View()**

Remove the `debug` variable and all `debug +` prefixes from `View()`. Remove `lastKey` field from Model struct.

- [ ] **Step 2: Save ranking on game over**

In `updateGame`, when `m.gameState.GameOver` is detected and screen transitions to result, call:
```go
if m.store != nil {
    score := 0
    for _, p := range m.gameState.Players {
        score += p.Gold
    }
    m.store.SaveRun(m.playerName, m.gameState.FloorNum, score)
}
```

- [ ] **Step 3: Block dead player actions**

In `updateGame` combat key handling, add guard:
```go
// Find this player and check if dead
isPlayerDead := false
for _, p := range m.gameState.Players {
    if p.Name == m.playerName && p.IsDead() {
        isPlayerDead = true
        break
    }
}
if isPlayerDead {
    return m, m.pollState() // spectator mode: just keep watching
}
```

- [ ] **Step 4: Save nickname on first connection**

In `updateTitle`, after setting `m.playerName = "Adventurer"` (when profile not found), add:
```go
if m.fingerprint != "" {
    m.store.SaveProfile(m.fingerprint, m.playerName)
}
```

- [ ] **Step 5: Build and run tests**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe build ./...
/c/Users/98kim/sdk/go1.25.1/bin/go.exe test ./ui/ -v -timeout 15s
```

- [ ] **Step 6: Commit**

```bash
git add ui/model.go
git commit -m "fix: remove debug, save rankings, block dead actions, save nickname"
```

---

### Task 2: Relic Effects

**Files:**
- Modify: `entity/player.go`, `game/turn.go`
- Test: `entity/player_test.go`

- [ ] **Step 1: Write relic effect test**

Add to `entity/player_test.go`:
```go
func TestRelicEffects(t *testing.T) {
    p := NewPlayer("test", ClassWarrior)
    p.Relics = append(p.Relics, Relic{Name: "Power Amulet", Effect: RelicATKBoost, Value: 3})
    if p.EffectiveATK() != 15 { // 12 base + 3 relic
        t.Errorf("ATK with relic: got %d, want 15", p.EffectiveATK())
    }

    p.Relics = append(p.Relics, Relic{Name: "Iron Ward", Effect: RelicDEFBoost, Value: 2})
    if p.EffectiveDEF() != 10 { // 8 base + 2 relic
        t.Errorf("DEF with relic: got %d, want 10", p.EffectiveDEF())
    }
}
```

- [ ] **Step 2: Run test (should fail)**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe test ./entity/ -run TestRelicEffects -v
```

- [ ] **Step 3: Add relic bonuses to EffectiveATK/EffectiveDEF**

In `entity/player.go`, update `EffectiveATK()`:
```go
func (p *Player) EffectiveATK() int {
    atk := p.ATK
    for _, item := range p.Inventory {
        if item.Type == ItemWeapon {
            atk += item.Bonus
        }
    }
    for _, r := range p.Relics {
        if r.Effect == RelicATKBoost {
            atk += r.Value
        }
    }
    return atk
}
```

Update `EffectiveDEF()` similarly for `RelicDEFBoost`.

- [ ] **Step 4: Add HealOnKill relic trigger in turn.go**

In `resolvePlayerActions()`, in the "Award gold" loop where `m.IsDead() && aliveBeforeTurn[i]`, add:
```go
// Apply RelicHealOnKill
for _, p := range s.state.Players {
    if !p.IsDead() {
        for _, r := range p.Relics {
            if r.Effect == entity.RelicHealOnKill {
                p.Heal(r.Value)
            }
        }
    }
}
```

And in the gold award loop, apply `RelicGoldBoost`:
```go
for _, p := range s.state.Players {
    if !p.IsDead() {
        bonus := 0
        for _, r := range p.Relics {
            if r.Effect == entity.RelicGoldBoost {
                bonus += r.Value
            }
        }
        p.Gold += goldReward + bonus
    }
}
```

- [ ] **Step 5: Run all tests**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe test ./entity/ ./game/ -v -timeout 15s
```

- [ ] **Step 6: Commit**

```bash
git add entity/player.go entity/player_test.go game/turn.go
git commit -m "feat: apply relic passive effects (ATK/DEF boost, heal on kill, gold boost)"
```

---

### Task 3: Combat Target Selection & Individual Flee

**Files:**
- Modify: `ui/model.go`, `ui/game_view.go`, `game/turn.go`

- [ ] **Step 1: Add target cursor to Model**

In `ui/model.go`, add field to Model:
```go
targetCursor int // selected enemy index during combat
```

- [ ] **Step 2: Add target selection keys**

In `updateGame` combat phase, add left/right or tab for target cycling:
```go
case "tab":
    if len(m.gameState.Monsters) > 0 {
        m.targetCursor = (m.targetCursor + 1) % len(m.gameState.Monsters)
    }
```

Update attack/skill to use `m.targetCursor`:
```go
case "1":
    m.session.SubmitAction(m.playerName, game.PlayerAction{Type: game.ActionAttack, TargetIdx: m.targetCursor})
```

- [ ] **Step 3: Show target indicator in HUD**

In `ui/game_view.go` `renderHUD`, add arrow marker to selected target:
```go
marker := "  "
if i == targetCursor {
    marker = "> "
}
sb.WriteString(enemyStyle.Render(fmt.Sprintf("%s[%d] %s ...", marker, i, m.Name)))
```

Pass `targetCursor` through `renderGame` and `renderHUD` (add parameter or embed in state).

- [ ] **Step 4: Change flee to individual**

In `game/turn.go` `resolvePlayerActions`, replace the unanimous flee block with individual flee:
```go
// Remove the fleeCount/aliveCount block entirely.
// In the per-player action switch, change ActionFlee to:
case ActionFlee:
    if combat.AttemptFlee() {
        s.addLog(fmt.Sprintf("%s fled from battle!", p.Name))
        // In solo, end combat
        if s.state.SoloMode {
            s.state.Phase = PhaseExploring
            return
        }
        // In multiplayer, mark player as fled (treat as dead for this combat)
        // For simplicity: solo flee ends combat, multiplayer flee wastes turn
    } else {
        s.addLog(fmt.Sprintf("%s failed to flee!", p.Name))
    }
```

- [ ] **Step 5: Add [Tab] hint to action bar**

In `ui/game_view.go`, update the action line:
```go
sb.WriteString(actionStyle.Render("[1]Attack  [2]Skill  [3]Item  [4]Flee  [5]Wait  [Tab]Target"))
```

- [ ] **Step 6: Build and test**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe build ./...
/c/Users/98kim/sdk/go1.25.1/bin/go.exe test ./... -timeout 15s
```

- [ ] **Step 7: Commit**

```bash
git add ui/model.go ui/game_view.go game/turn.go
git commit -m "feat: target selection with Tab, individual flee in solo"
```

---

### Task 4: Turn Timer Display

**Files:**
- Modify: `game/session.go`, `game/turn.go`, `ui/game_view.go`

- [ ] **Step 1: Add TurnDeadline to GameState**

In `game/session.go` `GameState` struct:
```go
TurnDeadline time.Time // when current turn times out
```

Import `"time"` in session.go.

- [ ] **Step 2: Set deadline in RunTurn**

In `game/turn.go` `RunTurn()`, after creating the timer:
```go
s.state.TurnDeadline = time.Now().Add(TurnTimeout)
```

After turn resolves (in resolve section):
```go
s.state.TurnDeadline = time.Time{} // clear deadline
```

- [ ] **Step 3: Display countdown in HUD**

In `ui/game_view.go` `renderHUD`, when in combat phase:
```go
if !state.TurnDeadline.IsZero() {
    remaining := time.Until(state.TurnDeadline)
    if remaining < 0 {
        remaining = 0
    }
    timerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
    sb.WriteString(timerStyle.Render(fmt.Sprintf("  Timer: %.1fs", remaining.Seconds())))
    sb.WriteString("\n")
}
```

Import `"time"` in game_view.go.

- [ ] **Step 4: Build and test**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe build ./...
```

- [ ] **Step 5: Commit**

```bash
git add game/session.go game/turn.go ui/game_view.go
git commit -m "feat: display turn countdown timer in combat HUD"
```

---

### Task 5: Fix StartGame Race & Overlapping Monster Ranges

**Files:**
- Modify: `game/session.go`, `game/event.go`

- [ ] **Step 1: Fix StartGame to only run once**

Add `started bool` field to `GameSession`:
```go
type GameSession struct {
    // ...
    started bool
}
```

Guard `StartGame()`:
```go
func (s *GameSession) StartGame() {
    s.mu.Lock()
    if s.started {
        s.mu.Unlock()
        return
    }
    s.started = true
    s.state.SoloMode = len(s.state.Players) == 1
    s.mu.Unlock()
    s.StartFloor()
    go s.combatLoop()
}
```

- [ ] **Step 2: Fix monster floor ranges to overlap**

In `game/event.go` `spawnMonsters`, replace the exclusive switch with weighted random selection from valid types:
```go
func (s *GameSession) spawnMonsters() {
    count := 1 + rand.Intn(5)
    floor := s.state.FloorNum
    s.state.Monsters = make([]*entity.Monster, count)

    // Build list of valid monster types for this floor
    type floorRange struct {
        mt       entity.MonsterType
        minFloor int
        maxFloor int
    }
    ranges := []floorRange{
        {entity.MonsterSlime, 1, 5},
        {entity.MonsterSkeleton, 3, 10},
        {entity.MonsterOrc, 6, 14},
        {entity.MonsterDarkKnight, 12, 20},
    }
    var valid []entity.MonsterType
    for _, r := range ranges {
        if floor >= r.minFloor && floor <= r.maxFloor {
            valid = append(valid, r.mt)
        }
    }
    if len(valid) == 0 {
        valid = []entity.MonsterType{entity.MonsterSlime}
    }

    for i := 0; i < count; i++ {
        mt := valid[rand.Intn(len(valid))]
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
```

- [ ] **Step 3: Build and test**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe build ./...
/c/Users/98kim/sdk/go1.25.1/bin/go.exe test ./game/ -v -timeout 15s
```

- [ ] **Step 4: Commit**

```bash
git add game/session.go game/event.go
git commit -m "fix: prevent double StartGame, use overlapping monster floor ranges"
```

---

### Task 6: BSP Dungeon with 2D Tile Map

**Files:**
- Modify: `dungeon/generator.go`, `dungeon/room.go`
- Create: `dungeon/render.go`
- Modify: `ui/game_view.go`
- Test: `dungeon/generator_test.go`

This is the biggest task. The goal is to replace the list-based room display with a proper 2D ASCII map.

- [ ] **Step 1: Update Room struct for tile coordinates**

In `dungeon/room.go`, ensure Room has proper tile-space bounds:
```go
type Room struct {
    Type      RoomType
    X, Y      int // top-left corner in tile space
    W, H      int // width, height in tiles
    CenterX   int
    CenterY   int
    Visited   bool
    Cleared   bool
    Neighbors []int
}
```

- [ ] **Step 2: Rewrite BSP generator**

Replace `dungeon/generator.go` with proper BSP:
```go
const (
    MapWidth  = 60
    MapHeight = 20
    MinRoomW  = 8
    MinRoomH  = 5
)

type bspNode struct {
    x, y, w, h int
    left, right *bspNode
    room        *Room
}

func GenerateFloor(floorNum int) *Floor {
    root := &bspNode{x: 0, y: 0, w: MapWidth, h: MapHeight}
    splitBSP(root, 0)

    var rooms []*Room
    collectRooms(root, &rooms)

    // Ensure 5-8 rooms
    for len(rooms) < 5 {
        // Re-generate if too few
        root = &bspNode{x: 0, y: 0, w: MapWidth, h: MapHeight}
        splitBSP(root, 0)
        rooms = nil
        collectRooms(root, &rooms)
    }
    if len(rooms) > 8 {
        rooms = rooms[:8]
    }

    // Assign room types
    for i := range rooms {
        rooms[i].Type = RandomRoomType()
    }
    rooms[len(rooms)-1].Type = RoomBoss

    // Connect rooms (sibling nodes + extra)
    connectBSP(root, rooms)

    // Build tile map
    tiles := buildTileMap(rooms)

    return &Floor{
        Number:      floorNum,
        Rooms:       rooms,
        CurrentRoom: 0,
        Tiles:       tiles,
        Width:       MapWidth,
        Height:      MapHeight,
    }
}
```

Add `Tiles [][]Tile`, `Width`, `Height` fields to `Floor` struct.

Define `Tile` type:
```go
type Tile int
const (
    TileWall Tile = iota
    TileFloor
    TileCorridor
    TileDoor
)
```

Implement `splitBSP`, `collectRooms`, `connectBSP`, `buildTileMap` functions.

- [ ] **Step 3: Create tile map renderer**

Create `dungeon/render.go`:
```go
func RenderFloor(floor *Floor, showFog bool) string {
    // Render tiles as ASCII: '#' wall, '.' floor, '+' door, '#' corridor
    // Mark current room's content
    // Apply fog of war: hidden rooms render as space, visited as dim
}
```

Characters:
- `#` wall
- `.` floor
- `+` door / corridor
- `@` player position (center of current room)
- `D` monster (in combat rooms)
- `$` treasure, `S` shop, `?` event, `B` boss

- [ ] **Step 4: Update game_view.go to use tile renderer**

Replace `renderMap()` in `ui/game_view.go`:
```go
func renderMap(floor *dungeon.Floor) string {
    return dungeon.RenderFloor(floor, true)
}
```

- [ ] **Step 5: Update generator_test.go**

Update existing tests to work with new generator. Verify:
- 5-8 rooms
- Exactly 1 boss room
- All rooms connected
- Tile map dimensions are MapWidth x MapHeight
- Player start room (index 0) has floor tiles

- [ ] **Step 6: Build and test**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe build ./...
/c/Users/98kim/sdk/go1.25.1/bin/go.exe test ./dungeon/ -v
```

- [ ] **Step 7: Commit**

```bash
git add dungeon/ ui/game_view.go
git commit -m "feat: BSP dungeon generation with 2D ASCII tile map rendering"
```

---

### Task 7: Lobby Join-by-Code

**Files:**
- Modify: `ui/model.go`, `ui/lobby_view.go`

- [ ] **Step 1: Add join-by-code input mode to lobby**

In `ui/model.go`, add to `lobbyState`:
```go
type lobbyState struct {
    rooms      []roomInfo
    input      string
    cursor     int
    joining    bool   // true when typing a code
    codeInput  string // accumulated code chars
}
```

- [ ] **Step 2: Handle 'j' key and code input**

In `updateLobby`:
```go
if m.lobbyState.joining {
    if isEnter(key) && len(m.lobbyState.codeInput) == 4 {
        if err := m.lobby.JoinRoom(m.lobbyState.codeInput, m.playerName); err == nil {
            m.roomCode = m.lobbyState.codeInput
            m.screen = screenClassSelect
        }
        m.lobbyState.joining = false
        m.lobbyState.codeInput = ""
    } else if isKey(key, "esc") {
        m.lobbyState.joining = false
        m.lobbyState.codeInput = ""
    } else if len(key.String()) == 1 && len(m.lobbyState.codeInput) < 4 {
        m.lobbyState.codeInput += strings.ToUpper(key.String())
    }
    return m, nil
}
// existing key handling...
if isKey(key, "j") {
    m.lobbyState.joining = true
    m.lobbyState.codeInput = ""
}
```

- [ ] **Step 3: Show code input in lobby view**

In `ui/lobby_view.go` `renderLobby`, add:
```go
if state.joining {
    inputStr := state.codeInput + strings.Repeat("_", 4-len(state.codeInput))
    sb.WriteString(fmt.Sprintf("\nEnter room code: [%s]  (Esc to cancel)", inputStr))
}
```

- [ ] **Step 4: Build**

```bash
/c/Users/98kim/sdk/go1.25.1/bin/go.exe build ./...
```

- [ ] **Step 5: Commit**

```bash
git add ui/model.go ui/lobby_view.go
git commit -m "feat: lobby join-by-code with J key and 4-char input"
```
