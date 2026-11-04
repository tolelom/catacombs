package store

import (
	"errors"

	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

var bucketPasswords = []byte("passwords")

var dummyHash []byte

func init() {
	if h, err := bcrypt.GenerateFromPassword([]byte("dummy-never-matches"), bcrypt.DefaultCost); err == nil {
		dummyHash = h
	}
}

// ErrPasswordExists means the nickname already has a password; the caller
// must log in instead of creating one.
var ErrPasswordExists = errors.New("password already set for nickname")

// CreatePassword sets the password only if the nickname has none. The check
// and the write share one transaction so two sessions cannot both claim it.
func (d *DB) CreatePassword(nickname, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketPasswords)
		if b.Get([]byte(nickname)) != nil {
			return ErrPasswordExists
		}
		return b.Put([]byte(nickname), hash)
	})
}

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
		if dummyHash != nil {
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		}
		return false, nil
	}
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	return err == nil, nil
}

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
