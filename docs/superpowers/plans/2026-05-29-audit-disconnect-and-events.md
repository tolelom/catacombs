# Audit Fixes: Disconnect-Mid-Vote & Event-Phase Gaps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix four audit findings introduced by `PhaseEvent` + diamond skill trees: (1) party stall when a player leaves mid-vote in PhaseExploring/PhaseShop/PhaseEvent, (2) solo-healer auto-revive not applied to event damage, (3) missing death log on event-induced deaths, (4) per-combat reset duplication across four spawn sites.

**Architecture:** Extract three quorum-tally helpers shared by submit handlers and a new `recheckPhaseQuorumLocked` that `RemovePlayer` calls. Extract a `tryReviveSoloHealerLocked` helper used by both combat death paths and the event-damage path. Extract a `resetPerCombatStateLocked` helper called by all four combat-entry sites. No new files — all changes land in `game/session.go`, `game/turn.go`, `game/event.go` and `_test.go` siblings.

**Tech Stack:** Go 1.25, BoltDB, Bubble Tea (UI untouched). Spec: `docs/superpowers/specs/2026-05-28-audit-disconnect-and-events-design.md`.

---

## File Structure (locked decisions)

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `game/session.go` | Add `tallyMoveQuorumLocked`, `tallyEventQuorumLocked`, `tallyShopLeaversLocked`, `recheckPhaseQuorumLocked`; refactor `SubmitMoveVote`/`SubmitEventVote`/`LeaveShop` to use them; call `recheckPhaseQuorumLocked` from `RemovePlayer`; replace inline PhaseEvent block in `combatLoop` |
| Modify | `game/turn.go` | Add `tryReviveSoloHealerLocked`; replace duplicated revive blocks in `resolvePlayerActions` + `resolveMonsterActions` (both AoE and single-target paths) |
| Modify | `game/event.go` | Add `resetPerCombatStateLocked`; call from `spawnMonsters`, `spawnBoss`, `spawnMiniBoss`, `handleMimic`; route `applyEventOutcome`'s death path through `tryReviveSoloHealerLocked` |
| Modify | `game/session_test.go` | Add three `TestRemovePlayerResolves*` tests |
| Modify | `game/event_choice_test.go` | Add `TestEventDamageRevivesSoloHealer`, `TestEventDamageLogsDeath`, `TestResetPerCombatStateClearsTimeGlass` |

Each file keeps its existing responsibility; this PR only adds helpers and routes through them.

---

## Task 1: Extract `tallyMoveQuorumLocked` and route `SubmitMoveVote` through it

**Files:**
- Modify: `game/session.go` (around `SubmitMoveVote`, ~lines 956–1028)

This is a pure refactor — no behavior change. The helper returns `(winnerRoom int, met bool)` so that the new `recheckPhaseQuorumLocked` in Task 4 can call the same tally without duplicating logic.

- [ ] **Step 1: Add the helper above `SubmitMoveVote`**

Insert this function in `game/session.go` immediately above `SubmitMoveVote` (after `LeaveShop`):

```go
// tallyMoveQuorumLocked returns the winning room index and true if every
// alive player has cast a move vote. Returns (-1, false) when quorum is
// not met or no alive players remain. Tie-break: lowest room index.
// Caller holds s.mu.
func (s *GameSession) tallyMoveQuorumLocked() (int, bool) {
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		return -1, false
	}
	voteCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.moveVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}
	if voteCount < aliveCount {
		return -1, false
	}
	tally := make(map[int]int)
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if room, ok := s.moveVotes[p.Fingerprint]; ok {
				tally[room]++
			}
		}
	}
	bestRoom := -1
	bestCount := 0
	for room, count := range tally {
		if count > bestCount || (count == bestCount && (bestRoom < 0 || room < bestRoom)) {
			bestRoom = room
			bestCount = count
		}
	}
	if bestRoom < 0 {
		return -1, false
	}
	return bestRoom, true
}
```

- [ ] **Step 2: Rewrite `SubmitMoveVote` to use the helper**

Replace the body of `SubmitMoveVote` (the quorum + tally block, currently lines ~977–1027) so the function becomes:

