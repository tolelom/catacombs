package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

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
	defer backupDB.Close()

	name, err := backupDB.GetProfile("fp1")
	if err != nil {
		t.Fatalf("failed to read from backup: %v", err)
	}
	if name != "player1" {
		t.Fatalf("expected player1, got %s", name)
	}
}

func TestBackupCreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	backupDir := filepath.Join(tmpDir, "nested", "backups")
	_, err = db.Backup(backupDir)
	if err != nil {
		t.Fatalf("backup with nested dir failed: %v", err)
	}
}
