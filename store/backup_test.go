package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.SaveProfile("fp1", "player1"); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	backupDir := filepath.Join(tmpDir, "backups")

	backupPath, err := db.Backup(backupDir)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("backup file does not exist")
	}

	base := filepath.Base(backupPath)
	if !strings.HasPrefix(base, "catacombs-") || !strings.HasSuffix(base, ".db") {
		t.Fatalf("unexpected backup filename: %s", base)
	}

	backupDB, err := Open(backupPath)
	if err != nil {
		t.Fatalf("failed to open backup: %v", err)
	}
	defer func() { _ = backupDB.Close() }()

	name, err := backupDB.GetProfile("fp1")
	if err != nil {
		t.Fatalf("failed to read from backup: %v", err)
	}
	if name != "player1" {
		t.Fatalf("expected player1, got %s", name)
	}
}

// TestPruneBackupsKeepsNewest verifies that PruneBackups removes only the
// oldest catacombs-*.db files past the keep limit. Without this the
// backup dir would grow unboundedly across a long deployment.
func TestPruneBackupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()

	// Create 5 fake backup files with staggered modification times.
	names := []string{
		"catacombs-20260101-000000.db",
		"catacombs-20260102-000000.db",
		"catacombs-20260103-000000.db",
		"catacombs-20260104-000000.db",
		"catacombs-20260105-000000.db",
	}
	base := time.Now().Add(-5 * time.Hour)
	for i, n := range names {
		full := filepath.Join(dir, n)
		if err := os.WriteFile(full, []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(full, base.Add(time.Duration(i)*time.Hour), base.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	// Add an unrelated file that should NOT be pruned.
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(other, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	deleted, err := PruneBackups(dir, 2)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}

	// The two newest backups and the unrelated file should remain.
	survivors, _ := os.ReadDir(dir)
	got := make(map[string]bool, len(survivors))
	for _, s := range survivors {
		got[s.Name()] = true
	}
	for _, want := range []string{"catacombs-20260104-000000.db", "catacombs-20260105-000000.db", "notes.txt"} {
		if !got[want] {
			t.Errorf("expected %q to survive, missing", want)
		}
	}
	if got["catacombs-20260101-000000.db"] {
		t.Error("oldest backup should have been deleted")
	}
}

func TestPruneBackupsMissingDir(t *testing.T) {
	deleted, err := PruneBackups(filepath.Join(t.TempDir(), "nonexistent"), 5)
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted on missing dir, got %d", deleted)
	}
}

func TestBackupCreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	backupDir := filepath.Join(tmpDir, "nested", "backups")
	_, err = db.Backup(backupDir)
	if err != nil {
		t.Fatalf("backup with nested dir failed: %v", err)
	}
}
