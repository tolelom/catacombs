# General Improvements Plan (Cycles 4–8)

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans for inline batched execution.

**Goal:** 협동 시스템 구현 후, 게임 전반의 재미·다양성·완성도를 5사이클로 끌어올린다.

**Architecture:** 각 사이클은 독립적이고 한 영역만 다룬다. 모든 변경은 기존 시스템과 호환, 솔로/멀티 양쪽 작동, DailyMode 결정성 유지.

**Spec/Design:** 각 사이클은 자체 완결된 기능 — 별도 spec 없이 본 문서가 spec 역할.

---

## Cycle 4 — Boss Depth (보스 페이즈·고유 기믹)

**문제:** 4 보스(B5/B10/B15/B20)가 패턴이 같음 — Poison/Burn/Freeze/Heal 중 하나, AoE 3턴마다. 페이즈 전환 X.

**목표:** 각 보스에 50%HP 이하에서 발동되는 phase 2 + 고유 시그니처 기믹.

### Tasks

- [ ] **4.1** `entity/monster.go`에 `Phase int` 필드 + `PhaseTransitioned bool` 추가. `Monster.CheckPhaseTransition()` 메서드 — HP < 50% MaxHP 시 Phase=2로 전환, true 반환.
- [ ] **4.2** `combat/combat.go` MonsterAI 보스 분기 강화 — Phase 2면 패턴 빈도 증가(매 2턴마다 AoE), 시그니처 기믹 발동.
- [ ] **4.3** B5 보스(첫 보스): Phase 2에서 "광폭화" — ATK +50%, HP 회복 0.
- [ ] **4.4** B10 보스: Phase 2에서 "분열" — 슬라임 잡몹 2마리 소환 (s.state.Monsters에 append, 세션 RNG 사용).
- [ ] **4.5** B15 보스: Phase 2에서 "취약점 공개" — 1턴마다 1턴 동안 받는 피해 1.5배 (PendingVulnerable 토글, MonsterAI 안에서 처리).
- [ ] **4.6** B20 최종 보스: Phase 2 진입시 화면 가득 로그 + 모든 패턴(Poison+Burn+Freeze) 동시 발동. ATK +30%, DEF +20%.
- [ ] **4.7** `combat/combat_test.go`에 phase 전환 테스트 4종.
- [ ] **4.8** Commit per task.

---

## Cycle 5 — Run Variety (새 룸 타입 4종)

**문제:** 룸 종류가 전투/상점/이벤트/보물/보스 5종뿐. 매 런이 비슷하게 흘러감.

**목표:** 4종 새 룸 추가 — 미믹/도박장/저주제단/방랑상인.

### Tasks

- [ ] **5.1** `dungeon/room.go`에 `RoomType` enum 확장: `RoomMimic`, `RoomGamble`, `RoomCursedAltar`, `RoomWanderingMerchant`. String() 갱신.
- [ ] **5.2** `dungeon/generator.go`에서 4~15층에 새 룸 타입 등장 확률 추가 (각 5%). 보스/상점층은 제외.
- [ ] **5.3** `game/room_handlers.go` (신규) — 4 룸 타입 진입 핸들러:
  - **Mimic**: 보물처럼 보이지만 50% 확률로 강한 몬스터 1마리(현 층 기준 1.5배 스탯). 나머지 50%는 일반 보물.
  - **Gamble**: 50G 베팅 → 50% 100G, 30% 아이템, 20% 손실. 1회만.
  - **CursedAltar**: 자기 HP 30% 희생 → ATK+5 영구 OR DEF+3 영구 OR 골드 100. 한 번 선택.
  - **WanderingMerchant**: 희귀 아이템 3종 (가격 2배, 1회 한정).
- [ ] **5.4** `game/session.go`에서 enterRoomLocked가 새 타입 처리 — 핸들러 dispatch.
- [ ] **5.5** UI `dungeon/render.go` 룸 표시 문자 — M(Mimic 시각상 보물 같음), $(Gamble), †(Altar), W(Merchant).
- [ ] **5.6** Tests: 각 룸 핸들러 단위 테스트.
- [ ] **5.7** Commit per task.

---

## Cycle 6 — Skill Tree Tier 4 (스킬트리 확장)

**문제:** 스킬트리 2 branch × 3 node로 4층까지면 끝. 8~20층은 스킬 진행 X.

**목표:** Tier 4 capstone 노드 추가 — 각 branch 끝에 강력한 1노드, 11층부터 획득.

