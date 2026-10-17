# Audit: Disconnect-Mid-Vote Stalls & Event-Phase Gaps

**Date:** 2026-05-28
**Status:** Design

## Background

The game has had three audit passes (2026-04-25, 2026-05-02, 2026-05-10). Since
then we shipped two large systems: party-vote events (`PhaseEvent`) and diamond
skill trees. This audit finds the new gaps that those systems introduced, plus
one related polish item.

All findings come from re-reading `game/session.go`, `game/turn.go`, and
`game/event.go` against the existing patterns documented in `CLAUDE.md` (turn
collection, IsDead vs IsOut, deterministic RNG).

## Findings

### 1. Stall when a player explicitly leaves mid-vote

**Symptom.** A party parked in `PhaseExploring` (move vote), `PhaseShop` (leave
consensus), or `PhaseEvent` (event vote) can hang forever after one player
presses Q.

**Cause.** Quorum is checked only inside the per-phase submission handlers
(`SubmitMoveVote`, `LeaveShop`, `SubmitEventVote`). When `RemovePlayer` drops
the leaver from `s.state.Players` it also clears their vote, but it never
re-runs the quorum check. The remaining players already voted, so no further
submission fires — the party is stuck.

The 60-second inactivity sweep in `combatLoop` partially patches PhaseEvent
(after the timeout), but explicit-leave shortcuts that. `PhaseExploring` and
`PhaseShop` are not handled at all even on inactivity.

**Concrete scenarios.**
- A and B in PhaseExploring. A votes for room 0. B presses Q.
  Alive=1, voteCount=1, but `SubmitMoveVote`'s quorum check never re-fires.
- A and B in PhaseShop. A presses "leave". B disconnects (web-socket drop).
  `LeaveShop`'s ready==available check never re-fires.
- A and B in PhaseEvent. A votes choice 0. B presses Q. Same shape.

**Fix.** Add a single helper `recheckPhaseQuorumLocked()` that runs the tally
logic for the current phase (exploring/shop/event) and resolves if quorum is
met. Call it:
- At the end of `RemovePlayer` (under `s.mu`) before returning.
- At the end of the `combatLoop` inactivity sweep when `changed == true` (in
  place of the PhaseEvent-only branch that exists today).

The helper does not duplicate the submission handlers — it shares the same
tally code by factoring `tallyMoveVotesLocked()`, `tallyEventVotesLocked()`,
and `tallyShopLeaversLocked()` out of the submission methods and reusing them.

### 2. Solo healer auto-revive not applied to event damage

**Symptom.** A solo healer can die to a negative-HP event outcome even though
the once-per-run "Divine Grace" auto-revive guarantee says their first death
should restore them to 30% HP.

**Cause.** `applyEventOutcome` calls `p.TakeDamage(dmg)` in a flat loop. The
auto-revive logic exists in `resolvePlayerActions` and `resolveMonsterActions`
(both check `s.state.SoloMode && p.Class == ClassHealer && !soloHealerRevived[fp]`)
but is duplicated rather than extracted, so the event path was never updated to
include it.

**Fix.** Extract `tryReviveSoloHealerLocked(p)` and call it from all three
sites: player-tick path, monster-attack path, and `applyEventOutcome`.

### 3. Death log missing from event damage

**Symptom.** When an event outcome kills a player, the combat log shows
"X N 피해를 받음" but no "☠ X 쓰러졌습니다!" line. Every other death site
prints the rip log. UX inconsistency.

**Fix.** Same helper as #2 — bundling the death-log emission into
`tryReviveSoloHealerLocked` (which returns "did we revive") lets the caller
emit the rip log on the non-revive path with one shared format.

### 4. Per-combat reset duplication

**Symptom.** Four spawn sites (`spawnMonsters`, `spawnBoss`, `spawnMiniBoss`,
`handleMimic`) each contain the same 4–5 lines:

```go
for _, p := range s.state.Players { p.SkillUses = s.effectiveSkillUses() }
s.coopUsedThisCombat = make(map[string]bool)
s.activeRelicUsed = make(map[string]bool)
s.extraActionNext = make(map[string]bool)
s.applyPoisonNextCombatIfPending()
```

Drift risk is real — `handleMimic` had to be patched in a previous commit
(`f6b8466` era) precisely because someone added Time Glass without remembering
to clear `extraActionNext` at the mimic combat start.

**Fix.** Extract `resetPerCombatStateLocked()` covering exactly those five
operations. Each of the four spawn sites calls it last. No behavior change.

## Non-goals (deferred)

- **Split `game/session.go` (1033 lines) and `game/turn.go` (992 lines).**
  Large files but well-tested. Splitting them is mostly mechanical and would
  inflate the diff without changing behavior. Separate PR if we want it.
