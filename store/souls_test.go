package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestAddAndGetSouls(t *testing.T) {
	d := openTestDB(t)
	if n, _ := d.GetSouls("fp1"); n != 0 {
		t.Errorf("initial souls = %d, want 0", n)
	}
	if err := d.AddSouls("fp1", 50); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := d.AddSouls("fp1", 25); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if n, _ := d.GetSouls("fp1"); n != 75 {
		t.Errorf("got %d souls, want 75", n)
	}
}

func TestBuyUpgradeSpendsSouls(t *testing.T) {
	d := openTestDB(t)
	_ = d.AddSouls("fp1", 200)
	lvl, err := d.BuyUpgrade("fp1", UpgradeStartGold)
	if err != nil {
		t.Fatalf("buy: %v", err)
	}
	if lvl != 1 {
		t.Errorf("level after first buy = %d, want 1", lvl)
	}
	souls, _ := d.GetSouls("fp1")
	if souls != 150 {
		t.Errorf("souls after buy = %d, want 150 (200-50)", souls)
	}
}

func TestBuyUpgradeNotEnoughSouls(t *testing.T) {
	d := openTestDB(t)
	_ = d.AddSouls("fp1", 10)
	_, err := d.BuyUpgrade("fp1", UpgradeStartGold)
	if !errors.Is(err, ErrNotEnoughSouls) {
		t.Errorf("expected ErrNotEnoughSouls, got %v", err)
	}
}

func TestBuyUpgradeMaxLevel(t *testing.T) {
	d := openTestDB(t)
	_ = d.AddSouls("fp1", 100000)
	def := UpgradeDef{ID: UpgradeInvSlots, MaxLevel: 3, BaseCost: 75}
	for i := 0; i < def.MaxLevel; i++ {
		if _, err := d.BuyUpgrade("fp1", UpgradeStartSkillPts); err != nil && !errors.Is(err, ErrAtMaxLevel) {
			t.Fatalf("buy %d: %v", i, err)
		}
	}
	// One more should fail with ErrAtMaxLevel
	_, err := d.BuyUpgrade("fp1", UpgradeStartSkillPts)
	if !errors.Is(err, ErrAtMaxLevel) {
		t.Errorf("expected ErrAtMaxLevel after maxing, got %v", err)
	}
}

func TestBuyUpgradeUnknownID(t *testing.T) {
	d := openTestDB(t)
	_ = d.AddSouls("fp1", 1000)
	_, err := d.BuyUpgrade("fp1", PermaUpgrade("nonsense"))
	if !errors.Is(err, ErrMissingUpgrade) {
		t.Errorf("expected ErrMissingUpgrade, got %v", err)
	}
}
