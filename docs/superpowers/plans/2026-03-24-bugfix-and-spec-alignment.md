# Catacombs Bug Fix & Spec Alignment Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 14 confirmed bugs and spec divergences in the Catacombs codebase — covering race conditions, game logic errors, session lifecycle leaks, and missing spec features.

**Architecture:** Incremental patches to existing files. No new packages or major restructuring. Each task is independently shippable and testable.

**Tech Stack:** Go 1.25+, charmbracelet/wish, charmbracelet/bubbletea, charmbracelet/lipgloss, go.etcd.io/bbolt

**Spec:** `docs/superpowers/specs/2026-03-23-catacombs-design.md`

**Go binary:** `go` (in PATH)

---

## File Map

```
entity/monster.go        — Task 1: add DEF scaling
entity/monster_test.go   — Task 1: test DEF scaling
game/session.go          — Task 3: deep-copy GetState; Task 11: add Stop(); Task 12: cleanup
game/session_test.go     — Task 3: race test
ui/model.go              — Task 4: fix SaveRun; Task 7: set RoomPlaying; Task 8: clamp targetCursor;
                           Task 10: use fingerprint as key; Task 11: session cleanup on result exit
game/lobby.go            — Task 7: add status transition helpers; Task 12: room removal
game/lobby_test.go       — Task 7: test status transition
combat/combat.go         — Task 5: fix taunt fallthrough
combat/combat_test.go    — Task 5: test dead warrior taunt
game/turn.go             — Task 6: multiplayer flee effect; Task 9: event log forwarding
game/event.go            — Task 9: add log messages for events
game/turn_test.go        — Task 6: test flee
server/ssh.go            — Task 13: disconnect detection
```

---

### Task 1: Monster DEF Floor Scaling

**Files:**
- Modify: `entity/monster.go:46-63`
- Test: `entity/monster_test.go`

- [ ] **Step 1: Write failing test for DEF scaling**

Add to `entity/monster_test.go`:
```go
func TestMonsterDEFScaling(t *testing.T) {
	// Slime base DEF=1, minFloor=1. At floor 5, scale = 1.15^4 ≈ 1.749
	m := NewMonster(MonsterSlime, 5)
	if m.DEF <= 1 {
		t.Errorf("Slime DEF at floor 5 should be scaled above base 1, got %d", m.DEF)
	}
	// Boss DEF should NOT scale
	boss := NewMonster(MonsterBoss5, 5)
	if boss.DEF != 8 {
		t.Errorf("Boss5 DEF should be base 8, got %d", boss.DEF)
	}
}
```

- [ ] **Step 2: Run test (should fail)**

```bash
go test ./entity/ -run TestMonsterDEFScaling -v
```

- [ ] **Step 3: Add DEF scaling to NewMonster**

In `entity/monster.go`, change line 60 from:
```go
DEF:    base.DEF,
```
to:
```go
DEF:    int(math.Round(float64(base.DEF) * scale)),
```

- [ ] **Step 4: Run test (should pass)**

```bash
go test ./entity/ -run TestMonsterDEFScaling -v
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -timeout 30s
```

- [ ] **Step 6: Commit**

```bash
git add entity/monster.go entity/monster_test.go
git commit -m "fix: scale monster DEF with floor level like HP/ATK"
```

---

### Task 2: (Skipped — Boss AoE timing confirmed correct)

CombatTurn starts at 0, `RunTurn()` increments to 1 before use. First AoE at turn 3. Correct per spec.

---

### Task 3: Fix GetState Race Condition — Deep Copy

**Files:**
- Modify: `game/session.go:155-159`
- Test: `game/session_test.go`

The root cause: `GetState()` returns the `GameState` struct by value, but `Players`, `Monsters`, and `Floor` are pointer/slice fields that still reference the same underlying data as the session. The combatLoop goroutine modifies these concurrently.

- [ ] **Step 1: Write race detection test**

