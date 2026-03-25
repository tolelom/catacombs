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
- `ui/model.go` ~19K 라인 (모든 화면 로직 집중)

## Phase 1: Foundation + Chat

### 1-1. UI Refactoring

**Problem:** `ui/model.go`가 ~19K 라인으로 모든 화면 상태를 단일 파일에서 관리. 새 기능 추가 시 버그 위험 증가, 유지보수 곤란.

**Design:**
- 각 화면의 `Update`/`View` 로직을 해당 `*_view.go` 파일로 이동
- 각 화면을 독립적인 Bubble Tea `Model` 인터페이스로 추출 (`LobbyModel`, `GameModel`, `ShopModel` 등)
- 메인 `Model`은 화면 간 라우팅만 담당하는 컨테이너 역할
- 공유 상태(게임 세션, DB, 뷰포트 크기 등)는 공통 `Context` 구조체로 추출

**Success criteria:** `model.go` 500라인 이하의 라우터로 축소.

### 1-2. In-game Chat

**Problem:** 멀티플레이에서 소통 수단이 없어 협동의 재미가 크게 떨어짐.

**Design:**
- `game/chat.go` — 채팅 메시지 브로드캐스트 (룸 내 플레이어 간)
- 게임 화면 하단에 채팅 로그 영역 추가
- `/` 키로 채팅 입력 모드 전환
- 이모트 프리셋 (`/hi`, `/gg` 등)

**Affected packages:** `game/`, `ui/`

## Phase 2: Combat & Dungeon Enhancement

### 2-1. Combat System Expansion

**Skill Tree:**
- 클래스별 3갈래 특성 트리 (예: Warrior → 탱커/버서커/전술가)
- 층 클리어 시 포인트 획득, 런 내에서만 유효 (로그라이크 특성 유지)
- `entity/skill_tree.go` — 트리 정의 및 포인트 적용 로직

**Combo Skills:**
- 2인 이상이 같은 턴에 특정 조합 사용 시 연계 효과 발동
- 예: Mage 빙결 + Warrior 강타 = 빙쇄 대미지
- `combat/combo.go` — 연계 조건 판정 및 효과 적용

**Elite Monsters:**
- 일반 몬스터의 강화 변종, 특수 접두사 (맹독의/불타는/흡혈의 등)
- 보스가 아닌 층에서 긴장감 추가
- `entity/elite.go` — 접두사 시스템 및 스탯 변형

**Status Effects Expansion:**
- 기존: 독, 화상
- 추가: 빙결(행동 불가), 출혈(턴마다 누적), 저주(회복량 감소)
- `entity/status.go` 확장

**Affected packages:** `entity/`, `combat/`, `game/`

### 2-2. Dungeon Event Diversification

**Random Event Rooms:**
- 선택지 기반 이벤트 (예: "수상한 제단 발견" → 제물 바치기/무시/파괴)
- 각 선택지마다 다른 보상/페널티
- `game/random_event.go` — 이벤트 풀 및 결과 처리

**Secret Rooms:**
- 낮은 확률(~10%)로 생성, 희귀 아이템/렐릭 보상
- `dungeon/generator.go` 확장

**Floor Themes:**
- 5층 단위로 환경 효과
  - 1~5층 습지: 독 대미지 증가
  - 6~10층 화산: 화상 대미지 증가
  - 11~15층 얼음: 빙결 확률 증가
  - 16~20층 지옥: 모든 상태이상 강화
- `dungeon/theme.go` — 테마별 환경 효과 정의

**Mini-boss Rooms:**
- 보스 층 직전(4, 9, 14, 19층)에 미니보스 등장
- 보스 전 준비 테스트 역할

**Affected packages:** `dungeon/`, `game/`, `entity/`

## Phase 3: Retention Systems

### 3-1. Daily Challenge

- 날짜 기반 시드로 동일한 던전 구조/몬스터 배치 생성
- 일일 전용 리더보드
- 참여 시 포인트 적립, 연속 참여 보너스
- `game/daily.go` — 시드 생성 및 일일 세션 관리
- `store/daily.go` — 일일 기록 및 연속 참여 추적

### 3-2. Meta Progression

**Unlock System Expansion:**
- 현재 업적 10개 → 조건부 언락 콘텐츠 추가
  - "10층 이상 클리어" → 5번째 클래스 해금
  - "3인 이상 클리어" → 하드 모드 해금
- `store/unlocks.go` — 언락 조건 및 상태 관리

**Player Titles:**
- 업적/기록 기반 칭호 부여
- 로비에서 닉네임 옆에 표시
- `store/titles.go` — 칭호 정의 및 부여 로직

**Codex System:**
- 조우한 몬스터/획득한 아이템/발견한 이벤트 기록
- 도감 완성률 표시
- `store/codex.go` — 도감 데이터 관리
- `ui/codex_view.go` — 도감 화면

### 3-3. Difficulty System

**Hard Mode:**
- 몬스터 스탯 1.5배, 상점 가격 2배, 회복량 절반
- 언락 조건 충족 시 선택 가능

**Weekly Mutations:**
- 매주 바뀌는 특수 규칙
  - "스킬 사용 불가", "엘리트만 등장", "상점 없음" 등
- `game/mutation.go` — 주간 변이 규칙 정의 및 적용

**Affected packages:** `game/`, `store/`, `ui/`

## Phase 4: Operational Stability

### 4-1. Logging & Monitoring

- `log/slog` (Go 표준) 도입, JSON 형식 구조화 로깅
- 이벤트: 접속, 게임 시작, 전투, 종료, 에러
- `/admin` HTTP 엔드포인트 — 접속자 수, 활성 방, 오늘의 런 수, 평균 클리어 층수
- 패닉 리커버리 미들웨어 — 세션 크래시 격리

**Affected packages:** `server/`, `web/`, `game/`

### 4-2. Data Safety

- 설정 가능한 주기로 DB 파일 자동 백업 (`./data/backup/`)
- 그레이스풀 셧다운: SIGTERM 시 진행 중인 세션 저장 후 종료
- 재시작 시 미완료 세션 정리

**Affected packages:** `store/`, `main.go`

### 4-3. Configuration Externalization

- `config.yaml` — 서버 포트, 턴 타임아웃, 몬스터 스케일링, 상점 가격 배율 등
- 밸런스 조정 시 재빌드 없이 설정 파일만 수정
- `config/config.go` — YAML 파싱 및 기본값 관리

**Affected packages:** 전체 (상수 참조 부분)

## Dependencies Between Phases

```
Phase 1 (Foundation)
  └── Phase 2 (Combat/Dungeon) — UI 리팩토링 완료 후 새 화면 추가가 용이
      └── Phase 3 (Retention) — 전투/던전 콘텐츠 위에 메타 시스템 구축
          └── Phase 4 (Operations) — 전체 기능 완성 후 운영 도구 보강
```

Phase 4의 로깅/설정 외부화는 독립적이므로, 필요 시 Phase 2~3과 병행 가능.

## Out of Scope

- 대규모(100+) 스케일링 (분산 서버, 로드밸런싱)
- 모바일/데스크톱 네이티브 클라이언트
- 유료 결제/과금 시스템
- PvP 대전 모드
- 외부 DB 마이그레이션 (BoltDB 유지)