```go
func (s *GameSession) SubmitMoveVote(fingerprint string, roomIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Phase != PhaseExploring {
		return false
	}

	s.lastActivity[fingerprint] = time.Now()

	if s.moveVotes == nil {
		s.moveVotes = make(map[string]int)
	}
	s.moveVotes[fingerprint] = roomIdx

	// Mirror to snapshot map for UI display.
	s.state.MoveVotes = make(map[string]int, len(s.moveVotes))
	for k, v := range s.moveVotes {
		s.state.MoveVotes[k] = v
	}

	winner, met := s.tallyMoveQuorumLocked()
	if !met {
		return false
	}

	// Clear votes and execute the move (caller holds s.mu).
	s.moveVotes = nil
	s.state.MoveVotes = nil
	s.enterRoomLocked(winner)
	return true
}
```

- [ ] **Step 3: Verify the existing move-vote tests still pass**

Run: `go test ./game/ -run 'TestSubmitMoveVote|TestMove'`
Expected: PASS (no behavior change). If there is no `TestSubmitMoveVote` yet, run the full game suite to catch regressions: `go test ./game/`

- [ ] **Step 4: Commit**

```bash
git add game/session.go
git commit -m "refactor(game): extract tallyMoveQuorumLocked from SubmitMoveVote"
```

---

## Task 2: Extract `tallyEventQuorumLocked` and route `SubmitEventVote` through it

**Files:**
- Modify: `game/session.go` (around `SubmitEventVote`, ~lines 1042–1100)

Pure refactor. **Important per spec risk note:** the anonymous-fingerprint solo test path (`aliveCount==1 && fingerprint=="" && len(s.eventVotes)==0`) stays inline in `SubmitEventVote` — the helper does not know about a "current submitter" so it cannot make that special case correctly. Existing tests cover this (`TestSubmitEventVote_SoloModeImmediateResolve`).

- [ ] **Step 1: Add the helper above `SubmitEventVote`**

Insert this function in `game/session.go` immediately above `SubmitEventVote`:

```go
// tallyEventQuorumLocked returns true if every alive player has cast an
// event vote. Does NOT compute a winner — winner selection lives in
// resolveEventVoteLocked. Returns false when no alive players remain.
// The anonymous-solo special case stays in SubmitEventVote because it
// depends on the caller's fingerprint context. Caller holds s.mu.
func (s *GameSession) tallyEventQuorumLocked() bool {
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		return false
	}
	voteCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.eventVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}
	return voteCount >= aliveCount
}
```

- [ ] **Step 2: Rewrite the quorum block in `SubmitEventVote`**

Replace the block from `// Quorum check:` through `s.resolveEventVoteLocked(); return true` (currently lines ~1069–1099) with:

```go
	// Defensive: nobody alive to receive the outcome. Clear and bail.
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		s.clearPendingEventLocked(true)
		return true
	}
	// Anonymous single-player test path: the caller passed no fingerprint so
	// the vote did not land in s.eventVotes — treat this call as the sole
	// alive vote.
	if aliveCount == 1 && fingerprint == "" && len(s.eventVotes) == 0 {
		s.resolveEventVoteLocked()
		return true
	}
	if !s.tallyEventQuorumLocked() {
		return false
	}
	s.resolveEventVoteLocked()
	return true
```

- [ ] **Step 3: Verify the existing event-vote tests still pass**