Add to `game/session_test.go`:
```go
func TestGetStateNoRace(t *testing.T) {
	s := NewGameSession()
	p := entity.NewPlayer("Racer", entity.ClassWarrior)
	s.AddPlayer(p)
	s.StartGame()

	// Concurrently read state while combat might modify it
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			st := s.GetState()
			// Access player fields to trigger race detector
			for _, p := range st.Players {
				_ = p.HP
				_ = p.Gold
			}
			for _, m := range st.Monsters {
				_ = m.HP
			}
		}
	}()

	// Submit actions to trigger state changes
	for i := 0; i < 10; i++ {
		s.SubmitAction("Racer", PlayerAction{Type: ActionWait})
		time.Sleep(10 * time.Millisecond)
	}
	<-done
}
```

- [ ] **Step 2: Run with race detector (should detect race)**

```bash
go test ./game/ -run TestGetStateNoRace -race -timeout 30s -v
```

- [ ] **Step 3: Implement deep copy in GetState**

In `game/session.go`, replace `GetState()`:

```go
func (s *GameSession) GetState() GameState {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deep copy players
	players := make([]*entity.Player, len(s.state.Players))
	for i, p := range s.state.Players {
		cp := *p
		cp.Inventory = make([]entity.Item, len(p.Inventory))
		copy(cp.Inventory, p.Inventory)
		cp.Relics = make([]entity.Relic, len(p.Relics))
		copy(cp.Relics, p.Relics)
		players[i] = &cp
	}

	// Deep copy monsters
	monsters := make([]*entity.Monster, len(s.state.Monsters))
	for i, m := range s.state.Monsters {
		cm := *m
		monsters[i] = &cm
	}

	// Shallow copy floor (rooms are read-only from UI side, only Visited/Cleared change under lock)
	var floorCopy *dungeon.Floor
	if s.state.Floor != nil {
		fc := *s.state.Floor
		fc.Rooms = make([]*dungeon.Room, len(s.state.Floor.Rooms))
		for i, r := range s.state.Floor.Rooms {
			rc := *r
			rc.Neighbors = make([]int, len(r.Neighbors))
			copy(rc.Neighbors, r.Neighbors)
			fc.Rooms[i] = &rc
		}
		// Tiles are not mutated after generation — share the slice
		floorCopy = &fc
	}

	// Copy combat log
	logCopy := make([]string, len(s.state.CombatLog))
	copy(logCopy, s.state.CombatLog)

	return GameState{
		Floor:        floorCopy,
		Players:      players,
		Monsters:     monsters,
		Phase:        s.state.Phase,
		FloorNum:     s.state.FloorNum,
		TurnNum:      s.state.TurnNum,
		CombatTurn:   s.state.CombatTurn,
		SoloMode:     s.state.SoloMode,
		GameOver:     s.state.GameOver,
		Victory:      s.state.Victory,
		ShopItems:    append([]entity.Item{}, s.state.ShopItems...),
		CombatLog:    logCopy,
		TurnDeadline: s.state.TurnDeadline,
	}
}
```

- [ ] **Step 4: Run race test (should pass)**

```bash
go test ./game/ -run TestGetStateNoRace -race -timeout 30s -v
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -timeout 30s
```

- [ ] **Step 6: Commit**

```bash
git add game/session.go game/session_test.go
git commit -m "fix: deep-copy GameState in GetState to prevent data race"
```

---

### Task 4: Fix Duplicate SaveRun on Game Over

**Files:**
- Modify: `ui/model.go:282-298`

- [ ] **Step 1: Add `rankingSaved` flag to Model**

In `ui/model.go`, add field to `Model` struct (after `moveCursor`):
```go
rankingSaved bool
```

- [ ] **Step 2: Guard SaveRun with flag**

In `updateGame`, replace the game-over block (lines 288-297):
```go
	if m.gameState.GameOver {
		if m.store != nil && !m.rankingSaved {
			score := 0
			for _, p := range m.gameState.Players {
				score += p.Gold
			}
			m.store.SaveRun(m.playerName, m.gameState.FloorNum, score)
			m.rankingSaved = true
		}
		m.screen = screenResult
		return m, nil
	}
```

- [ ] **Step 3: Reset flag when returning to lobby**

In `updateResult`, after `m.screen = screenLobby`, add:
```go
m.rankingSaved = false
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add ui/model.go
git commit -m "fix: prevent duplicate SaveRun calls on game over"
```

