package store

import (
	"testing"
)

func TestSaveAndGetDaily(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_daily.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	_ = db.SaveDaily(DailyRecord{Date: "2026-03-25", Player: "fp1", PlayerName: "Alice", FloorReached: 10, GoldEarned: 200})
	_ = db.SaveDaily(DailyRecord{Date: "2026-03-25", Player: "fp2", PlayerName: "Bob", FloorReached: 15, GoldEarned: 100})
	_ = db.SaveDaily(DailyRecord{Date: "2026-03-25", Player: "fp3", PlayerName: "Charlie", FloorReached: 15, GoldEarned: 300})

	records, err := db.GetDailyLeaderboard("2026-03-25", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	// Charlie and Bob both floor 15, Charlie has more gold so first
	if records[0].PlayerName != "Charlie" {
		t.Errorf("expected Charlie first, got %s", records[0].PlayerName)
	}
	if records[1].PlayerName != "Bob" {
		t.Errorf("expected Bob second, got %s", records[1].PlayerName)
	}
	if records[2].PlayerName != "Alice" {
		t.Errorf("expected Alice third, got %s", records[2].PlayerName)
	}
}

func TestDailyStreak(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_streak.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	_ = db.SaveDaily(DailyRecord{Date: "2026-03-23", Player: "fp1", PlayerName: "Alice", FloorReached: 5, GoldEarned: 50})
	_ = db.SaveDaily(DailyRecord{Date: "2026-03-24", Player: "fp1", PlayerName: "Alice", FloorReached: 8, GoldEarned: 100})
	_ = db.SaveDaily(DailyRecord{Date: "2026-03-25", Player: "fp1", PlayerName: "Alice", FloorReached: 10, GoldEarned: 200})

	streak, err := db.GetStreak("fp1", "2026-03-25")
	if err != nil {
		t.Fatal(err)
	}
	if streak != 3 {
		t.Errorf("expected streak 3, got %d", streak)
	}

	// Gap in streak
	streak2, err := db.GetStreak("fp1", "2026-03-27")
	if err != nil {
		t.Fatal(err)
	}
	if streak2 != 0 {
		t.Errorf("expected streak 0 after gap, got %d", streak2)
	}
}