Run: `go test ./game/ -run 'TestSubmitEventVote|TestEventVote|TestApplyEventOutcome|TestPendingEvent|TestInactivitySweep'`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add game/session.go
git commit -m "refactor(game): extract tallyEventQuorumLocked from SubmitEventVote"
```

---

## Task 3: Extract `tallyShopLeaversLocked` and route `LeaveShop` through it

**Files:**
- Modify: `game/session.go` (around `LeaveShop`, ~lines 915–951)

Pure refactor. The helper returns `(ready bool)` — `LeaveShop` keeps responsibility for clearing `shopLeavers` and flipping the phase.

- [ ] **Step 1: Add the helper above `LeaveShop`**

Insert this function in `game/session.go` immediately above `LeaveShop`:

```go
// tallyShopLeaversLocked returns true when every available (non-out)
// player has signaled they are ready to leave the shop. Treats a zero-
// available party as ready so a wipe in the shop doesn't deadlock.
// Caller holds s.mu.
func (s *GameSession) tallyShopLeaversLocked() bool {
	available := 0
	ready := 0
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		available++
		if p.Fingerprint != "" && s.shopLeavers[p.Fingerprint] {
			ready++
		}
	}
	if available == 0 {
		return true
	}
	return ready >= available
}
```

- [ ] **Step 2: Rewrite `LeaveShop` to use the helper**

Replace the counting block in `LeaveShop` (currently lines ~927–944) so the function becomes:

```go
func (s *GameSession) LeaveShop(fingerprint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop {
		return false
	}
	if s.shopLeavers == nil {
		s.shopLeavers = make(map[string]bool)
	}
	if fingerprint != "" {
		s.shopLeavers[fingerprint] = true
	}
	// Anonymous solo test path: empty fingerprint with a single alive player
	// means the caller is the only one available — treat as ready.
	if fingerprint == "" {
		available := 0
		for _, p := range s.state.Players {
			if !p.IsOut() {
				available++
			}
		}
		if available == 1 {
			s.shopLeavers = nil
			s.state.Phase = PhaseExploring
			if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
				s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
			}
			return true
		}
	}
	if !s.tallyShopLeaversLocked() {
		return false
	}
	s.shopLeavers = nil
	s.state.Phase = PhaseExploring
	if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
		s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
	}
	return true
}
```

- [ ] **Step 3: Verify the existing shop-leave tests still pass**

Run: `go test ./game/ -run 'TestLeaveShop'`
Expected: PASS — `TestLeaveShopWaitsForAllPlayers`, `TestLeaveShopExposesLeaversForUI`, `TestLeaveShopIgnoresFledPlayers` all green.

- [ ] **Step 4: Commit**

```bash
git add game/session.go
git commit -m "refactor(game): extract tallyShopLeaversLocked from LeaveShop"
```

---

## Task 4: Add `recheckPhaseQuorumLocked` and wire it into `RemovePlayer` (fix #1)

**Files:**
- Modify: `game/session.go` (`RemovePlayer` at ~line 394, plus new helper)
- Modify: `game/session_test.go` (three new tests)

This is the behavior fix. Tests first, then helper, then the wire-in.

- [ ] **Step 1: Write the three failing regression tests**

Append to `game/session_test.go`:

```go
// TestRemovePlayerResolvesMoveVote — a 2-player party in PhaseExploring
// where A has voted and B disconnects must auto-advance instead of
// stalling forever waiting for B's vote.
func TestRemovePlayerResolvesMoveVote(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassWarrior)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	// Hand-build a two-room floor (room 0 = current, room 1 = empty target).
	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms: []*dungeon.Room{
			{Type: dungeon.RoomEmpty, Cleared: true},
			{Type: dungeon.RoomEmpty, Neighbors: []int{0}},
		},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.Phase = PhaseExploring
	s.mu.Unlock()

	// A votes for room 1; B does not vote.
	if s.SubmitMoveVote("fp-a", 1) {
		t.Fatal("partial vote should not resolve")
	}
	if s.GetState().Phase != PhaseExploring {
		t.Fatal("phase should remain PhaseExploring before B leaves")
	}

	// B disconnects — quorum should auto-recheck and advance the party.
	s.RemovePlayer("fp-b")

	st := s.GetState()
	if st.Floor.CurrentRoom != 1 {
		t.Errorf("expected CurrentRoom=1 after auto-advance, got %d", st.Floor.CurrentRoom)
	}
}

// TestRemovePlayerResolvesShopLeave — a 2-player party in PhaseShop where
// A has pressed leave and B drops must flip back to PhaseExploring.
func TestRemovePlayerResolvesShopLeave(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomShop}},
		CurrentRoom: 0,
	}
	s.state.Phase = PhaseShop
	s.mu.Unlock()

	if s.LeaveShop("fp-a") {
		t.Fatal("LeaveShop should not transition while B is still browsing")
	}
	s.RemovePlayer("fp-b")
	if got := s.GetState().Phase; got != PhaseExploring {
		t.Errorf("phase should be PhaseExploring after B leaves, got %v", got)
	}
}

// TestRemovePlayerResolvesEventVote — a 2-player party in PhaseEvent where
// A has voted and B drops must resolve the vote and clear PendingEvent
// without requiring a redundant re-submit.
func TestRemovePlayerResolvesEventVote(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	a.HP = a.MaxHP - 10
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	b.HP = b.MaxHP - 10
	s.AddPlayer(a)
	s.AddPlayer(b)
	enterDeterministicEvent(t, s, "shrine")

	if s.SubmitEventVote("fp-a", 0) {
		t.Fatal("partial event vote should not resolve")
	}
	s.RemovePlayer("fp-b")

	st := s.GetState()
	if st.Phase != PhaseExploring {
		t.Errorf("phase should be PhaseExploring after auto-resolve, got %v", st.Phase)
	}
	if st.PendingEvent != nil {
		t.Error("PendingEvent must be cleared after auto-resolve")
	}
}
```

(The `enterDeterministicEvent` helper already exists in `game/event_choice_test.go` — same package, so it is visible to this test file.)

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./game/ -run 'TestRemovePlayerResolves'`
Expected: all three FAIL — phase does not change after `RemovePlayer` because no quorum re-check fires.

