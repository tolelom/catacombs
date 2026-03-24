package store

import (
	bolt "go.etcd.io/bbolt"
)

var bucketAchievements = []byte("achievements")

type Achievement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Unlocked    bool   `json:"unlocked"`
}

var AchievementDefs = []Achievement{
	{ID: "first_clear", Name: "Dungeon Delver", Description: "Clear floor 5 for the first time"},
	{ID: "boss_slayer", Name: "Boss Slayer", Description: "Defeat any boss"},
	{ID: "floor10", Name: "Deep Explorer", Description: "Reach floor 10"},
	{ID: "floor20", Name: "Conqueror", Description: "Conquer the Catacombs (floor 20)"},
	{ID: "solo_clear", Name: "Lone Wolf", Description: "Clear floor 5 solo"},
	{ID: "gold_hoarder", Name: "Gold Hoarder", Description: "Accumulate 200+ gold in one run"},
	{ID: "no_death", Name: "Untouchable", Description: "Complete a floor without anyone dying"},
	{ID: "full_party", Name: "Fellowship", Description: "Start a game with 4 players"},
	{ID: "relic_collector", Name: "Relic Collector", Description: "Collect 3+ relics in one run"},
	{ID: "flee_master", Name: "Tactical Retreat", Description: "Successfully flee from combat"},
}

func (d *DB) initAchievements() error {
	return d.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketAchievements)
		return err
	})
}

func (d *DB) UnlockAchievement(player, achievementID string) (bool, error) {
	key := []byte(player + ":" + achievementID)
	alreadyUnlocked := false
	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAchievements)
		if b.Get(key) != nil {
			alreadyUnlocked = true
			return nil
		}
		return b.Put(key, []byte("1"))
	})
	// Returns true if newly unlocked (not already had it)
	return !alreadyUnlocked, err
}

func (d *DB) GetAchievements(player string) ([]Achievement, error) {
	unlocked := make(map[string]bool)
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAchievements)
		if b == nil {
			return nil
		}
		for _, a := range AchievementDefs {
			key := []byte(player + ":" + a.ID)
			if b.Get(key) != nil {
				unlocked[a.ID] = true
			}
		}
		return nil
	})
	result := make([]Achievement, len(AchievementDefs))
	for i, a := range AchievementDefs {
		result[i] = a
		result[i].Unlocked = unlocked[a.ID]
	}
	return result, err
}
