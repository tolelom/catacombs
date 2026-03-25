# Catacombs Game Enhancement Design

## Overview

Catacombs 멀티플레이 로그라이크 던전 크롤러의 종합 고도화 설계.
실제 유저 운영(소규모 10~30명 커뮤니티) 목표로, 재미/리텐션/안정성 중심.

접근 방식: **기반 정비 → 콘텐츠 → 리텐션 → 운영** (접근 A)

## Current State

- Go + Bubble Tea TUI, SSH(:2222) + WebSocket(:8080) 듀얼 접속
- 4 클래스(Warrior/Mage/Healer/Rogue), 8 몬스터 + 4 보스, 20층 던전
- 턴제 전투(5초 타임아웃), 상점, 업적 10개, 리더보드
- BoltDB 영속화, SSH 핑거프린트 재접속
- 인게임 채팅 구현됨 (`/` 키로 입력, `SendChat()` 브로드캐스트)
- 상태이상: 독, 화상, 빙결 이미 존재
- `ui/model.go` ~712라인 (라우팅/업데이트 디스패치), 화면별 `*_view.go`로 분리 완료
- 기존 협동 보너스: 2인 이상 같은 타겟 공격 시 +10% 대미지

## Phase 1: Foundation + Structured Logging

### 1-1. UI Architecture Refinement

**Problem:** `model.go` (712라인)의 `Update()` 메서드가 모든 화면의 키 입력을 하나의 switch문에서 처리. 새 화면/기능 추가 시 분기가 복잡해짐.

**Design:**
- 각 화면을 독립적인 Bubble Tea `Model` 인터페이스로 추출 (`LobbyModel`, `GameModel`, `ShopModel` 등)
- 각 `*_view.go`에 해당 화면의 `Update()`/`View()` 로직을 완전히 위임
- 메인 `Model`은 화면 전환 라우팅만 담당
- 공유 상태(게임 세션, DB, 뷰포트 크기 등)는 공통 `Context` 구조체로 추출

**Success criteria:** `model.go`의 `Update()` 메서드가 화면별 라우팅만 수행 (각 화면 로직 0라인).

### 1-2. Chat Emote System

**Note:** 기본 채팅은 이미 구현됨 (`game/session.go:SendChat`, `/` 키 입력).

**Design:**
- 이모트 프리셋 시스템 추가 (`/hi`, `/gg`, `/go`, `/wait`, `/help` 등)
- 이모트는 채팅 로그에 강조 스타일로 표시
- `game/emote.go` — 이모트 정의 및 파싱

### 1-3. Structured Logging

**Rationale:** Phase 2~3 디버깅을 위해 초기에 도입.

**Design:**
- `log/slog` (Go 표준) 도입, JSON 형식 구조화 로깅
- 이벤트: 접속, 게임 시작, 전투, 종료, 에러
- 패닉 리커버리 미들웨어 — 세션 크래시 격리

### 1-4. Configuration Externalization

**Rationale:** Phase 2~3에서 밸런스 상수를 추가하기 전에 설정 구조를 먼저 도입.

**Design:**
- `config/config.go` — YAML 파싱 및 기본값 관리
- `config.yaml` — 서버 포트, 턴 타임아웃, 몬스터 스케일링, 상점 가격 배율 등

```go
type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Game     GameConfig     `yaml:"game"`
    Combat   CombatConfig   `yaml:"combat"`
    Dungeon  DungeonConfig  `yaml:"dungeon"`
    Backup   BackupConfig   `yaml:"backup"`
}

type GameConfig struct {
    TurnTimeout    time.Duration `yaml:"turn_timeout"`    // default: 5s
    MaxPlayers     int           `yaml:"max_players"`     // default: 4
    MaxFloors      int           `yaml:"max_floors"`      // default: 20
    CoopBonus      float64       `yaml:"coop_bonus"`      // default: 0.10
    InventoryLimit int           `yaml:"inventory_limit"` // default: 10
}
```

**Affected packages:** 전체 (상수 참조 부분), `server/`, `game/`

## Phase 2: Combat & Dungeon Enhancement

### 2-1. Combat System Expansion

**Skill Tree:**
- 클래스별 2갈래 특성 트리 (MVP, 추후 3갈래 확장 가능)
  - Warrior: 탱커 / 버서커
  - Mage: 원소술사 / 시간술사
  - Healer: 수호자 / 사제
  - Rogue: 암살자 / 약사
- 층 클리어 시 1포인트 획득, 브랜치당 3노드, 런 내에서만 유효
- 스킬 포인트 배분은 층 이동 화면에서 수행 (턴 동기화 불필요)

