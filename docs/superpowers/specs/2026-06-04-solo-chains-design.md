# Solo Chains (연계기) — 솔로 모드 콤보 대체 메커닉 설계

날짜: 2026-06-04
상태: 자율 진행 (사용자 위임)

## 문제

이 게임의 차별점은 협동 콤보인데, 솔로 모드에서는 그 레이어가 통째로 죽는다:

- **콤보 11종 전부** 2~3개의 *서로 다른 클래스*를 요구 → 솔로에서 발동 불가능
- `CombosFiredThisRun` 기반 업적/도감도 솔로에서 달성 불가능
- **CoopBonus +10%** (같은 대상 2번째 공격자)도 솔로 불가능
- 협동기 자체(Bulwark/Ward/신성가호/정찰)는 솔로 대응이 있으나, 솔로 보정은
  스탯 감산(`SoloHPReduction`/`SoloATKReduction`)뿐 — *숫자*는 맞췄지만 *결정*이 없다

솔로 플레이어(웹/SSH 첫 방문자의 대부분)는 전투에서 "공격 누르기"만 남는다.

## 검토한 대안

- **A. 연계기 (행동 시퀀스 체인)** ← 채택. 자기 행동의 *순서*가 콤보가 된다.
  매 턴 실제 결정 생성, 기존 콤보 인프라와 동형, 스코프 적정.
- B. 모멘텀 게이지 (연속 공격 스택): 수동적 숫자 램프 — 결정이 약함.
- C. AI 정령 동료 (가상 2번째 클래스로 기존 콤보 활성화): 전투 AI/타겟팅/파티
  크기 가정 변경 — 스코프 과대, 리스크 큼.

## 설계

### 체인 정의 (SoloMode 전용, 전투 중)

| 체인 | 시퀀스 | 효과 | 파티 콤보 대응 |
|------|--------|------|---------------|
| 잔향 일격 (Echo Strike) | 스킬(성공) → 공격 | 그 공격 데미지 **1.4×** | Ice Shatter 1.5× |
| 연속 공세 (Relentless) | 공격 3연속째부터 | 데미지 **1.25× +3 ATK** | Full Assault 1.3×+5 |
| 반격 태세 (Counter Stance) | 협동기(성공) → 공격 | 그 공격 **크리 확정(2.0×)** | Expose Weakness |

파티 콤보보다 약간 낮은 수치: 솔로 체인은 혼자 마음대로 발동 가능하므로
(파티 콤보는 2인 이상의 조율 필요).

**만들어지는 결정:**
- 스킬(3회/전투 한정)을 "큰 한 방"이 아니라 "다음 턴 체인 셋업"으로 쓸지
- 아이템/대기/도주는 연속 공세 스트릭을 끊음 → 회복 타이밍이 트레이드오프가 됨
- 협동기(1회/전투)를 방어가 아니라 크리 셋업으로 태울지

**상호 배제:** 스킬/협동기/아이템은 공격 스트릭을 리셋하므로 한 공격에 체인은
최대 1개만 적용된다. 크리 확정은 자연 크리(2.0×)와 중첩되지 않음(max). 잔향
일격은 자연 크리와 곱연산(파티 콤보와 동일 규칙).

### 행동 기록 규칙

체인 상태는 **성공한 행동**만 기록한다:

| 이번 턴 행동 | lastAction | attackStreak |
|---------------|-----------|--------------|
| 공격 | "attack" | +1 |
| 스킬 성공 | "skill" | 0 |
| 스킬 실패 (횟수 없음) | "" | 0 |
| 협동기 성공 | "coop" | 0 |
| 협동기 실패 (이미 사용/로그 정찰) | "" | 0 |
| 아이템 | "item" | 0 |
| 도주 실패 / 대기 / 동결 | "" | 0 |

전투 시작 시(스폰 4사이트 + 미믹) 리셋.

### 컴포넌트

1. **`combat/solo_chain.go` (신규)** — 순수 함수, 세션 비의존:
   ```go
   type SoloChain struct {
       Name             string
       Message          string
       DamageMultiplier float64
       BonusDamage      int
       GuaranteeCrit    bool
   }
   // DetectSoloChain returns the chain an attack triggers given the previous
   // action and the consecutive-attack streak BEFORE this attack, or nil.
   func DetectSoloChain(lastAction string, attackStreak int) *SoloChain
   ```
   - `lastAction=="skill"` → Echo Strike
   - `lastAction=="coop"` → Counter Stance
   - `attackStreak >= 2` (이번이 3연타째) → Relentless

2. **`game/session.go`** — 상태 2필드 + HUD 뷰:
   - `soloLastAction string`, `soloAttackStreak int` (per-combat, s.mu 보호)
   - `GameState.SoloChain *SoloChainView { LastAction string; AttackStreak int }`
     — SoloMode 전투 중에만 populate, GetState에서 복사

3. **`game/turn.go`** — 배선:
   - `ActionAttack` 케이스: SoloMode면 `DetectSoloChain` 호출 → 그 intent의
     Multiplier/ATK 조정 + 로그 + `CombosFiredThisRun[chain.Name] = true`
     (솔로에서도 콤보 업적/도감 경로 열림)
   - 각 액션 케이스 끝에 `s.recordSoloAction(actionType)` 1-liner (성공 여부 반영)
   - 협동기 성공 판정: `applyCoopAbility` 호출 전 `coopUsedThisCombat` 체크 결과 사용

4. **`game/event.go`** — 스폰 사이트 4곳의 중복 per-combat 리셋 꼬리
   (coopUsedThisCombat/activeRelicUsed/extraActionNext/skillUses/applyPoison)를
   `resetPerCombatStateLocked()` 헬퍼로 추출하고 솔로 체인 리셋 추가
   (기존 코드 개선 — 동일 5줄이 4곳에 중복돼 있음)

5. **`ui/game_view.go`** — 전투 HUD에 솔로 체인 힌트 1줄 (SoloChain != nil일 때):
   - lastAction=="skill": `"연계 준비: 공격 시 잔향 일격(1.4×)!"`
   - lastAction=="coop": `"연계 준비: 공격 시 반격 태세(크리 확정)!"`
   - attackStreak>=2: `"연속 공세 발동 중 (N연타, 1.25×)"`
   - attackStreak==1: `"공격 1연타 (3연타부터 연속 공세)"`
   - 그 외: 표시 없음

### 스코프 제외

- 파티 모드 콤보/CoopBonus 변경 없음 (SoloMode 가드)
- 솔로 스탯 감산(`SoloATKReduction` 등) 재조정 없음 — 체인 추가로 솔로가
  쉬워지면 별도 밸런스 패스에서 조정 (한 번에 한 변수)
- 시간유리(extraAction) 추가 공격은 스트릭에 미반영 (보너스 행동은 체인 외부)

## 테스트 계획

- `combat/solo_chain_test.go`: 시퀀스별 체인 판정 (skill→Echo, coop→Counter,
  streak 0/1/2/5, 우선순위 배제), nil 케이스
- `game/turn` 테스트: 솔로 skill→attack 턴 시퀀스로 1.4× 적용 검증, 3연타
  Relentless, coop→attack 크리, 아이템이 스트릭 리셋, 스킬 실패는 체인 미설정,
  파티 모드(2인)에서 체인 미발동, 전투 시작 리셋, CombosFiredThisRun 등록
- `ui`: SoloChain 힌트 렌더 분기
- 기존 콤보/코업 테스트 회귀 없음
