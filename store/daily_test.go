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

// TestSaveDailyKeepsBest verifies a worse retry on the same date does not
// overwrite the player's earlier best record.
func TestSaveDailyKeepsBest(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_daily_best.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	date := "2026-04-25"
	good := DailyRecord{Date: date, Player: "fp1", PlayerName: "Alice", FloorReached: 12, GoldEarned: 500}
	worse := DailyRecord{Date: date, Player: "fp1", PlayerName: "Alice", FloorReached: 5, GoldEarned: 100}
	if err := db.SaveDaily(good); err != nil {
		t.Fatalf("save good: %v", err)
	}
	if err := db.SaveDaily(worse); err != nil {
		t.Fatalf("save worse: %v", err)
	}
	records, err := db.GetDailyLeaderboard(date, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].FloorReached != 12 || records[0].GoldEarned != 500 {
		t.Errorf("expected best record kept, got floor=%d gold=%d", records[0].FloorReached, records[0].GoldEarned)
	}

	// A higher floor should overwrite
	better := DailyRecord{Date: date, Player: "fp1", PlayerName: "Alice", FloorReached: 18, GoldEarned: 100}
	if err := db.SaveDaily(better); err != nil {
		t.Fatalf("save better: %v", err)
	}
	records, _ = db.GetDailyLeaderboard(date, 10)
	if records[0].FloorReached != 18 {
		t.Errorf("expected better record stored, got floor=%d", records[0].FloorReached)
	}

	// Same floor, more gold should overwrite
	moreGold := DailyRecord{Date: date, Player: "fp1", PlayerName: "Alice", FloorReached: 18, GoldEarned: 999}
	if err := db.SaveDaily(moreGold); err != nil {
		t.Fatalf("save more gold: %v", err)
	}
	records, _ = db.GetDailyLeaderboard(date, 10)
	if records[0].GoldEarned != 999 {
		t.Errorf("expected gold-tiebreak to overwrite, got gold=%d", records[0].GoldEarned)
	}
}