---

### Task 5: Fix Taunt Targeting Dead Warrior

**Files:**
- Modify: `combat/combat.go:84-94`
- Test: `combat/combat_test.go`

When a warrior dies but `TauntTarget` is still true on monsters, the monster searches for a living warrior, finds none, but doesn't clear the taunt — it just falls through to random targeting. This is mostly harmless but the taunt flag stays dirty. The real issue: if there's no warrior at all and `TauntTarget` was somehow set, the search wastes cycles. Fix: clear taunt if warrior not found.

- [ ] **Step 1: Write test for dead warrior taunt fallthrough**

Add to `combat/combat_test.go`:
```go
func TestMonsterAITauntDeadWarrior(t *testing.T) {
	warrior := entity.NewPlayer("Tank", entity.ClassWarrior)
	warrior.TakeDamage(warrior.HP) // kill warrior
	mage := entity.NewPlayer("Mage", entity.ClassMage)

	m := &entity.Monster{Name: "Orc", HP: 50, ATK: 10, DEF: 5, TauntTarget: true, TauntTurns: 2}
	idx, isAoE := MonsterAI(m, []*entity.Player{warrior, mage}, 1)

	if isAoE {
		t.Error("should not AoE")
	}
	// Should target mage (index 1) since warrior is dead
	if idx != 1 {
		t.Errorf("expected target mage at index 1, got %d", idx)
	}
	// TauntTarget should be cleared since no living warrior
	if m.TauntTarget {
		t.Error("TauntTarget should be cleared when warrior is dead")
	}
}
```

- [ ] **Step 2: Run test (should fail on TauntTarget assertion)**

```bash
go test ./combat/ -run TestMonsterAITauntDeadWarrior -v
```

- [ ] **Step 3: Fix MonsterAI taunt handling**

In `combat/combat.go`, replace the taunt block (lines 88-94):
```go
	if m.TauntTarget {
		for i, p := range players {
			if !p.IsDead() && p.Class == entity.ClassWarrior {
				return i, false
			}
		}
		// No living warrior found — clear taunt
		m.TauntTarget = false
		m.TauntTurns = 0
	}
```

- [ ] **Step 4: Run test (should pass)**

```bash
go test ./combat/ -run TestMonsterAITauntDeadWarrior -v
```

- [ ] **Step 5: Run all combat tests**

```bash
go test ./combat/ -v
```

- [ ] **Step 6: Commit**

```bash
git add combat/combat.go combat/combat_test.go
git commit -m "fix: clear monster taunt when warrior is dead"
```

---

### Task 6: Multiplayer Flee — Mark Player as Fled

**Files:**
- Modify: `entity/player.go` — add `Fled` field
- Modify: `game/turn.go:145-154` — set Fled on success
- Modify: `game/turn.go:78-80` — skip fled players like dead players
- Test: `game/turn_test.go`

In multiplayer, a successful flee currently does nothing. Fix: mark the player as "fled" so they're skipped for the rest of this combat (like being dead, but without the death penalty). Reset fled status when combat ends.

- [ ] **Step 1: Add Fled field to Player**

In `entity/player.go`, add to `Player` struct:
```go
Fled bool
```

- [ ] **Step 2: Update IsDead check comment and add IsOut helper**

In `entity/player.go`, add method:
```go
// IsOut returns true if the player cannot act in combat (dead or fled)
func (p *Player) IsOut() bool {
	return p.Dead || p.Fled
}
```

- [ ] **Step 3: Update turn.go to use IsOut for action collection**

In `game/turn.go`, replace all `p.IsDead()` checks in `RunTurn()` and `resolvePlayerActions()` with `p.IsOut()`:

Line 23: `if !p.IsOut() {` (alive count)
Line 54-55: `if !p.IsOut() {` (default action)
Line 79: `if p.IsOut() {` (skip dead/fled in resolve)
Line 189: `if !p.IsOut() {` (gold award)
Line 259: `if !p.IsOut() {` (boss relic)

