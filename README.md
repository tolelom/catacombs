# Catacombs

> SSH와 웹 브라우저로 접속하는 멀티플레이 로그라이크 던전 크롤러

Go 1.26 · Bubble Tea · Wish(SSH) · BoltDB · xterm.js · 데모 https://catacombs.tolelom.xyz/static/

## 기능

- 최대 4인 턴제 협동 전투와 협동 콤보
- 터미널과 브라우저 접속자가 같은 로비 사용
- 직업(전사, 마법사, 힐러, 도적)마다 두 갈래 스킬 트리
- 층마다 새로 만드는 20층 던전, 5층마다 보스
- 보스가 떨어뜨리는 유물
- 모두가 같은 던전을 도는 일일 도전, 주마다 바뀌는 변이 규칙
- 업적, 랭킹, 도감, 통계
- 연결이 끊겨도 60초 안에 다시 들어오면 이어서 플레이

## 시작하기

Go 1.26이 필요합니다.

```bash
git clone https://github.com/tolelom/catacombs.git
cd catacombs
go run .                # 또는 docker compose up
```

| 접속 | 주소 |
|---|---|
| 터미널 | `ssh -p 2222 localhost` |
| 브라우저 | http://localhost:8080/static/ |

설정은 `config.yaml`에서 읽고, 파일이 없으면 기본값을 씁니다.

## 문서

| 문서 | 내용 |
|---|---|
| `ARCHITECTURE.md` | 패키지 구성과 지켜야 할 규칙 |
| `docs/code-notes.md` | 코드 설계 메모 |

## 기여

빌드·테스트 명령과 배포 흐름은 `CONTRIBUTING.md`에 있습니다.
