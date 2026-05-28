# Skill Tree Branching — Design Spec

**Date:** 2026-05-28
**Status:** Approved (in-conversation)
**Author:** Claude + tolelom

## Problem

`entity/skill_tree.go` defines each branch as a linear array `Nodes [4]SkillNode` and `Allocate` advances `Allocated` by 1 each call. There is **no choice within a branch** — every Warrior Tank run unlocks Iron Skin → Fortitude → Bastion → Unbreakable in that exact order. The only decision is the one-time `BranchIndex` pick. Once made, the next 20 floors have zero build-shaping decisions. This is fun-killer #2 from the gameplay diagnosis (after event auto-resolve, fixed in event-choices).

## Goal

Convert each branch from a linear sequence into a **diamond-shaped tree with 2 fork points**: T2 (mid-tier) and T4 (capstone). One run still spends 4 points, but now picks A or B at each fork — 4 distinct builds per branch.

## Non-Goals

- Changing the 4-points-per-run cap or per-floor grant rate
- Changing branch lock policy (`BranchIndex` still locks after first allocation)
- Persisting skills to DB (still run-scoped)
- Adding cross-branch nodes or respec mechanics

## Design Decisions (from brainstorm)

| Decision | Value | Rationale |
|----------|-------|-----------|
| Tree shape | Diamond (2 fork points: T2 + T4 capstone) | Sweet spot: 4 builds per branch × 8 branches = 32 unique build paths with manageable content load |
| Path lock | T2 choice locks (same as BranchIndex) | Consistent with existing lock-after-first-allocation policy; gives decisions weight |
| Content scale | 6 designed nodes per branch × 8 branches = 48 total (was 32) | +16 new nodes |
| Data shape | Keep `Allocated int`; add `Choices string` (max length 2) | Cleaner than single Path string; reuses existing semantics |
| Migration | None (skills not persisted) | Run-scoped state; new structure applies on next run |
| Keybindings | `[` / `]` reused; meaning depends on `NextIsBranch()` step | Smallest UX change |

## Architecture

```
       T1 (auto)
        |
     /     \
   T2A    T2B    ← first fork (Choices[0])
     \     /
       T3 (auto)
        |
     /     \
   T4A    T4B    ← capstone fork (Choices[1])
```

A run with `Choices = "AB"` picks T1 → T2A → T3 → T4B.

## Components

### Data model (entity/skill_tree.go)

```go
type PlayerSkills struct {
    BranchIndex int    // -1 (unselected), 0, 1 — unchanged
    Points      int    // total earned (1 per floor clear, +mastery bonus) — unchanged
    Allocated   int    // points spent in chosen branch (0–4) — semantics preserved
    Choices     string // fork choices, max length 2: e.g. "" | "A" | "AB"
}

type SkillTree struct {
    Name string
    T1   SkillNode    // unconditional first allocation
    T2   [2]SkillNode // index 0 = 'A', index 1 = 'B' — first fork
    T3   SkillNode    // unconditional third allocation
    T4   [2]SkillNode // index 0 = 'A', index 1 = 'B' — capstone fork
}
```

Field semantics (`PlayerSkills`):
- `Allocated == 0`: nothing yet (or BranchIndex unset)
- `Allocated == 1`: T1 only
- `Allocated == 2`: T1 + T2A or T2B (`Choices` length 1)
- `Allocated == 3`: T1 + T2x + T3 (`Choices` length still 1)
- `Allocated == 4`: T1 + T2x + T3 + T4y (`Choices` length 2)

Invariant on `len(Choices)` given `Allocated`:

| `Allocated` | `len(Choices)` | Why |
|-------------|----------------|-----|
| 0 | 0 | nothing allocated |
| 1 | 0 | T1 (auto) only |
| 2 | 1 | T1 + T2 fork (choice recorded) |
| 3 | 1 | T3 (auto) added; no new choice |
| 4 | 2 | T4 capstone fork (choice recorded) |