In `resolveMonsterActions()`:
Line 278: `if !p.IsOut() {` (AoE targets)
Line 287: `if !p.IsOut() {` (single target validation)
Line 298: `if !p.IsOut() {` (all-dead check)

- [ ] **Step 4: Set Fled on successful flee**

In `game/turn.go`, replace the ActionFlee case (lines 145-154):
```go
		case ActionFlee:
			if combat.AttemptFlee() {
				s.addLog(fmt.Sprintf("%s fled from battle!", p.Name))
				if s.state.SoloMode {
					s.state.Phase = PhaseExploring
					return
				}
				p.Fled = true
			} else {
				s.addLog(fmt.Sprintf("%s failed to flee!", p.Name))
			}
```

- [ ] **Step 5: Check if all remaining players fled — end combat**

In `resolvePlayerActions()`, after the action loop (before intents resolution), add:
```go
	// Check if all alive players have fled
	allFled := true
	for _, p := range s.state.Players {
		if !p.IsDead() && !p.Fled {
			allFled = false
			break
		}
	}
	if allFled && !s.state.SoloMode {
		s.state.Phase = PhaseExploring
		s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		s.addLog("All players fled!")
		// Reset fled status
		for _, p := range s.state.Players {
			p.Fled = false
		}
		return
	}
```

- [ ] **Step 6: Reset Fled when combat ends**

In `resolvePlayerActions()`, where combat ends (line 220-228, the `len(s.state.Monsters) == 0` block), add before phase change:
```go
		for _, p := range s.state.Players {
			p.Fled = false
		}
```

Also in `advanceFloor()` (line 244-248), add:
```go
	for _, p := range s.state.Players {
		p.Fled = false
	}
```

- [ ] **Step 7: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 30s
```

- [ ] **Step 8: Commit**

```bash
git add entity/player.go game/turn.go
git commit -m "feat: multiplayer flee marks player as out for current combat"
```

---

### Task 7: Set Room Status to Playing on Game Start

**Files:**
- Modify: `ui/model.go:251-267` (class select → game start)
- Modify: `game/lobby.go` — add `StartRoom` helper
- Test: `game/lobby_test.go`

- [ ] **Step 1: Write test for room status transition**

Add to `game/lobby_test.go`:
```go
func TestRoomStatusTransition(t *testing.T) {
	l := NewLobby()
	code := l.CreateRoom("Test")
	l.JoinRoom(code, "Alice")
	r := l.GetRoom(code)
	if r.Status != RoomWaiting {
		t.Errorf("new room should be Waiting, got %d", r.Status)
	}
	l.StartRoom(code)
	r = l.GetRoom(code)
	if r.Status != RoomPlaying {
		t.Errorf("started room should be Playing, got %d", r.Status)
	}
	// Joining a Playing room should fail
	err := l.JoinRoom(code, "Bob")
	if err == nil {
		t.Error("should not be able to join a Playing room")
	}
}
```

- [ ] **Step 2: Run test (should fail — StartRoom doesn't exist)**

```bash
go test ./game/ -run TestRoomStatusTransition -v
```

- [ ] **Step 3: Add StartRoom to lobby.go**

```go
func (l *Lobby) StartRoom(code string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room, ok := l.rooms[code]; ok {
		room.Status = RoomPlaying
	}
}
```

- [ ] **Step 4: Call StartRoom when game starts**

In `ui/model.go` `updateClassSelect`, after `m.session.StartGame()` (line 263), add:
```go
					m.lobby.StartRoom(m.roomCode)
```

- [ ] **Step 5: Run test (should pass)**

```bash
go test ./game/ -run TestRoomStatusTransition -v
```

- [ ] **Step 6: Commit**

```bash
git add game/lobby.go game/lobby_test.go ui/model.go
git commit -m "fix: set room status to Playing when game starts"
```

---

### Task 8: Clamp Target Cursor After Monster Death

**Files:**
- Modify: `ui/model.go:282-286`

- [ ] **Step 1: Add cursor clamping in updateGame**

In `updateGame`, right after `m.gameState = m.session.GetState()` (line 285), add:
```go
		// Clamp target cursor to valid range after monsters die
		if len(m.gameState.Monsters) > 0 {
			if m.targetCursor >= len(m.gameState.Monsters) {
				m.targetCursor = len(m.gameState.Monsters) - 1
			}
		} else {
			m.targetCursor = 0
		}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add ui/model.go
