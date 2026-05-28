# Event Choice — Design Spec

**Date:** 2026-05-28
**Status:** Approved (in-conversation), pending written review
**Author:** Claude + tolelom

## Problem

`game/event.go:455-457` auto-resolves random-event rooms with `event.Choices[s.rng.Intn(len(event.Choices))]`. The choice data exists in `game/random_event.go` (8 events, 2-3 labeled choices each, all with `Resolve` functions) but the player never sees it. This kills the single biggest source of meaningful decisions in a roguelike — "interesting choices under uncertainty."

## Goal

Convert event rooms from a slot machine into a real player decision: show the choices, let everyone vote, resolve by majority, apply to the party.

## Non-Goals

- Redesigning the 8 existing events (we use them as-is, only adding category tags)
- Adding new event types
- Per-player outcome targeting (party-wide is decided)
- Timeout / default-to-skip (we wait for all alive votes — matches `SubmitMoveVote`)

## Design Decisions (from brainstorm)

| Decision | Value | Rationale |
|----------|-------|-----------|
| Who selects | All alive players, majority vote | Matches existing `SubmitMoveVote` pattern |
| Outcome target | Whole party | Avoids lottery feel; reinforces co-op |
| Info disclosure | Category-icon hints | Middle ground: keep mystery on values, expose category |
| Timeout | None (wait for all alive votes) | Matches `SubmitMoveVote`; reconnect path handles dropouts |

## Architecture

Two-stage flow replaces single-stage `triggerEvent`:

1. **Stage 1 — `triggerEvent` (room entry):** picks an event, stashes it on session state, flips to `PhaseEvent`. Does NOT resolve.
2. **Stage 2 — `SubmitEventVote(fingerprint, choiceIdx)` (player input):** records vote, on full-quorum resolves majority, applies outcome party-wide, returns to `PhaseExploring` + `room.Cleared = true`.

### New phase

```go
// game/session.go
const (
    PhaseExploring GamePhase = iota
    PhaseCombat
    PhaseShop
    PhaseResult
    PhaseEvent   // ← NEW
)
```

### New GameState fields (exposed in UI snapshot)

```go
PendingEvent *PendingEventView // nil when not in PhaseEvent
EventVotes   map[string]int    // fingerprint → choice index (live display)
```

`PendingEventView` is a UI-safe view of the active event (Name, Description, []ChoiceView where each ChoiceView has Label + Tags). The session also keeps the original typed `*RandomEvent` in a **private** field `s.pendingEvent` so it can call the `Resolve` func at vote-tally time — `Resolve` is a function value and must not leak into the snapshot path (which deep-copies state for UI consumption).

## Components

### Data model changes (`game/random_event.go`)

```go
type ChoiceTag int

const (
    TagHeal ChoiceTag = iota // ❤  HP recovery possible
    TagHarm                  // 🔥 HP loss risk
    TagGoldGain              // 💰+ gold gain
    TagGoldCost              // 💰- gold cost
    TagItem                  // ✨ item drop
    TagGamble                // 🎲 uncertain outcome (RNG branch in Resolve)
    TagSkip                  // ⤳ no effect (safe pass)
)

type EventChoice struct {
    Label   string
    Tags    []ChoiceTag       // ← NEW
    Resolve func(floor int, rng *rand.Rand) EventOutcome
}
```

All 8 existing events get `Tags` populated. Example for altar:
- "제단에서 기도하기" → `[TagHeal, TagHarm, TagGamble]` (60/40 heal-or-damage)
- "골드 바치기" → `[TagGoldCost, TagItem]`
- "그냥 지나가기" → `[TagSkip]`

### Outcome application rules (party-wide)

