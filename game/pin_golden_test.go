package game

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

// 리팩터링 전 동작을 고정하는 골든 트레이스 공용 도구.
// 고정 시드로 시나리오를 돌린 뒤 상태 전체와 "다음 난수 값"을 기록한다.
// 다음 난수 값이 같으면 RNG 소비 횟수와 순서도 같다는 뜻이다.
// 골든 갱신: go test ./game/ -run Pin -update

var updateGolden = flag.Bool("update", false, "rewrite golden files")

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
	if got == wantText {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(wantText, "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("golden %s differs at line %d\n got: %q\nwant: %q", path, i+1, g, w)
		}
	}
}

type traceWriter struct {
	sb strings.Builder
}

func (tw *traceWriter) line(format string, args ...any) {
	fmt.Fprintf(&tw.sb, format, args...)
	tw.sb.WriteByte('\n')
}

func (tw *traceWriter) session(s *GameSession) {
	for _, p := range s.state.Players {
		tw.line("  player %s class=%v hp=%d/%d atk=%d def=%d gold=%d dead=%v fled=%v freezeImmune=%d effects=%v",
			p.Name, p.Class, p.HP, p.MaxHP, p.ATK, p.DEF, p.Gold, p.Dead, p.Fled, p.FreezeImmune, p.Effects)
		for _, it := range p.Inventory {
			tw.line("    item %+v", it)
		}
	}
	for i, m := range s.state.Monsters {
		tw.line("  monster[%d] %s hp=%d/%d atk=%d def=%d boss=%v mini=%v elite=%v prefix=%v taunt=%v/%d pattern=%v phase=%d transitioned=%v vulnerable=%v effects=%v",
			i, m.Name, m.HP, m.MaxHP, m.ATK, m.DEF, m.IsBoss, m.IsMiniBoss, m.IsElite, m.ElitePrefix,
			m.TauntTarget, m.TauntTurns, m.Pattern, m.Phase, m.PhaseTransitioned, m.Vulnerable, m.Effects)
	}
	tw.line("  phase=%v gameOver=%v anyDied=%v bossPhase2=%v", s.state.Phase, s.state.GameOver, s.state.AnyPlayerDied, s.state.BossPhase2Reached)
	for _, l := range s.state.PendingLogs {
		tw.line("  pending %s", l)
	}
	for _, l := range s.state.CombatLog {
		tw.line("  log %s", l)
	}
	tw.line("  nextRand=%d", s.rng.Int63())
}

func pinSession(t *testing.T, seed int64, classes ...entity.Class) *GameSession {
	t.Helper()
	s := setupCombatCoop(t, classes...)
	s.rng = rand.New(rand.NewSource(seed))
	s.state.Monsters = nil
	s.state.PendingLogs = nil
	s.state.CombatLog = nil
	s.state.TurnResolving = true
	return s
}