git commit -m "fix: clamp target cursor when monsters die"
```

---

### Task 9: Add Event Room Log Messages

**Files:**
- Modify: `game/event.go:132-148`

- [ ] **Step 1: Add log messages to triggerEvent**

Replace `triggerEvent()` in `game/event.go`:
```go
func (s *GameSession) triggerEvent() {
	for _, p := range s.state.Players {
		if p.IsDead() {
			continue
		}
		if rand.Float64() < 0.5 {
			dmg := 10 + rand.Intn(11)
			p.TakeDamage(dmg)
			s.addLog(fmt.Sprintf("Trap! %s takes %d damage", p.Name, dmg))
		} else {
			heal := 15 + rand.Intn(11)
			before := p.HP
			p.Heal(heal)
			s.addLog(fmt.Sprintf("Blessing! %s heals %d HP", p.Name, p.HP-before))
		}
	}
}
```

Also add log for treasure in `grantTreasure()`:
```go
func (s *GameSession) grantTreasure() {
	for _, p := range s.state.Players {
		if rand.Float64() < 0.5 {
			item := entity.Item{
				Name: "Iron Sword", Type: entity.ItemWeapon, Bonus: 3 + rand.Intn(6),
			}
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s found %s (ATK+%d)", p.Name, item.Name, item.Bonus))
		} else {
			item := entity.Item{
				Name: "Iron Shield", Type: entity.ItemArmor, Bonus: 2 + rand.Intn(4),
			}
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s found %s (DEF+%d)", p.Name, item.Name, item.Bonus))
		}
	}
}
```

- [ ] **Step 2: Show combat log on game screen even outside combat**

In `ui/model.go` `updateGame`, in the `PhaseExploring` branch, after `m.session.EnterRoom(roomIdx)`, `m.gameState` is already refreshed. The log will render via `renderCombatLog` which already shows `state.CombatLog`. No change needed — the log displays on the game view regardless of phase.

- [ ] **Step 3: Build**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add game/event.go
git commit -m "feat: show log messages for trap, blessing, and treasure events"
```

---

### Task 10: Use Fingerprint as Action Key (Prevent Name Collision)

**Depends on:** Task 6 (for `IsOut()` method)

**Files:**
- Modify: `game/session.go` — change `actions` map key
- Modify: `game/turn.go` — use fingerprint in action lookup
- Modify: `ui/model.go` — submit with fingerprint

The `actions` map uses `playerName` as the key. If two players share a name, one overwrites the other. Fix: use `Fingerprint` as the key.

- [ ] **Step 1: Change SubmitAction to use fingerprint**

In `game/session.go`, rename `playerActionMsg.PlayerName` to `PlayerID`:
```go
type playerActionMsg struct {
	PlayerID string
	Action   PlayerAction
}
```

Update `SubmitAction`:
```go
func (s *GameSession) SubmitAction(playerID string, action PlayerAction) {
	s.actionCh <- playerActionMsg{PlayerID: playerID, Action: action}
}
```

- [ ] **Step 2: Update RunTurn action collection**

In `game/turn.go`, line 39:
```go
			s.actions[msg.PlayerID] = msg.Action
```

- [ ] **Step 3: Update resolvePlayerActions to match by fingerprint**

In `game/turn.go` `resolvePlayerActions()`, change the action lookup (line 82):
```go
		action, ok := s.actions[p.Fingerprint]
```

- [ ] **Step 4: Update default action loop**

In `game/turn.go`, change the default-action loop (lines 54-59):
```go
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.actions[p.Fingerprint]; !ok {
				s.actions[p.Fingerprint] = PlayerAction{Type: ActionWait}
			}
		}
	}
```

- [ ] **Step 5: Generate fallback fingerprint for keyless SSH users**

In `ui/model.go` `updateTitle()`, when `m.fingerprint` is empty (password auth, no public key), generate a unique fallback ID so keyless players don't collide on `""`:
```go
			if m.fingerprint == "" {
				m.fingerprint = fmt.Sprintf("anon-%d", time.Now().UnixNano())
			}
```
Add this right after the `m.playerName` is set (before `m.screen = screenLobby`).

