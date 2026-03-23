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
