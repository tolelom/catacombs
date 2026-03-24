# Terminal Visuals Enhancement — Design Spec

## Overview

Catacombs 터미널 비주얼 개선. 4가지 영역: 몬스터 아스키아트, 전투 레이아웃 리디자인, 컬러 효과, 타이틀 화면.

## 1. Monster ASCII Art

몬스터 타입별 3~4줄 아스키아트. 전투 HUD의 ENEMIES 패널에 표시.

```
일반 몬스터 (3줄):

 /\OO/\        ,--.        .---.       /|||\
 \    /        |oo|       /o   o\      |===|
  |__|         /||\       | --- |      |   |
 Slime       Skeleton      Orc      Dark Knight

보스 (5줄):

   [B5]          [B10]         [B15]          [B20]
  /\  /\        __|__        ,=====.       ___/\___
 | @  @ |      /|o o|\      / \   / \     |  x  x  |
 |  ||  |      | === |     |  (O)  |      |  ===   |
 | \__/ |      |\___/|      \ |=| /       |\_____/|
  \    /        |___|        `==='         |_|   |_|
 Guardian      Warden      Overlord       Archlich
```

`entity/monster.go`에 `Art() []string` 메서드 추가. 타입별 하드코딩된 문자열 슬라이스 반환.

## 2. Combat Layout Redesign

lipgloss 박스드로잉으로 전투 화면을 패널 구조로 리디자인.

```
┌─ Catacombs B3 ──────────────────────────────────────────────┐
│  [미니맵 영역]                                                │
├────────────────────────────┬────────────────────────────────┤
│  PARTY                     │  ENEMIES                       │
│                            │                                │
│  ♦ Tank (Warrior)          │    /\OO/\                      │
│    ██████████░░░░ 80/120   │    \    /   > [0] Slime  8/20  │
│    ATK:15  DEF:10          │     |__|                       │
│                            │                                │
│  ♦ Mage (Mage)             │    ,--.                        │
│    ██████░░░░░░░░ 45/70    │    |oo|     [1] Skeleton 30/35 │
│    ATK:23  DEF:5           │    /||\                        │
│                            │                                │
├────────────────────────────┴────────────────────────────────┤
│  > Tank hit Slime for 12 dmg                                │
│  > Mage hit all enemies for 18 total dmg (co-op!)           │
├─────────────────────────────────────────────────────────────┤
│  [1]Attack [2]Skill [3]Item [4]Flee [5]Wait [Tab]Target     │
│  Skill: Taunt — enemies attack you for 2 turns   Timer: 4.2s│
└─────────────────────────────────────────────────────────────┘
```

구조:
- 상단: 미니맵 (dungeon.RenderFloor 호출, 축소)
- 중단 좌: PARTY 패널 — 각 플레이어 이름, 클래스, HP바, ATK/DEF
- 중단 우: ENEMIES 패널 — 아스키아트 + 이름 + HP바 + 타겟 커서
- 하단 로그: CombatLog 메시지 (컬러 적용)
- 최하단: 액션 바 + 타이머

탐험 모드에서는 중단이 맵 + 방 목록으로 전환.

lipgloss `JoinHorizontal`/`JoinVertical` + `Border` 사용.

## 3. Color Effects

로그 메시지와 HUD 요소에 의미적 컬러 적용:

| 요소 | 색상 | lipgloss Color |
|------|------|---------------|
| 데미지 수치 | 빨강 볼드 | `"196"` Bold |
| 힐 수치 | 초록 볼드 | `"46"` Bold |
| co-op 보너스 | 노랑 볼드 | `"226"` Bold |
| 도주 | 시안 | `"51"` |
| 상태효과(도발 등) | 마젠타 | `"201"` |
| 몬스터 이름 | 빨강 | `"196"` |
| 플레이어 이름 | 흰색 볼드 | `"255"` Bold |
| 골드 | 노랑 | `"226"` |
| 시스템 메시지 | 회색 이탤릭 | `"240"` Italic |

HP 바 3단계 색상:
- 50%↑: 초록 (`"46"`)
- 25~50%: 노랑 (`"226"`)
- 25%↓: 빨강 (`"196"`)

로그 메시지에 인라인 컬러링 적용. `addLog` 대신 구조화된 로그 메시지 사용:

```go
type LogEntry struct {
    Text   string
    Color  string // lipgloss color code
}
```

또는 더 간단하게: 로그 문자열에 lipgloss 스타일을 렌더링 시점에 적용. 로그 메시지에 키워드 패턴 매칭으로 자동 컬러링.

**선택: 렌더링 시점 패턴 매칭 방식.** `addLog`는 plain text 유지. `renderCombatLog`에서 숫자+`dmg` → 빨강, 숫자+`HP` → 초록, `co-op` → 노랑, `fled` → 시안 등 매칭.

## 4. Title Screen

대형 아스키아트 로고 + 분위기 서브텍스트:

```
   ██████╗ █████╗ ████████╗ █████╗  ██████╗ ██████╗ ███╗   ███╗██████╗ ███████╗
  ██╔════╝██╔══██╗╚══██╔══╝██╔══██╗██╔════╝██╔═══██╗████╗ ████║██╔══██╗██╔════╝
  ██║     ███████║   ██║   ███████║██║     ██║   ██║██╔████╔██║██████╔╝███████╗
  ██║     ██╔══██║   ██║   ██╔══██║██║     ██║   ██║██║╚██╔╝██║██╔══██╗╚════██║
  ╚██████╗██║  ██║   ██║   ██║  ██║╚██████╗╚██████╔╝██║ ╚═╝ ██║██████╔╝███████║
   ╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚══════╝

                      ⚔  A Cooperative Dungeon Crawler  ⚔
                           ssh catacombs.tolelom.xyz
```

로고는 그라데이션 컬러 (lipgloss로 행별 색상 변화: 빨강→주황→노랑).

메뉴:
```
                            [Enter] Start
                            [Q]     Quit
```

센터 정렬 (lipgloss `Place` 사용).

## File Map

```
entity/monster.go        — Art() 메서드 추가
ui/ascii_art.go          — NEW: 몬스터/보스 아스키아트 데이터
ui/game_view.go          — 전투 레이아웃 리디자인, 컬러 로그
ui/title.go              — 타이틀 화면 개선
ui/styles.go             — NEW: 공유 lipgloss 스타일 상수
```