- **Lossy `PendingLogs` on rapid turn boundaries.** `RunTurn` sets
  `s.state.PendingLogs = nil` at the start of every turn — if the UI hasn't
  drained the previous turn's logs, they're dropped. Believed to be a
  designed-in tradeoff (don't stall the turn timer on a slow client). Out of
  scope here.
- **Bleed stack-vs-refresh semantics.** `AddEffect` refreshes rather than
  stacks, but `StatusBleed.TickEffects` does `e.Value++` so escalation persists
  on a single application. Working as documented.

## Architecture

### New helpers in `game/session.go`

```go
// recheckPhaseQuorumLocked runs the per-phase quorum tally and resolves if
// met. Used by RemovePlayer and the combatLoop inactivity sweep so a player
// leaving (explicit or timeout) can't strand the party waiting for a vote
// they will never cast. Caller holds s.mu.
func (s *GameSession) recheckPhaseQuorumLocked() { ... }

// tally{Move,Event,Shop}Quorum extract the alive-count vs vote-count
// computations that were inlined in the submit handlers. Shared by the
// submit handlers AND recheckPhaseQuorumLocked so there is one definition
// of "is quorum met for phase X".
```

### New helper in `game/turn.go` (or wherever the death logic lives)

```go
// tryReviveSoloHealerLocked checks the once-per-run solo-healer auto-revive.
// Returns true if it revived (caller skips death log + AnyPlayerDied flag).
// Returns false if the player stays dead (caller emits the rip log).
// Caller holds s.mu. Only call after p.TakeDamage left p.IsDead() == true.
func (s *GameSession) tryReviveSoloHealerLocked(p *entity.Player) bool { ... }
```

### New helper in `game/event.go`

```go
// resetPerCombatStateLocked refreshes every map and flag that is scoped to a
// single combat encounter. Call at every combat-entry site (spawnMonsters,
// spawnBoss, spawnMiniBoss, handleMimic) so adding a new per-combat flag
// requires updating one place, not four. Caller holds s.mu.
func (s *GameSession) resetPerCombatStateLocked() { ... }
```

## Data flow (fix #1, the only one that adds new control flow)

```
Player presses Q
  → ui sends LeaveSession
  → Lobby.LeaveSession → session.RemovePlayer
    → prune Players + delete from vote maps      (existing)
    → s.recheckPhaseQuorumLocked()               (NEW)
      → if Phase == Event && quorum met: resolveEventVoteLocked
      → if Phase == Exploring && quorum met: enterRoomLocked(winner)
      → if Phase == Shop && all leavers ready: flip to Exploring
    → wake playerLeftCh                          (existing)
```

`combatLoop`'s inactivity sweep gets the same single-line call after pruning,
replacing the inline PhaseEvent-only block.

## Testing

Add tests in `game/`:

| Test | Phase | Setup | Assert |
|------|-------|-------|--------|
| `TestRemovePlayerResolvesMoveVote` | Exploring | A votes room 0, RemovePlayer(B) | Phase flips to Combat/Shop/etc. for room 0 |
| `TestRemovePlayerResolvesShopLeave` | Shop | A.LeaveShop(), RemovePlayer(B) | Phase flips to Exploring |
| `TestRemovePlayerResolvesEventVote` | Event | A.SubmitEventVote(0), RemovePlayer(B) | PendingEvent cleared, outcome applied |
| `TestEventDamageRevivesSoloHealer` | Event | Solo healer at 5 HP, event deals 100 | HP = 30%MaxHP, soloHealerRevived[fp]=true |
| `TestEventDamageLogsDeath` | Event | 2-player party, event kills one | CombatLog contains "☠ ... 쓰러졌습니다" |
| `TestResetPerCombatStateClearsTimeGlass` | — | Set extraActionNext[fp]; call helper | extraActionNext is empty |

Plus update existing tests that currently exercise the duplicated reset lines
to call through the helper.

## Risks

- **`recheckPhaseQuorumLocked` from RemovePlayer can call `enterRoomLocked`.**
  That mutates `s.state.Floor` and may flip Phase to `PhaseCombat`. RemovePlayer
  is called from `Lobby.LeaveSession`, which releases the lobby lock before
  calling into the session — there is no lobby↔session lock ordering issue.
  But callers up the stack must tolerate Phase changing during the LeaveSession
  call. They already do (LeaveSession returns just the player count; phase is
  checked next tick).
- **Solo healer revive is a once-per-run guarantee.** Burning it on event
  damage is correct per design (any death counts), but worth a comment so a
  future reader doesn't "fix" it back to combat-only.
- **`tallyEventVotesLocked` re-extract must preserve the anonymous-fingerprint
  test path** (`aliveCount==1 && fingerprint=="" && len(eventVotes)==0`). The
  recheck caller does not pass a fingerprint, so the helper signature must
  accept "no fingerprint context" — easiest by removing the special case from
  the tally helper and keeping it inline in `SubmitEventVote` only.

## Acceptance

- All four findings fixed under one PR.
- Six new tests pass; existing tests unchanged or migrated to helpers.
- `go test -race ./...` clean.
- `go vet ./...` clean.
- No new files; all changes land in `game/session.go`, `game/turn.go`,
  `game/event.go` and their `_test.go` siblings.
