package ui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

// 1c. HUD 렌더 결과를 골든 파일로 고정한다.
// lipgloss는 출력 대상의 색 지원을 자동 감지하므로 프로필을 테스트에서 직접 정한다.
// Ascii는 레이아웃과 문구를, TrueColor는 색 코드까지 고정한다.
// 패널 폭은 코드에 고정돼 있고(35/38) 터미널 폭을 읽지 않는다.
// 프로필이 패키지 전역이라 이 테스트는 t.Parallel을 쓰지 않는다.
// 골든 갱신: go test ./ui/ -run Pin -update

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

func withColorProfile(t *testing.T, p termenv.Profile) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(p)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}

func hudPlayer(name, fp string, class entity.Class) *entity.Player {
	p := entity.NewPlayer(name, class)
	p.Fingerprint = fp
	return p
}

func hudParty() []*entity.Player {
	w := hudPlayer("Warrior", "fp-w", entity.ClassWarrior)
	w.Gold = 120
	w.Inventory = []entity.Item{{Name: "Sword", Type: entity.ItemWeapon, Bonus: 5}}
	w.Effects = []entity.ActiveEffect{{Type: entity.StatusPoison, Duration: 2, Value: 5}, {Type: entity.StatusBurn, Duration: 1, Value: 8}}
	w.Relics = []entity.Relic{{Name: "연막탄", Effect: entity.RelicActiveSmokeBomb}, {Name: "시간의 모래시계", Effect: entity.RelicActiveTimeGlass}}
	h := hudPlayer("Healer", "fp-h", entity.ClassHealer)
	h.HP = h.MaxHP / 3
	h.Effects = []entity.ActiveEffect{{Type: entity.StatusFreeze, Duration: 1}}
	h.Relics = []entity.Relic{{Name: "피의 돌", Effect: entity.RelicActiveBloodstone}, {Name: "공격의 반지", Effect: entity.RelicATKBoost}}
	m := hudPlayer("Mage", "fp-m", entity.ClassMage)
	m.HP = 0
	m.Dead = true
	r := hudPlayer("Rogue", "fp-r", entity.ClassRogue)
	r.HP = r.MaxHP / 5
	r.Fled = true
	return []*entity.Player{w, h, m, r}
}

func hudMonsters() []*entity.Monster {
	return []*entity.Monster{
		{Name: "Slime", Type: entity.MonsterSlime, HP: 30, MaxHP: 30},
		{Name: "Dead", HP: 0, MaxHP: 10},
		{Name: "Boss", Type: entity.MonsterSlime, HP: 20, MaxHP: 100, TauntTarget: true, TauntTurns: 2},
	}
}

func hudFloor() *dungeon.Floor {
	return &dungeon.Floor{
		Number:      3,
		CurrentRoom: 0,
		Rooms: []*dungeon.Room{
			{Type: dungeon.RoomEmpty, Neighbors: []int{1, 2, 3, 9}},
			{Type: dungeon.RoomCombat},
			{Type: dungeon.RoomShop, Cleared: true},
			{Type: dungeon.RoomBoss},
		},
	}
}

func combatStates() map[string]game.GameState {
	base := func() game.GameState {
		return game.GameState{
			Phase:            game.PhaseCombat,
			Players:          hudParty(),
			Monsters:         hudMonsters(),
			SubmittedActions: map[string]string{"fp-w": "공격"},
			CoopAvailable:    map[string]bool{"fp-w": true, "fp-h": false},
		}
	}
	states := map[string]game.GameState{}
	states["party"] = base()

	timer := base()
	timer.TurnDeadline = time.Unix(1, 0)
	timer.SoloChain = &game.SoloChainView{LastAction: "attack", AttackStreak: 2}
	states["expired timer and chain"] = timer

	noCoop := base()
	noCoop.CoopAvailable = nil
	states["coop map missing"] = noCoop

	for _, streak := range []int{0, 1, 3} {
		st := base()
		st.SoloMode = true
		st.Players = st.Players[:1]
		st.SoloChain = &game.SoloChainView{LastAction: "attack", AttackStreak: streak}
		states[fmt.Sprintf("solo streak %d", streak)] = st
	}
	return states
}