Choices grow only on tiers 2 and 4. T1 (after first allocation) and T3 (after third) are automatic.

### API surface (entity/skill_tree.go)

```go
// Allocate spends one skill point. choice is interpreted only at fork tiers
// (T2 and T4); it is ignored at auto tiers (T1 and T3). For fork tiers,
// choice must be 'A' or 'B'.
func (ps *PlayerSkills) Allocate(branchIdx int, class Class, choice byte) error

// NextIsBranch reports whether the next Allocate call will be at a fork
// (T2 or T4). Caller uses this to decide whether to prompt the user.
func (ps *PlayerSkills) NextIsBranch() bool

// NextStepLabel returns a short Korean label describing what the next
// allocation is for. Used by UI to render the right prompt.
func (ps *PlayerSkills) NextStepLabel(class Class) string

// GetBranches returns the two SkillTrees for the given class.
func GetBranches(class Class) [2]SkillTree
```

`Allocate` behavior:
- If `BranchIndex == -1` → set `BranchIndex = branchIdx`, then proceed to allocate as if for `Allocated == 0`.
- If `BranchIndex != -1 && BranchIndex != branchIdx` → return `errors.New("cannot switch branch after first allocation")`.
- If `Allocated >= 4` → return `errors.New("branch fully allocated")`.
- If `Points <= Allocated` → return `errors.New("no available skill points")`.
- Then advance based on current `Allocated`:
  - 0 → `Allocated = 1` (T1 done). `choice` ignored.
  - 1 → fork: `choice` must be `'A'` or `'B'`. Append to `Choices`. `Allocated = 2`.
  - 2 → `Allocated = 3` (T3 done). `choice` ignored.
  - 3 → fork: `choice` must be `'A'` or `'B'`. Append to `Choices`. `Allocated = 4`.

Bonus-aggregation functions (`GetATKBonus`/`DEFBonus`/`MaxHPBonus`/`SkillPower`/`CritChance`/`HealBoost`) keep their signatures. Internally, `allocatedNodes(class)` walks the path:

```go
func (ps *PlayerSkills) allocatedNodes(class Class) []SkillNode {
    if ps == nil || ps.BranchIndex < 0 || ps.Allocated == 0 {
        return nil
    }
    branches := GetBranches(class)
    tree := branches[ps.BranchIndex]
    out := make([]SkillNode, 0, 4)
    if ps.Allocated >= 1 {
        out = append(out, tree.T1)
    }
    if ps.Allocated >= 2 && len(ps.Choices) >= 1 {
        out = append(out, tree.T2[choiceToIdx(ps.Choices[0])])
    }
    if ps.Allocated >= 3 {
        out = append(out, tree.T3)
    }
    if ps.Allocated >= 4 && len(ps.Choices) >= 2 {
        out = append(out, tree.T4[choiceToIdx(ps.Choices[1])])
    }
    return out
}
```

### Branch content (48 nodes)

Each branch's existing 4 nodes become T1, one of T2, T3, one of T4. New nodes added:

#### Warrior

**Tank** (defense identity)
| Tier | A | B |
|------|---|---|
| T1 | Iron Skin (DEF +3) | — |
| T2 | Fortitude (MaxHP +20) | Spike Mail (DEF +3, retaliation theme) |
| T3 | Bastion (DEF +5) | — |
| T4 | Unbreakable (MaxHP +50) | Last Stand (ATK +12, capstone for tanks who counterpunch) |

**Berserker** (rage identity)
| Tier | A | B |
|------|---|---|
| T1 | Fury (ATK +4) | — |
| T2 | Wrath (SkillPower +20) | Reckless (ATK +5) |
| T3 | Rampage (ATK +6) | — |
| T4 | Bloodthirst (ATK +12) | Unending Rage (MaxHP +40) |

#### Mage

