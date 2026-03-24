package store

import (
	"os"
	"testing"
)

func TestPlayerProfile(t *testing.T) {
	db, err := Open("test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Close()
		os.Remove("test.db")
	}()

	err = db.SaveProfile("SHA256:abc123", "TestPlayer")
	if err != nil {
		t.Fatal(err)
	}
	name, err := db.GetProfile("SHA256:abc123")
	if err != nil {
		t.Fatal(err)
	}
	if name != "TestPlayer" {
		t.Errorf("Name: got %q, want %q", name, "TestPlayer")
	}
}

func TestRanking(t *testing.T) {
	db, err := Open("test_rank.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Close()
		os.Remove("test_rank.db")
	}()

	db.SaveRun("Alice", 20, 1500)
	db.SaveRun("Bob", 15, 1000)
	db.SaveRun("Charlie", 20, 2000)

	rankings, err := db.TopRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rankings) != 3 {
		t.Errorf("Rankings: got %d, want 3", len(rankings))
	}
	if rankings[0].Player != "Charlie" {
		t.Errorf("Top player: got %q, want Charlie", rankings[0].Player)
	}
}

func TestGetStats(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Save some runs
	db.SaveRun("Alice", 5, 100)
	db.SaveRun("Alice", 10, 250)
	db.SaveRun("Alice", 20, 500) // victory (floor >= 20)
	db.SaveRun("Bob", 3, 50)

	stats, err := db.GetStats("Alice")
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRuns != 3 {
		t.Errorf("expected 3 total runs, got %d", stats.TotalRuns)
	}
	if stats.BestFloor != 20 {
		t.Errorf("expected best floor 20, got %d", stats.BestFloor)
	}
	if stats.Victories != 1 {
		t.Errorf("expected 1 victory, got %d", stats.Victories)
	}
	if stats.TotalGold != 850 { // 100+250+500
		t.Errorf("expected total gold 850, got %d", stats.TotalGold)
	}

	// Bob's stats should be separate
	bobStats, _ := db.GetStats("Bob")
	if bobStats.TotalRuns != 1 {
		t.Errorf("Bob should have 1 run, got %d", bobStats.TotalRuns)
	}
}
