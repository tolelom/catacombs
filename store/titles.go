package store

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

var bucketTitles = []byte("titles")

type TitleDef struct {
	ID          string
	Name        string
	Description string
}

var TitleDefs = []TitleDef{
	{ID: "novice", Name: "Novice", Description: "Default title for new players"},
	{ID: "explorer", Name: "Explorer", Description: "Reach floor 5"},
	{ID: "veteran", Name: "Veteran", Description: "Reach floor 10"},
	{ID: "champion", Name: "Champion", Description: "Reach floor 20"},
	{ID: "gold_king", Name: "Gold King", Description: "Accumulate 500+ gold in one run"},
	{ID: "team_player", Name: "Team Player", Description: "Complete 5 multiplayer runs"},
	{ID: "survivor", Name: "Survivor", Description: "Complete a run without dying"},
}

type PlayerTitleData struct {
	ActiveTitle string   `json:"active_title"`
	Earned      []string `json:"earned"`
}

func (d *DB) EarnTitle(fingerprint, titleID string) (bool, error) {
	newlyEarned := false
	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTitles)
		if b == nil {
			return errBucketMissing
		}
		key := []byte(fingerprint)
		var data PlayerTitleData

		v := b.Get(key)
		if v != nil {
			if err := json.Unmarshal(v, &data); err != nil {
				return err
			}
		}

		// Check if already earned
		for _, e := range data.Earned {
			if e == titleID {
				return nil
			}
		}

		newlyEarned = true
		data.Earned = append(data.Earned, titleID)

		// Auto-set first earned title as active
		if data.ActiveTitle == "" {
			data.ActiveTitle = titleID
		}

		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return b.Put(key, encoded)
	})
	return newlyEarned, err
}

func (d *DB) SetActiveTitle(fingerprint, titleID string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTitles)
		if b == nil {
			return errBucketMissing
		}
		key := []byte(fingerprint)
		var data PlayerTitleData

		v := b.Get(key)
		if v != nil {
			if err := json.Unmarshal(v, &data); err != nil {
				return err
			}
		}

		// Verify title is earned
		found := false
		for _, e := range data.Earned {
			if e == titleID {
				found = true
				break
			}
		}
		if !found {
			return nil
		}

		data.ActiveTitle = titleID
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return b.Put(key, encoded)
	})
}

func (d *DB) GetTitleData(fingerprint string) (PlayerTitleData, error) {
	var data PlayerTitleData
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTitles)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(fingerprint))
		if v == nil {
			return nil
		}
		return json.Unmarshal(v, &data)
	})
	return data, err
}
