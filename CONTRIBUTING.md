# 기여

```bash
go build -o catacombs .   # 빌드
go test ./...             # 테스트
go vet ./...              # 정적 검사
```

패키지 구성과 지켜야 할 규칙은 `ARCHITECTURE.md`에 있다.

## 배포

`master`에 push하면 GitHub Actions(`.github/workflows/ci.yml`)가 테스트를 돌리고 arm64 이미지
`ghcr.io/tolelom/catacombs:main`을 올린다. Renovate가 `server-config` 저장소의 이미지 digest를
올리면 GitOps 동기화가 서버 컨테이너를 다시 띄운다.

- `catacombs.tolelom.xyz`는 8080 포트로 이어진다. SSH 포트는 열지 않아 외부 플레이어는 웹으로만
  접속한다.
- 실행 이미지에는 바이너리만 있어 운영에서는 `config.yaml` 없이 기본값을 쓴다.
- 관리용 `/admin`은 `CATACOMBS_ADMIN_USERNAME`과 `CATACOMBS_ADMIN_PASSWORD`를 둘 다 줄 때만 열린다.
- 데이터(BoltDB, 백업, SSH 호스트 키)는 `/app/data` 볼륨에 있다.