Also update the `NewPlayer` call in `updateClassSelect` to ensure fingerprint is set:
```go
				player := entity.NewPlayer(m.playerName, selectedClass)
				player.Fingerprint = m.fingerprint
```
(This line already exists, so no change needed here — just verify.)

- [ ] **Step 6: Update UI to submit with fingerprint**

In `ui/model.go`, change all `m.session.SubmitAction(m.playerName, ...)` calls to `m.session.SubmitAction(m.fingerprint, ...)`.

Lines 358-366: replace `m.playerName` → `m.fingerprint` in all 5 SubmitAction calls.

- [ ] **Step 7: Update BuyItem to use fingerprint**

In `game/session.go` `BuyItem()`, change `playerName` parameter to `playerID` and match by fingerprint:
```go
func (s *GameSession) BuyItem(playerID string, itemIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop || itemIdx < 0 || itemIdx >= len(s.state.ShopItems) {
		return false
	}
	item := s.state.ShopItems[itemIdx]
	for _, p := range s.state.Players {
		if p.Fingerprint == playerID && p.Gold >= item.Price {
			p.Gold -= item.Price
			p.Inventory = append(p.Inventory, item)
			return true
		}
	}
	return false
}
```

In `ui/model.go` `updateShop()`, change:
```go
m.session.BuyItem(m.fingerprint, idx)
```

- [ ] **Step 8: Update dead-player check in updateGame**

In `ui/model.go` `updateGame()`, change the dead player check (lines 340-344) to use fingerprint:
```go
			isPlayerDead := false
			for _, p := range m.gameState.Players {
				if p.Fingerprint == m.fingerprint && p.IsDead() {
					isPlayerDead = true
					break
				}
			}
```

- [ ] **Step 9: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 30s
```

Some tests may need updating if they call `SubmitAction` with a name — update to use fingerprint. Check `game/session_test.go` — the test player will need `Fingerprint` set.

- [ ] **Step 10: Commit**

```bash
git add game/session.go game/turn.go ui/model.go
git commit -m "fix: use fingerprint as player ID to prevent name collision"
```

---

### Task 11: Session Cleanup & Goroutine Leak Fix

**Files:**
- Modify: `game/session.go` — add `Stop()` method and done channel
- Modify: `ui/model.go` — call Stop on result exit

- [ ] **Step 1: Add done channel and Stop method**

In `game/session.go`, add `done` channel to `GameSession`:
```go
type GameSession struct {
	mu           sync.Mutex
	state        GameState
	started      bool
	actions      map[string]PlayerAction
	actionCh     chan playerActionMsg
	combatSignal chan struct{}
	done         chan struct{}
}
```

Update `NewGameSession`:
```go
func NewGameSession() *GameSession {
	return &GameSession{
		state: GameState{
			FloorNum: 1,
		},
		actions:      make(map[string]PlayerAction),
		actionCh:     make(chan playerActionMsg, 4),
		combatSignal: make(chan struct{}, 1),
		done:         make(chan struct{}),
	}
}
```

Add `Stop()`:
```go
func (s *GameSession) Stop() {
	select {
	case <-s.done:
		// already stopped
	default:
		close(s.done)
	}
}
```

- [ ] **Step 2: Update combatLoop to respect done channel**

Replace `combatLoop()`:
```go
func (s *GameSession) combatLoop() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.mu.Lock()
		phase := s.state.Phase
		gameOver := s.state.GameOver
		s.mu.Unlock()

		if gameOver {
			return
		}

		if phase == PhaseCombat {
			s.RunTurn()
		} else {
			select {
			case <-s.combatSignal:
			case <-s.done:
				return
			}
		}
	}
}
```

Also update `RunTurn` action collection to respect done:

In `game/turn.go`, update the select in the action collection loop (lines 36-44):
```go
	for collected < aliveCount {
		select {
		case msg := <-s.actionCh:
			s.mu.Lock()
			s.actions[msg.PlayerID] = msg.Action
			s.mu.Unlock()
			collected++
		case <-timer.C:
			goto resolve
		case <-s.done:
			timer.Stop()
			return
		}
	}
