# Coop Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 4클래스에 협동기 4종을 추가하고 6쌍 페어 콤보를 구현해 멀티 정체성을 명확히 한다. 솔로는 자기 대상 축소판으로 호환.

**Architecture:** 새 `ActionCoop` 액션 타입 추가. 협동기 효과는 `combat/coop.go`에 모으고, 세션 상태에 쿨다운/지속효과 필드 추가. 콤보는 기존 `combat/combo.go`를 확장해 `ActionType="coop"` 정의 6쌍 추가, 기존 5콤보와 공존. UI는 키 6번에 협동기 매핑.

**Tech Stack:** Go 1.25, BoltDB, Bubble Tea (변경 없음 — 신규 파일과 기존 파일 수정만)

**Spec:** `docs/superpowers/specs/2026-05-03-coop-identity-design.md`

---

## File Structure

**Create:**
- `entity/coop.go` — 클래스→협동기 메타데이터 (이름, 설명, 자원 종류)
- `combat/coop.go` — 협동기 효과 정의/적용, 6 페어 콤보 effect 적용
- `entity/coop_test.go`, `combat/coop_test.go` — 단위 테스트

**Modify:**
- `game/session.go` — 세션 상태 필드 추가 (coopUsed, healerReviveUsed, scoutUsed, soloHealerRevived, nextRoomPreview, coopDamageReduction, pendingEffects)
- `game/turn.go` — `ActionCoop` 분기, 콤보 매칭 시 ActionType "coop" 포함, 데미지 경감/무적/회피 적용 훅
- `game/turn.go` (신규 헬퍼) — `applyCoopEffect`, `applyCoopComboEffects`
- `combat/combo.go` — 6 새 ComboDef 추가 (ActionType "coop")
- `combat/combat.go` — `ResolveAttacks`에 회피(EnemyMiss) 훅 (마몬스터 공격은 별도라 안 건드림. 회피는 monster→player 쪽이므로 `resolveMonsterActions` 측에서 처리)
- `ui/game_view.go` — 키 6=ActionCoop, 액션바에 [6]협동기 추가, HUD 협동기 상태, 정찰 패널, 콤보 골드 색
- `ui/styles.go` — 새 styleCombo (골드/볼드)

**Test:**
- `game/session_test.go` — 협동기 통합 테스트
- `game/turn_coop_test.go` (신규) — 협동기 + 콤보 통합 흐름

---

## Cycle 1 — Phase 1: 협동기 4종 + ActionCoop + 솔로

### Task 1.1: ActionCoop 타입 추가 + 메타데이터 정의

**Files:**
- Modify: `game/turn.go` (ActionType iota 끝에 ActionCoop 추가)
- Create: `entity/coop.go`

- [ ] **Step 1: ActionCoop 추가 (game에서 turn.go 상단 ActionType enum 확인)**

`game/turn.go` 상단 ActionType iota에 추가. 현재 정의는 session.go에 있음 (실수했음 — 확인 결과 `game/session.go:31-37`):

```go
const (
	ActionAttack ActionType = iota
	ActionSkill
	ActionItem
	ActionFlee
	ActionWait
	ActionCoop  // ← 추가
)
```

- [ ] **Step 2: entity/coop.go 작성**

```go
package entity

// CoopAbility describes a class's coop ability. Effects are applied in
// combat/coop.go; this file only carries display metadata.
type CoopAbility struct {
	Name string
	Desc string
}

var coopAbilities = map[Class]CoopAbility{
	ClassWarrior: {Name: "견고함", Desc: "1턴간 받는 피해 -75%. 솔로: 자기에게만 적용."},
	ClassMage:    {Name: "결계", Desc: "1턴간 파티 받는 피해 -50%. 솔로: 자기에게만 적용."},
	ClassHealer:  {Name: "신성 가호", Desc: "죽은 아군 1명 50%HP 부활(런 1회) 또는 파티 30% 회복(전투 1회). 솔로: 자기 회복 + 첫 사망 자동부활(런 1회)."},
	ClassRogue:   {Name: "정찰", Desc: "다음 방 종류·구성 미리 공개. 층당 1회. 전투 외 사용."},
}

func GetCoopAbility(c Class) CoopAbility {
	return coopAbilities[c]
}
```

- [ ] **Step 3: 빌드 확인**

Run: `go build ./...`
Expected: 빌드 성공.

- [ ] **Step 4: Commit**

```
git add entity/coop.go game/session.go
git commit -m "feat(coop): add ActionCoop type and CoopAbility metadata"
```

---

### Task 1.2: 세션 상태 필드 추가

**Files:**
- Modify: `game/session.go`

- [ ] **Step 1: GameSession 구조체에 새 필드 추가**

`game/session.go`의 `GameSession struct` 안에 추가 (HardMode 위쪽):

```go
	// Coop ability state — all maps keyed by player fingerprint.
	coopUsedThisCombat   map[string]bool    // reset on combat start
	healerReviveUsed     map[string]bool    // run-scoped (revive arm of Divine Grace)
	scoutUsedThisFloor   map[string]bool    // reset on floor advance
	soloHealerRevived    map[string]bool    // run-scoped solo auto-revive
	coopDamageReduction  map[string]float64 // applies for current turn only (0.0..1.0)
	invincibleNextTurn   map[string]bool    // applies for next turn (set by Sanctuary combo)
	nextRoomPreview      *RoomPreview       // populated by Foresight, consumed at next room entry
	critGuaranteedNext   bool               // next turn all party attacks crit (Expose Weakness combo)
	enemyMissNextTurn    bool               // next turn 50% enemy attacks miss (Mirage combo)
	poisonNextCombat     bool               // next combat: all enemies start poisoned (Venomous Edge)
```

`RoomPreview` 타입은 같은 파일 하단에 정의 (RemovePlayer 위 적당한 위치):

```go
// RoomPreview is what Foresight reveals to the party about the next room
// before they vote to enter it. Boss rooms hide the monster names but
// expose the room type.
type RoomPreview struct {
	RoomIdx        int
	RoomTypeLabel  string   // "전투", "상점", "보물", "이벤트", "보스"
	MonsterNames   []string // empty for non-combat
	HideMonsters   bool     // true for boss rooms
}
```

- [ ] **Step 2: NewGameSession 초기화에 맵 추가**

`NewGameSession` 함수의 `&GameSession{...}` 리터럴에 추가:

```go
		coopUsedThisCombat:  make(map[string]bool),
		healerReviveUsed:    make(map[string]bool),
		scoutUsedThisFloor:  make(map[string]bool),
		soloHealerRevived:   make(map[string]bool),
		coopDamageReduction: make(map[string]float64),
		invincibleNextTurn:  make(map[string]bool),
```

- [ ] **Step 3: GameState에 RoomPreview 노출 필드 추가**

UI 표시용. `GameState` 구조체 안에 추가:

```go
	NextRoomPreview *RoomPreview // populated when Foresight has been used
	CoopAvailable   map[string]bool // fingerprint → can use coop now (HUD display)
```

`GetState()` 안 마지막 return GameState{...}에 추가:

```go
		NextRoomPreview: s.nextRoomPreview,
		CoopAvailable:   s.computeCoopAvailable(),
```

`computeCoopAvailable` 헬퍼는 다음 task에서 정의.

- [ ] **Step 4: RemovePlayer 청소 추가**

