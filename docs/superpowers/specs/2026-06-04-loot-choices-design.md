# Loot Choices — 보물방 선택 기반 루팅 설계

날짜: 2026-06-04
상태: 자율 진행 (사용자 위임)

## 문제

보물방(`RoomTreasure`)은 플레이어 입력 없이 `grantTreasure`가 무작위 아이템을 자동
지급한다. 아이템은 사실상 ATK+N / DEF+N 두 종류뿐이고, `bestBonus`(최고 1개만 적용)
정책과 결합되면 현재 장비보다 낮은 드랍은 줍는 순간 쓰레기가 된다. 루팅에 "결정"이
없어서 진행의 절반이 무의미하다 — 이벤트 투표(완료), 스킬트리 분기(완료)에 이은
재미 진단 #3 해소 대상.

## 목표

보물방 진입 시 **수치가 공개된 3개의 루팅 선택지**를 제시하고, 각 플레이어가
**개인별로** 하나를 골라 자기 인벤토리/스탯에 적용받는다.

다수결이 아닌 개인 선택인 이유: 루팅은 개인 인벤토리 문제다. 이벤트(파티 전체에
효과 적용)와 달리 워리어는 무기, 힐러는 방어구를 원하는 게 자연스럽고, 다수결은
소수의 선택권을 빼앗는다.

## 설계

### 1. EventOutcome 확장 (`game/random_event.go`)

```go
type EventOutcome struct {
    HPChange    int
    GoldChange  int
    ItemDrop    bool          // 기존: 무작위 장비 1개 롤
    Item        *entity.Item  // 신규: 이 구체 아이템을 지급
    Description string
}
```

### 2. 루팅 이벤트 생성 (`game/loot.go` 신규)

`BuildLootEvent(floor int, rng *rand.Rand) RandomEvent` — 방 진입 시점에 아이템을
**미리 굴려서** 라벨에 수치를 노출한다 (정보 공개가 결정을 만든다):

- **슬롯 1 — 무기**: bonus = `3 + rng.Intn(6) + floor/3` (기존 공식).
  라벨 예: `"Steel Sword (ATK+7)"`. 태그: `TagItem`
- **슬롯 2 — 방어구**: bonus = `2 + rng.Intn(4) + floor/4`.
  라벨 예: `"Steel Shield (DEF+5)"`. 태그: `TagItem`
- **슬롯 3 — 회전 슬롯** (rng로 4종 중 1개):
  - **골드 주머니**: 확정 `25 + floor*3` G. 태그: `TagGoldGain`
  - **치유의 성수**: 즉시 HP `20 + floor*2` 회복. 태그: `TagHeal`
  - **저주받은 무기**: bonus = 무기 공식 + 5, 획득 시 HP `-(10 + floor*2)`.
    라벨에 두 수치 모두 표시. 태그: `TagItem`, `TagHarm`
  - **미확인 상자**: 35% 세트 아이템 / 40% 일반 장비 / 25% 함정(`12+floor` 데미지).
    수치 비공개. 태그: `TagItem`, `TagGamble`

기존 `grantTreasure`의 10% 세트템 확률은 미확인 상자의 35%로 흡수된다 (도박을
선택해야 세트템 기회가 생김 — 이것도 결정이다).

### 3. 세션 플럼빙 (`game/session.go`, `game/event.go`)

- `GameSession.eventPerPlayer bool` 필드 + `PendingEventView.PerPlayer bool` 추가.
- `triggerEvent`의 페이즈 진입 코드를 `startEventPhaseLocked(event RandomEvent,
  perPlayer bool)`로 추출. `triggerEvent`(파티 투표)와 신규
  `triggerLoot`(개인 선택)가 공유.
- `enterRoomLocked`의 `RoomTreasure` 케이스: `grantTreasure()` + 즉시 Cleared →
  `triggerLoot()` 호출, Cleared는 resolution 시점에 (이벤트 방과 동일 패턴).
- `resolveEventVoteLocked` 분기:
  - 파티 모드(기존): 다수결 → 단일 outcome → `applyEventOutcome` (변경 없음)
  - **개인 모드(신규)**: `s.state.Players` 순서(결정론)로 순회, 각 플레이어의
    선택을 `Resolve(floor, s.rng)`하고 `applyOutcomeToPlayer(p, outcome)`로
    본인에게만 적용. 투표 기록이 없는 alive 플레이어(연결 끊김 스윕 경로)는 스킵.
- `applyOutcomeToPlayer(p *entity.Player, o EventOutcome)` 신규 헬퍼:
  - HPChange: `Heal`/`TakeDamage`, 사망 시 `AnyPlayerDied = true`
  - GoldChange: 본인 골드에 가감 (음수 바닥 0)
  - Item: 인벤토리 여유 시 지급. **인벤 풀이면 `bonus*5` 골드로 전환**
    (`handleGamble`의 잭팟-인벤풀 보상 패턴, 상점 가격 공식 `40+bonus*5`와 일관)
  - ItemDrop: 기존 무작위 롤 1개 (미확인 상자의 일반 장비 경로에서 사용)
- 기존 inactivity sweep의 PhaseEvent quorum 재평가는 개인 모드에도 그대로 적용됨
  (같은 `eventVotes` / `resolveEventVoteLocked` 경로).

### 4. 비변경 사항 (스코프 제외)

- `grantTreasure`는 미믹 보물 경로(`handleMimic`)와 비밀방(`grantSecretTreasure`)
  에서 계속 사용 — 즉시 보상이 서프라이즈로 적절하고, Cleared 지연 처리 복잡도 회피.
- 상점, 이벤트 방 투표 로직, 데일리 결정론 구조는 변경 없음 (모든 RNG는 `s.rng`).

### 5. UI (`ui/event_view.go`)

`EventScreen` 재사용. `PendingEvent.PerPlayer == true`일 때:

- 헤더: `"이벤트: <name>"` → `"보물 발견!"`
- 투표 현황 라벨: `"투표 현황:"` → `"선택 현황:"`
- 제출 상태 문구: `"선택지 N에 투표 — ..."` → `"선택 완료 — 다른 플레이어 대기 중"`

키 바인딩, 커서, 폴링은 그대로.

## 테스트 계획

- `game/loot_test.go`: 선택지 3개 생성, 라벨에 실수치 포함, 시드 고정 결정론,
  회전 슬롯 4종 모두 도달 가능
- `game/session_test.go`(또는 `loot_test.go`): 2인 파티 서로 다른 선택 → 각자
  자기 outcome만 적용, 저주 무기 HP 차감+아이템 지급, 인벤 풀 → 골드 전환,
  보물방 resolution 후 room Cleared + PhaseExploring 복귀, 미투표 alive 플레이어
  스킵, 이벤트 데미지 사망 시 AnyPlayerDied 셋
- `ui/event_view_test.go`: PerPlayer 뷰 문구 분기
- 기존 이벤트 투표 테스트 전부 통과 (회귀 없음)
