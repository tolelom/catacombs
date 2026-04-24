package store

import (
	bolt "go.etcd.io/bbolt"
)

var bucketUnlocks = []byte("unlocks")

type UnlockDef struct {
	ID          string
	Name        string
	Description string
	Condition   string
}

var UnlockDefs = []UnlockDef{
	{ID: "fifth_class", Name: "Fifth Class", Description: "Reach floor 10 or higher", Condition: "floor 10+"},
	{ID: "hard_mode", Name: "Hard Mode", Description: "Clear the game with 3+ players", Condition: "3+ player clear"},
	{ID: "mutations", Name: "Mutations", Description: "Achieve victory on floor 20", Condition: "floor 20 victory"},
}

func (d *DB) UnlockContent(fingerprint, unlockID string) (bool, error) {
	key := []byte(fingerprint + ":" + unlockID)
	alreadyUnlocked := false
	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUnlocks)
		if b.Get(key) != nil {
			alreadyUnlocked = true
			return nil
		}
		return b.Put(key, []byte("1"))
	})
	return !alreadyUnlocked, err
}

func (d *DB) IsUnlocked(fingerprint, unlockID string) bool {
	key := []byte(fingerprint + ":" + unlockID)
	unlocked := false
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUnlocks)
		if b.Get(key) != nil {
			unlocked = true
		}
		return nil
	})
	if err != nil {
		return false
	}
	return unlocked
}