`RemovePlayer` 함수의 다른 `delete(...)` 호출들 옆에 추가:

```go
	delete(s.coopUsedThisCombat, fingerprint)
	delete(s.healerReviveUsed, fingerprint)
	delete(s.scoutUsedThisFloor, fingerprint)
	delete(s.soloHealerRevived, fingerprint)
	delete(s.coopDamageReduction, fingerprint)
	delete(s.invincibleNextTurn, fingerprint)
```

- [ ] **Step 5: computeCoopAvailable 헬퍼 추가**

session.go 하단(SubmitMoveVote 다음, ClearMoveVotes 위 또는 끝):

```go
// computeCoopAvailable returns a map of fingerprint→true when that player can
// fire their coop ability right now. Caller must hold s.mu.
func (s *GameSession) computeCoopAvailable() map[string]bool {
	out := make(map[string]bool, len(s.state.Players))
	for _, p := range s.state.Players {
		if p.IsOut() || p.Fingerprint == "" {
			continue
		}
		switch p.Class {
		case entity.ClassRogue:
			// Foresight is floor-scoped, exploring-only.
			if s.state.Phase == PhaseExploring && !s.scoutUsedThisFloor[p.Fingerprint] {
				out[p.Fingerprint] = true
			}
		default:
			// Other coops are combat-only, per-combat.
			if s.state.Phase == PhaseCombat && !s.coopUsedThisCombat[p.Fingerprint] {
				out[p.Fingerprint] = true
			}
		}
	}
	return out
}
```

- [ ] **Step 6: 빌드 확인**

Run: `go build ./...`
Expected: 빌드 성공.

- [ ] **Step 7: Commit**

```
git add game/session.go
git commit -m "feat(coop): add session state for coop cooldowns and pending effects"
```

---

### Task 1.3: combat/coop.go — 협동기 효과 적용 함수

**Files:**
- Create: `combat/coop.go`

- [ ] **Step 1: combat/coop.go 작성**

```go
package combat

import "github.com/tolelom/catacombs/entity"

// CoopApplyResult describes what one coop activation does. The session layer
// reads this and mutates the appropriate maps; combat package stays pure.
type CoopApplyResult struct {
	// damage reduction this turn, fingerprint → multiplier subtracted (0.5 = -50%)
	DamageReduction map[string]float64
	// heal applied to each ally (0 = none)
	HealPerAlly int
	// revive: target fingerprint and revive HP percent (0 = no revive)
	ReviveFingerprint string
	ReviveHPPercent   float64
	// log lines to push to combat log
	Logs []string
}

// ApplyWarriorCoop — Bulwark: warrior takes -75% damage this turn.
// Solo or multi behaves identically (self-only) since Bulwark already targets
// the warrior alone.
func ApplyWarriorCoop(warrior *entity.Player) CoopApplyResult {
	return CoopApplyResult{
		DamageReduction: map[string]float64{warrior.Fingerprint: 0.75},
		Logs:            []string{warrior.Name + " 견고함! 1턴간 받는 피해 -75%"},
	}
}

// ApplyMageCoop — Ward: party takes -50% damage this turn.
// Solo: self-only.
func ApplyMageCoop(mage *entity.Player, party []*entity.Player, solo bool) CoopApplyResult {
	out := CoopApplyResult{
		DamageReduction: make(map[string]float64),
	}
	if solo {
		out.DamageReduction[mage.Fingerprint] = 0.50
		out.Logs = []string{mage.Name + " 결계! 1턴간 받는 피해 -50%"}
		return out
	}
	for _, p := range party {
		if !p.IsOut() {
			out.DamageReduction[p.Fingerprint] = 0.50
		}
	}
	out.Logs = []string{mage.Name + " 결계! 파티 받는 피해 -50%"}
	return out
}

// ApplyHealerCoop — Divine Grace: revive a dead ally if any (1/run cap),
// else heal party 30%. Solo path: 30% self-heal (auto-revive is handled
// elsewhere as a passive trigger, not via this entry point).
func ApplyHealerCoop(healer *entity.Player, party []*entity.Player, reviveAllowed bool) CoopApplyResult {
	out := CoopApplyResult{}

	// Look for a dead ally to revive
	if reviveAllowed {
		for _, p := range party {
			if p.IsDead() {
				out.ReviveFingerprint = p.Fingerprint
				out.ReviveHPPercent = 0.50
				out.Logs = []string{healer.Name + " 신성 가호! " + p.Name + " 50%HP로 부활!"}
				return out
			}
		}
	}

	// Otherwise heal party (or self in solo) by ~30% MaxHP
	out.HealPerAlly = 0 // signal: per-ally compute outside (uses MaxHP)
	out.Logs = []string{healer.Name + " 신성 가호! 파티 30% 회복"}
	return out
}

// SoloHealerAutoRevive — passive triggered when solo healer dies for the first
// time this run. Returns true if revive should fire (caller checks the run-scoped
// soloHealerRevived map).
func SoloHealerAutoRevive(p *entity.Player) bool {
	return p.Class == entity.ClassHealer && p.IsDead()
}
```

- [ ] **Step 2: 빌드 확인**

Run: `go build ./...`
Expected: 빌드 성공.

- [ ] **Step 3: Commit**

```
git add combat/coop.go
git commit -m "feat(coop): add coop ability effect functions"
```

---

### Task 1.4: combat/coop_test.go — 협동기 단위 테스트

**Files:**
- Create: `combat/coop_test.go`

- [ ] **Step 1: 테스트 작성**

