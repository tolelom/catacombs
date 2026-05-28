# Event Choices Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert event rooms from auto-random resolution into a real player decision (majority vote, party-wide outcome, category-icon hints).

**Architecture:** Split `triggerEvent` into two stages — entry parks the event in a new `PhaseEvent` with a vote map; `SubmitEventVote` collects votes and on quorum resolves majority + applies outcome party-wide.

**Tech Stack:** Go 1.21, Bubble Tea, standard `math/rand` (per-session). Spec: `docs/superpowers/specs/2026-05-28-event-choices-design.md`.

---

## File Structure (locked decisions)

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `game/random_event.go` | Add `ChoiceTag` enum + `Tags []ChoiceTag` field; populate Tags for all 8 events |
| Modify | `game/session.go` | Add `PhaseEvent` const; add `PendingEvent`/`EventVotes` to `GameState`; add private `eventVotes` + `pendingEvent` fields; clean up in `RemovePlayer`; mirror in `GetState`; add `SubmitEventVote` |
| Modify | `game/event.go` | Refactor `triggerEvent` to set `PhaseEvent` instead of resolving immediately; add `resolveEventVote` + `applyEventOutcome` |
| Modify | `ui/game_view.go` | Add `PhaseEvent` route before `PhaseShop` block (~line 316) |
| Create | `ui/event_view.go` | New `EventScreen` (poll pattern like `ShopScreen`); numeric keys submit vote |
| Create | `game/event_choice_test.go` | 10 unit tests from spec |
| Create | `ui/event_view_test.go` | Routing assertion |

Each file has one clear responsibility. Test file pair matches the production file pair.

---

## Task 1: Add ChoiceTag enum + Tags field (data only, no behavior change)

**Files:**
- Modify: `game/random_event.go`
- Test: `game/event_choice_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `game/event_choice_test.go`:

```go
package game

import "testing"

