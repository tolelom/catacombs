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

func (d *DB) SaveDaily(record DailyRecord) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDailyRuns)
		key := []byte(record.Date + ":" + record.Player)
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