### Tasks

- [ ] **6.1** `entity/skill_tree.go`에 각 클래스 branch당 Tier 4 노드 정의 (총 8 노드). 강한 효과 — 예: Warrior "불굴" branch capstone = MaxHP +50, "분노" branch capstone = ATK +10.
- [ ] **6.2** `Allocate` / `GetXBonus` 메서드들이 Tier 4 노드 인식하도록 수정.
- [ ] **6.3** advanceFloor에서 11층 이후 추가 스킬 포인트 1점 더 (총 2점/층) — 또는 Tier 4 자동 잠금해제.
- [ ] **6.4** UI 스킬 트리 표시 — Tier 4 노드 추가 (다른 색상/별 마크).
- [ ] **6.5** Tests: Tier 4 효과, allocate 흐름.
- [ ] **6.6** Commit per task.

---

## Cycle 7 — Meta Progression (영구 통화)

**문제:** 죽으면 모든 게 리셋. "한 판 더" 후크 약함.

**목표:** 런 종료 시 모은 골드의 10%를 "영혼"으로 영구 보관 → 새 런 시작 전 소비해 영구 보너스.

### Tasks

- [ ] **7.1** `store/db.go`에 souls 버킷 (fingerprint → int64). `GetSouls`, `AddSouls`, `SpendSouls` 메서드.
- [ ] **7.2** Run 종료 시 (`ui/game_view.go`의 GameOver 분기) — `AddSouls(fingerprint, totalGold/10)`.
- [ ] **7.3** 새 영구 unlock 시스템: `store/perma_upgrades.go` — 5종 업그레이드:
  - 시작 골드 +50 (50 영혼)
  - 시작 HP +20 (100 영혼)
  - 시작 ATK +3 (150 영혼)
  - 시작 인벤토리 슬롯 +2 (75 영혼)
  - 시작 스킬 포인트 +1 (200 영혼)
- [ ] **7.4** UI: lobby_view 또는 nickname_view 다음에 "영구 강화" 화면 추가. 영혼 잔액 + 5 업그레이드 매수 가능.
- [ ] **7.5** `game/session.AddPlayer`에서 영구 강화 적용 — fingerprint로 store 조회, 시작 스탯에 가산.
- [ ] **7.6** Tests: 영혼 적립/소비, 업그레이드 적용 검증.
- [ ] **7.7** Commit per task.

---

## Cycle 8 — Polish (코덱스/업적/UI 마감)

**문제:** Cycle 1-7로 추가된 콘텐츠가 코덱스/업적에 반영 X.

**목표:** 코덱스 콤보 탭 + 신규 업적 + UI 마감.

### Tasks

- [ ] **8.1** 코덱스 신규 탭: 콤보(11종 = 기존 5 + 새 6), 룸 타입(새 4종), 보스 페이즈(4종 발견 여부). `store/codex.go`에 새 카테고리.
- [ ] **8.2** 신규 업적 8종:
  - `coop_first` 콤보 1회
  - `coop_master` 한 런에 6 콤보
  - `revive_5` 부활 5회 누적
  - `mimic_bait` 미믹 5회 처치
  - `cursed_lover` 저주 제단 3회 사용
  - `tier4_unlocked` Tier 4 노드 1개 획득
  - `soul_collector` 영혼 1000 누적
  - `boss_phase2` 보스 phase 2 도달 첫 1회
- [ ] **8.3** `ui/codex_view.go`에 콤보 탭 + 룸 탭 추가. 미발견은 ???.
- [ ] **8.4** `ui/achievements_view.go`에 신규 8종 표시.
- [ ] **8.5** HUD 정리 — 협동기 상태 위치 조정, 콤보 발동 시 강조 깜빡임.
- [ ] **8.6** Tests: 코덱스 마이그레이션, 업적 트리거.
- [ ] **8.7** 전체 회귀 + vet + 빌드.
- [ ] **8.8** Commit per task.

---

## 실행 전략

- **순서**: 1→8 순차. 각 사이클은 이전 사이클 위에 빌드.
- **테스트**: 매 task 후 `go test ./... -count=1` 실행, 매 commit 전 PASS 확인.
- **Commit 단위**: task 1개 = commit 1개 (TDD 사이클 분리).
- **롤백**: 사이클 단위로 `git revert` 가능하도록 사이클 간 커밋 명확히 분리.

## 명시적 비목표

- 새 클래스 추가
- 그래픽 변경
- 멀티 서버 / 매치메이킹
- 사운드
