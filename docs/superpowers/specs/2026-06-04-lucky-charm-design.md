# Lucky Charm (행운 부적) — RNG 헤지 수단 설계

날짜: 2026-06-04
상태: 자율 진행 (사용자 위임)

## 문제

게임에 도박성 RNG 순간이 많다 — 미확인 상자(25% 함정), 제단 기도(40% 역효과),
미믹 상자 이벤트(30% 물림), 버섯(33% 독), 도박장(50% 손실) — 그런데 나쁜 결과에
대한 **사전 헤지 수단이 전혀 없다**. 나쁜 굴림은 그냥 맞아야 하는 벌이라서,
도박 선택지가 "재미있는 리스크"가 아니라 "피해야 할 함정"으로 읽힌다.
재미 진단 #5이자 마지막 항목.

## 검토한 대안

- **A. 행운 부적 (구매형 보험 소모품)** ← 채택. 상점에서 사두면 도박성 손해
  1회를 자동 무효화. 구매가 곧 헤지 결정. 스코프 최소, 골드 싱크 부수 효과.
- B. 운명 토큰 + 인터랙티브 리롤: 나쁜 결과 직후 "토큰으로 리롤?" 프롬프트.
  더 극적이지만 모든 RNG 사이트에 인터랙션 훅 필요 — 스코프 3배.
- C. 헤지 유물 추가: 유물 획득 자체가 보스 드랍 RNG라 "보험을 운에 맡기는"
  자기모순.

## 설계

### 행운 부적 규칙

- 새 아이템 타입 `ItemCharm`. 인벤토리에 **들고만 있으면 자동 발동** (사용 키 없음).
- 발동 조건: 소지자가 **도박을 선택한 결과로 입는 손해** 1회를 무효화하고 소모됨.
  - 이벤트/루팅 선택지 중 `TagGamble`이 붙은 선택의 음수 결과
    (HP 피해와 골드 손실 모두 — 한 결과의 손해 전부를 부적 1개로 막음)
  - 도박장(`handleGamble`) 베팅 실패 → 50G 환불
- 로그: `"🍀 <이름>의 행운 부적이 불운을 막아냈다!"`
- 여러 개 소지 가능 (각 1회). 인벤토리 1칸 차지 (10칸 한도와 트레이드오프).

**헤지 경계 (적용 제외):**
- 확정 트레이드오프 (저주받은 무기 HP 비용, 제물 훔치기, 저주 제단) — 도박이
  아니라 *가격*이므로 보호하지 않음. `TagGamble` 유무가 정확히 이 경계를 가름.
- 전투 RNG (회피/빗맞음/크리/도주 실패) — 전투는 체인·스킬·유물의 영역.
- 미믹 방 매복 — 전투 발생 자체는 손해가 아님.

**만들어지는 결정:**
- 상점에서 "부적 30G vs 장비/포션" — 사전 헤지 구매 결정
- 부적을 들고 있으면 도박 선택지를 공격적으로 고를 수 있음 (미확인 상자
  픽률 상승 → 루팅 선택과 시너지)
- 후반 잉여 골드의 새 사용처 (골드 싱크)

### 컴포넌트

1. **`entity/item.go`**:
   - `ItemCharm ItemType` 추가
   - `NewLuckyCharm(floor int) Item` — `{Name: "행운 부적", Type: ItemCharm,
     Price: 30 + floor}`
   - `(p *Player) ConsumeCharm() bool` — 인벤토리에서 부적 1개 제거, 있었으면 true

2. **`game/random_event.go`**:
   - `EventOutcome.Gambled bool` 추가
   - `tagsContain(tags []ChoiceTag, want ChoiceTag) bool` production 헬퍼
     (loot_test.go의 hasTag 테스트 헬퍼를 이것 재사용으로 정리)
   - **Resolve 함수들은 수정하지 않음** — 호출부(`resolveEventVoteLocked`,
     `resolveLootPicksLocked`)에서 `outcome.Gambled = tagsContain(choice.Tags,
     TagGamble)`로 표시

3. **`game/event.go`**:
   - `applyEventOutcome`(파티)와 `applyOutcomeToPlayer`(개인) 양쪽:
     손해 적용 직전 `o.Gambled && p.ConsumeCharm()`이면 그 플레이어의 HP 피해와
     골드 손실을 스킵 + 부적 로그. (파티 이벤트는 피해자 각자 자기 부적 소모)
   - `handleGamble` 실패 분기: 부적 소모 시 50G 환불
   - `generateShopItems`: 4번째 슬롯에 부적 상시 추가 (HardMode 가격 배율 적용)
   - `handleWanderingMerchant`: 부적 추가 (2배 가격)

4. **`game/session.go` `BuyItem`**: 품절 조건을 `Type != ItemConsumable`에서
   "무기/방어구만 품절"로 변경 — 부적도 포션처럼 반복 구매 가능

5. **`ui/shop_view.go`**: `[1-3]` 키 → `[1-4]`, 메뉴 라벨 갱신,
   `itemTypeLabel`에 ItemCharm → `"[불운 1회 방어]"`

### 스코프 제외

- 부적은 상점 전용 — 루팅/보물/이벤트 드랍에서 안 나옴 (구매 결정이 핵심)
- 미믹 방, 전투 RNG, 도주 실패 헤지 없음
- 부적 가격/효과의 config 노출 없음 (기존 상점 아이템들과 동일하게 하드코딩)

## 테스트 계획

- entity: ConsumeCharm (있음/없음/여러 개 중 1개만 소모)
- game: TagGamble 음수 이벤트에서 부적 소모+피해 무효 / TagGamble 없는 확정
  손해는 부적 무시 / 양수 결과는 부적 미소모 / 도박장 실패 환불 / 파티
  이벤트에서 부적 소지자만 보호 / 루팅 미확인 상자 함정 보호 /
  상점 4슬롯 생성 + 부적 반복 구매 / HardMode 가격 배율
- ui: itemTypeLabel charm 라벨, 상점 키 [4]
- 기존 테스트 회귀 없음 (특히 BuyItem 품절 정책 변경)