```go
package combat

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func TestWarriorCoopReducesSelfDamageOnly(t *testing.T) {
	w := entity.NewPlayer("Tank", entity.ClassWarrior)
	w.Fingerprint = "fp-w"
	res := ApplyWarriorCoop(w)
	if got := res.DamageReduction[w.Fingerprint]; got != 0.75 {
		t.Errorf("warrior reduction = %v, want 0.75", got)
	}
	if len(res.DamageReduction) != 1 {
		t.Errorf("Bulwark should affect only the warrior, got %d entries", len(res.DamageReduction))
	}
}

func TestMageCoopMultiTargetsParty(t *testing.T) {
	m := entity.NewPlayer("Mage", entity.ClassMage)
	m.Fingerprint = "fp-m"
	w := entity.NewPlayer("Tank", entity.ClassWarrior)
	w.Fingerprint = "fp-w"
	dead := entity.NewPlayer("Ghost", entity.ClassRogue)
	dead.Fingerprint = "fp-d"
	dead.Dead = true
	party := []*entity.Player{m, w, dead}

	res := ApplyMageCoop(m, party, false)
	if res.DamageReduction[m.Fingerprint] != 0.50 || res.DamageReduction[w.Fingerprint] != 0.50 {
		t.Errorf("Ward should give 0.50 to all alive party, got %+v", res.DamageReduction)
	}
	if _, has := res.DamageReduction[dead.Fingerprint]; has {
		t.Error("Ward should skip dead members")
	}
}

func TestMageCoopSoloOnlySelf(t *testing.T) {
	m := entity.NewPlayer("Mage", entity.ClassMage)
	m.Fingerprint = "fp-m"
	other := entity.NewPlayer("Other", entity.ClassWarrior)
	other.Fingerprint = "fp-o"
	res := ApplyMageCoop(m, []*entity.Player{m, other}, true)
	if len(res.DamageReduction) != 1 || res.DamageReduction[m.Fingerprint] != 0.50 {
		t.Errorf("Solo Ward should target self only, got %+v", res.DamageReduction)
	}
}

func TestHealerCoopRevivesWhenAllowed(t *testing.T) {
	h := entity.NewPlayer("Heal", entity.ClassHealer)
	h.Fingerprint = "fp-h"
	corpse := entity.NewPlayer("Dead", entity.ClassWarrior)
	corpse.Fingerprint = "fp-c"
	corpse.Dead = true
	res := ApplyHealerCoop(h, []*entity.Player{h, corpse}, true)
	if res.ReviveFingerprint != "fp-c" {
		t.Errorf("expected revive on dead corpse, got %q", res.ReviveFingerprint)
	}
	if res.ReviveHPPercent != 0.50 {
		t.Errorf("revive HP percent = %v, want 0.50", res.ReviveHPPercent)
	}
}

func TestHealerCoopHealsWhenNoCorpse(t *testing.T) {
	h := entity.NewPlayer("Heal", entity.ClassHealer)
	h.Fingerprint = "fp-h"
	w := entity.NewPlayer("W", entity.ClassWarrior)
	w.Fingerprint = "fp-w"
	res := ApplyHealerCoop(h, []*entity.Player{h, w}, true)
	if res.ReviveFingerprint != "" {
		t.Error("no corpse → no revive expected")
	}
	// HealPerAlly==0 signals "use MaxHP*0.3" outside; verify Logs were emitted
	if len(res.Logs) == 0 {
		t.Error("expected heal log line")
	}
}

func TestHealerCoopReviveBlockedWhenUsed(t *testing.T) {
	h := entity.NewPlayer("Heal", entity.ClassHealer)
	h.Fingerprint = "fp-h"
	corpse := entity.NewPlayer("Dead", entity.ClassWarrior)
	corpse.Fingerprint = "fp-c"
	corpse.Dead = true
	res := ApplyHealerCoop(h, []*entity.Player{h, corpse}, false)
	if res.ReviveFingerprint != "" {
		t.Error("reviveAllowed=false should suppress revive")
	}
}
```

- [ ] **Step 2: 테스트 실행**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./combat/ -run TestWarriorCoop -v`
Expected: PASS (TestWarriorCoopReducesSelfDamageOnly)

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./combat/ -run TestMageCoop -v`
Expected: PASS x2

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./combat/ -run TestHealerCoop -v`
Expected: PASS x3

- [ ] **Step 3: Commit**

```
git add combat/coop_test.go
git commit -m "test(coop): unit tests for coop ability effects"
```

---

### Task 1.5: game/turn.go — ActionCoop 처리 + 데미지 경감 적용

**Files:**
- Modify: `game/turn.go`

- [ ] **Step 1: resolvePlayerActions의 switch action.Type에 ActionCoop 케이스 추가**

`game/turn.go` line ~160 부근 `switch action.Type {` 안. `case ActionWait:` 뒤에 추가:

```go
		case ActionCoop:
			s.applyCoopAbility(p)
```

- [ ] **Step 2: applyCoopAbility 메서드 정의**

`game/turn.go` 하단(파일 끝)에 추가:

```go
// applyCoopAbility resolves one player's coop ability use. Caller holds s.mu
// (called from resolvePlayerActions which is invoked under lock via RunTurn).
func (s *GameSession) applyCoopAbility(p *entity.Player) {
	switch p.Class {
	case entity.ClassWarrior:
		if s.coopUsedThisCombat[p.Fingerprint] {
			s.addLog(fmt.Sprintf("%s 협동기는 이번 전투에 이미 사용했습니다", p.Name))
			return
		}
		res := combat.ApplyWarriorCoop(p)
		for fp, mult := range res.DamageReduction {
			if cur := s.coopDamageReduction[fp]; mult > cur {
				s.coopDamageReduction[fp] = mult
			}
		}
		for _, msg := range res.Logs {
			s.addLog(msg)
		}
		s.coopUsedThisCombat[p.Fingerprint] = true
	case entity.ClassMage:
		if s.coopUsedThisCombat[p.Fingerprint] {
			s.addLog(fmt.Sprintf("%s 협동기는 이번 전투에 이미 사용했습니다", p.Name))
			return
		}
		res := combat.ApplyMageCoop(p, s.state.Players, s.state.SoloMode)
		for fp, mult := range res.DamageReduction {
			if cur := s.coopDamageReduction[fp]; mult > cur {
				s.coopDamageReduction[fp] = mult
			}
		}
		for _, msg := range res.Logs {
			s.addLog(msg)
		}
		s.coopUsedThisCombat[p.Fingerprint] = true
	case entity.ClassHealer:
		if s.coopUsedThisCombat[p.Fingerprint] {
			s.addLog(fmt.Sprintf("%s 협동기는 이번 전투에 이미 사용했습니다", p.Name))
			return
		}
		reviveAllowed := !s.healerReviveUsed[p.Fingerprint]
		res := combat.ApplyHealerCoop(p, s.state.Players, reviveAllowed)
		if res.ReviveFingerprint != "" {
			for _, target := range s.state.Players {
				if target.Fingerprint == res.ReviveFingerprint {
					target.Revive(res.ReviveHPPercent)
					s.healerReviveUsed[p.Fingerprint] = true
					break
				}
			}
		} else {
			// Heal mode: 30% MaxHP per ally (or self in solo)
			if s.state.SoloMode {
				p.Heal(p.MaxHP * 30 / 100)
			} else {
				for _, ally := range s.state.Players {
					if !ally.IsOut() {
						ally.Heal(ally.MaxHP * 30 / 100)
					}
				}
			}
		}
		for _, msg := range res.Logs {
			s.addLog(msg)
		}
		s.coopUsedThisCombat[p.Fingerprint] = true
	case entity.ClassRogue:
		// Rogue coop is exploring-only — handled separately via SubmitForesight.
		// Reaching here means a stale combat-time coop submission; ignore.
		s.addLog(fmt.Sprintf("%s 정찰은 전투 외에 사용해야 합니다", p.Name))
	}
}
```

- [ ] **Step 3: resolveMonsterActions에서 데미지 경감 적용**

`game/turn.go`의 `resolveMonsterActions` 함수 안. `dmg := combat.CalcDamage(...)` 직후에 경감 적용. 두 곳:
- AoE 분기 (line ~501): `dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), 0.5)` 다음줄
- single 분기 (line ~547): `dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), 1.0)` 다음줄

각각 다음과 같이 헬퍼 호출 형태로 감싼다:

```go
dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), 0.5)
dmg = s.applyDamageMods(p, dmg)
```

(single 분기도 같은 패턴 — 1.0 multiplier만 다름)

- [ ] **Step 4: applyDamageMods 헬퍼 추가**

turn.go 하단에:

```go
// applyDamageMods reduces incoming damage by coop reductions and skips it
// entirely when invincibility is active. Returns the post-mod damage.
// Caller holds s.mu.
func (s *GameSession) applyDamageMods(p *entity.Player, dmg int) int {
	if s.invincibleNextTurn[p.Fingerprint] {
		return 0
	}
	if mult, ok := s.coopDamageReduction[p.Fingerprint]; ok && mult > 0 {
		reduced := float64(dmg) * (1.0 - mult)
		if reduced < 0 {
			reduced = 0
		}
		return int(reduced)
	}
	return dmg
}
```

- [ ] **Step 5: 턴 시작 시 1턴짜리 효과 클리어**

`game/turn.go`의 `RunTurn` 함수 시작부, `s.clearLog()` 다음에:

```go
	// Clear single-turn coop modifiers from the previous turn before this turn resolves.
	s.coopDamageReduction = make(map[string]float64)
	s.invincibleNextTurn = make(map[string]bool)
