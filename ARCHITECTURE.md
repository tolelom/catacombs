# 아키텍처

## 접속 경로

```text
터미널 ── ssh :2222 ──────────────────▶ server (Wish SSH)
브라우저 ── WebSocket /ws :8080 ─▶ web ──▶ server   (서버 안 SSH 브리지)
                                           │
                                           ▼
                                  ui (Bubble Tea) ──▶ game (Lobby, GameSession) ──▶ store (BoltDB)
```

SSH와 웹은 같은 `Lobby`와 DB를 씁니다. 웹 브리지에는 공개 키가 없어서 연결마다 `anon:<hex>` 지문을
만듭니다.

## 코드 지도

의존 방향은 `main` → `server`·`web`·`store` → `game` → `dungeon` → `entity` → `combat`입니다.

| 패키지 | 책임 |
|---|---|
| `main.go` | DB 열기, SSH(:2222)·HTTP(:8080) 서버 시작, 백업 스케줄러, 종료 때 세션 정리 |
| `game/` | `Lobby`(방, 플레이어, 재접속)와 `GameSession`(턴 진행, 방 이벤트, 주간 변이) |
| `ui/` | Bubble Tea 화면 상태 기계. 화면마다 파일 하나 |
| `dungeon/` | BSP 층 생성(60×20), ASCII 렌더링, 시야 |
| `entity/` | 플레이어·직업·스킬 트리, 몬스터, 아이템, 유물 |
| `combat/` | 피해 계산, 몬스터 AI, 도주, 협동 콤보 |
| `record/` | 게임이 끝날 때 기록·업적·해금·일일 기록 저장 |
| `store/` | BoltDB 저장과 백업 |
| `server/` | Wish SSH 서버, 연결 종료 때 `Lobby.Disconnect` |
| `web/` | WebSocket→SSH 브리지, 내장 xterm.js 화면, `/admin` |
| `config/` | `config.yaml` 읽기와 환경 변수 덮어쓰기 |

## 지켜야 할 규칙

- `GameSession.mu`가 세션 상태 전부를 지키고, UI는 `GetState()` 깊은 복사본을 읽습니다. `Lobby.mu`는
  방 목록만 지키며, 로비 잠금을 쥔 채 세션을 부르지 않습니다.
- 일일 도전은 `DailySeed(날짜) + 층`으로 세션 RNG를 다시 시드합니다. 콘텐츠 경로에서 패키지 수준
  `rand`를 부르지 않고 세션 RNG를 넘깁니다.
- 업적·해금·칭호·도감·일일 기록은 지문으로, 랭킹과 통계는 플레이어 이름으로 묶습니다.
