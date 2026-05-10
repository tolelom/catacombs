package store

import (
	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

var bucketPasswords = []byte("passwords")

// dummyHash is a bcrypt hash run against the unknown-user code path so a
// timing observer can't distinguish "user does not exist" (instant return)
// from "user exists, password mismatch" (slow bcrypt). Computed once at
// startup; if generation somehow fails (extremely unlikely) the timing leak
// remains but the rest of auth still works.
var dummyHash []byte

func init() {
	if h, err := bcrypt.GenerateFromPassword([]byte("dummy-never-matches"), bcrypt.DefaultCost); err == nil {
		dummyHash = h
	}
}

// SavePassword stores a bcrypt-hashed password for the given nickname.
func (d *DB) SavePassword(nickname, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return d.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketPasswords).Put([]byte(nickname), hash)
	})
}

// CheckPassword verifies a password against the stored bcrypt hash.
func (d *DB) CheckPassword(nickname, password string) (bool, error) {
	var hash []byte
	err := d.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketPasswords).Get([]byte(nickname))
		if v != nil {
			hash = make([]byte, len(v))
			copy(hash, v)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if hash == nil {
		// Run the dummy hash so an attacker can't enumerate accounts by
		// measuring the early-return latency. We discard the result.
		if dummyHash != nil {
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		}
		return false, nil
	}
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	return err == nil, nil
}

// HasPassword checks whether an account with a password exists for the nickname.
func (d *DB) HasPassword(nickname string) bool {
	found := false
	err := d.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketPasswords).Get([]byte(nickname))
		found = v != nil
		return nil
	})
	if err != nil {
		return false
	}
	return found
}
