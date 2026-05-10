package store

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

var bucketCodex = []byte("codex")

type Codex struct {
	Monsters map[string]bool `json:"monsters"`
	Items    map[string]bool `json:"items"`
	Events   map[string]bool `json:"events"`
}

func newCodex() Codex {
	return Codex{
		Monsters: make(map[string]bool),
		Items:    make(map[string]bool),
		Events:   make(map[string]bool),
	}
}

func (d *DB) RecordCodexEntry(fingerprint, category, id string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCodex)
		if b == nil {
			return errBucketMissing
		}
		key := []byte(fingerprint)
		codex := newCodex()

		v := b.Get(key)
		if v != nil {
			if err := json.Unmarshal(v, &codex); err != nil {
				return err
			}
			// Ensure maps are initialized after unmarshal
			if codex.Monsters == nil {
				codex.Monsters = make(map[string]bool)
			}
			if codex.Items == nil {
				codex.Items = make(map[string]bool)
			}
			if codex.Events == nil {
				codex.Events = make(map[string]bool)
			}
		}

		switch category {
		case "monster":
			codex.Monsters[id] = true
		case "item":
			codex.Items[id] = true
		case "event":
			codex.Events[id] = true
		}

		data, err := json.Marshal(codex)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func (d *DB) GetCodex(fingerprint string) (Codex, error) {
	codex := newCodex()
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCodex)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(fingerprint))
		if v == nil {
			return nil
		}
		if err := json.Unmarshal(v, &codex); err != nil {
			return err
		}
		if codex.Monsters == nil {
			codex.Monsters = make(map[string]bool)
		}
		if codex.Items == nil {
			codex.Items = make(map[string]bool)
		}
		if codex.Events == nil {
			codex.Events = make(map[string]bool)
		}
		return nil
	})
	return codex, err
}
