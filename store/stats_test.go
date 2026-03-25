package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGetTodayRunCount(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	today := time.Now().Format("2006-01-02")

	count, err := db.GetTodayRunCount()
	if err != nil {
		t.Fatalf("GetTodayRunCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	db.SaveDaily(DailyRecord{Date: today, Player: "fp1", PlayerName: "A", FloorReached: 10, GoldEarned: 100})
	db.SaveDaily(DailyRecord{Date: today, Player: "fp2", PlayerName: "B", FloorReached: 15, GoldEarned: 200})
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	db.SaveDaily(DailyRecord{Date: yesterday, Player: "fp3", PlayerName: "C", FloorReached: 5, GoldEarned: 50})

	count, err = db.GetTodayRunCount()
	if err != nil {
		t.Fatalf("GetTodayRunCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestGetTodayAvgFloor(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	today := time.Now().Format("2006-01-02")

	avg, err := db.GetTodayAvgFloor()
	if err != nil {
		t.Fatalf("GetTodayAvgFloor: %v", err)
	}
	if avg != 0 {
		t.Fatalf("expected 0, got %f", avg)
	}

	db.SaveDaily(DailyRecord{Date: today, Player: "fp1", PlayerName: "A", FloorReached: 10, GoldEarned: 100})
	db.SaveDaily(DailyRecord{Date: today, Player: "fp2", PlayerName: "B", FloorReached: 20, GoldEarned: 200})

	avg, err = db.GetTodayAvgFloor()
	if err != nil {
		t.Fatalf("GetTodayAvgFloor: %v", err)
	}
	if avg != 15.0 {
		t.Fatalf("expected 15.0, got %f", avg)
	}
}
