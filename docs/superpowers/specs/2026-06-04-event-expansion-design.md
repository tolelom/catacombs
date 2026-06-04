# Event Expansion (이벤트 8→12종) + 데드코드 정리 설계

날짜: 2026-06-04
상태: 자율 진행 (사용자 위임)

## 배경

재미 진단 5종 처리로 메커닉 골격(투표/개인선택/체인/부적)이 완성됨 — 이제 콘텐츠
확장 단계. 이벤트는 현재 8종이라 반복 플레이 시 금방 소진된다. 이번에 추가된
outcome 어휘(`Item` 구체 지급, `Gambled` 부적 시너지)를 쓰는 신규 이벤트 4종을
추가한다.

부수 작업(데드코드): `NextStepLabel`은 production 호출이 없고 HUD가 더 풍부한
인라인 렌더링을 쓰므로 **삭제** (연결하면 오히려 정보 손실).

## 신규 이벤트 4종

기존 8종과 결정 프로필이 겹치지 않게 설계. 도박 선택지는 전부 `TagGamble` →
행운 부적 적용 대상.

### 1. gravekeeper (묘지기의 무덤)
무덤 도굴 리스크 — **이벤트에서 세트템을 얻는 유일한 경로**.
- 무덤 파헤치기 `[TagItem, TagHarm, TagGamble]`: 50% 세트 아이템
  (`randomSetItem`) / 50% 영혼의 분노 `-(15+floor)` HP
- 묘비 정리하기 `[TagGoldGain]`: 확정 `8+floor` G
- 명복 빌기 `[TagHeal]`: `12+floor` 회복

### 2. forge (버려진 대장간)
- 용광로 뒤지기 `[TagItem, TagHarm, TagGamble]`: 60% 고급 무기
  (bonus `5+rng.Intn(6)+floor/3` — 일반보다 +2 프리미엄) / 40% 잔불 화상
  `-(12+floor)` HP
- 고철 모아 팔기 `[TagGoldGain]`: 확정 `12+floor*2` G
- 지나가기 `[TagSkip]`

### 3. toll_bridge (트롤의 다리)
자원 전환 삼지선다 (골드 vs HP vs 도박).
- 통행료 내기 `[TagGoldCost]`: 확정 `-(15+floor)` G, 안전
- 몰래 건너기 `[TagHarm, TagGamble]`: 60% 무사 / 40% 추락 `-(20+floor)` HP
- 다리 밑 뒤지기 `[TagGoldGain, TagHarm, TagGamble]`: 50% `+(30+floor*2)` G /
  50% 트롤에게 `-(15+floor)` HP

### 4. feast (수상한 연회)
- 마음껏 먹기 `[TagHeal, TagHarm, TagGamble]`: 70% `+(25+floor*2)` 회복 /
  30% 식중독 `-(15+floor)` HP
- 음식 싸가기 `[TagItem]`: 확정 HP 포션 지급 (`Item` 구체 지급 — Bonus `30+floor`)
- 의심하며 떠나기 `[TagSkip]`

## 필요한 인프라 변경

**`applyEventOutcome`(파티 경로)에 `o.Item` 지원 추가** — 현재 파티 경로는
`ItemDrop`만 처리하고 구체 `Item`은 조용히 무시한다 (feast/forge/gravekeeper가
필요로 함). 각 생존자에게 복사본 지급, 인벤 풀이면 `bonus*5` 골드 전환 —
`applyOutcomeToPlayer`와 동일 invariant.

## 도감/로어 갱신

- `ui/codex_view.go` `allEvents`: 4종 추가 (totalEvents 8→12 자동)
- `ui/lore.go` `eventLore`: 4종 플레이버 텍스트 추가

## 데드코드 정리 (완료 항목)

- `entity/skill_tree.go` `NextStepLabel` 삭제 + nil 테스트의 해당 단언 제거.
  `NextIsBranch`는 production 사용 중이라 유지.

## 스코프 제외

- 연계기 클래스별 차별화, 부적 변형 — 별도 사이클 (플레이테스트 후)
- 이벤트 가중치/층별 출현 제한 없음 (기존 균등 추첨 유지)

## 테스트 계획

- 이벤트 12종 카운트, 신규 4종 존재 + 이름 유일성
- 기존 `TestEventChoiceTagsPresent`가 신규 태그 자동 커버
- 신규 이벤트 Resolve 시드 스캔 새너티 (도박 분기 양쪽 도달, 수치 부호 확인)
- feast 음식 싸가기 → 파티 전원 HP 포션 지급 (파티 경로 Item 지원)
- 파티 경로 인벤 풀 → bonus*5 골드 전환
- 부적 + gravekeeper 도굴 실패 → 피해 무효 (Gambled 통합 확인)
- 도감 totalEvents 12 반영
