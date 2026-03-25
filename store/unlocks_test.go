package store

import (
	"testing"
)

func TestUnlockContent(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_unlocks.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Initially not unlocked
	if db.IsUnlocked("fp1", "fifth_class") {
		t.Error("should not be unlocked initially")
	}

	// Unlock returns true for new unlock
	newlyUnlocked, err := db.UnlockContent("fp1", "fifth_class")
	if err != nil {
		t.Fatal(err)
	}
	if !newlyUnlocked {
		t.Error("should be newly unlocked")
	}

	// Now it should be unlocked
	if !db.IsUnlocked("fp1", "fifth_class") {
		t.Error("should be unlocked after UnlockContent")
	}

	// Second unlock returns false (already unlocked)
	newlyUnlocked2, err := db.UnlockContent("fp1", "fifth_class")
	if err != nil {
		t.Fatal(err)
	}
	if newlyUnlocked2 {
		t.Error("should not be newly unlocked on second call")
	}

	// Different player still not unlocked
	if db.IsUnlocked("fp2", "fifth_class") {
		t.Error("different player should not have unlock")
	}
}