```go
// entity/skill_tree.go
type SkillBranch struct {
    Name        string
    Nodes       [3]SkillNode  // 3 sequential unlocks per branch
    Description string
}

type SkillNode struct {
    Name     string
    Effect   SkillEffect  // enum: ATKBoost, DEFBoost, SkillPower, etc.
    Value    float64      // modifier amount
    Required int          // points spent in branch to unlock
}

type PlayerSkills struct {
    BranchIndex int       // 0 or 1 (chosen branch)
    Points      int       // total points earned
    Allocated   int       // points allocated in chosen branch
}
```

**Combo Skills:**
- 2인 이상이 같은 턴에 특정 조합 사용 시 연계 효과 발동
- 기존 협동 보너스(+10%)와 별개로 스택됨
- 예: Mage 빙결 + Warrior 강타 = 빙쇄 (대미지 1.5배 + 빙결 해제)

```go
// combat/combo.go
type ComboDefinition struct {
    RequiredActions []ComboAction  // class + action type pairs
    Effect          ComboEffect
    Name            string
    Description     string
}

type ComboAction struct {
    Class      entity.Class
    ActionType string  // "skill", "attack"
    SkillName  string  // optional: specific skill required
}
```

**Elite Monsters:**
- 일반 몬스터의 강화 변종, 층당 ~20% 확률로 등장
- 접두사별 스탯 변형 및 특수 능력

```go
// entity/elite.go
type ElitePrefix struct {
    Name       string        // "맹독의", "불타는", "흡혈의"
    StatMod    StatModifier  // HP/ATK/DEF multipliers
    OnHit      StatusEffect  // applied on monster's attack
    DropBonus  float64       // extra loot chance
}
```

**Status Effects Expansion:**
- 기존: 독, 화상, 빙결
- 추가: 출혈(턴마다 대미지 누적 +1), 저주(회복량 50% 감소)
- `entity/player.go`에 `StatusBleed`, `StatusCurse` 추가

**Success criteria:** 8개 스킬 브랜치, 5개 이상 콤보 조합, 5개 엘리트 접두사, 5개 상태이상.

### 2-2. Dungeon Event Diversification

**Random Event Rooms:**
- 선택지 기반 이벤트 (예: "수상한 제단 발견" → 제물 바치기/무시/파괴)
- 최소 8개 이벤트 풀

```go
// game/random_event.go
type RandomEvent struct {
    ID          string
    Description string
    Choices     []EventChoice
}

type EventChoice struct {
    Text    string
    Outcome EventOutcome  // reward/penalty/mixed
    Weight  float64       // probability weight
}
```

**Secret Rooms:**
- 낮은 확률(~10%)로 생성, `dungeon/room.go`에 `RoomSecret` 타입 추가
- 희귀 아이템/렐릭 보상

**Floor Themes:**
- 5층 단위로 환경 효과, 보스 패턴과 테마 정렬:
  - 1~5층 습지 (독 강화) → Guardian 보스 (독 패턴으로 변경)
  - 6~10층 화산 (화상 강화) → Warden 보스 (화상 패턴으로 변경)
  - 11~15층 얼음 (빙결 강화) → Overlord 보스 (빙결 패턴)
  - 16~20층 지옥 (전체 강화) → Archlich 최종 보스 (AoE + 회복 패턴, 기존 Guardian의 AoE를 최종 보스로 이동)

```go
// dungeon/theme.go
type FloorTheme struct {
    Name           string
    StatusBoost    entity.StatusEffect  // which status is empowered
    DamageModifier float64              // multiplier for boosted status
    AsciiStyle     string               // visual theme for rendering
}
```

**Mini-boss Rooms:**
- 보스 층 직전(4, 9, 14, 19층)에 미니보스 등장
- 기존 `Monster` 구조체에 `IsMiniBoss bool` 추가, `BossPattern` 사용하되 HP/ATK는 보스의 60%

**Prerequisite for Phase 3-1:** 던전 생성을 시드 기반으로 리팩토링.
- `GenerateFloor(floor int)` → `GenerateFloor(floor int, rng *rand.Rand)`
- 모든 랜덤 호출을 `rng` 인스턴스로 교체

**Success criteria:** 8개 이상 랜덤 이벤트, 4개 층 테마, 비밀 방, 4개 미니보스.

**Affected packages:** `dungeon/`, `game/`, `entity/`, `combat/`

## Phase 3: Retention Systems

### 3-1. Daily Challenge

- 날짜 기반 시드(`time.Now().Format("2006-01-02")` → hash → seed)로 동일 던전 생성
- Phase 2-2에서 리팩토링된 시드 기반 생성기 활용
- 일일 전용 리더보드

```go
// store/daily.go  (BoltDB bucket: "daily_runs")
// Key: "YYYY-MM-DD:playerFingerprint"
type DailyRecord struct {
    Date       string `json:"date"`
    Player     string `json:"player"`
    FloorReached int  `json:"floor_reached"`
    GoldEarned int    `json:"gold_earned"`
    Streak     int    `json:"streak"`  // consecutive days played
}
```

**Success criteria:** 동일 날짜에 같은 시드 던전 보장, 일일 리더보드 표시.

