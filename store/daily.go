package store

import (
	"encoding/json"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucketDailyRuns = []byte("daily_runs")

type DailyRecord struct {
	Date         string `json:"date"`
	Player       string `json:"player"`
	PlayerName   string `json:"player_name"`
	FloorReached int    `json:"floor_reached"`
	GoldEarned   int    `json:"gold_earned"`
}

// SaveDaily persists a daily-challenge record, preserving the player's best
// attempt for the day. A later attempt only overwrites the existing record
// when it reaches a higher floor, or the same floor with more gold. This
// matches the leaderboard sort and prevents a worse retry from clobbering
// a better score earlier in the day.
func (d *DB) SaveDaily(record DailyRecord) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		if b == nil {
			return errBucketMissing
		}
		key := []byte(record.Date + ":" + record.Player)
		if existing := b.Get(key); existing != nil {
			var prev DailyRecord
			if err := json.Unmarshal(existing, &prev); err == nil {
				if prev.FloorReached > record.FloorReached {
					return nil
				}
				if prev.FloorReached == record.FloorReached && prev.GoldEarned >= record.GoldEarned {
					return nil
				}
			}
		}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func (d *DB) GetDailyLeaderboard(date string, limit int) ([]DailyRecord, error) {
	var records []DailyRecord
	prefix := []byte(date + ":")
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var r DailyRecord
			if json.Unmarshal(v, &r) == nil {
				records = append(records, r)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].FloorReached != records[j].FloorReached {
			return records[i].FloorReached > records[j].FloorReached
		}
		return records[i].GoldEarned > records[j].GoldEarned
	})
	if len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func (d *DB) GetStreak(fingerprint, currentDate string) (int, error) {
	streak := 0
	date, err := time.Parse("2006-01-02", currentDate)
	if err != nil {
		return 0, err
	}
	err = d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		if b == nil {
			return nil
		}
		for {
			key := []byte(date.Format("2006-01-02") + ":" + fingerprint)
			if b.Get(key) == nil {
				break
			}
			streak++
			date = date.AddDate(0, 0, -1)
		}
		return nil
	})
	return streak, err
}