"Alive" here means `!p.IsOut()` — excludes both dead and fled players. (Fled doesn't actually apply in PhaseEvent because flee is a combat action, but we keep the same predicate for consistency with the rest of the codebase.)

Refactor current single-target apply into `applyEventOutcome(outcome EventOutcome)`:

| Field | Application |
|-------|-------------|
| `HPChange > 0` | Each alive player heals by `HPChange` |
| `HPChange < 0` | Each alive player takes `-HPChange` damage |
| `GoldChange < 0` | Cost split evenly across alive players; integer division, remainder paid by the first alive player in `s.state.Players` slice order; per-player gold clamped to ≥0 |
| `GoldChange > 0` | Reward split evenly across alive players; remainder to the first alive player in `s.state.Players` slice order |
| `ItemDrop` | Each alive player gets one rolled item (`50% weapon / 50% armor`, current floor scaling); skip with log if their inventory is full |

### Session API (`game/session.go`)

```go
// SubmitEventVote records a player's event choice. When all alive players
// have voted, the majority choice (lowest index breaks ties) is resolved and
// applied to the party. Returns true if this call triggered resolution.
func (s *GameSession) SubmitEventVote(fingerprint string, choiceIdx int) bool
```

Direct sibling of `SubmitMoveVote`:
- Validates phase == `PhaseEvent`, `choiceIdx` in range
- Writes to `s.eventVotes` (private map) and mirrors to `s.state.EventVotes`
- After write, recounts alive vs voted; if `votes < alive`, returns false
- Otherwise: tallies, picks majority (tie → lowest index), calls `applyEventOutcome`, clears `PendingEvent` and `EventVotes`, sets `room.Cleared = true`, flips to `PhaseExploring`

### `triggerEvent` refactor

```go
func (s *GameSession) triggerEvent() {
    event := PickRandomEvent(s.rng)
    s.state.LastEventName = event.Name

    // Edge case: nobody alive (post-wipe). Skip + clear.
    if s.aliveCount() == 0 {
        s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
        return
    }

    s.state.PendingEvent = makePendingEventView(event)
    s.state.EventVotes = make(map[string]int)
    s.eventVotes = make(map[string]int)
    s.pendingEvent = event // private — needed to call Resolve on the typed value
    s.state.Phase = PhaseEvent
    s.addLog(fmt.Sprintf("이벤트: %s — %s", event.Name, event.Description))
}
```

### UI (`ui/event_view.go` — NEW)

Cribbed from `shop_view.go` + `lobby_view.go` styles:

```
┌─ 이벤트: 고대 제단 ────────────────────────────────┐
│ 이상한 에너지로 빛나는 고대 제단을 발견했습니다.    │
│                                                  │
│  ▶ 1. ❤🔥🎲  제단에서 기도하기                     │
│    2. 💰-✨   골드 바치기                          │
│    3. ⤳     그냥 지나가기                         │
│                                                  │
│ 투표 현황:                                        │
│  Alice  → #1                                     │
│  Bob    → 대기 중...                              │
│  Carol  → #3                                     │
└──────────────────────────────────────────────────┘
[1/2/3] 선택   [↑/↓] 커서 이동   [Enter] 투표
```

- Routed via `ui/model.go` switch on `Phase == PhaseEvent`
- Numeric keys 1-3 (and Enter on cursor) call `Context.Session.SubmitEventVote`
- ESC is intentionally disabled — events cannot be skipped
- Tag icons defined as constants in `ui/event_view.go`: `❤ 🔥 💰+ 💰- ✨ 🎲 ⤳`

## Data Flow

```
EnterRoom(RoomEvent)
  ↓
triggerEvent()
  - pendingEvent = PickRandomEvent
  - state.PendingEvent, state.EventVotes, state.Phase = PhaseEvent
  - addLog (event name + description)
  → room.Cleared still FALSE
  ↓
[each player] SubmitEventVote(fp, idx)
  - record vote, mirror to state.EventVotes
  - if votes < alive: return false
  ↓ (all alive voted)
  - tally → majority (ties: lowest idx)
  - outcome = pendingEvent.Choices[winner].Resolve(floor, s.rng)
  - applyEventOutcome(outcome)
  - addLog ("→ <label>: <description>", per-player effect logs)
  - clear PendingEvent / EventVotes / eventVotes / pendingEvent
  - room.Cleared = TRUE
  - state.Phase = PhaseExploring
  → return true
```

## Edge Cases & Error Handling

| Case | Behavior |
|------|----------|
| 0 alive on room entry | Skip event entirely, mark cleared, stay in PhaseExploring (handled in `triggerEvent`) |
| Invalid `choiceIdx` (<0 or ≥len) | No-op, return false |
| Same player re-votes | Last vote wins (matches `SubmitMoveVote`) |
| Player leaves mid-vote | `RemovePlayer` also removes from `eventVotes` + `state.EventVotes`; loop re-evaluates alive count, may immediately resolve |
| Solo mode (alive == 1) | One vote triggers resolution naturally — no special case |
| Reconnect during PhaseEvent | Reconnect path lands them in PhaseEvent; vote map preserved, they can vote |
| Inventory full on ItemDrop | Skip with log line for that player; others still get item |
| Negative balance after gold split | Clamp to 0 (matches current `triggerEvent` behavior) |
| Cleared room re-entry | `enterRoomLocked` already early-returns — no change |

## Determinism

- `triggerEvent` and `Resolve` already use `s.rng` (per-session seeded RNG)
- Vote resolution is deterministic given the vote map (ties → lowest index)
- `applyEventOutcome` uses `s.rng` for item-drop rolls (matches existing `triggerEvent`)
- Daily-challenge replay safety: same seed + same player votes → identical outcomes ✓

## Testing Plan

New file: `game/event_choice_test.go`

1. `TestTriggerEvent_TransitionsToPhaseEvent` — entering RoomEvent sets `PendingEvent` + `PhaseEvent`; `room.Cleared` is still false.
2. `TestSubmitEventVote_MajorityResolvesAndAppliesToParty` — 3 alive, 2 vote choice 0, 1 votes choice 1 → choice 0 resolves; HP/gold changes affect all 3.
3. `TestSubmitEventVote_TieBreakLowestIndex` — 2v2 tie → lower index wins.
4. `TestSubmitEventVote_PartialVotesDoNotResolve` — 1 of 3 votes → still PhaseEvent, Cleared false.
5. `TestEventVote_SoloModeImmediateResolve` — alive == 1; one vote triggers resolution.
6. `TestEventVote_PlayerLeaveReevaluates` — 3 alive, 2 voted, 1 leaves → resolves with remaining 2.
7. `TestApplyEventOutcome_PartywideHealAndGoldSplit` — `HPChange=+20` heals all; `GoldChange=-30` with 3 alive → each pays 10.
8. `TestApplyEventOutcome_ItemDropPerPlayer` — `ItemDrop=true` with 3 alive (one with full inventory) → 2 items granted, 1 skip log.
9. `TestEventChoiceTagsPresent` — every Choice in every event has ≥1 Tag (regression guard).
10. `TestEventVote_TriggerWithZeroAlive` — entering RoomEvent with no alive players → cleared without phase change.

All tests inject `rand.New(rand.NewSource(42))` for deterministic outcomes.

UI side: extend `ui/model_test.go` with `PhaseEvent` routing assertion (mirror existing screen-routing tests).

## Migration / Compatibility

- `EventChoice.Tags` is a new field, populated for all 8 existing events as part of this change. Test #9 enforces this.
- No DB schema changes — events are ephemeral phase state.
- `LastEventName` preserved for UI message continuity.

## Out of Scope (Future Work)

- New event content / additional 10-20 events
- Class-conditional choices ("Warrior only: smash the altar")
- Visible probability disclosure
- Re-roll / refusal mechanics

## Estimate

- Code: ~250 lines (split: 70 game/event.go refactor + tag data, 80 session.go API + state, 90 ui/event_view.go, 10 ui/model.go routing)
- Tests: ~200 lines
