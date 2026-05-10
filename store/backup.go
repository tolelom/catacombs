package store

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Backup creates a consistent snapshot of the database in destDir.
// Returns the path to the backup file.
func (d *DB) Backup(destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("catacombs-%s.db", timestamp)
	destPath := filepath.Join(destDir, filename)

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}

	// BoltDB View transaction provides a consistent snapshot
	err = d.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(f)
		return err
	})
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("close backup file: %w", closeErr)
	}
	if err != nil {
		_ = os.Remove(destPath)
		return "", fmt.Errorf("backup write: %w", err)
	}

	return destPath, nil
}

// PruneBackups deletes the oldest catacombs-*.db files in destDir until at
// most keep entries remain. Returns the number of files deleted. A long
// running deployment that runs the scheduled backup hourly otherwise grows
// the backup directory unboundedly.
func PruneBackups(destDir string, keep int) (int, error) {
	if keep < 1 {
		keep = 1
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	type fileInfo struct {
		name string
		mod  time.Time
	}
	var backups []fileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "catacombs-") || !strings.HasSuffix(name, ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, fileInfo{name: name, mod: info.ModTime()})
	}
	if len(backups) <= keep {
		return 0, nil
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].mod.Before(backups[j].mod)
	})
	deleted := 0
	for i := 0; i < len(backups)-keep; i++ {
		path := filepath.Join(destDir, backups[i].name)
		if err := os.Remove(path); err != nil {
			// Don't abort prune on a single failure (perm issue, file in
			// use); log and keep going so the rest of the rotation works.
			slog.Warn("backup prune remove failed", "path", path, "error", err)
			continue
		}
		deleted++
	}
	return deleted, nil
}