**Elementalist** (skill power identity)
| Tier | A | B |
|------|---|---|
| T1 | Arcane Focus (SkillPower +15) | — |
| T2 | Elemental Fury (ATK +5) | Mana Surge (SkillPower +15) |
| T3 | Overload (SkillPower +25) | — |
| T4 | Cataclysm (SkillPower +50) | Pyroclasm (ATK +14) |

**Chronomancer** (defense/HP identity)
| Tier | A | B |
|------|---|---|
| T1 | Temporal Shield (DEF +3) | — |
| T2 | Time Warp (ATK +3) | Slow Field (DEF +3) |
| T3 | Stasis (MaxHP +15) | — |
| T4 | Eternity (MaxHP +40) | Aeon Strike (ATK +12) |

#### Healer

**Guardian** (heal-boost identity)
| Tier | A | B |
|------|---|---|
| T1 | Blessing (HealBoost +20) | — |
| T2 | Divine Armor (DEF +4) | Aegis (MaxHP +20) |
| T3 | Miracle (HealBoost +30) | — |
| T4 | Resurrection (HealBoost +60) | Sanctum (DEF +8) |

**Priest** (damage-via-faith identity)
| Tier | A | B |
|------|---|---|
| T1 | Smite (ATK +5) | — |
| T2 | Holy Power (SkillPower +20) | Vow (ATK +4) |
| T3 | Judgment (ATK +7) | — |
| T4 | Wrath of God (ATK +14) | Crusade (SkillPower +40) |

#### Rogue

**Assassin** (crit identity)
| Tier | A | B |
|------|---|---|
| T1 | Backstab (ATK +5) | — |
| T2 | Precision (CritChance +15) | Shadow Step (ATK +4) |
| T3 | Execute (ATK +8) | — |
| T4 | Death Mark (CritChance +30) | Bloodletter (ATK +14) |

**Alchemist** (sustain identity)
| Tier | A | B |
|------|---|---|
| T1 | Tonic (HealBoost +15) | — |
| T2 | Brew (SkillPower +20) | Bitter Tonic (HealBoost +15) |
| T3 | Elixir (MaxHP +25) | — |
| T4 | Panacea (MaxHP +60) | Universal Solvent (HealBoost +50) |

**Design intent**: T2 splits each branch into "stay-on-theme" (A path keeps the original 4-node character) vs "off-theme dip" (B path leans into a complementary stat). T4 doubles down on whichever sub-identity the player committed to at T2.

### Game session API (game/session.go)

```go
// Before:
func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int) error

// After:
func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int, choice byte) error
```

All call sites must update: the only one is `ui/game_view.go::handleAllocateInput`. The session method delegates to `PlayerSkills.Allocate` and recomputes MaxHP delta (existing behavior preserved).

### UI (ui/game_view.go)

Current behavior: pressing `[` or `]` calls `AllocateSkillPoint(fp, 0)` or `(fp, 1)`. Replace with stage-aware logic:

```go
case "[", "]":
    branchIdx := 0
    if key == "]" {
        branchIdx = 1
    }
    var choice byte
    for _, p := range gs.Players {
        if p.Fingerprint == ctx.Fingerprint && p.Skills != nil {
            if p.Skills.NextIsBranch() {
                // [ = A, ] = B
                if key == "[" {
                    choice = 'A'
                } else {
                    choice = 'B'
                }
                branchIdx = p.Skills.BranchIndex // preserve existing branch
            }
            break
        }
    }
    err := ctx.Session.AllocateSkillPoint(ctx.Fingerprint, branchIdx, choice)
```

UI display next to the player HUD adapts:
- `BranchIndex == -1`: "스킬 포인트: [/] 분기 선택 (Tank / Berserker)"
- `NextIsBranch()` returns true at T2 or T4: "[/] A or B — `<NextStepLabel>`"
- Otherwise (T3 auto): "[/] 다음 노드 (T3 자동)" — both keys do the same thing

### Mastery interaction (ui/perma.go)

