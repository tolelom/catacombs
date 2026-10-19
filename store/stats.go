package store

import (
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

func (d *DB) GetTodayRunCount() (int, error) {
	today := time.Now().Format("2006-01-02")
	count := 0
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(today + ":")
		for k, _ := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			count++
		}
		return nil
	})
	return count, err
}

func (d *DB) GetTodayAvgFloor() (float64, error) {
	today := time.Now().Format("2006-01-02")
	total := 0
	count := 0
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(today + ":")
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var r DailyRecord
			if json.Unmarshal(v, &r) == nil {
				total += r.FloorReached
				count++
			}
		}
		return nil
	})
	if count == 0 {
		return 0, err
	}
	return float64(total) / float64(count), err
}
