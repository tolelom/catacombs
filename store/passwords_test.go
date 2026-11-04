package store

import (
	"errors"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// New account should not have a password.
	if db.HasPassword("alice") {
		t.Fatal("expected no password for alice")
	}

	// Save and check password.
	if err := db.CreatePassword("alice", "secret123"); err != nil {
		t.Fatal(err)
	}
	if !db.HasPassword("alice") {
		t.Fatal("expected alice to have a password")
	}

	ok, err := db.CheckPassword("alice", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected correct password to pass")
	}

	ok, err = db.CheckPassword("alice", "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}

	// Non-existent user returns false, no error.
	ok, err = db.CheckPassword("bob", "anything")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected non-existent user to fail")
	}
}

func TestCreatePasswordRefusesExistingNickname(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := db.CreatePassword("alice", "first-secret"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePassword("alice", "attacker-pw"); !errors.Is(err, ErrPasswordExists) {
		t.Fatalf("second CreatePassword err = %v, want ErrPasswordExists", err)
	}
	if ok, _ := db.CheckPassword("alice", "first-secret"); !ok {
		t.Fatal("original password must still work")
	}
	if ok, _ := db.CheckPassword("alice", "attacker-pw"); ok {
		t.Fatal("second password must not replace the first")
	}
}