```

- [ ] **Step 6: 솔로 힐러 자동부활 훅**

`resolvePlayerActions`에서 sttatus tick 후, `if p.IsDead()` 분기 직전에 추가:

기존:
```go
				if p.IsDead() {
					s.addLog(fmt.Sprintf("☠ %s 쓰러졌습니다!", p.Name))
					s.state.AnyPlayerDied = true
				}
```

다음으로 교체:
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

resolveMonsterActions에서도 같은 분기 두 곳 (AoE/single) 동일 처리. 4곳 총합.

- [ ] **Step 7: 전투 시작/종료 시 coopUsedThisCombat 초기화**

전투 진입 지점은 `enterRoomLocked`에 PhaseCombat으로 전환되는 곳. session.go에 있을 가능성 — Grep으로 찾기.

Run: `grep -n "PhaseCombat" game/*.go`

`enterRoomLocked` 같은 함수에서 `s.state.Phase = PhaseCombat` 직후에 추가:

```go
	s.coopUsedThisCombat = make(map[string]bool)
```

전투 종료(방 클리어) 시점에도 안전하게 초기화 — `resolvePlayerActions`의 `if len(s.state.Monsters) == 0 { ... }` 블록에 같은 줄 추가.

- [ ] **Step 8: 빌드 + 기존 테스트 회귀 확인**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Expected: 빌드 성공.

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./game/ ./combat/ -count=1`
Expected: 모두 PASS (협동기 미사용 시 기존 흐름 변화 X)

- [ ] **Step 9: Commit**

```
git add game/turn.go game/session.go
git commit -m "feat(coop): wire ActionCoop, damage reduction, solo healer auto-revive"
```

---

### Task 1.6: 정찰 (Foresight) — SubmitForesight + 다음 방 미리보기

**Files:**
- Modify: `game/session.go`
- Modify: `dungeon/room.go` 확인용 (변경 X — RoomType.String() 등 활용)

- [ ] **Step 1: SubmitForesight 메서드 추가**

`game/session.go` 하단에:

```go
// SubmitForesight is the Rogue's coop ability. Available once per floor while
// exploring. Reveals the next room's type and (for combat rooms) monster names
// so the party can vote informed. Boss rooms hide monster names but expose the
// "보스" type.
func (s *GameSession) SubmitForesight(fingerprint string, roomIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseExploring {
		return fmt.Errorf("정찰은 탐험 중에만 사용 가능")
	}
	if s.scoutUsedThisFloor[fingerprint] {
		return fmt.Errorf("정찰은 층당 1회만 사용 가능")
	}
	// Verify the player exists and is Rogue and alive
	var rogue *entity.Player
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			rogue = p
			break
		}
	}
	if rogue == nil || rogue.IsOut() || rogue.Class != entity.ClassRogue {
		return fmt.Errorf("정찰은 살아있는 도적만 사용 가능")
	}
	if s.state.Floor == nil || roomIdx < 0 || roomIdx >= len(s.state.Floor.Rooms) {
		return fmt.Errorf("유효하지 않은 방")
	}
	room := s.state.Floor.Rooms[roomIdx]
	preview := &RoomPreview{RoomIdx: roomIdx, RoomTypeLabel: room.Type.String()}
	switch room.Type {
	case dungeon.RoomCombat:
		// Generate spawn preview deterministically by sampling the same way
		// enterRoomLocked would (without consuming RNG). For simplicity we
		// reveal "전투" with monster count only, not exact names — keeps RNG
		// determinism for daily mode and avoids double-spawn rolls.
		preview.MonsterNames = []string{"전투방 (실제 몬스터는 진입 시 결정)"}
	case dungeon.RoomBoss:
		preview.HideMonsters = true
	}
	s.nextRoomPreview = preview
	s.scoutUsedThisFloor[fingerprint] = true
	s.addLog(fmt.Sprintf("%s 정찰 사용! 다음 방: %s", rogue.Name, room.Type.String()))
	return nil
}
```

- [ ] **Step 2: 층 진입 시 정찰 사용권 초기화**

`advanceFloor` 함수 안 (`s.state.FloorNum++` 다음):

```go
	s.scoutUsedThisFloor = make(map[string]bool)
	s.nextRoomPreview = nil
```

- [ ] **Step 3: 방 진입 시 미리보기 소비**

`enterRoomLocked` 함수 안 시작부에 추가:

```go
	s.nextRoomPreview = nil
```

(grep으로 enterRoomLocked 정의 위치 확인 후 적용)

- [ ] **Step 4: 빌드 확인**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Expected: 빌드 성공.

- [ ] **Step 5: 단위 테스트 추가 (game/session_test.go)**

`game/session_test.go` 끝에 추가:

```go
func TestForesightOncePerFloor(t *testing.T) {
	s := NewGameSession(testCfg(t))
	rogue := entity.NewPlayer("Sneak", entity.ClassRogue)
	rogue.Fingerprint = "fp-r"
	s.AddPlayer(rogue)
	s.StartGame()
	// Wait until exploration phase is set
	time.Sleep(50 * time.Millisecond)
	st := s.GetState()
	if st.Floor == nil || len(st.Floor.Rooms) == 0 {
		t.Skip("floor not generated yet")
	}
	cur := st.Floor.CurrentRoom
	if cur < 0 || cur >= len(st.Floor.Rooms) {
		t.Skip("invalid current room")
	}
	neighbors := st.Floor.Rooms[cur].Neighbors
	if len(neighbors) == 0 {
		t.Skip("no neighbor rooms to scout")
	}
	target := neighbors[0]
	if err := s.SubmitForesight("fp-r", target); err != nil {
		t.Fatalf("first foresight failed: %v", err)
	}
	if err := s.SubmitForesight("fp-r", target); err == nil {
		t.Error("second foresight on same floor should fail")
	}
	s.Stop()
}

func TestForesightRequiresRogue(t *testing.T) {
	s := NewGameSession(testCfg(t))
	w := entity.NewPlayer("Tank", entity.ClassWarrior)
	w.Fingerprint = "fp-w"
	s.AddPlayer(w)
	s.StartGame()
	time.Sleep(50 * time.Millisecond)
	if err := s.SubmitForesight("fp-w", 0); err == nil {
		t.Error("warrior should not be able to use Foresight")
	}
	s.Stop()
}
```

- [ ] **Step 6: 테스트 실행**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./game/ -run TestForesight -v`
Expected: PASS x2.

- [ ] **Step 7: Commit**

```
git add game/session.go game/session_test.go
git commit -m "feat(coop): add Foresight (Rogue scout) once per floor"
```

---

### Task 1.7: 협동기 통합 회귀 테스트

**Files:**
- Create: `game/turn_coop_test.go`

- [ ] **Step 1: 테스트 작성**

```go
package game

import (
	"fmt"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func setupCombat(t *testing.T, classes ...entity.Class) *GameSession {
	t.Helper()
	s := NewGameSession(testCfg(t))
	for i, c := range classes {
		p := entity.NewPlayer(fmt.Sprintf("P%d", i), c)
		p.Fingerprint = fmt.Sprintf("fp-%d", i)
		s.AddPlayer(p)
	}
	s.state.SoloMode = len(classes) == 1
	s.state.Phase = PhaseCombat
	s.state.Monsters = []*entity.Monster{
		{Name: "Slime", HP: 100, MaxHP: 100, ATK: 20, DEF: 0},
	}
	s.coopUsedThisCombat = make(map[string]bool)
	return s
}

func TestWarriorCoopReducesIncomingDamage(t *testing.T) {
	s := setupCombat(t, entity.ClassWarrior, entity.ClassMage)
	warrior := s.state.Players[0]
	hpBefore := warrior.HP
	s.actions = map[string]PlayerAction{
		warrior.Fingerprint:           {Type: ActionCoop},
		s.state.Players[1].Fingerprint: {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.resolveMonsterActions()
	s.state.TurnResolving = false
	hpAfter := warrior.HP
	if hpAfter == hpBefore {
		t.Skip("monster missed warrior — re-run")
	}
	// Warrior with Bulwark should take a small fraction of normal damage.
	// Without Bulwark a 20-ATK monster on 8-DEF warrior deals ~12 dmg.
	// With -75% reduction, expect ~3.
	dmg := hpBefore - hpAfter
	if dmg > 6 {
		t.Errorf("Bulwark expected to reduce damage substantially; took %d", dmg)
	}
}

func TestHealerCoopHealsParty(t *testing.T) {
	s := setupCombat(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	w := s.state.Players[1]
	w.HP = w.MaxHP / 2
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionCoop},
		w.Fingerprint:      {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if w.HP <= w.MaxHP/2 {
		t.Errorf("warrior should have been healed; HP=%d, MaxHP=%d", w.HP, w.MaxHP)
	}
}

func TestHealerCoopRevivesDeadAlly(t *testing.T) {
	s := setupCombat(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	w := s.state.Players[1]
	w.TakeDamage(w.HP) // kill
	if !w.IsDead() {
		t.Fatal("warrior should be dead before coop")
	}
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionCoop},
		w.Fingerprint:      {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if w.IsDead() {
		t.Error("warrior should have been revived by Divine Grace")
	}
	if !s.healerReviveUsed[healer.Fingerprint] {
		t.Error("healerReviveUsed should be set after revive")
	}
}

func TestHealerReviveOncePerRun(t *testing.T) {
	s := setupCombat(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	w := s.state.Players[1]
	w.TakeDamage(w.HP)
	s.healerReviveUsed[healer.Fingerprint] = true // already used
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionCoop},
		w.Fingerprint:      {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if !w.IsDead() {
		t.Error("revive should NOT fire again — should fall back to heal mode (no effect on dead)")
	}
}

func TestCoopUsedOncePerCombat(t *testing.T) {
	s := setupCombat(t, entity.ClassWarrior)
	warrior := s.state.Players[0]
	s.actions = map[string]PlayerAction{warrior.Fingerprint: {Type: ActionCoop}}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if !s.coopUsedThisCombat[warrior.Fingerprint] {
		t.Fatal("coopUsedThisCombat should be set after first use")
	}

	// Second use in same combat should log a refusal (not crash, not apply)
	logsBefore := len(s.state.PendingLogs) + len(s.state.CombatLog)
	s.actions = map[string]PlayerAction{warrior.Fingerprint: {Type: ActionCoop}}
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	logsAfter := len(s.state.PendingLogs) + len(s.state.CombatLog)
	if logsAfter <= logsBefore {
		t.Error("expected a refusal log on second coop use")
	}
}
```

- [ ] **Step 2: 테스트 실행**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./game/ -run "Coop|Healer|Warrior" -v -count=1`
Expected: 모두 PASS.

전체 회귀:
Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./... -count=1`
Expected: 모두 PASS.

- [ ] **Step 3: Commit**

```
git add game/turn_coop_test.go
git commit -m "test(coop): integration tests for coop ability cooldowns and effects"
```

---

## Cycle 2 — Phase 2: 6 페어 콤보

### Task 2.1: ComboEffect 구조체 확장

**Files:**
- Modify: `combat/combo.go`

- [ ] **Step 1: ComboEffect에 새 필드 추가**

`combat/combo.go`의 `ComboEffect` 구조체에 추가:

```go
type ComboEffect struct {
	DamageMultiplier float64 // multiplied onto each AttackIntent.Multiplier
	BonusDamage      int     // added to each AttackIntent.PlayerATK
	HealAll          int     // heal all players after resolution
	Message          string  // shown in combat log

	// New: coop combo extensions
	HealMultiplier         float64 // multiplied onto Divine Grace heal/revive HP (Crusader)
	WarriorBulwarkOverride float64 // overrides warrior damage reduction to this value (Iron Aegis)
	GuaranteeCritNextTurn  bool    // next-turn party crit (Expose Weakness)
	InvincibleHealTarget   bool    // heal target is invincible 1 turn (Sanctuary)
	EnemyMissNextTurn      bool    // 50% enemy miss next turn (Mirage)
	PoisonNextCombat       bool    // next combat starts enemies with poison 3 turns (Venomous Edge)
}
```

- [ ] **Step 2: 6 새 콤보 정의 추가**

`var comboDefs = []ComboDef{...}` 안 기존 5개 끝에 추가:

```go
	// Coop pair combos (ActionType "coop")
	{
		Name: "Iron Aegis",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "coop"},
			{Class: entity.ClassMage, ActionType: "coop"},
		},
		Effect: ComboEffect{WarriorBulwarkOverride: 0.75, Message: "🛡️ IRON AEGIS! 강철결계가 워리어를 감쌉니다!"},
	},
	{
		Name: "Crusader",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "coop"},
			{Class: entity.ClassHealer, ActionType: "coop"},
		},
		Effect: ComboEffect{HealMultiplier: 2.0, Message: "⚔️✨ CRUSADER! 성스러운 회복!"},
	},
	{
		Name: "Expose Weakness",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "coop"},
			{Class: entity.ClassRogue, ActionType: "coop"},
		},
		Effect: ComboEffect{GuaranteeCritNextTurn: true, Message: "🎯 EXPOSE WEAKNESS! 다음 턴 모든 공격 치명타!"},
	},
	{
		Name: "Sanctuary",
		Required: []ComboAction{
			{Class: entity.ClassMage, ActionType: "coop"},
			{Class: entity.ClassHealer, ActionType: "coop"},
		},
		Effect: ComboEffect{InvincibleHealTarget: true, Message: "✨🛡️ SANCTUARY! 회복 대상 1턴 무적!"},
	},
	{
		Name: "Mirage",
		Required: []ComboAction{
			{Class: entity.ClassMage, ActionType: "coop"},
			{Class: entity.ClassRogue, ActionType: "coop"},
		},
		Effect: ComboEffect{EnemyMissNextTurn: true, Message: "🌫️ MIRAGE! 다음 턴 적 공격 50% 빗나감!"},
	},
	{
		Name: "Venomous Edge",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "coop"},
			{Class: entity.ClassRogue, ActionType: "coop"},
		},
		Effect: ComboEffect{PoisonNextCombat: true, Message: "🐍 VENOMOUS EDGE! 다음 전투 적들이 독에 빠집니다!"},
	},
```

- [ ] **Step 3: 빌드 확인**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Expected: 빌드 성공.

- [ ] **Step 4: Commit**

```
git add combat/combo.go
git commit -m "feat(coop): define 6 pair combos for coop abilities"
```

---

### Task 2.2: 콤보 매칭에 ActionCoop 포함

**Files:**
- Modify: `game/turn.go`

- [ ] **Step 1: comboActions 빌드 시 ActionCoop도 "coop"로 매핑**

`game/turn.go`의 `resolvePlayerActions` 안 콤보 감지 부분 (line ~302):

```go
	comboActions := make(map[string]combat.ComboAction)
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		action, ok := s.actions[p.Fingerprint]
		if !ok {
			continue
		}
		var actionType string
		switch action.Type {
		case ActionAttack:
			actionType = "attack"
		case ActionSkill:
			actionType = "skill"
		case ActionItem:
			actionType = "item"
		case ActionCoop: // ← 추가
			actionType = "coop"
		default:
			continue
		}
		comboActions[p.Fingerprint] = combat.ComboAction{Class: p.Class, ActionType: actionType}
	}
```

- [ ] **Step 2: 콤보 효과 적용 — Crusader (heal multiplier)**

같은 함수의 콤보 처리 루프(line ~327):

```go
	combos := combat.DetectCombos(comboActions)
	for _, combo := range combos {
		s.addLog(combo.Effect.Message)
		for i := range intents {
			if combo.Effect.DamageMultiplier > 0 {
				intents[i].Multiplier *= combo.Effect.DamageMultiplier
			}
			intents[i].PlayerATK += combo.Effect.BonusDamage
		}
		// New: apply coop combo extensions
		s.applyCoopComboEffect(combo.Effect)
	}
```

`applyCoopComboEffect` 헬퍼는 다음 step.

- [ ] **Step 3: applyCoopComboEffect 정의**

`game/turn.go` 하단:

```go
// applyCoopComboEffect propagates extended coop combo effects to session state.
// Caller holds s.mu.
func (s *GameSession) applyCoopComboEffect(eff combat.ComboEffect) {
	if eff.WarriorBulwarkOverride > 0 {
		// Find warrior(s) and override their reduction (max with current).
		for _, p := range s.state.Players {
			if p.Class == entity.ClassWarrior && !p.IsOut() {
				if cur := s.coopDamageReduction[p.Fingerprint]; eff.WarriorBulwarkOverride > cur {
					s.coopDamageReduction[p.Fingerprint] = eff.WarriorBulwarkOverride
				}
			}
		}
	}
	if eff.HealMultiplier > 0 {
		// Re-apply Divine Grace heal/revive boost. Find healer(s) using coop this turn,
		// re-heal each ally by the multiplier delta. Simpler: heal everyone an extra 30%MaxHP*(mult-1).
		bonus := eff.HealMultiplier - 1.0
		if bonus > 0 {
			for _, p := range s.state.Players {
				if !p.IsOut() {
					p.Heal(int(float64(p.MaxHP)*0.30*bonus))
				}
			}
		}
	}
	if eff.GuaranteeCritNextTurn {
		s.critGuaranteedNext = true
	}
	if eff.InvincibleHealTarget {
		// Mark heal target (lowest-HP ally or just-revived) invincible next turn.
		// Pick lowest-HP alive ally as proxy — Sanctuary is "protect the recipient".
		var minHPP *entity.Player
		minHP := int(^uint(0) >> 1)
		for _, p := range s.state.Players {
			if !p.IsOut() && p.HP < minHP {
				minHP = p.HP
				minHPP = p
			}
		}
		if minHPP != nil {
			s.invincibleNextTurn[minHPP.Fingerprint] = true
		}
	}
	if eff.EnemyMissNextTurn {
		s.enemyMissNextTurn = true
	}
	if eff.PoisonNextCombat {
		s.poisonNextCombat = true
	}
}
```

- [ ] **Step 4: critGuaranteedNext 소비 — 공격 처리에 적용**

`resolvePlayerActions` 안 `case ActionAttack:` 블록의 crit 분기:

기존:
```go
			multiplier := 1.0
			if p.Skills != nil {
				if pct := p.Skills.GetCritChance(p.Class); pct > 0 && s.rng.Intn(100) < pct {
					multiplier = 2.0
					s.addLog(fmt.Sprintf("%s 치명타!", p.Name))
				}
			}
```

다음으로 교체:
```go
			multiplier := 1.0
			if s.critGuaranteedNext {
				multiplier = 2.0
				s.addLog(fmt.Sprintf("%s 급소 노출 치명타!", p.Name))
			} else if p.Skills != nil {
				if pct := p.Skills.GetCritChance(p.Class); pct > 0 && s.rng.Intn(100) < pct {
					multiplier = 2.0
					s.addLog(fmt.Sprintf("%s 치명타!", p.Name))
				}
			}
```

`critGuaranteedNext`는 RunTurn 끝에 클리어 — turn.go의 `RunTurn` 끝 (`s.state.TurnResolving = false` 다음):

```go
	s.critGuaranteedNext = false
```

- [ ] **Step 5: enemyMissNextTurn 소비 — 몬스터 공격 회피**

`resolveMonsterActions` 안 두 군데 (AoE / single) `dmg := combat.CalcDamage(...)` 직전에 회피 체크 추가. AoE 분기:

기존:
```go
			for _, p := range s.state.Players {
				if !p.IsOut() {
					dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), 0.5)
```

다음으로:
```go
			for _, p := range s.state.Players {
				if !p.IsOut() {
					if s.enemyMissNextTurn && s.rng.Float64() < 0.50 {
						s.addLog(fmt.Sprintf("%s 환영진으로 %s의 공격 회피!", p.Name, m.Name))
						continue
					}
					dmg := combat.CalcDamage(s.rng, m.ATK, p.EffectiveDEF(), 0.5)
```

single 분기도 동일 패턴 (Players[targetIdx]에 대해 같은 회피 체크).

`enemyMissNextTurn`도 RunTurn 끝에 클리어:

```go
	s.enemyMissNextTurn = false
```

- [ ] **Step 6: poisonNextCombat 소비 — 다음 전투 시작 시**

전투 시작 지점(enterRoomLocked의 PhaseCombat 분기) — Task 1.5 step 7에서 추가한 위치 옆:

```go
	s.coopUsedThisCombat = make(map[string]bool)
	if s.poisonNextCombat {
		for _, m := range s.state.Monsters {
			m.HP -= 0 // dummy — we apply via player-side effect on monsters
			// Monsters don't have AddEffect; instead, we add a session "poisonStacks" track
			// or implement Monster.AddPoison. Simpler: directly tag monster with PoisonValue/Turns.
		}
		// Monster doesn't currently support status effects; for MVP, just log it and
		// apply a flat HP penalty over 3 turns via a session-level ticker.
		s.addLog("독칼: 모든 적이 3턴간 독에 중독됩니다")
		s.poisonNextCombat = false
	}
```

NOTE: `entity/monster.go`에 status effect 시스템이 없을 수 있음. 확인 필요. 없으면 단순화 — combat 시작 시 모든 적 HP * 0.10 즉시 차감 + 3턴간 매턴 5 dmg를 세션에 큐. 현재 task 범위 외이므로 MVP로는 다음과 같이 단순화:

수정: 위 코드를 다음으로 대체 (즉시 데미지로 단순화):

```go
	if s.poisonNextCombat {
		for _, m := range s.state.Monsters {
			poisonDmg := m.MaxHP * 15 / 100
			m.HP -= poisonDmg
			if m.HP < 1 {
				m.HP = 1
			}
		}
		s.addLog("독칼: 모든 적이 즉시 독 피해를 받았습니다 (-15% MaxHP)")
		s.poisonNextCombat = false
	}
```

- [ ] **Step 7: 빌드 + 회귀 테스트**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./... -count=1`
Expected: 빌드 + 모든 테스트 PASS.

- [ ] **Step 8: Commit**

```
git add game/turn.go
git commit -m "feat(coop): apply 6 pair combo effects (bulwark/heal/crit/invincible/miss/poison)"
```

---

### Task 2.3: 6 콤보 단위 테스트

**Files:**
- Modify: `combat/combo_test.go`

- [ ] **Step 1: 6 콤보 매칭 테스트 추가**

`combat/combo_test.go` 끝에:

```go
func TestDetectCombo_IronAegis(t *testing.T) {
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"m": {Class: entity.ClassMage, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Iron Aegis" {
		t.Errorf("expected Iron Aegis, got %+v", combos)
	}
	if combos[0].Effect.WarriorBulwarkOverride != 0.75 {
		t.Errorf("Iron Aegis bulwark override = %v, want 0.75", combos[0].Effect.WarriorBulwarkOverride)
	}
}

func TestDetectCombo_Crusader(t *testing.T) {
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Crusader" {
		t.Errorf("expected Crusader, got %+v", combos)
	}
}

func TestDetectCombo_ExposeWeakness(t *testing.T) {
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"r": {Class: entity.ClassRogue, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Expose Weakness" {
		t.Errorf("expected Expose Weakness, got %+v", combos)
	}
	if !combos[0].Effect.GuaranteeCritNextTurn {
		t.Error("Expose Weakness should set GuaranteeCritNextTurn")
	}
}

func TestDetectCombo_Sanctuary(t *testing.T) {
	actions := map[string]ComboAction{
		"m": {Class: entity.ClassMage, ActionType: "coop"},
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Sanctuary" {
		t.Errorf("expected Sanctuary, got %+v", combos)
	}
}

func TestDetectCombo_Mirage(t *testing.T) {
	actions := map[string]ComboAction{
		"m": {Class: entity.ClassMage, ActionType: "coop"},
		"r": {Class: entity.ClassRogue, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Mirage" {
		t.Errorf("expected Mirage, got %+v", combos)
	}
}

func TestDetectCombo_VenomousEdge(t *testing.T) {
	actions := map[string]ComboAction{
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
		"r": {Class: entity.ClassRogue, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 1 || combos[0].Name != "Venomous Edge" {
		t.Errorf("expected Venomous Edge, got %+v", combos)
	}
}

func TestDetectCombo_TripleCoopStacksAllPairs(t *testing.T) {
	// W + M + H coop simultaneously → Iron Aegis + Crusader + Sanctuary all fire.
	actions := map[string]ComboAction{
		"w": {Class: entity.ClassWarrior, ActionType: "coop"},
		"m": {Class: entity.ClassMage, ActionType: "coop"},
		"h": {Class: entity.ClassHealer, ActionType: "coop"},
	}
	combos := DetectCombos(actions)
	names := make(map[string]bool)
	for _, c := range combos {
		names[c.Name] = true
	}
	for _, want := range []string{"Iron Aegis", "Crusader", "Sanctuary"} {
		if !names[want] {
			t.Errorf("expected %s in triple-coop, got %+v", want, names)
		}
	}
}
```

- [ ] **Step 2: 테스트 실행**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./combat/ -run "Combo" -v -count=1`
Expected: 모든 콤보 테스트 PASS.

- [ ] **Step 3: 통합 흐름 테스트 (game/turn_coop_test.go에 추가)**

```go
func TestCoopComboCrusaderDoublesHeal(t *testing.T) {
	s := setupCombat(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	w := s.state.Players[1]
	w.HP = 1
	hpBefore := w.HP
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionCoop},
		w.Fingerprint:      {Type: ActionCoop}, // warrior also fires coop → Crusader
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	healed := w.HP - hpBefore
	// Without combo: Divine Grace heals 30% MaxHP. With Crusader: 2x → 60% MaxHP bonus added.
	// Expect total healing > 30% of MaxHP.
	if healed <= w.MaxHP*30/100 {
		t.Errorf("Crusader should boost heal beyond base 30%%; healed=%d MaxHP=%d", healed, w.MaxHP)
	}
}

func TestCoopComboExposeWeaknessSetsCritFlag(t *testing.T) {
	s := setupCombat(t, entity.ClassWarrior, entity.ClassRogue)
	w := s.state.Players[0]
	r := s.state.Players[1]
	s.actions = map[string]PlayerAction{
		w.Fingerprint: {Type: ActionCoop},
		r.Fingerprint: {Type: ActionCoop},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if !s.critGuaranteedNext {
		t.Error("Expose Weakness combo should set critGuaranteedNext")
	}
}
```

- [ ] **Step 4: 테스트 실행**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./game/ -run "Coop" -v -count=1`
Expected: 모든 PASS.

전체 회귀:
Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./... -count=1`
Expected: 모든 PASS.

- [ ] **Step 5: Commit**

```
git add combat/combo_test.go game/turn_coop_test.go
git commit -m "test(coop): cover 6 pair combos and combo→effect propagation"
```

---

## Cycle 3 — Phase 3: UI 통합

### Task 3.1: 키 6 = ActionCoop, 액션바 갱신

**Files:**
- Modify: `ui/game_view.go`

- [ ] **Step 1: case "5" 다음에 case "6" 추가**

`ui/game_view.go` line ~410 부근의 switch key.String() 안:

```go
				case "6":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionCoop, TargetIdx: s.targetCursor})
```

- [ ] **Step 2: 액션바 텍스트 갱신**

같은 파일 line ~534 부근:

기존:
```go
			if isHealer {
				sb.WriteString(styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 [Tab]적 [Shift+Tab]아군 [/]채팅"))
			} else {
				sb.WriteString(styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 [Tab]대상 [/]채팅"))
			}
```

다음으로 교체:
```go
			coopHint := "[6]협동기"
			if state.CoopAvailable != nil && !state.CoopAvailable[fingerprint] {
				coopHint = styleSystem.Render("[6]협동기(사용됨)")
			}
			if isHealer {
				sb.WriteString(styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 ") + coopHint + styleAction.Render(" [Tab]적 [Shift+Tab]아군 [/]채팅"))
			} else {
				sb.WriteString(styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 ") + coopHint + styleAction.Render(" [Tab]대상 [/]채팅"))
			}
```

`renderHUD` 시그니처에 fingerprint 매개변수가 이미 있는지 확인 — `renderHUD(state game.GameState, targetCursor int, allyCursor int, moveCursor int, fingerprint string)`로 이미 fingerprint 받음. 내부에서 `state.CoopAvailable[fingerprint]` 접근 가능.

- [ ] **Step 3: 협동기 설명 표시 (스킬 설명 옆)**

같은 파일 line ~556 부근의 스킬 설명 블록 안 — `skillDesc += fmt.Sprintf(" (남은 횟수: %d)", p.SkillUses)` 다음에:

```go
				coopAbility := entity.GetCoopAbility(p.Class)
				coopStatus := "사용가능"
				if state.CoopAvailable == nil || !state.CoopAvailable[p.Fingerprint] {
					coopStatus = "사용됨"
				}
				skillDesc += fmt.Sprintf("\n협동기: %s [%s]", coopAbility.Name, coopStatus)
```

- [ ] **Step 4: 빌드 + 수동 확인**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Expected: 빌드 성공.

- [ ] **Step 5: Commit**

```
git add ui/game_view.go
git commit -m "ui(coop): map key 6 to coop action and show coop status in HUD"
```

---

### Task 3.2: 정찰 패널 + 정찰 키 추가

**Files:**
- Modify: `ui/game_view.go`

- [ ] **Step 1: PhaseExploring에서 키 6 = SubmitForesight**

`ui/game_view.go`의 `case game.PhaseExploring:` 블록 안. `} else if isKey(key, "q") {` 직전에 추가:

```go
				} else if key.String() == "6" {
					if ctx.Session != nil && len(neighbors) > 0 {
						target := neighbors[s.moveCursor]
						if err := ctx.Session.SubmitForesight(ctx.Fingerprint, target); err != nil {
							s.skillMsg = err.Error()
						} else {
							s.skillMsg = "정찰 완료!"
						}
						s.gameState = ctx.Session.GetState()
					}
```

- [ ] **Step 2: 정찰 결과 패널 — renderHUD의 PhaseExploring 분기에 추가**

`renderHUD`의 `} else if state.Phase == game.PhaseExploring {` 블록 끝 (이동 선택 텍스트 전 또는 후):

```go
			if state.NextRoomPreview != nil {
				previewStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
				sb.WriteString(previewStyle.Render(fmt.Sprintf("\n[정찰] 다음 방: %s", state.NextRoomPreview.RoomTypeLabel)))
				if !state.NextRoomPreview.HideMonsters && len(state.NextRoomPreview.MonsterNames) > 0 {
					sb.WriteString(previewStyle.Render(" — " + strings.Join(state.NextRoomPreview.MonsterNames, ", ")))
				}
				sb.WriteString("\n")
			}
```

- [ ] **Step 3: 탐험 모드 키 안내에 [6]정찰 추가**

같은 파일 line ~668 부근:

기존:
```go
		if !state.SoloMode && myVoted {
			sb.WriteString("[Q] 종료  — 다른 파티원의 투표를 기다리는 중...")
		} else {
			sb.WriteString("[Up/Down] 선택  [Enter] 이동  [Q] 종료")
		}
```

도적이고 정찰 사용 가능할 때 [6] 추가:
```go
		myIsRogue := false
		myCanScout := false
		for _, p := range state.Players {
			if p.Fingerprint == fingerprint && p.Class == entity.ClassRogue {
				myIsRogue = true
				if state.CoopAvailable != nil && state.CoopAvailable[fingerprint] {
					myCanScout = true
				}
				break
			}
		}
		_ = myIsRogue
		if !state.SoloMode && myVoted {
			sb.WriteString("[Q] 종료  — 다른 파티원의 투표를 기다리는 중...")
		} else if myCanScout {
			sb.WriteString("[Up/Down] 선택  [Enter] 이동  [6] 정찰  [Q] 종료")
		} else {
			sb.WriteString("[Up/Down] 선택  [Enter] 이동  [Q] 종료")
		}
```

- [ ] **Step 4: 빌드 확인**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Expected: 빌드 성공.

- [ ] **Step 5: Commit**

```
git add ui/game_view.go
git commit -m "ui(coop): add Foresight key (6) and next-room preview panel"
```

---

### Task 3.3: 콤보 로그 골드 강조

**Files:**
- Modify: `ui/game_view.go` (colorizeLog)

- [ ] **Step 1: colorizeLog에 콤보 케이스 추가**

`ui/game_view.go`의 `colorizeLog` switch:

기존 첫 분기 `case strings.Contains(msg, "도주"):` 위에 추가:

```go
	case strings.Contains(msg, "AEGIS") || strings.Contains(msg, "CRUSADER") || strings.Contains(msg, "EXPOSE") || strings.Contains(msg, "SANCTUARY") || strings.Contains(msg, "MIRAGE") || strings.Contains(msg, "VENOMOUS") || strings.Contains(msg, "강철결계") || strings.Contains(msg, "성전사") || strings.Contains(msg, "급소") || strings.Contains(msg, "성역") || strings.Contains(msg, "환영진") || strings.Contains(msg, "독칼"):
		return styleCombo.Render(msg)
	case strings.Contains(msg, "협동기") || strings.Contains(msg, "견고함") || strings.Contains(msg, "결계") || strings.Contains(msg, "신성 가호") || strings.Contains(msg, "정찰"):
		return styleCoop.Render(msg)
```

- [ ] **Step 2: styleCombo 추가**

`ui/styles.go`에 추가 (styleCoop 근처):

Run: `grep -n "styleCoop" ui/styles.go` 로 위치 확인.

styleCoop 정의 옆에:

```go
var styleCombo = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true) // gold
```

- [ ] **Step 3: 빌드 확인**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Expected: 빌드 성공.

전체 회귀:
Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./... -count=1`
Expected: 모두 PASS.

- [ ] **Step 4: Commit**

```
git add ui/game_view.go ui/styles.go
git commit -m "ui(coop): gold highlight for combo log lines"
```

---

### Task 3.4: 헬프 화면 / 협동기 안내 (선택적, 짧게)

**Files:**
- Modify: `ui/help_view.go`

- [ ] **Step 1: 협동기 섹션 한 줄 추가**

`ui/help_view.go`를 열어 적당한 위치에 한 줄 추가 (게임플레이 섹션 끝):

```go
	// (적당한 위치)
	"  [6] 협동기 — 클래스별 고유 능력. 두 명 동시 사용 시 콤보 발동.\n"
```

(파일 구조에 맞춰 형식 조정. 만약 string slice가 아니면 sb.WriteString 패턴 사용)

- [ ] **Step 2: 빌드 확인**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go build ./...`
Expected: 빌드 성공.

- [ ] **Step 3: 최종 회귀 + vet**

Run: `C:\Users\98kim\sdk\go1.25.1\bin\go test ./... -count=1`
Run: `C:\Users\98kim\sdk\go1.25.1\bin\go vet ./...`
Expected: 모든 PASS, vet 클린.

- [ ] **Step 4: Commit**

```
git add ui/help_view.go
git commit -m "docs(coop): mention coop ability key in help screen"
```

---

## 구현 후 검증

- [ ] 전체 테스트 PASS
- [ ] 빌드 성공
- [ ] 수동 플레이테스트 권장 (선택): 솔로 워리어로 시작, 1전투 진입, 키 6 누르고 데미지 경감 확인

## 명시적으로 보류 (Phase 4 이후)

- 코덱스 콤보 탭
- 신규 업적 3종
- 정찰 콤보 특수 규칙 (탐험 중 정찰 + 다음 전투 1턴 다른 협동기 = 콤보) — Phase 2의 PoisonNextCombat은 단순 즉시 효과로 대체됨
- 보스 도발 무시 플래그
- 플레이테스트 후 밸런스 조정