```

- [ ] **Step 3: Clean up session on result exit**

In `ui/model.go` `updateResult()`, when going back to lobby:
```go
	if isEnter(key) {
		if m.session != nil {
			m.session.Stop()
			m.session = nil
		}
		m.rankingSaved = false
		m.screen = screenLobby
		m = m.withRefreshedLobby()
	}
```

- [ ] **Step 4: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 30s
```

- [ ] **Step 5: Commit**

```bash
git add game/session.go game/turn.go ui/model.go
git commit -m "fix: stop combatLoop goroutine on session exit to prevent leak"
```

---

### Task 12: Remove Zombie Lobby Rooms on Game End

**Depends on:** Task 11 (extends `updateResult` from Task 11)

**Files:**
- Modify: `ui/model.go` — remove room from lobby on game end

- [ ] **Step 1: Remove lobby room on result exit**

In `ui/model.go` `updateResult()`, update the enter handler (extending Task 11's change):
```go
	if isEnter(key) {
		if m.session != nil {
			m.session.Stop()
			m.session = nil
		}
		if m.lobby != nil && m.roomCode != "" {
			m.lobby.RemoveRoom(m.roomCode)
		}
		m.roomCode = ""
		m.rankingSaved = false
		m.screen = screenLobby
		m = m.withRefreshedLobby()
	}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add ui/model.go
git commit -m "fix: remove lobby room when game session ends"
```

---

### Task 13: Disconnect Handling — Auto-Defend & Timeout

**Depends on:** Task 10 (fingerprint-based IDs), Task 11 (done channel)

**Files:**
- Modify: `game/session.go` — add `lastActivity` tracking, timeout check in combatLoop
- Modify: `game/event.go` — update lastActivity on EnterRoom
- Modify: `ui/model.go` — update lastActivity on exploration actions

**Design:** Track `lastActivity` timestamp per player (by fingerprint). Update it on both `SubmitAction` (combat) and `EnterRoom` (exploration). In `combatLoop`, check for 60-second inactive players and remove them. Auto-defend already works via `ActionWait` default. No SSH-layer changes needed.

- [ ] **Step 1: Add lastActivity to GameSession**

In `game/session.go`, add to `GameSession` struct:
```go
	lastActivity map[string]time.Time // fingerprint -> last activity time
```

Initialize in `NewGameSession`:
```go
	lastActivity: make(map[string]time.Time),
```

- [ ] **Step 2: Update lastActivity in SubmitAction**

```go
func (s *GameSession) SubmitAction(playerID string, action PlayerAction) {
	s.mu.Lock()
	s.lastActivity[playerID] = time.Now()
	s.mu.Unlock()
	s.actionCh <- playerActionMsg{PlayerID: playerID, Action: action}
}
```

- [ ] **Step 3: Update lastActivity in EnterRoom**

In `game/event.go` `EnterRoom()`, add at the top (after lock, before setting CurrentRoom):
```go
	// Update activity for all connected players (exploration is party-wide)
	now := time.Now()
	for _, p := range s.state.Players {
		if p.Fingerprint != "" {
			s.lastActivity[p.Fingerprint] = now
		}
	}
```

- [ ] **Step 4: Add TouchActivity method for UI heartbeat**

In `game/session.go`, add:
```go
func (s *GameSession) TouchActivity(fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity[fingerprint] = time.Now()
}
```

- [ ] **Step 5: Call TouchActivity from UI on any key press**

In `ui/model.go` `updateGame()`, add at the top (before the session GetState call):
```go
	if m.session != nil && m.fingerprint != "" {
		m.session.TouchActivity(m.fingerprint)
	}
```

- [ ] **Step 6: Initialize lastActivity in StartGame**

```go
func (s *GameSession) StartGame() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.state.SoloMode = len(s.state.Players) == 1
	now := time.Now()
	for _, p := range s.state.Players {
		s.lastActivity[p.Fingerprint] = now
	}
	s.mu.Unlock()
	s.StartFloor()
	go s.combatLoop()
}
```

- [ ] **Step 7: Add timeout check to combatLoop**

In `combatLoop`, add this block right after the `gameOver` check:
```go
		// Remove players inactive for >60 seconds
		s.mu.Lock()
		now := time.Now()
		changed := false
		remaining := make([]*entity.Player, 0, len(s.state.Players))
		for _, p := range s.state.Players {
			if p.Fingerprint != "" && !p.IsOut() {
				if last, ok := s.lastActivity[p.Fingerprint]; ok {
					if now.Sub(last) > 60*time.Second {
						s.addLog(fmt.Sprintf("%s removed (disconnected)", p.Name))
						changed = true
						continue
					}
				}
			}
			remaining = append(remaining, p)
		}
		if changed {
			s.state.Players = remaining
			if len(s.state.Players) <= 1 {
				s.state.SoloMode = true
			}
			if len(s.state.Players) == 0 {
				s.state.GameOver = true
				s.mu.Unlock()
				return
			}
		}
		s.mu.Unlock()
```

- [ ] **Step 8: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 30s
```

- [ ] **Step 9: Commit**

```bash
git add game/session.go game/event.go ui/model.go
git commit -m "feat: remove inactive players after 60s disconnect timeout"
```

---

### Task 14: Chat System (/ key)

**Files:**
- Modify: `game/session.go` — add chat method
- Modify: `ui/model.go` — add chat input mode
- Modify: `ui/game_view.go` — render chat input

- [ ] **Step 1: Add SendChat to GameSession**

In `game/session.go`, add:
```go
func (s *GameSession) SendChat(playerName, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addLog(fmt.Sprintf("[%s] %s", playerName, message))
}
```

- [ ] **Step 2: Add chat mode to Model**

In `ui/model.go`, add to `Model` struct:
```go
	chatting    bool
	chatInput   string
```

- [ ] **Step 3: Handle `/` key and chat input in updateGame**

In `updateGame`, add chat handling at the top of the `tea.KeyMsg` section (before phase switch):
```go
	if key, ok := msg.(tea.KeyMsg); ok {
		// Chat mode
		if m.chatting {
			if isEnter(key) && len(m.chatInput) > 0 {
				if m.session != nil {
					m.session.SendChat(m.playerName, m.chatInput)
					m.gameState = m.session.GetState()
				}
				m.chatting = false
				m.chatInput = ""
			} else if isKey(key, "esc") || key.Type == tea.KeyEsc {
				m.chatting = false
				m.chatInput = ""
			} else if key.Type == tea.KeyBackspace && len(m.chatInput) > 0 {
				m.chatInput = m.chatInput[:len(m.chatInput)-1]
			} else if len(key.Runes) == 1 && len(m.chatInput) < 40 {
				m.chatInput += string(key.Runes)
			}
			if m.gameState.Phase == game.PhaseCombat {
				return m, m.pollState()
			}
			return m, nil
		}

		if isKey(key, "/") {
			m.chatting = true
			m.chatInput = ""
			if m.gameState.Phase == game.PhaseCombat {
				return m, m.pollState()
			}
			return m, nil
		}

		// existing phase switch...
```

- [ ] **Step 4: Show chat input in HUD**

In `ui/game_view.go`, update `renderGame` to accept and show chat state. Add parameters:
```go
func renderGame(state game.GameState, width, height int, targetCursor int, moveCursor int, chatting bool, chatInput string) string {
```

At the bottom of `renderGame`, before return:
```go
	if chatting {
		chatStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
		chatView := chatStyle.Render(fmt.Sprintf("> %s_", chatInput))
		return lipgloss.JoinVertical(lipgloss.Left, mapView, hudView, logView, chatView)
	}
```

Update call in `ui/model.go` `View()`:
```go
	case screenGame:
		return renderGame(m.gameState, m.width, m.height, m.targetCursor, m.moveCursor, m.chatting, m.chatInput)
```

- [ ] **Step 5: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 30s
```

- [ ] **Step 6: Commit**

```bash
git add game/session.go ui/model.go ui/game_view.go
git commit -m "feat: add in-game chat with / key"
```