### 3-2. Meta Progression

**Unlock System:**

```go
// store/unlocks.go  (BoltDB bucket: "unlocks")
// Key: "playerFingerprint:unlockID"
type Unlock struct {
    ID        string `json:"id"`
    Condition string `json:"condition"`  // human-readable
    Unlocked  bool   `json:"unlocked"`
}
```

언락 콘텐츠:
- "10층 이상 클리어" → 5번째 클래스 해금
- "3인 이상 클리어" → 하드 모드 해금
- "20층 클리어" → 주간 변이 모드 해금

**Player Titles:**

```go
// store/titles.go  (BoltDB bucket: "titles")
// Key: "playerFingerprint"
type PlayerTitle struct {
    ActiveTitle string   `json:"active_title"`
    Earned      []string `json:"earned"`  // list of title IDs
}
```

**Codex System:**

```go
// store/codex.go  (BoltDB bucket: "codex")
// Key: "playerFingerprint"
type Codex struct {
    Monsters map[string]bool `json:"monsters"`  // monsterID → encountered
    Items    map[string]bool `json:"items"`     // itemID → acquired
    Events   map[string]bool `json:"events"`    // eventID → discovered
}
```

- `ui/codex_view.go` — 도감 화면, 완성률 표시

**Success criteria:** 3개 이상 언락 콘텐츠, 5개 이상 칭호, 도감 완성률 추적.

### 3-3. Difficulty System

**Hard Mode:**
- 난이도 배율은 `config.yaml`에서 관리 (Phase 1-4의 설정 구조 활용)
- 몬스터 스탯 1.5배, 상점 가격 2배, 회복량 절반
- 언락 조건 충족 시 로비에서 선택 가능

**Weekly Mutations:**
- 매주 바뀌는 특수 규칙, 주 번호 기반 시드로 결정

```go
// game/mutation.go
type Mutation struct {
    ID          string
    Name        string  // "스킬 봉인", "엘리트 범람", "상점 폐쇄" 등
    Description string
    Apply       func(cfg *config.GameConfig)  // config 값 오버라이드
}
```

**Affected packages:** `game/`, `store/`, `ui/`, `config/`

## Phase 4: Operational Stability

### 4-1. Admin Dashboard

- `/admin` HTTP 엔드포인트 (Basic Auth 인증)
- JSON 응답 형식:

```go
type AdminStats struct {
    OnlinePlayers  int            `json:"online_players"`
    ActiveRooms    int            `json:"active_rooms"`
    TodayRuns      int            `json:"today_runs"`
    AvgFloorReach  float64        `json:"avg_floor_reached"`
    Uptime         time.Duration  `json:"uptime"`
}
```

### 4-2. Data Safety

- 설정 가능한 주기로 DB 파일 자동 백업 (`./data/backup/`, `config.yaml`의 `backup.interval`)
- 그레이스풀 셧다운: SIGTERM 시 진행 중인 세션 저장 후 종료
- 재시작 시 미완료 세션 정리

**Affected packages:** `store/`, `web/`, `main.go`

## Data Migration

기존 BoltDB 데이터와의 호환성:
- 새 버킷(`daily_runs`, `unlocks`, `titles`, `codex`)은 `CreateBucketIfNotExists`로 안전하게 추가
- 기존 `profiles`, `rankings`, `achievements` 버킷은 구조 변경 없음
- 업적 시스템과 언락 시스템은 별도 버킷으로 독립 운영 (기존 업적 데이터 보존)

## Testing Strategy

- **단위 테스트:** 새 패키지별 `*_test.go` (스킬 트리 포인트 계산, 콤보 판정, 엘리트 스탯 변형)
- **시드 결정성 테스트:** 동일 시드 → 동일 던전 보장 검증
- **밸런스 시뮬레이션:** 엘리트/미니보스 스탯 범위가 솔로/파티 모두에서 적정한지 수치 검증
- **통합 테스트:** 턴 실행 시 콤보 판정 → 대미지 계산 → 상태이상 적용 흐름

## Dependencies Between Phases

```
Phase 1 (Foundation: UI정리 + 로깅 + 설정 외부화)
  ├── Phase 2 (Combat/Dungeon) — 설정 구조 위에 밸런스 상수 추가
  │   └── Phase 3 (Retention) — 시드 기반 생성기, 콘텐츠 위에 메타 시스템
  └── Phase 4 (Operations) — 로깅 기반 위에 어드민/백업 추가
```

Phase 4는 Phase 1 완료 후 독립 진행 가능 (Phase 2~3과 병행).

## Out of Scope

- 대규모(100+) 스케일링 (분산 서버, 로드밸런싱)
- 모바일/데스크톱 네이티브 클라이언트
- 유료 결제/과금 시스템
- PvP 대전 모드
- 외부 DB 마이그레이션 (BoltDB 유지)