No change needed. Mastery still grants `Points` only; allocation gating is unchanged. A player with mastery Points=4 at run start can pre-allocate all 4 points before entering floor 1 — same as today, but now with two real choice points.

## Data flow

```
Player clears floor → Points++ (turn.go::advanceFloor, unchanged)
  ↓
UI HUD checks p.Skills.Points > p.Skills.Allocated && Allocated < 4
  → renders prompt with NextStepLabel
  ↓
Player presses [ or ]
  → UI determines (branchIdx, choice) based on NextIsBranch()
  → Session.AllocateSkillPoint(fp, branchIdx, choice)
  → PlayerSkills.Allocate updates Allocated + Choices
  → MaxHP delta applied (HP scales proportionally, existing)
```

## Edge cases

| Case | Behavior |
|------|----------|
| `choice == 0` at a fork | Allocate returns error; UI shows message |
| `choice == 'A'` at auto tier | choice ignored, Allocated++ |
| Player tries to switch branch mid-run | Allocate returns "cannot switch branch" |
| Allocated already 4 | Allocate returns "fully allocated" |
| Points <= Allocated | Allocate returns "no available skill points" |
| Mastery starting Points >= 4 | Player can allocate all 4 immediately; choices still meaningful |
| Reconnect mid-allocation | Skills state on Player struct; reconnect carries Choices intact |

## Testing

`entity/skill_tree_test.go` — extend with:

1. `TestAllocate_T1IsAutoAndIgnoresChoice` — first call advances Allocated to 1, Choices remains "", regardless of choice byte.
2. `TestAllocate_T2RequiresChoice` — second call with choice=0 returns error.
3. `TestAllocate_T2AppendsChoice` — second call with choice='B' sets Choices = "B".
4. `TestAllocate_T3IsAutoAndIgnoresChoice` — third call advances Allocated to 3, Choices length stays 1.
5. `TestAllocate_T4Capstone` — fourth call with choice='A' sets Choices = "BA".
6. `TestAllocate_LocksBranchAfterFirstAllocation` — switching branchIdx between calls returns error.
7. `TestAllocate_LocksFifthAllocation` — fifth call returns "fully allocated".
8. `TestAllocatedNodes_PathTraversal` — Choices "AB" with Allocated 4 returns [T1, T2[0], T3, T4[1]].
9. `TestGetATKBonus_SumsAcrossPath` — Warrior Berserker with Choices "BB" sums Fury+Reckless+Rampage+UnendingRage's ATK contributions (which is just Fury+Reckless+Rampage; UnendingRage is HP, but check ATK total).
10. `TestNextIsBranch_TrueAtT2AndT4Only` — false at Allocated=0/2, true at Allocated=1/3.
11. `TestEveryBranchTreeIsWellFormed` — for every Class × branch in `branchTrees`, T1 and T3 nodes have non-empty names; T2[0/1] and T4[0/1] have non-empty names; no duplicate names within a branch.

11 tests covering the data layer. Game/UI tests are exercised by existing integration paths plus the routing change in `ui/game_view.go`.

## Migration / compatibility

- `PlayerSkills.Allocated` semantics preserved (0–4 range).
- `Allocate` signature **changed** (added `choice byte`). All call sites must update — there's only one: `game/session.go::AllocateSkillPoint`, which adds `choice byte` to its own signature and forwards.
- DB: no change (skills not persisted).
- UI: changes the meaning of `[`/`]` keys context-dependently; all other input flows unchanged.

## Out of scope (future)

- Respec / refund mechanics
- Cross-branch nodes
- Class-specific UI rendering for builds
- Achievement: "completed all 32 build paths"

## Estimate

- Code: ~350 lines (140 entity/skill_tree.go full rewrite + 30 player/session/turn touch-ups + 20 ui/game_view.go branch+choice logic + 16 new node definitions)
- Tests: ~250 lines (11 tests)