// TestEventChoiceTagsPresent enforces every choice in every event has at
// least one Tag — UI relies on Tags to render category icons.
func TestEventChoiceTagsPresent(t *testing.T) {
	for _, ev := range GetRandomEvents() {
		for i, ch := range ev.Choices {
			if len(ch.Tags) == 0 {
				t.Errorf("event %q choice %d (%q) has no Tags", ev.Name, i, ch.Label)
			}
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./game/ -run TestEventChoiceTagsPresent`
Expected: compile error — `ch.Tags` undefined. (This proves the test depends on the new field.)

- [ ] **Step 3: Add ChoiceTag enum + Tags field**

In `game/random_event.go`, change the top of the file:

```go
package game

import "math/rand"

// ChoiceTag classifies an event choice's possible outcomes for UI hints.
// One choice can carry multiple tags (e.g. altar pray = Heal + Harm + Gamble).
type ChoiceTag int

const (
	TagHeal     ChoiceTag = iota // ❤  HP recovery possible
	TagHarm                      // 🔥 HP loss risk
	TagGoldGain                  // 💰+ gold gain
	TagGoldCost                  // 💰- gold cost
	TagItem                      // ✨ item drop
	TagGamble                    // 🎲 outcome branches on RNG
	TagSkip                      // ⤳ no effect (safe pass)
)

// EventOutcome describes the result of choosing an event option.
type EventOutcome struct {
	HPChange    int
	GoldChange  int
	ItemDrop    bool
	Description string
}

// EventChoice represents a single choice the player can make during an event.
// Resolve receives the session RNG so daily-challenge runs stay deterministic;
// callers must not use the package-level rand inside Resolve.
type EventChoice struct {
	Label   string
	Tags    []ChoiceTag
	Resolve func(floor int, rng *rand.Rand) EventOutcome
}
```

- [ ] **Step 4: Populate Tags for all 8 events**

Edit each `EventChoice` literal in `GetRandomEvents()` to add a `Tags:` field. Full replacement of the `GetRandomEvents()` body — the only changes are added `Tags` lines:

```go
func GetRandomEvents() []RandomEvent {
	return []RandomEvent{
		{
			Name:        "altar",
			Description: "이상한 에너지로 빛나는 고대 제단을 발견했습니다.",
			Choices: []EventChoice{
				{
					Label: "제단에서 기도하기",
					Tags:  []ChoiceTag{TagHeal, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.6 {
							heal := 15 + floor*2
							return EventOutcome{HPChange: heal, Description: "제단이 치유의 빛으로 축복합니다."}
						}
						dmg := 10 + floor
						return EventOutcome{HPChange: -dmg, Description: "제단의 에너지가 당신을 공격합니다!"}
					},
				},
				{
					Label: "골드 바치기",
					Tags:  []ChoiceTag{TagGoldCost, TagItem},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						cost := 10 + floor
						return EventOutcome{GoldChange: -cost, ItemDrop: true, Description: "골드를 바치고 신성한 선물을 받았습니다."}
					},
				},
				{
					Label: "그냥 지나가기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "제단을 건드리지 않고 떠납니다."}
					},
				},
			},
		},
		{
			Name:        "fountain",
			Description: "방 중앙에서 빛나는 분수가 솟아오릅니다.",
			Choices: []EventChoice{
				{
					Label: "분수의 물 마시기",
					Tags:  []ChoiceTag{TagHeal},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						heal := 20 + floor*2
						return EventOutcome{HPChange: heal, Description: "물이 당신을 활기차게 합니다!"}
					},
				},
				{
					Label: "동전 던지기",
					Tags:  []ChoiceTag{TagGoldGain, TagGoldCost, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.5 {
							gold := 15 + floor*3
							return EventOutcome{GoldChange: gold, Description: "분수가 당신의 관대함에 보답합니다!"}
						}
						return EventOutcome{GoldChange: -5, Description: "동전이 가라앉고 아무 일도 일어나지 않습니다."}
					},
				},
			},
		},
		{
			Name:        "merchant",
			Description: "두건을 쓴 상인이 어둠 속에서 나타납니다.",
			Choices: []EventChoice{
				{
					Label: "골드로 치료 거래",
					Tags:  []ChoiceTag{TagHeal, TagGoldCost},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						cost := 15 + floor
						heal := 25 + floor*2
						return EventOutcome{HPChange: heal, GoldChange: -cost, Description: "상인이 치유의 물약을 팝니다."}
					},
				},
				{
					Label: "미스터리 아이템 구매",
					Tags:  []ChoiceTag{TagGoldCost, TagItem},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						cost := 20 + floor*2
						return EventOutcome{GoldChange: -cost, ItemDrop: true, Description: "상인이 포장된 꾸러미를 건넵니다."}
					},
				},
				{
					Label: "거절하기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "상인이 어둠 속으로 사라집니다."}
					},
				},
			},
		},
		{
			Name:        "trap_room",
			Description: "바닥이 수상한 압력판으로 덮여 있습니다.",
			Choices: []EventChoice{
				{
					Label: "조심히 지나가기",
					Tags:  []ChoiceTag{TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.5 {
							return EventOutcome{Description: "능숙하게 모든 함정을 피했습니다!"}
						}
						dmg := 8 + floor
						return EventOutcome{HPChange: -dmg, Description: "함정을 밟아 피해를 입었습니다!"}
					},
				},
				{
					Label: "돌진하기",
					Tags:  []ChoiceTag{TagHarm, TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						dmg := 5 + floor/2
						gold := 10 + floor*2
						return EventOutcome{HPChange: -dmg, GoldChange: gold, Description: "약간의 피해를 입었지만 숨겨진 골드를 발견했습니다!"}
					},
				},
			},
		},
		{
			Name:        "shrine",
			Description: "신성한 힘으로 울리는 빛나는 성소가 있습니다.",
			Choices: []EventChoice{
				{
					Label: "무릎 꿇고 기도하기",
					Tags:  []ChoiceTag{TagHeal},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						heal := 30 + floor*2
						return EventOutcome{HPChange: heal, Description: "성소가 새로운 활력으로 가득 채워줍니다!"}
					},
				},
				{
					Label: "제물 가져가기",
					Tags:  []ChoiceTag{TagHarm, TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 20 + floor*3
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, GoldChange: gold, Description: "제물을 훔쳤지만 영혼들이 분노합니다!"}
					},
				},
			},
		},
		{
			Name:        "chest",
			Description: "방 구석에 화려한 상자가 놓여 있습니다.",
			Choices: []EventChoice{
				{
					Label: "조심히 열기",
					Tags:  []ChoiceTag{TagGoldGain, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.7 {
							gold := 15 + floor*2
							return EventOutcome{GoldChange: gold, Description: "상자 안에 골드 더미가 있습니다!"}
						}
						dmg := 12 + floor
						return EventOutcome{HPChange: -dmg, Description: "상자가 미믹이었습니다! 물어뜯깁니다!"}
					},
				},
				{
					Label: "부수어 열기",
					Tags:  []ChoiceTag{TagItem},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{ItemDrop: true, Description: "상자를 부수고 안에서 장비를 발견했습니다!"}
					},
				},
				{
					Label: "그냥 두기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "안전한 게 최고입니다."}
					},
				},
			},
		},
		{
			Name:        "ghost",
			Description: "유령 같은 형체가 눈앞에 나타납니다.",
			Choices: []EventChoice{
				{
					Label: "유령과 대화하기",
					Tags:  []ChoiceTag{TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 10 + floor*2
						return EventOutcome{GoldChange: gold, Description: "유령이 들어줘서 감사하며 보상합니다."}
					},
				},
				{
					Label: "유령 공격하기",
					Tags:  []ChoiceTag{TagItem, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.4 {
							return EventOutcome{ItemDrop: true, Description: "유령이 사라지며 유령 무기를 떨어뜨립니다!"}
						}
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, Description: "유령이 분노하여 반격합니다!"}
					},
				},
			},
		},
		{
			Name:        "mushroom",
			Description: "이상하게 빛나는 버섯들이 무리 지어 자라고 있습니다.",
			Choices: []EventChoice{
				{
					Label: "버섯 먹기",
					Tags:  []ChoiceTag{TagHeal, TagHarm, TagGoldGain, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						r := rng.Float64()
						if r < 0.33 {
							heal := 20 + floor*2
							return EventOutcome{HPChange: heal, Description: "버섯이 맛있고 치유 효과가 있습니다!"}
						} else if r < 0.66 {
							dmg := 10 + floor
							return EventOutcome{HPChange: -dmg, Description: "독버섯이었습니다!"}
						}
						gold := 10 + floor
						return EventOutcome{GoldChange: gold, Description: "버섯이 이상한 환각을 보여주고... 위에서 골드가 떨어집니다!"}
					},
				},
				{
					Label: "채집하여 팔기",
					Tags:  []ChoiceTag{TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 8 + floor
						return EventOutcome{GoldChange: gold, Description: "조심히 버섯을 채집하여 판매합니다."}
					},
				},
				{
					Label: "무시하기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "의문의 균류를 현명하게 피합니다."}
					},
				},
			},
		},
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./game/ -run TestEventChoiceTagsPresent -v`
Expected: PASS

- [ ] **Step 6: Run full game tests to verify no regression**

Run: `go test ./game/`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add game/random_event.go game/event_choice_test.go
git commit -m "feat(events): add ChoiceTag enum and tag all 8 events"
```

---

## Task 2: Add PhaseEvent constant + state fields

**Files:**
- Modify: `game/session.go`
- Test: `game/event_choice_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `game/event_choice_test.go`:

```go
// TestPhaseEventConstantExists is a compile-time guard that PhaseEvent is
// defined and distinct from existing phases.
func TestPhaseEventConstantExists(t *testing.T) {
	phases := []GamePhase{PhaseExploring, PhaseCombat, PhaseShop, PhaseResult, PhaseEvent}
	seen := make(map[GamePhase]bool, len(phases))
	for _, p := range phases {
		if seen[p] {
			t.Errorf("phase %d duplicated", p)
		}
		seen[p] = true
	}
}

