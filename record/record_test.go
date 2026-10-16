package record

import (
	"path/filepath"
	"testing"

	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

func openStore(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "record.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func member(name, fp string, gold int) *entity.Player {
	p := entity.NewPlayer(name, entity.ClassWarrior)
	p.Fingerprint = fp
	p.Gold = gold
	return p
}

func unlocked(t *testing.T, db *store.DB, fp, id string) bool {
	t.Helper()
	list, err := db.GetAchievements(fp)
	if err != nil {
		t.Fatalf("achievements: %v", err)
	}
	for _, a := range list {
		if a.ID == id {
			return a.Unlocked
		}
	}
	return false
}

func TestEveryMemberGetsTheirRunRecorded(t *testing.T) {
	db := openStore(t)
	result := game.RunResult{State: game.GameState{
		FloorNum: 6,
		Players:  []*entity.Player{member("alice", "fp-a", 120), member("bob", "fp-b", 30)},
	}}

	New(db).RecordRun(result)

	runs, err := db.TopRuns(10)
	if err != nil {
		t.Fatalf("top runs: %v", err)
	}
	names := map[string]bool{}
	for _, r := range runs {
		names[r.Player] = true
	}
	if !names["alice"] || !names["bob"] {
		t.Fatalf("runs recorded for %v, want alice and bob", names)
	}
	for _, fp := range []string{"fp-a", "fp-b"} {
		if !unlocked(t, db, fp, "first_clear") {
			t.Errorf("%s did not get first_clear for reaching floor 6", fp)
		}
	}
	if souls, _ := db.GetSouls("fp-a"); souls != 12 {
		t.Errorf("alice souls: got %d, want 12 (10%% of 120 gold)", souls)
	}
}

func TestPersonalAchievementsFollowEachMember(t *testing.T) {
	db := openStore(t)
	result := game.RunResult{State: game.GameState{
		FloorNum: 2,
		Players:  []*entity.Player{member("rich", "fp-rich", 250), member("poor", "fp-poor", 10)},
	}}

	New(db).RecordRun(result)

	if !unlocked(t, db, "fp-rich", "gold_hoarder") {
		t.Error("member with 250 gold did not get gold_hoarder")
	}
	if unlocked(t, db, "fp-poor", "gold_hoarder") {
		t.Error("member with 10 gold got gold_hoarder")
	}
}

func TestDailyRunIsSavedForEveryMember(t *testing.T) {
	db := openStore(t)
	result := game.RunResult{
		State: game.GameState{
			FloorNum: 4,
			Players:  []*entity.Player{member("solo", "fp-solo", 77)},
		},
		DailyMode: true,
		DailyDate: "2026-10-16",
	}

	New(db).RecordRun(result)

	daily, err := db.GetDailyLeaderboard("2026-10-16", 10)
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if len(daily) != 1 || daily[0].Player != "fp-solo" || daily[0].GoldEarned != 77 || daily[0].FloorReached != 4 {
		t.Fatalf("daily records: %+v", daily)
	}
}
