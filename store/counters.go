package store

import (
	"encoding/binary"

	bolt "go.etcd.io/bbolt"
)

var bucketCounters = []byte("counters")

// EnsureCountersBucket creates the counters bucket if missing.
func (d *DB) EnsureCountersBucket() error {
	return d.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketCounters)
		return err
	})
}

// IncCounter atomically adds delta to a per-fingerprint named counter.
func (d *DB) IncCounter(fingerprint, name string, delta int64) error {
	if delta == 0 {
		return nil
	}
	key := []byte(fingerprint + ":" + name)
	return d.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketCounters)
		if err != nil {
			return err
		}
		var cur int64
		if v := b.Get(key); v != nil && len(v) == 8 {
			cur = int64(binary.BigEndian.Uint64(v))
		}
		cur += delta
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(cur))
		return b.Put(key, buf)
	})
}

// GetCounter returns the current value of a per-fingerprint counter.
func (d *DB) GetCounter(fingerprint, name string) (int64, error) {
	var n int64
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCounters)
		if b == nil {
			return nil
		}
		key := []byte(fingerprint + ":" + name)
		if v := b.Get(key); v != nil && len(v) == 8 {
			n = int64(binary.BigEndian.Uint64(v))
		}
		return nil
	})
	return n, err
}