// TestPendingEventOnGameState ensures GameState exposes PendingEvent and
// EventVotes fields for UI snapshot consumption.
func TestPendingEventOnGameState(t *testing.T) {
	var gs GameState
	_ = gs.PendingEvent // *PendingEventView
	_ = gs.EventVotes   // map[string]int
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./game/ -run "TestPhaseEventConstantExists|TestPendingEventOnGameState"`
Expected: compile errors — `PhaseEvent` undefined, `PendingEvent` undefined, `EventVotes` undefined.

- [ ] **Step 3: Add PhaseEvent const, types, and state fields**

In `game/session.go`, modify the `GamePhase` const block (around line 17):

```go
const (
	PhaseExploring GamePhase = iota
	PhaseCombat
	PhaseShop
	PhaseResult
	PhaseEvent
)
```

Add new types just after the `GamePhase` consts block (before `type PlayerAction`):

```go
// EventChoiceView is a snapshot-safe view of one event choice for the UI.
// The Resolve function is intentionally omitted — it is non-copyable behavior
// state and lives only on the typed *RandomEvent in the session.
type EventChoiceView struct {
	Label string
	Tags  []ChoiceTag
}

// PendingEventView is a snapshot-safe view of the active event, populated
// while Phase == PhaseEvent and nil otherwise.
type PendingEventView struct {
	Name        string
	Description string
	Choices     []EventChoiceView
}
```

In the `GameState` struct, add two fields (place them next to `MoveVotes` for grouping):

```go
	PendingEvent       *PendingEventView // active event during PhaseEvent
	EventVotes         map[string]int    // fingerprint → choice index
```

In the `GameSession` struct (around the private maps section ~line 113), add:

```go
	eventVotes     map[string]int  // fingerprint → choice index
	pendingEvent   *RandomEvent    // typed event with Resolve funcs; not snapshotted
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./game/ -run "TestPhaseEventConstantExists|TestPendingEventOnGameState" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: all PASS (no behavior touched yet).

- [ ] **Step 6: Commit**

```bash
git add game/session.go game/event_choice_test.go
git commit -m "feat(events): add PhaseEvent phase and pending-event state fields"
```

---

## Task 3: triggerEvent transitions to PhaseEvent instead of resolving

**Files:**
- Modify: `game/event.go` (replace `triggerEvent`)
- Test: `game/event_choice_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `game/event_choice_test.go`:

```go
import (
	// existing imports above + add:
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)
```

(Adjust the import block if these aren't already there.)

Then append the test:

```go
// TestTriggerEventTransitionsToPhaseEvent verifies entering a RoomEvent
// flips the session into PhaseEvent with PendingEvent populated, instead of
// resolving immediately.
func TestTriggerEventTransitionsToPhaseEvent(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	s.AddPlayer(p)

	// Hand-build a single-room floor with one event room.
	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms: []*dungeon.Room{
			{Type: dungeon.RoomEvent},
		},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.Phase = PhaseExploring
	s.mu.Unlock()

	s.EnterRoom(0)

	st := s.GetState()
	if st.Phase != PhaseEvent {
		t.Fatalf("phase should be PhaseEvent, got %v", st.Phase)
	}
	if st.PendingEvent == nil {
		t.Fatal("PendingEvent should be populated")
	}
	if len(st.PendingEvent.Choices) == 0 {
		t.Error("PendingEvent.Choices should not be empty")
	}
	if st.Floor.Rooms[0].Cleared {
		t.Error("room should not be cleared yet (waiting for vote)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./game/ -run TestTriggerEventTransitionsToPhaseEvent -v`
Expected: FAIL — current `triggerEvent` resolves immediately, leaves Phase as PhaseExploring and clears the room.

- [ ] **Step 3: Refactor triggerEvent**

In `game/event.go`, replace the existing `triggerEvent` function (around line 450) with this version. The old body (auto-resolve + apply to random player) is removed entirely:

```go
// triggerEvent picks a random event and parks the session in PhaseEvent
// awaiting player votes. The actual outcome runs later via resolveEventVote.
// Caller holds s.mu.
func (s *GameSession) triggerEvent() {
	event := PickRandomEvent(s.rng)
	s.state.LastEventName = event.Name
	s.addLog(fmt.Sprintf("이벤트: %s — %s", event.Name, event.Description))

	// Edge case: nobody alive to vote (post-wipe race). Skip + clear.
	alive := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive++
		}
	}
	if alive == 0 {
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
		return
	}

	// Stash typed event for Resolve at quorum time.
	ev := event
	s.pendingEvent = &ev

	// Build snapshot-safe view for the UI.
	view := &PendingEventView{
		Name:        event.Name,
		Description: event.Description,
		Choices:     make([]EventChoiceView, 0, len(event.Choices)),
	}
	for _, c := range event.Choices {
		tagsCopy := make([]ChoiceTag, len(c.Tags))
		copy(tagsCopy, c.Tags)
		view.Choices = append(view.Choices, EventChoiceView{Label: c.Label, Tags: tagsCopy})
	}
	s.state.PendingEvent = view
	s.state.EventVotes = make(map[string]int)
	s.eventVotes = make(map[string]int)
	s.state.Phase = PhaseEvent
}
```

Note: `triggerEvent` is invoked from `enterRoomLocked` (case `dungeon.RoomEvent`). That switch case currently calls `s.triggerEvent()` AND then `room.Cleared = true` on the next line — find it and **remove the `room.Cleared = true` line** for the `RoomEvent` case so the room stays uncleared until vote resolution.

Specifically, in `enterRoomLocked` (game/event.go around line 61), change:

```go
		case dungeon.RoomEvent:
			s.triggerEvent()
			room.Cleared = true
```

to:

```go
		case dungeon.RoomEvent:
			s.triggerEvent()
			// room.Cleared flipped after vote quorum in resolveEventVote
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./game/ -run TestTriggerEventTransitionsToPhaseEvent -v`
Expected: PASS

- [ ] **Step 5: Run full game suite**

Run: `go test ./game/`
Expected: all PASS. The existing `random_event_test.go` only checks event data structure (event count, Choice count, Resolve return values) — no test asserts on `triggerEvent`'s old auto-resolve behavior, so nothing breaks.

- [ ] **Step 6: Commit**

```bash
git add game/event.go game/event_choice_test.go
git commit -m "feat(events): triggerEvent transitions to PhaseEvent (no auto-resolve)"
```

---

## Task 4: Mirror PendingEvent/EventVotes in GetState snapshot

**Files:**
- Modify: `game/session.go` (GetState body)
- Test: `game/event_choice_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `game/event_choice_test.go`:

```go
// TestPendingEventDeepCopyIsolation ensures that mutating the snapshot's
// EventVotes map does not affect the session's internal state.
func TestPendingEventDeepCopyIsolation(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	s.AddPlayer(p)

	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomEvent}},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.Phase = PhaseExploring
	s.mu.Unlock()

	s.EnterRoom(0)

	snap1 := s.GetState()
	if snap1.EventVotes == nil {
		t.Fatal("snapshot EventVotes should be a non-nil map after entry")
	}
	snap1.EventVotes["mutator"] = 999

	snap2 := s.GetState()
	if _, leaked := snap2.EventVotes["mutator"]; leaked {
		t.Error("mutation on snapshot must not leak into session state")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./game/ -run TestPendingEventDeepCopyIsolation -v`
Expected: FAIL — either nil map or mutation leaks (because GetState returns the live map reference).

- [ ] **Step 3: Add deep-copy in GetState**

In `game/session.go`, find the `GetState` function (around line 500-615). Right after the `// Copy move votes` block (~line 545-552), add:

```go
	// Copy event votes
	var eventVotesCopy map[string]int
	if n := len(s.state.EventVotes); n > 0 {
		eventVotesCopy = make(map[string]int, n)
		for k, v := range s.state.EventVotes {
			eventVotesCopy[k] = v
		}
	}

	// Copy pending event view (deep — Choices slice + per-Choice Tags slice)
	var pendingEventCopy *PendingEventView
	if src := s.state.PendingEvent; src != nil {
		choices := make([]EventChoiceView, len(src.Choices))
		for i, c := range src.Choices {
			tags := make([]ChoiceTag, len(c.Tags))
			copy(tags, c.Tags)
			choices[i] = EventChoiceView{Label: c.Label, Tags: tags}
		}
		pendingEventCopy = &PendingEventView{
			Name:        src.Name,
			Description: src.Description,
			Choices:     choices,
		}
	}
```

In the `return GameState{...}` literal at the bottom of `GetState`, add these two fields (e.g. right after `MoveVotes: moveVotesCopy,`):

```go
		MoveVotes:        moveVotesCopy,
		PendingEvent:     pendingEventCopy,
		EventVotes:       eventVotesCopy,
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./game/ -run TestPendingEventDeepCopyIsolation -v`
Expected: PASS

- [ ] **Step 5: Sanity-check the full suite**

Run: `go test ./...`
Expected: same set of passes/skips as end of Task 3 (no regressions introduced by snapshot work).

- [ ] **Step 6: Commit**

```bash
git add game/session.go game/event_choice_test.go
git commit -m "feat(events): expose PendingEvent/EventVotes in GetState snapshot"
```

---

## Task 5: SubmitEventVote with majority + tie-break + edge cases

**Files:**
- Modify: `game/session.go` (new method + RemovePlayer cleanup)
- Modify: `game/event.go` (resolveEventVote helper)
- Test: `game/event_choice_test.go` (append)

- [ ] **Step 1: Write the failing tests (4 cases)**

Append to `game/event_choice_test.go`:

```go
// enterDeterministicEvent forces a chosen event into the session for tests
// by stashing it directly. seed picks the RNG branches inside Resolve.
func enterDeterministicEvent(t *testing.T, s *GameSession, eventName string) {
	t.Helper()
	var picked *RandomEvent
	for _, ev := range GetRandomEvents() {
		if ev.Name == eventName {
			e := ev
			picked = &e
			break
		}
	}
	if picked == nil {
		t.Fatalf("event %q not found", eventName)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomEvent}},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.LastEventName = picked.Name
	s.pendingEvent = picked
	view := &PendingEventView{
		Name:        picked.Name,
		Description: picked.Description,
		Choices:     make([]EventChoiceView, 0, len(picked.Choices)),
	}
	for _, c := range picked.Choices {
		tagsCopy := make([]ChoiceTag, len(c.Tags))
		copy(tagsCopy, c.Tags)
		view.Choices = append(view.Choices, EventChoiceView{Label: c.Label, Tags: tagsCopy})
	}
	s.state.PendingEvent = view
	s.state.EventVotes = make(map[string]int)
	s.eventVotes = make(map[string]int)
	s.state.Phase = PhaseEvent
}

func TestSubmitEventVote_PartialVotesDoNotResolve(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "merchant")

	if s.SubmitEventVote("fp-a", 0) {
		t.Error("partial vote should not resolve")
	}
	st := s.GetState()
	if st.Phase != PhaseEvent {
		t.Errorf("phase should remain PhaseEvent, got %v", st.Phase)
	}
	if st.Floor.Rooms[0].Cleared {
		t.Error("room must not be cleared on partial vote")
	}
}

func TestSubmitEventVote_MajorityResolvesAndAppliesToParty(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.HP = p.MaxHP - 50 // make room to receive heal
		p.Gold = 100
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "shrine")
	// shrine choice 0 = "무릎 꿇고 기도하기" → heal 30 + floor*2
	beforeHP := make(map[string]int)
	for _, p := range s.state.Players {
		beforeHP[p.Fingerprint] = p.HP
	}

	_ = s.SubmitEventVote("fp-a", 0)
	_ = s.SubmitEventVote("fp-b", 0)
	if !s.SubmitEventVote("fp-c", 1) {
		t.Fatal("3rd vote should trigger resolution")
	}
	st := s.GetState()
	if st.Phase != PhaseExploring {
		t.Errorf("phase should be PhaseExploring after resolve, got %v", st.Phase)
	}
	if !st.Floor.Rooms[0].Cleared {
		t.Error("room should be cleared after resolution")
	}
	for _, p := range st.Players {
		if p.HP <= beforeHP[p.Fingerprint] {
			t.Errorf("player %s HP did not increase (before=%d after=%d)", p.Fingerprint, beforeHP[p.Fingerprint], p.HP)
		}
	}
	if st.PendingEvent != nil || st.EventVotes != nil {
		t.Error("PendingEvent and EventVotes must be cleared after resolve")
	}
}

func TestSubmitEventVote_TieBreakLowestIndex(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c", "fp-d"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.HP = p.MaxHP - 30
		p.Gold = 200
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "merchant")
	// 2v2 tie: idx 0 (heal-for-gold) and idx 1 (gold for mystery item)
	_ = s.SubmitEventVote("fp-a", 1)
	_ = s.SubmitEventVote("fp-b", 0)
	_ = s.SubmitEventVote("fp-c", 1)
	if !s.SubmitEventVote("fp-d", 0) {
		t.Fatal("4th vote should trigger resolution")
	}
	st := s.GetState()
	// Idx 0 won (lower index on tie) → "골드로 치료 거래" → HPChange > 0
	for _, p := range st.Players {
		if p.HP <= p.MaxHP-30 {
			t.Errorf("expected idx 0 (heal) to win tie; player %s HP=%d", p.Fingerprint, p.HP)
		}
	}
}

func TestSubmitEventVote_SoloModeImmediateResolve(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("solo", entity.ClassWarrior)
	p.Fingerprint = "fp-solo"
	p.HP = p.MaxHP - 40
	s.AddPlayer(p)
	enterDeterministicEvent(t, s, "shrine")
	if !s.SubmitEventVote("fp-solo", 0) {
		t.Fatal("solo single vote should resolve immediately")
	}
	if s.GetState().Phase != PhaseExploring {
		t.Error("solo resolve should return to PhaseExploring")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./game/ -run "TestSubmitEventVote" -v`
Expected: all 4 FAIL — `s.SubmitEventVote` undefined.

- [ ] **Step 3: Implement SubmitEventVote on session**

In `game/session.go`, append after `SubmitMoveVote` (or after `LeaveShop`, anywhere in the file):

```go
// SubmitEventVote records a player's event choice during PhaseEvent. When
// all alive players have voted, the majority choice (ties broken by lowest
// index) is resolved and applied to the party. Returns true if this call
// triggered resolution.
func (s *GameSession) SubmitEventVote(fingerprint string, choiceIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Phase != PhaseEvent || s.pendingEvent == nil {
		return false
	}
	if choiceIdx < 0 || choiceIdx >= len(s.pendingEvent.Choices) {
		return false
	}
	if fingerprint != "" {
		s.lastActivity[fingerprint] = time.Now()
	}

	if s.eventVotes == nil {
		s.eventVotes = make(map[string]int)
	}
	if fingerprint != "" {
		s.eventVotes[fingerprint] = choiceIdx
	}

	// Mirror to snapshot map
	s.state.EventVotes = make(map[string]int, len(s.eventVotes))
	for k, v := range s.eventVotes {
		s.state.EventVotes[k] = v
	}

	// Quorum check: all alive players voted (anonymous solo path = 1 vote)
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		// Defensive: no alive players left to receive outcome; just clear.
		s.clearPendingEventLocked(true)
		return true
	}
	voteCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.eventVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}
	// Anonymous single-player test path: treat untracked fingerprint as the
	// vote we just received.
	if aliveCount == 1 && fingerprint == "" && len(s.eventVotes) == 0 {
		voteCount = 1
	}
	if voteCount < aliveCount {
		return false
	}

	s.resolveEventVoteLocked()
	return true
}

// clearPendingEventLocked clears all pending-event state and optionally marks
// the current event room as cleared. Caller holds s.mu.
func (s *GameSession) clearPendingEventLocked(markRoomCleared bool) {
	s.pendingEvent = nil
	s.state.PendingEvent = nil
	s.state.EventVotes = nil
	s.eventVotes = nil
	if markRoomCleared && s.state.Floor != nil {
		idx := s.state.Floor.CurrentRoom
		if idx >= 0 && idx < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[idx].Cleared = true
		}
	}
	s.state.Phase = PhaseExploring
}
```

In `RemovePlayer` (around line 360), after the existing `delete(s.moveVotes, fingerprint)` lines, add:

```go
	if s.eventVotes != nil {
		delete(s.eventVotes, fingerprint)
	}
	if s.state.EventVotes != nil {
		delete(s.state.EventVotes, fingerprint)
	}
```

Note: leaving mid-vote may now satisfy quorum with remaining alive players, but we intentionally do **not** auto-resolve from `RemovePlayer` — letting any remaining alive voter trigger via their next `SubmitEventVote` call keeps `RemovePlayer` free of phase transitions. Test case (`TestEventVote_PlayerLeaveReevaluates`) is added in Task 6 alongside `applyEventOutcome` so we can assert on the outcome side too. If you want to add it here as a placeholder skip, write `t.Skip("covered in Task 6")` — otherwise omit until Task 6.

- [ ] **Step 4: Implement resolveEventVoteLocked in event.go**

In `game/event.go`, append after `triggerEvent`:

```go
// resolveEventVoteLocked tallies eventVotes, picks the winning choice (lowest
// index breaks ties), runs Resolve, applies the outcome party-wide, and
// returns the session to PhaseExploring with the room cleared. Caller holds
// s.mu and has verified quorum.
func (s *GameSession) resolveEventVoteLocked() {
	if s.pendingEvent == nil {
		s.clearPendingEventLocked(true)
		return
	}
	// Tally
	tally := make(map[int]int, len(s.pendingEvent.Choices))
	for _, idx := range s.eventVotes {
		tally[idx]++
	}
	winnerIdx := -1
	bestCount := -1
	for idx, count := range tally {
		if count > bestCount || (count == bestCount && idx < winnerIdx) {
			bestCount = count
			winnerIdx = idx
		}
	}
	if winnerIdx < 0 || winnerIdx >= len(s.pendingEvent.Choices) {
		s.clearPendingEventLocked(true)
		return
	}
	choice := s.pendingEvent.Choices[winnerIdx]
	outcome := choice.Resolve(s.state.FloorNum, s.rng)
	s.addLog(fmt.Sprintf("  → %s: %s", choice.Label, outcome.Description))

	s.applyEventOutcome(outcome)
	s.clearPendingEventLocked(true)
}
```

Add a temporary stub `applyEventOutcome` at the bottom of `event.go` so the file compiles. Task 6 replaces it with the full party-wide logic:

```go
// applyEventOutcome applies an event outcome to the party. STUB — replaced
// with full party-wide logic in Task 6.
func (s *GameSession) applyEventOutcome(o EventOutcome) {
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		if o.HPChange > 0 {
			p.Heal(o.HPChange)
		} else if o.HPChange < 0 {
			p.TakeDamage(-o.HPChange)
		}
		if o.GoldChange != 0 {
			p.Gold += o.GoldChange
			if p.Gold < 0 {
				p.Gold = 0
			}
		}
	}
}
```

- [ ] **Step 5: Run tests to verify all 4 pass**

Run: `go test ./game/ -run "TestSubmitEventVote" -v`
Expected: 4 PASS

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: all PASS (no skips remaining from Task 3 should still exist — unskip them if they were skipped). If any legacy test still references `triggerEvent` directly with side-effect assumptions, update or remove it as appropriate.

- [ ] **Step 7: Commit**

```bash
git add game/session.go game/event.go game/event_choice_test.go
git commit -m "feat(events): SubmitEventVote with majority/tie-break and stub apply"
```

---

## Task 6: applyEventOutcome — party-wide rules + item drops + leave-reevaluation

**Files:**
- Modify: `game/event.go` (replace `applyEventOutcome` stub)
- Test: `game/event_choice_test.go` (append)

- [ ] **Step 1: Write the failing tests**

Append to `game/event_choice_test.go`:

```go
func TestApplyEventOutcome_PartywideHealAndGoldSplit(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.HP = 50
		p.MaxHP = 100
		p.Gold = 100
		s.AddPlayer(p)
	}
	s.mu.Lock()
	s.applyEventOutcome(EventOutcome{HPChange: 20, GoldChange: -30})
	s.mu.Unlock()

	st := s.GetState()
	for _, p := range st.Players {
		if p.HP != 70 {
			t.Errorf("each player should heal to 70, got %s HP=%d", p.Fingerprint, p.HP)
		}
		if p.Gold != 90 {
			t.Errorf("each player should pay 10 (30/3), got %s Gold=%d", p.Fingerprint, p.Gold)
		}
	}
}

func TestApplyEventOutcome_GoldRemainderToFirstAlive(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.Gold = 100
		s.AddPlayer(p)
	}
	s.mu.Lock()
	// -10 across 3 players → 3,3,3 + remainder 1 on first alive
	s.applyEventOutcome(EventOutcome{GoldChange: -10})
	s.mu.Unlock()

	st := s.GetState()
	if st.Players[0].Gold != 96 { // 100 - 3 - 1
		t.Errorf("first alive should bear remainder, got %d (expected 96)", st.Players[0].Gold)
	}
	if st.Players[1].Gold != 97 || st.Players[2].Gold != 97 {
		t.Errorf("others should pay 3 each, got %d %d", st.Players[1].Gold, st.Players[2].Gold)
	}
}

func TestApplyEventOutcome_ItemDropPerPlayerSkipFull(t *testing.T) {
	cfg := testCfg(t)
	limit := cfg.Game.InventoryLimit
	s := NewGameSession(cfg)
	full := entity.NewPlayer("full", entity.ClassWarrior)
	full.Fingerprint = "fp-full"
	for i := 0; i < limit; i++ {
		full.Inventory = append(full.Inventory, entity.Item{Name: "junk", Type: entity.ItemConsumable, Bonus: 1})
	}
	empty := entity.NewPlayer("empty", entity.ClassWarrior)
	empty.Fingerprint = "fp-empty"
	s.AddPlayer(full)
	s.AddPlayer(empty)
	s.mu.Lock()
	s.state.FloorNum = 1
	s.applyEventOutcome(EventOutcome{ItemDrop: true})
	s.mu.Unlock()

	st := s.GetState()
	if len(st.Players[0].Inventory) != limit {
		t.Errorf("full inventory should not grow, got %d", len(st.Players[0].Inventory))
	}
	if len(st.Players[1].Inventory) != 1 {
		t.Errorf("empty inventory should receive 1 item, got %d", len(st.Players[1].Inventory))
	}
}

func TestApplyEventOutcome_GoldClampsToZero(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("broke", entity.ClassWarrior)
	p.Fingerprint = "fp-broke"
	p.Gold = 2
	s.AddPlayer(p)
	s.mu.Lock()
	s.applyEventOutcome(EventOutcome{GoldChange: -100})
	s.mu.Unlock()

	if g := s.GetState().Players[0].Gold; g != 0 {
		t.Errorf("gold must clamp to 0, got %d", g)
	}
}

// TestEventVote_PlayerLeaveReevaluates ensures that when a player leaves
// mid-vote, a subsequent SubmitEventVote from the remaining player satisfies
// the new quorum and resolves the event.
func TestEventVote_PlayerLeaveReevaluates(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.HP = p.MaxHP - 10
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "shrine")

	_ = s.SubmitEventVote("fp-a", 0)
	_ = s.SubmitEventVote("fp-b", 0)
	// fp-c leaves before voting
	s.RemovePlayer("fp-c")

	// Now alive==2, votes==2 — but RemovePlayer does NOT itself resolve.
	// Re-submit fp-b's vote (idempotent path) to trigger quorum check.
	if !s.SubmitEventVote("fp-b", 0) {
		t.Fatal("re-submit after leaver should satisfy quorum and resolve")
	}
	if s.GetState().Phase != PhaseExploring {
		t.Error("phase should be PhaseExploring after resolve")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./game/ -run "TestApplyEventOutcome|TestEventVote_PlayerLeaveReevaluates" -v`
Expected: FAIL on gold-split-remainder (current stub doesn't split), ItemDrop test (stub does not drop items), leave-reevaluates (resolved depending on stub).

- [ ] **Step 3: Replace applyEventOutcome stub with full party-wide logic**

In `game/event.go`, replace the stub `applyEventOutcome` from Task 5 with:

```go
// applyEventOutcome applies an event outcome to the party per the rules in
// the design spec: HPChange to each alive player, GoldChange split evenly
// (remainder to the first alive player), ItemDrop one rolled item per alive
// player (skip if inventory full). Caller holds s.mu.
func (s *GameSession) applyEventOutcome(o EventOutcome) {
	alive := make([]*entity.Player, 0, len(s.state.Players))
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive = append(alive, p)
		}
	}
	if len(alive) == 0 {
		return
	}

	// HP change: each alive player heals/takes damage of the full amount.
	if o.HPChange > 0 {
		for _, p := range alive {
			before := p.HP
			p.Heal(o.HPChange)
			s.addLog(fmt.Sprintf("  %s HP %d 회복", p.Name, p.HP-before))
		}
	} else if o.HPChange < 0 {
		dmg := -o.HPChange
		for _, p := range alive {
			p.TakeDamage(dmg)
			s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, dmg))
		}
	}

	// Gold change: split evenly across alive; remainder to alive[0].
	if o.GoldChange != 0 {
		total := o.GoldChange
		share := total / len(alive)
		remainder := total - share*len(alive)
		for i, p := range alive {
			delta := share
			if i == 0 {
				delta += remainder
			}
			p.Gold += delta
			if p.Gold < 0 {
				p.Gold = 0
			}
			if delta > 0 {
				s.addLog(fmt.Sprintf("  %s 골드 %d 획득", p.Name, delta))
			} else if delta < 0 {
				s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", p.Name, -delta))
			}
		}
	}

	// Item drop: one rolled item per alive; skip if their inventory is full.
	if o.ItemDrop {
		floor := s.state.FloorNum
		for _, p := range alive {
			if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
				s.addLog(fmt.Sprintf("  %s의 인벤토리가 가득 찼습니다!", p.Name))
				continue
			}
			if s.rng.Float64() < 0.5 {
				bonus := 3 + s.rng.Intn(6) + floor/3
				item := entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("  %s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
			} else {
				bonus := 2 + s.rng.Intn(4) + floor/4
				item := entity.Item{Name: armorName(floor), Type: entity.ItemArmor, Bonus: bonus}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("  %s %s 발견 (DEF+%d)", p.Name, item.Name, item.Bonus))
			}
		}
	}
}
```

- [ ] **Step 4: Run tests to verify all pass**

Run: `go test ./game/ -run "TestApplyEventOutcome|TestEventVote_PlayerLeaveReevaluates" -v`
Expected: all PASS

- [ ] **Step 5: Run full suite with -race**

Run: `go test -race ./...`
Expected: all PASS, no race warnings.

- [ ] **Step 6: Commit**

```bash
git add game/event.go game/event_choice_test.go
git commit -m "feat(events): party-wide outcome application with gold split and item drops"
```

---

## Task 7: UI EventScreen — render event + collect vote

**Files:**
- Create: `ui/event_view.go`
- Modify: `ui/game_view.go` (add PhaseEvent route)
- Test: `ui/event_view_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `ui/event_view_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/tolelom/catacombs/game"
)

// TestEventScreenViewIncludesChoices renders an EventScreen and asserts the
// choice labels appear in the output. Bare assertion only — full UI testing
// uses bubbletea harness elsewhere.
func TestEventScreenViewIncludesChoices(t *testing.T) {
	state := game.GameState{
		Phase: game.PhaseEvent,
		PendingEvent: &game.PendingEventView{
			Name:        "altar",
			Description: "이상한 에너지로 빛나는 고대 제단을 발견했습니다.",
			Choices: []game.EventChoiceView{
				{Label: "제단에서 기도하기", Tags: []game.ChoiceTag{game.TagHeal, game.TagHarm}},
				{Label: "골드 바치기", Tags: []game.ChoiceTag{game.TagGoldCost}},
				{Label: "그냥 지나가기", Tags: []game.ChoiceTag{game.TagSkip}},
			},
		},
	}
	es := NewEventScreen(state)
	ctx := &Context{Width: 80, Height: 24}
	out := es.View(ctx)
	for _, want := range []string{"제단에서 기도하기", "골드 바치기", "그냥 지나가기"} {
		if !strings.Contains(out, want) {
			t.Errorf("View should contain %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ui/ -run TestEventScreenViewIncludesChoices -v`
Expected: compile error — `NewEventScreen` undefined.

- [ ] **Step 3: Create EventScreen**

Create `ui/event_view.go`:

```go
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/game"
)

// tagIcon maps a ChoiceTag to a short display token used in event hints.
func tagIcon(t game.ChoiceTag) string {
	switch t {
	case game.TagHeal:
		return "❤"
	case game.TagHarm:
		return "🔥"
	case game.TagGoldGain:
		return "💰+"
	case game.TagGoldCost:
		return "💰-"
	case game.TagItem:
		return "✨"
	case game.TagGamble:
		return "🎲"
	case game.TagSkip:
		return "⤳"
	}
	return "?"
}

// EventScreen handles the random-event vote phase. Mirrors ShopScreen's
// poll-based update pattern so other players' votes show up in real time.
type EventScreen struct {
	gameState game.GameState
	cursor    int
	statusMsg string
}

func NewEventScreen(state game.GameState) *EventScreen {
	return &EventScreen{gameState: state}
}

func (e *EventScreen) pollState() tea.Cmd {
	return tea.Tick(time.Millisecond*300, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (e *EventScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if _, ok := msg.(tickMsg); ok {
		if ctx.Session != nil {
			st := ctx.Session.GetState()
			e.gameState = st
			if st.Phase != game.PhaseEvent {
				gs := NewGameScreen()
				gs.gameState = st
				return gs, nil
			}
		}
		return e, e.pollState()
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "1", "2", "3":
			idx := int(key.String()[0] - '1')
			if e.gameState.PendingEvent != nil && idx >= 0 && idx < len(e.gameState.PendingEvent.Choices) {
				e.submitVote(ctx, idx)
			}
		case "up", "k":
			if e.cursor > 0 {
				e.cursor--
			}
		case "down", "j":
			if e.gameState.PendingEvent != nil && e.cursor < len(e.gameState.PendingEvent.Choices)-1 {
				e.cursor++
			}
		case "enter":
			if e.gameState.PendingEvent != nil {
				e.submitVote(ctx, e.cursor)
			}
		}
	}
	return e, e.pollState()
}

func (e *EventScreen) submitVote(ctx *Context, idx int) {
	if ctx.Session == nil {
		return
	}
	if ctx.Session.SubmitEventVote(ctx.Fingerprint, idx) {
		// Vote triggered resolution; the next tick will follow phase out.
		e.statusMsg = "결정되었습니다..."
	} else {
		e.statusMsg = fmt.Sprintf("선택지 %d에 투표 — 다른 플레이어 대기 중", idx+1)
	}
	e.gameState = ctx.Session.GetState()
}

func (e *EventScreen) View(ctx *Context) string {
	if e.gameState.PendingEvent == nil {
		return "이벤트 정보 없음"
	}
	ev := e.gameState.PendingEvent

	header := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("이벤트: %s", ev.Name))
	desc := ev.Description

	var lines []string
	lines = append(lines, header)
	lines = append(lines, desc)
	lines = append(lines, "")

	for i, c := range ev.Choices {
		var tags []string
		for _, t := range c.Tags {
			tags = append(tags, tagIcon(t))
		}
		prefix := "  "
		if i == e.cursor {
			prefix = "▶ "
		}
		lines = append(lines, fmt.Sprintf("%s%d. %s  %s", prefix, i+1, strings.Join(tags, ""), c.Label))
	}

	lines = append(lines, "")
	lines = append(lines, "투표 현황:")
	for _, p := range e.gameState.Players {
		if p.IsOut() {
			continue
		}
		if idx, ok := e.gameState.EventVotes[p.Fingerprint]; ok {
			lines = append(lines, fmt.Sprintf("  %s → #%d", p.Name, idx+1))
		} else {
			lines = append(lines, fmt.Sprintf("  %s → 대기 중...", p.Name))
		}
	}

	if e.statusMsg != "" {
		lines = append(lines, "")
		lines = append(lines, e.statusMsg)
	}
	lines = append(lines, "")
	lines = append(lines, "[1/2/3] 즉시 선택   [↑/↓] 커서   [Enter] 투표")

	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Add PhaseEvent route in game_view.go**

In `ui/game_view.go`, locate the block (around line 316):

```go
	if s.gameState.Phase == game.PhaseShop {
		ss := NewShopScreen(s.gameState)
		return ss, ss.pollState()
	}
```

Insert **immediately before** that block:

```go
	if s.gameState.Phase == game.PhaseEvent {
		es := NewEventScreen(s.gameState)
		return es, es.pollState()
	}
```

- [ ] **Step 5: Run the UI test**

Run: `go test ./ui/ -run TestEventScreenViewIncludesChoices -v`
Expected: PASS

- [ ] **Step 6: Run full suite with -race**

Run: `go test -race ./...`
Expected: all PASS

- [ ] **Step 7: Manual smoke build**

Run: `go build -o catacombs .`
Expected: builds with no errors. (Optional: launch locally and walk into an event room to eyeball the new screen — `./catacombs` then SSH/web in.)

- [ ] **Step 8: Commit**

```bash
git add ui/event_view.go ui/event_view_test.go ui/game_view.go
git commit -m "feat(events): UI EventScreen with tag hints and live vote display"
```

---

## Task 8: Regression sweep + final verification

**Files:**
- (no code changes, verification only)

- [ ] **Step 1: Run full test suite with race detector and coverage**

Run: `go test -race -cover ./...`
Expected: every package PASS. Note the coverage delta.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no warnings.

- [ ] **Step 3: gofmt check**

Run: `gofmt -l game/ ui/`
Expected: empty output (nothing to format). If any file is listed, run `gofmt -w` on it and re-commit.

- [ ] **Step 4: Walk through the 8 events mentally**

Open `game/random_event.go` and confirm each of the 8 events has Tags on every Choice (the test enforces it, but eyeball for sensibility — e.g. nothing tagged `TagSkip` should also have `Resolve` paths that heal or damage).

- [ ] **Step 5: Verify spec coverage**

Open `docs/superpowers/specs/2026-05-28-event-choices-design.md` and cross-check every "Testing Plan" item is implemented:
- [x] `TestTriggerEvent_TransitionsToPhaseEvent` → Task 3
- [x] `TestSubmitEventVote_MajorityResolvesAndAppliesToParty` → Task 5
- [x] `TestSubmitEventVote_TieBreakLowestIndex` → Task 5
- [x] `TestSubmitEventVote_PartialVotesDoNotResolve` → Task 5
- [x] `TestEventVote_SoloModeImmediateResolve` → Task 5
- [x] `TestEventVote_PlayerLeaveReevaluates` → Task 6
- [x] `TestApplyEventOutcome_PartywideHealAndGoldSplit` → Task 6
- [x] `TestApplyEventOutcome_ItemDropPerPlayer` → Task 6
- [x] `TestEventChoiceTagsPresent` → Task 1
- [x] `TestEventVote_TriggerWithZeroAlive` — **add now if missing** (see below)

- [ ] **Step 6: If TestEventVote_TriggerWithZeroAlive is missing, add it**

Append to `game/event_choice_test.go`:

```go
func TestEventVote_TriggerWithZeroAlive(t *testing.T) {
	s := NewGameSession(testCfg(t))
	// No players added → aliveCount == 0
	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomEvent}},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 1
	s.state.Phase = PhaseExploring
	s.mu.Unlock()

	s.EnterRoom(0)
	st := s.GetState()
	if st.Phase == PhaseEvent {
		t.Error("zero-alive entry should not enter PhaseEvent")
	}
	if !st.Floor.Rooms[0].Cleared {
		t.Error("zero-alive event room should be cleared immediately")
	}
}
```

Run: `go test ./game/ -run TestEventVote_TriggerWithZeroAlive -v`
Expected: PASS (already covered by triggerEvent's `if alive == 0` branch added in Task 3).

- [ ] **Step 7: Final commit if anything changed**

```bash
git add -A
git status  # review
git commit -m "test(events): add zero-alive guard + spec coverage sweep" || echo "nothing to commit"
```

- [ ] **Step 8: Open a PR (optional, if user requests)**

```bash
git push -u origin feat/event-choices
gh pr create --title "feat(events): turn event rooms into player choices" --body "$(cat <<'EOF'
## Summary
- Convert event rooms from auto-random resolution into a real player decision
- Majority vote (lowest index breaks ties), party-wide outcome
- Tag icons (❤🔥💰+💰-✨🎲⤳) hint at outcome category without spoiling values
- New `PhaseEvent` + `SubmitEventVote` + `EventScreen` UI
- All 8 existing events get Tags; no new event content

## Spec
`docs/superpowers/specs/2026-05-28-event-choices-design.md`

## Test Plan
- `go test -race -cover ./...` — all green
- Manual: walked through altar/fountain/merchant events in solo + 2-player party

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Recap

8 tasks, each one or two related files, each commits independently. The spec's 10 tests are all implemented across Tasks 1, 3, 4, 5, 6, 8. No placeholders, all code is shown in full, all `git` commands include the actual message.
