# Web Terminal Frontend — Design Spec

## Overview

xterm.js 기반 웹 브라우저 터미널. WebSocket → SSH 프록시 방식으로 기존 SSH 게임 서버에 접속.

## Architecture

```
Browser (xterm.js) ←WebSocket→ Go HTTP Server (:8080) ←SSH→ Wish SSH Server (:2222)
```

단일 Go 바이너리에서 SSH 서버와 HTTP 서버를 동시에 실행.

## Server: web/server.go

- `net/http`로 `:8080` 리스닝
- `/` — 정적 파일 서빙 (`web/static/` 디렉토리, embed 사용)
- `/ws` — WebSocket 엔드포인트

### WebSocket → SSH 프록시 흐름

1. 클라이언트가 `/ws`에 WebSocket 연결
2. 서버가 `golang.org/x/crypto/ssh`로 `localhost:2222`에 비밀번호 인증 SSH 접속
3. SSH 세션에서 PTY 요청 (초기 크기: 80x24)
4. 두 goroutine으로 양방향 중계:
   - SSH stdout → WebSocket 텍스트 프레임
   - WebSocket 텍스트 프레임 → SSH stdin
5. 리사이즈: WebSocket 바이너리 프레임으로 `{"type":"resize","cols":N,"rows":N}` 수신 → SSH WindowChange 요청
6. 연결 종료: 어느 쪽이든 끊기면 양쪽 모두 정리

### SSH 접속 설정

- Host: `localhost:2222`
- Auth: 비밀번호 인증 (빈 비밀번호 — 게임 서버가 모든 비밀번호 수용)
- HostKey: InsecureIgnoreHostKey (로컬 접속)
- User: `web-player`

## Client: web/static/index.html

단일 HTML 파일, 빌드 도구 없음.

### CDN 의존성

- `xterm` — 터미널 에뮬레이터
- `xterm-addon-fit` — 터미널을 컨테이너에 맞춤

### 기능

- 전체화면 다크 테마 터미널
- 자동 WebSocket 연결 (`ws://` or `wss://`)
- xterm.js `onData` → WebSocket send (키 입력)
- WebSocket `onmessage` → xterm.js `write` (출력)
- `FitAddon`으로 브라우저 리사이즈 감지 → 리사이즈 메시지 전송
- 연결 끊김 시 재연결 안내 표시

### 스타일

- 배경: `#1a1a2e` (진한 네이비)
- 폰트: 시스템 모노스페이스
- 터미널이 화면 전체를 차지 (margin: 0, overflow: hidden)

## Dependencies

- `github.com/gorilla/websocket` — WebSocket 업그레이드/핸들링
- `golang.org/x/crypto/ssh` — 이미 프로젝트에 포함

## File Map

```
web/
├── server.go              — HTTP 서버 + WebSocket→SSH 프록시
└── static/
    └── index.html         — xterm.js 클라이언트 (단일 파일)
main.go                    — web.Start() 호출 추가
docker-compose.yml         — 8080 포트 노출 추가
```

## Deployment

- Docker에서 2222 (SSH) + 8080 (HTTP) 두 포트 노출
- Caddy/nginx 리버스 프록시로 `play.catacombs.tolelom.xyz` → `:8080` 가능