func exploringStates() map[string]game.GameState {
	base := func() game.GameState {
		return game.GameState{
			Phase:         game.PhaseExploring,
			Players:       hudParty(),
			Floor:         hudFloor(),
			MoveVotes:     map[string]int{"fp-h": 1},
			CoopAvailable: map[string]bool{"fp-r": true},
		}
	}
	states := map[string]game.GameState{}
	states["party"] = base()

	voted := base()
	voted.MoveVotes = map[string]int{"fp-w": 2, "fp-h": 2}
	states["voted"] = voted

	solo := base()
	solo.SoloMode = true
	solo.Players = solo.Players[:1]
	solo.MoveVotes = nil
	states["solo"] = solo

	preview := base()
	preview.NextRoomPreview = &game.RoomPreview{RoomIdx: 1, RoomTypeLabel: "전투", MonsterNames: []string{"Slime", "Bat"}}
	states["preview"] = preview

	hidden := base()
	hidden.NextRoomPreview = &game.RoomPreview{RoomIdx: 1, RoomTypeLabel: "보스", MonsterNames: []string{"Boss"}, HideMonsters: true}
	states["preview hidden"] = hidden

	noFloor := base()
	noFloor.Floor = nil
	states["no floor"] = noFloor

	for _, sk := range []entity.PlayerSkills{
		{BranchIndex: -1, Points: 1, Allocated: 0},
		{BranchIndex: 0, Points: 2, Allocated: 1},
		{BranchIndex: 1, Points: 3, Allocated: 2},
		{BranchIndex: 0, Points: 4, Allocated: 3},
		{BranchIndex: 1, Points: 5, Allocated: 4},
	} {
		sk := sk
		st := base()
		st.Players[0].Skills = &sk
		states[fmt.Sprintf("skills branch=%d points=%d allocated=%d", sk.BranchIndex, sk.Points, sk.Allocated)] = st
	}
	return states
}

func sortedKeys(m map[string]game.GameState) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func renderHUDTrace() string {
	var sb strings.Builder
	viewers := []string{"fp-w", "fp-h", "fp-r", "fp-x"}
	combat := combatStates()
	for _, name := range sortedKeys(combat) {
		for _, fp := range viewers {
			for _, cursor := range []int{0, 2} {
				fmt.Fprintf(&sb, "=== combat %s viewer=%s cursor=%d\n", name, fp, cursor)
				sb.WriteString(renderHUD(combat[name], cursor, cursor, cursor, cursor, fp))
				sb.WriteString("\n")
			}
		}
	}
	exploring := exploringStates()
	for _, name := range sortedKeys(exploring) {
		for _, fp := range viewers {
			for _, cursor := range []int{0, 1, 5} {
				fmt.Fprintf(&sb, "=== exploring %s viewer=%s cursor=%d\n", name, fp, cursor)
				sb.WriteString(renderHUD(exploring[name], cursor, cursor, cursor, cursor, fp))
				sb.WriteString("\n")
			}
		}
	}
	for _, hp := range [][2]int{{0, 0}, {-5, 10}, {15, 10}, {6, 10}, {3, 10}, {2, 10}} {
		fmt.Fprintf(&sb, "=== hpbar %d/%d\n%s\n", hp[0], hp[1], renderHPBar(hp[0], hp[1], 10))
	}
	return sb.String()
}

func TestPinHUDAscii(t *testing.T) {
	withColorProfile(t, termenv.Ascii)
	checkGolden(t, "hud_ascii", renderHUDTrace())
}

func TestPinHUDTrueColor(t *testing.T) {
	withColorProfile(t, termenv.TrueColor)
	checkGolden(t, "hud_truecolor", renderHUDTrace())
}
