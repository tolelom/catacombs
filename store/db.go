package store

import (
	"encoding/json"
	"fmt"
	"sort"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketProfiles = []byte("profiles")
	bucketRankings = []byte("rankings")
)

type DB struct {
	db *bolt.DB
}

type RunRecord struct {
	Player  string   `json:"player"`
	Floor   int      `json:"floor"`
	Score   int      `json:"score"`
	Class   string   `json:"class,omitempty"`
	Members []string `json:"members,omitempty"` // party member names (empty for solo)
}

func Open(path string) (*DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketProfiles); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketRankings); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketAchievements); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketDailyRuns); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketUnlocks); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketTitles); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketCodex); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketPasswords); err != nil {
			return err
		}
		return nil
	})
	return &DB{db: db}, err
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) SaveProfile(fingerprint, name string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProfiles).Put([]byte(fingerprint), []byte(name))
	})
}

func (d *DB) GetProfile(fingerprint string) (string, error) {
	var name string
	err := d.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketProfiles).Get([]byte(fingerprint))
		if v == nil {
			return fmt.Errorf("profile not found")
		}
		name = string(v)
		return nil
	})
	return name, err
}

func (d *DB) SaveRun(player string, floor, score int, class string, members []string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRankings)
		id, _ := b.NextSequence()
		record := RunRecord{Player: player, Floor: floor, Score: score, Class: class, Members: members}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put([]byte(fmt.Sprintf("%010d", id)), data)
	})
}

func (d *DB) TopRunsByGold(limit int) ([]RunRecord, error) {
	var runs []RunRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRankings)
		return b.ForEach(func(k, v []byte) error {
			var r RunRecord
			if json.Unmarshal(v, &r) == nil {
				runs = append(runs, r)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Score > runs[j].Score
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

type PlayerStats struct {
	TotalRuns  int
	BestFloor  int
	TotalGold  int
	TotalKills int
	Victories  int
}

func (d *DB) GetStats(player string) (PlayerStats, error) {
	var stats PlayerStats
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRankings)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var r RunRecord
			if json.Unmarshal(v, &r) == nil && r.Player == player {
				stats.TotalRuns++
				if r.Floor > stats.BestFloor {
					stats.BestFloor = r.Floor
				}
				stats.TotalGold += r.Score
				if r.Floor >= 20 {
					stats.Victories++
				}
			}
			return nil
		})
	})
	return stats, err
}

func (d *DB) TopRuns(limit int) ([]RunRecord, error) {
	var runs []RunRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRankings)
		return b.ForEach(func(k, v []byte) error {
			var r RunRecord
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			runs = append(runs, r)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Floor != runs[j].Floor {
			return runs[i].Floor > runs[j].Floor
		}
		return runs[i].Score > runs[j].Score
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}
