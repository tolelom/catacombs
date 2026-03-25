package store

import (
	"fmt"
	"os"
	"path/filepath"
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
	defer f.Close()

	// BoltDB View transaction provides a consistent snapshot
	err = d.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(f)
		return err
	})
	if err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("backup write: %w", err)
	}

	return destPath, nil
}