- [ ] **Step 3: Add `recheckPhaseQuorumLocked` helper**

Insert this function in `game/session.go` immediately above `RemovePlayer`:

```go
// recheckPhaseQuorumLocked runs the per-phase quorum tally and resolves
// the phase if quorum is met. Used by RemovePlayer and the combatLoop
// inactivity sweep so a player leaving (explicit Q or 60s timeout) cannot
// strand the party waiting for a vote they will never cast. Caller holds
// s.mu.
func (s *GameSession) recheckPhaseQuorumLocked() {
	switch s.state.Phase {
	case PhaseExploring:
		if winner, met := s.tallyMoveQuorumLocked(); met {
			s.moveVotes = nil
			s.state.MoveVotes = nil
			s.enterRoomLocked(winner)
		}
	case PhaseShop:
		if !s.tallyShopLeaversLocked() {
			return
		}
		s.shopLeavers = nil
		s.state.Phase = PhaseExploring
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
	case PhaseEvent:
		// Mirror the sweep behavior: prune any stale votes for fingerprints
		// no longer in the alive set (RemovePlayer already deletes from the
		// maps but defense-in-depth keeps the tally correct if a future
		// caller forgets).
		alive := make(map[string]bool, len(s.state.Players))
		for _, p := range s.state.Players {
			if !p.IsOut() && p.Fingerprint != "" {
				alive[p.Fingerprint] = true
			}
		}
		for fp := range s.eventVotes {
			if !alive[fp] {
				delete(s.eventVotes, fp)
				delete(s.state.EventVotes, fp)
			}
		}
		if len(alive) == 0 {
			s.clearPendingEventLocked(true)
			return
		}
		if s.tallyEventQuorumLocked() {
			s.resolveEventVoteLocked()
		}
	}
}
```

- [ ] **Step 4: Call the helper at the end of `RemovePlayer`**

In `game/session.go`, in `RemovePlayer`, insert the helper call after the final `delete(s.activeRelicUsed, fingerprint)` line and before the `// Wake RunTurn's collect loop` comment block. The relevant region becomes:

```go
	delete(s.coopUsedThisCombat, fingerprint)
	delete(s.healerReviveUsed, fingerprint)
	delete(s.scoutUsedThisFloor, fingerprint)
	delete(s.soloHealerRevived, fingerprint)
	delete(s.coopDamageReduction, fingerprint)
	delete(s.invincibleNextTurn, fingerprint)
	delete(s.dodgeBonusThisTurn, fingerprint)
	delete(s.extraActionNext, fingerprint)
	delete(s.activeRelicUsed, fingerprint)
	// The leaver might have been the last voter the party was waiting on —
	// re-evaluate the current phase's quorum so we don't deadlock.
	s.recheckPhaseQuorumLocked()
	// Wake RunTurn's collect loop in case it's blocked waiting for this
	// player's never-arriving action.
	select {
	case s.playerLeftCh <- struct{}{}:
	default:
	}
	return len(s.state.Players)
}
```

- [ ] **Step 5: Run the three new tests to verify they pass**

Run: `go test ./game/ -run 'TestRemovePlayerResolves'`
Expected: PASS (all three).

- [ ] **Step 6: Run the broader regression suite**

Run: `go test ./game/`
Expected: PASS — all existing tests still green (especially `TestRemovePlayer`, `TestRemovePlayerClearsShopLeavers`, `TestRunTurnUnblocksOnPlayerLeave`, `TestEventVote_PlayerLeaveReevaluates`).

- [ ] **Step 7: Commit**

```bash
git add game/session.go game/session_test.go
git commit -m "fix(game): re-check phase quorum on RemovePlayer to unstick mid-vote leaves"
```

---

## Task 5: Replace inline `PhaseEvent` block in `combatLoop` sweep with `recheckPhaseQuorumLocked`

**Files:**
- Modify: `game/session.go` (`combatLoop`, ~lines 337–362)

