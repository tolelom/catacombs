package store

import (
	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

var bucketPasswords = []byte("passwords")

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