Pure refactor — the inline block was the proto-version of the helper, exercised by `TestInactivitySweepReleasesEventDeadlock`. After this replacement, sweeps that drop a player during PhaseExploring or PhaseShop will also re-check (today they don't).

- [ ] **Step 1: Replace the inline block**

In `game/session.go`, find the block inside `combatLoop` that starts with `if s.state.Phase == PhaseEvent {` (after `if changed { ... if len(s.state.Players) == 0 { ... } `) and ends with the closing brace before `}` of `if changed`. Replace the entire `if s.state.Phase == PhaseEvent { ... }` block with a single line:

```go
			s.recheckPhaseQuorumLocked()
```

The surrounding `changed` branch should now read:

```go
		if changed {
			s.state.Players = remaining
			if len(s.state.Players) == 0 {
				s.state.GameOver = true
				s.mu.Unlock()
				return
			}
			// One or more disconnected players were swept — re-check the
			// active phase's quorum so the party isn't stranded on a vote
			// that will never complete.
			s.recheckPhaseQuorumLocked()
		}
		s.mu.Unlock()
```

- [ ] **Step 2: Update `TestInactivitySweepReleasesEventDeadlock` to invoke the helper**

The existing test manually replicates the inline cleanup block. Now that cleanup lives in `recheckPhaseQuorumLocked`, the test should call the helper directly. In `game/session_test.go`, replace the body of `TestInactivitySweepReleasesEventDeadlock` from the `s.mu.Lock()` after dropping fp-c through the closing `s.mu.Unlock()` with:

```go
	// Manually drop fp-c the way the inactivity sweep does (direct
	// mutation, NOT via RemovePlayer), then invoke recheckPhaseQuorumLocked
	// — that is exactly what the live sweep now calls.
	s.mu.Lock()
	kept := make([]*entity.Player, 0, len(s.state.Players))
	for _, p := range s.state.Players {
		if p.Fingerprint != "fp-c" {
			kept = append(kept, p)
		}
	}
	s.state.Players = kept
	s.recheckPhaseQuorumLocked()
	s.mu.Unlock()
```

- [ ] **Step 3: Run the sweep test**

Run: `go test ./game/ -run TestInactivitySweepReleasesEventDeadlock`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add game/session.go game/session_test.go
git commit -m "refactor(game): combatLoop sweep routes through recheckPhaseQuorumLocked"
```

---

## Task 6: Extract `tryReviveSoloHealerLocked` and replace the three combat death sites

**Files:**
- Modify: `game/turn.go` (`resolvePlayerActions` ~line 150, `resolveMonsterActions` AoE ~line 605, single-target ~line 677)

Pure refactor (existing tests stay green). Sets up Task 7 which uses the same helper for the event path.

- [ ] **Step 1: Add the helper at the bottom of `game/turn.go`**

Append this function at the end of `game/turn.go`:

```go
// tryReviveSoloHealerLocked handles a player's death uniformly: if the
// once-per-run solo-healer auto-revive applies, restores the player to
// 30% HP and logs the revive; otherwise logs the rip line and flips
// AnyPlayerDied for the no_death achievement. Returns true when revived.
// Burning the revive on event damage is intentional — any death counts
// per the Divine Grace design. Caller holds s.mu. Only call when
// p.IsDead() == true.
func (s *GameSession) tryReviveSoloHealerLocked(p *entity.Player) bool {
	if s.state.SoloMode && p.Class == entity.ClassHealer && !s.soloHealerRevived[p.Fingerprint] {
		p.Revive(0.30)
		s.soloHealerRevived[p.Fingerprint] = true
		s.addLog(fmt.Sprintf("✦ %s 신성 가호로 30%%HP로 부활!", p.Name))
		return true
	}
	s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
	s.state.AnyPlayerDied = true
	return false
}
```

- [ ] **Step 2: Replace the player-tick death block in `resolvePlayerActions`**

In `game/turn.go`, in `resolvePlayerActions`, find this block (currently around lines 150–159):

```go
			if p.IsDead() {
				if s.state.SoloMode && p.Class == entity.ClassHealer && !s.soloHealerRevived[p.Fingerprint] {
					p.Revive(0.30)
					s.soloHealerRevived[p.Fingerprint] = true
					s.addLog(fmt.Sprintf("✦ %s 신성 가호로 30%%HP로 부활!", p.Name))
				} else {
					s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
					s.state.AnyPlayerDied = true
				}
			}
```

Replace with:

```go
			if p.IsDead() {
				s.tryReviveSoloHealerLocked(p)
			}
```

- [ ] **Step 3: Replace the AoE-attack death block in `resolveMonsterActions`**

In `game/turn.go`, in `resolveMonsterActions`, find the matching block in the AoE branch (currently around lines 605–614):

```go
						if p.IsDead() {
							if s.state.SoloMode && p.Class == entity.ClassHealer && !s.soloHealerRevived[p.Fingerprint] {
								p.Revive(0.30)
								s.soloHealerRevived[p.Fingerprint] = true
								s.addLog(fmt.Sprintf("✦ %s 신성 가호로 30%%HP로 부활!", p.Name))
							} else {
								s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
								s.state.AnyPlayerDied = true
							}
						}
```

Replace with:

```go
						if p.IsDead() {
							s.tryReviveSoloHealerLocked(p)
						}
```

- [ ] **Step 4: Replace the single-target death block in `resolveMonsterActions`**

In the same function, find the duplicate block in the single-target branch (currently around lines 677–686). The contents are identical — apply the same replacement.

- [ ] **Step 5: Verify combat tests still pass**

Run: `go test ./game/ -run 'TestTurn|TestRunTurn|TestSoloScaling|TestSolo|TestCombat'`
Expected: PASS — the helper preserves exactly the prior behavior at all three sites.

- [ ] **Step 6: Commit**

```bash
git add game/turn.go
git commit -m "refactor(combat): extract tryReviveSoloHealerLocked for the three death sites"
```

---

## Task 7: Route `applyEventOutcome` through `tryReviveSoloHealerLocked` (fixes #2 + #3)

**Files:**
- Modify: `game/event.go` (`applyEventOutcome` HP-loss branch, ~lines 472–481)
- Modify: `game/event_choice_test.go` (two new tests)

This is the behavior fix for solo-healer revive on event damage AND the missing death-log message on event-induced deaths.

- [ ] **Step 1: Write the two failing tests**

Append to `game/event_choice_test.go`:

```go
// TestEventDamageRevivesSoloHealer — a solo healer with the once-per-run
// Divine Grace untouched must auto-revive at 30% HP when a negative-HP
// event outcome would otherwise kill them. Mirrors the in-combat behavior.
func TestEventDamageRevivesSoloHealer(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Cleric", entity.ClassHealer)
	p.Fingerprint = "fp-cleric"
	p.HP = 5
	s.AddPlayer(p)
	s.mu.Lock()
	s.state.SoloMode = true
	s.applyEventOutcome(EventOutcome{HPChange: -100})
	s.mu.Unlock()

	st := s.GetState()
	want := st.Players[0].MaxHP * 30 / 100
	if got := st.Players[0].HP; got != want {
		t.Errorf("expected solo healer revived to %d HP (30%%), got %d", want, got)
	}
	s.mu.Lock()
	revived := s.soloHealerRevived["fp-cleric"]
	s.mu.Unlock()
	if !revived {
		t.Error("soloHealerRevived flag must be set after the auto-revive")
	}
}

// TestEventDamageLogsDeath — in a multi-player party an event outcome
// that kills a player must emit the standard "☠ X 쓰러졌습니다!" log so
// the UI shows the same death feedback as combat. Regression for the
// missing event-death log audit finding.
func TestEventDamageLogsDeath(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("Alive", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	a.HP = 999
	a.MaxHP = 999
	b := entity.NewPlayer("Doomed", entity.ClassMage)
	b.Fingerprint = "fp-b"
	b.HP = 1
	s.AddPlayer(a)
	s.AddPlayer(b)
	s.mu.Lock()
	s.applyEventOutcome(EventOutcome{HPChange: -500})
	s.mu.Unlock()

	logs := s.GetState().CombatLog
	found := false
	for _, line := range logs {
		if strings.Contains(line, "쓰러졌습니다") && strings.Contains(line, "Doomed") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a 쓰러졌습니다 log for Doomed, got %v", logs)
	}
}
```

Add `"strings"` to the file's import block (alongside `"testing"`).

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./game/ -run 'TestEventDamageRevivesSoloHealer|TestEventDamageLogsDeath'`
Expected: both FAIL — revive test fails because event path bypasses revive (HP stays at 0 or negative-clamped); log test fails because event path only emits "X N 피해를 받음" with no death line.

- [ ] **Step 3: Route the event HP-loss path through the helper**

In `game/event.go`, in `applyEventOutcome`, find the HP-loss branch (currently lines 472–481):

```go
	} else if o.HPChange < 0 {
		dmg := -o.HPChange
		for _, p := range alive {
			p.TakeDamage(dmg)
			if p.Dead {
				s.state.AnyPlayerDied = true
			}
			s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, dmg))
		}
	}
```

Replace with:

```go
	} else if o.HPChange < 0 {
		dmg := -o.HPChange
		for _, p := range alive {
			p.TakeDamage(dmg)
			s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, dmg))
			if p.IsDead() {
				// Uniform with combat: try solo-healer auto-revive,
				// otherwise log the rip and flip AnyPlayerDied.
				s.tryReviveSoloHealerLocked(p)
			}
		}
	}
```

(Note: switching from `p.Dead` to `p.IsDead()` matches the rest of the codebase — and the helper itself needs `IsDead()`-consistent semantics.)

- [ ] **Step 4: Run the new tests + the existing event-outcome tests**

Run: `go test ./game/ -run 'TestEventDamage|TestApplyEventOutcome'`
Expected: PASS — both new tests green, and `TestApplyEventOutcomeSetsAnyPlayerDiedOnLethalDamage` still passes because the helper also flips `AnyPlayerDied` on non-revive deaths.

- [ ] **Step 5: Commit**

```bash
git add game/event.go game/event_choice_test.go
git commit -m "fix(events): route lethal event damage through solo-healer revive + emit death log"
```

---

## Task 8: Extract `resetPerCombatStateLocked` and call from all four spawn sites (fix #4)

**Files:**
- Modify: `game/event.go` (`spawnMonsters` ~line 298, `spawnBoss` ~line 337, `spawnMiniBoss` ~line 666, `handleMimic` ~line 108)
- Modify: `game/event_choice_test.go` (one new test)

Pure refactor at the call sites; the helper centralizes the 5 reset operations so adding a future per-combat flag means editing one place.

- [ ] **Step 1: Write the failing test**

Append to `game/event_choice_test.go`:

```go
// TestResetPerCombatStateClearsTimeGlass — adding any per-combat flag
// must clear from a single helper, so an extraActionNext entry pre-set
// before the helper runs must be wiped afterward. Regression guard for
// the historical "handleMimic forgot to clear Time Glass" drift.
func TestResetPerCombatStateClearsTimeGlass(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Bearer", entity.ClassWarrior)
	p.Fingerprint = "fp-bearer"
	s.AddPlayer(p)

	s.mu.Lock()
	s.extraActionNext = map[string]bool{"fp-bearer": true}
	s.coopUsedThisCombat = map[string]bool{"fp-bearer": true}
	s.activeRelicUsed = map[string]bool{"fp-bearer": true}
	s.resetPerCombatStateLocked()
	s.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.extraActionNext) != 0 {
		t.Errorf("extraActionNext should be empty after reset, got %v", s.extraActionNext)
	}
	if len(s.coopUsedThisCombat) != 0 {
		t.Errorf("coopUsedThisCombat should be empty after reset, got %v", s.coopUsedThisCombat)
	}
	if len(s.activeRelicUsed) != 0 {
		t.Errorf("activeRelicUsed should be empty after reset, got %v", s.activeRelicUsed)
	}
}
```

- [ ] **Step 2: Run the new test to verify it fails**

Run: `go test ./game/ -run TestResetPerCombatStateClearsTimeGlass`
Expected: FAIL — `s.resetPerCombatStateLocked` undefined.

- [ ] **Step 3: Add the helper**

Insert in `game/event.go` immediately above `spawnMonsters`:

```go
// resetPerCombatStateLocked refreshes every map and flag scoped to a
// single combat encounter: skill uses, coop-used, active-relic-used,
// Time Glass extra actions, plus the pending Venomous Edge poison
// application. Every combat-entry site (spawnMonsters, spawnBoss,
// spawnMiniBoss, handleMimic) calls this so adding a new per-combat
// flag means editing one place, not four. Caller holds s.mu.
func (s *GameSession) resetPerCombatStateLocked() {
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.applyPoisonNextCombatIfPending()
}
```

- [ ] **Step 4: Replace the inline block in `spawnMonsters`**

In `game/event.go`, in `spawnMonsters`, replace the trailing block (currently lines ~299–305):

```go
	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.applyPoisonNextCombatIfPending()
```

with:

```go
	s.resetPerCombatStateLocked()
```

- [ ] **Step 5: Replace the inline block in `spawnBoss`**

In `spawnBoss`, replace the trailing block (currently lines ~338–344):

```go
	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.applyPoisonNextCombatIfPending()
```

with:

```go
	s.resetPerCombatStateLocked()
```

- [ ] **Step 6: Replace the inline block in `spawnMiniBoss`**

In `spawnMiniBoss`, replace the trailing block (currently lines ~667–673):

```go
	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.applyPoisonNextCombatIfPending()
```

with:

```go
	s.resetPerCombatStateLocked()
```

- [ ] **Step 7: Replace the inline block in `handleMimic`**

In `handleMimic`, the reset block lives BEFORE `signalCombat()` (currently lines ~108–115). Replace this block:

```go
		// Match other combat-start sites: reset every per-combat map and
		// apply pending Venomous Edge poison. Without this, Time Glass's
		// extraActionNext would carry over from the prior combat into the
		// mimic fight (free attack), and Venomous Edge would skip the mimic.
		s.coopUsedThisCombat = make(map[string]bool)
		s.activeRelicUsed = make(map[string]bool)
		s.extraActionNext = make(map[string]bool)
		s.applyPoisonNextCombatIfPending()
		s.signalCombat()
		for _, p := range s.state.Players {
			p.SkillUses = s.effectiveSkillUses()
		}
```

with:

```go
		s.resetPerCombatStateLocked()
		s.signalCombat()
```

- [ ] **Step 8: Run the new test plus combat regression**

Run: `go test ./game/ -run 'TestResetPerCombatState|TestTurn|TestRunTurn|TestSolo'`
Expected: PASS — the new helper test and all combat regressions stay green.

- [ ] **Step 9: Commit**

```bash
git add game/event.go game/event_choice_test.go
git commit -m "refactor(combat): centralize per-combat reset in one helper"
```

---

## Task 9: Final verification — full suite, race detector, vet

**Files:** (none modified)

- [ ] **Step 1: Run the full test suite under the race detector**

Run: `go test -race ./...`
Expected: PASS — no race warnings, all packages green. (Race coverage is critical for `recheckPhaseQuorumLocked` because it now runs from both `RemovePlayer` and `combatLoop`.)

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no output (clean).

- [ ] **Step 3: Spot-check no stray references to the old inline blocks**

Run: `grep -n "s.coopUsedThisCombat = make" game/*.go` (or use the project's Grep tool)
Expected: only one match — the line inside `resetPerCombatStateLocked` in `game/event.go`. Any other match means a spawn site was missed.

Run: `grep -n "신성 가호로 30%%HP로 부활" game/*.go`
Expected: only one match — the line inside `tryReviveSoloHealerLocked` in `game/turn.go`.

- [ ] **Step 4: Final no-op commit if nothing was missed**

If steps 1–3 all clean, the PR is done — no further commit needed. Otherwise fix the stragglers and amend a quick cleanup commit:

```bash
git add -A
git commit -m "chore: remove leftover duplicates of refactored blocks"
```

---

## Self-Review (already done — list maintained for the executor's reference)

**Spec coverage check:**
- Finding #1 (mid-vote stall) → Tasks 1–5 (extract tally helpers + recheck on leave + sweep migration).
- Finding #2 (solo healer event revive) → Task 7.
- Finding #3 (event death log) → Task 7 (same helper).
- Finding #4 (per-combat reset duplication) → Task 8.

**Test table coverage from spec:**
- `TestRemovePlayerResolvesMoveVote` → Task 4.
- `TestRemovePlayerResolvesShopLeave` → Task 4.
- `TestRemovePlayerResolvesEventVote` → Task 4.
- `TestEventDamageRevivesSoloHealer` → Task 7.
- `TestEventDamageLogsDeath` → Task 7.
- `TestResetPerCombatStateClearsTimeGlass` → Task 8.

**Helper signature consistency:**
- `tallyMoveQuorumLocked() (int, bool)` — used by `SubmitMoveVote` (Task 1) and `recheckPhaseQuorumLocked` (Task 4).
- `tallyEventQuorumLocked() bool` — used by `SubmitEventVote` (Task 2) and `recheckPhaseQuorumLocked` (Task 4).
- `tallyShopLeaversLocked() bool` — used by `LeaveShop` (Task 3) and `recheckPhaseQuorumLocked` (Task 4).
- `recheckPhaseQuorumLocked()` — called from `RemovePlayer` (Task 4) and `combatLoop` sweep (Task 5).
- `tryReviveSoloHealerLocked(p *entity.Player) bool` — called from 3 combat sites (Task 6) and `applyEventOutcome` (Task 7).
- `resetPerCombatStateLocked()` — called from 4 spawn sites (Task 8).

All signatures match between tasks. No placeholders. Every step has the actual code to write or the exact command to run.
