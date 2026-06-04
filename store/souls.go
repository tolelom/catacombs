package store

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

var bucketSouls = []byte("souls")
var bucketUpgrades = []byte("upgrades")

// PermaUpgrade lists the per-fingerprint upgrades purchased so far. Each entry
// stores the level (number of times bought).
type PermaUpgrade string

const (
	UpgradeStartGold      PermaUpgrade = "start_gold"
	UpgradeStartHP        PermaUpgrade = "start_hp"
	UpgradeStartATK       PermaUpgrade = "start_atk"
	UpgradeInvSlots       PermaUpgrade = "inv_slots"
	UpgradeStartSkillPts  PermaUpgrade = "start_skill_pts"
)

type UpgradeDef struct {
	ID       PermaUpgrade
	Name     string
	Desc     string
	BaseCost int
	MaxLevel int
}

var upgradeDefs = []UpgradeDef{
	{UpgradeStartGold, "시작 골드 +50", "다음 런 시작시 +50G/레벨", 50, 5},
	{UpgradeStartHP, "시작 HP +20", "MaxHP/HP +20/레벨", 100, 5},
	{UpgradeStartATK, "시작 ATK +3", "ATK +3/레벨", 150, 5},
	{UpgradeStartSkillPts, "시작 스킬 포인트 +1", "런 시작시 추가 SP/레벨", 200, 3},
}

// AllUpgrades returns the catalog of available upgrades.
func AllUpgrades() []UpgradeDef {
	out := make([]UpgradeDef, len(upgradeDefs))
	copy(out, upgradeDefs)
	return out
}

// EnsureSoulsBuckets creates the souls/upgrades buckets if missing.
// Called from Open via the catch-all CreateBucketIfNotExists block.
func (d *DB) EnsureSoulsBuckets() error {
	return d.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketSouls); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketUpgrades); err != nil {
			return err
		}
		return nil
	})
}

// GetSouls returns the current soul balance for a fingerprint.
func (d *DB) GetSouls(fingerprint string) (int64, error) {
	var n int64
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSouls)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(fingerprint))
		if v != nil && len(v) == 8 {
			n = int64(binary.BigEndian.Uint64(v))
		}
		return nil
	})
	return n, err
}

// AddSouls credits souls to a fingerprint. Idempotent on positive deltas only;
// callers shouldn't pass negative.
func (d *DB) AddSouls(fingerprint string, amount int64) error {
	if amount <= 0 {
		return nil
	}
	return d.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketSouls)
		if err != nil {
			return err
		}
		var cur int64
		if v := b.Get([]byte(fingerprint)); v != nil && len(v) == 8 {
			cur = int64(binary.BigEndian.Uint64(v))
		}
		cur += amount
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(cur))
		return b.Put([]byte(fingerprint), buf)
	})
}

// GetUpgrades returns the per-upgrade level map for a fingerprint.
func (d *DB) GetUpgrades(fingerprint string) (map[PermaUpgrade]int, error) {
	out := make(map[PermaUpgrade]int)
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(fingerprint))
		if v == nil {
			return nil
		}
		if err := json.Unmarshal(v, &out); err != nil {
			return fmt.Errorf("corrupted upgrade data for %s: %w", fingerprint, err)
		}
		return nil
	})
	return out, err
}

// BuyUpgrade attempts to spend souls to raise an upgrade by 1 level. Returns
// (newLevel, error). Errors: not enough souls, max level reached, unknown id.
func (d *DB) BuyUpgrade(fingerprint string, id PermaUpgrade) (int, error) {
	var def *UpgradeDef
	for i := range upgradeDefs {
		if upgradeDefs[i].ID == id {
			def = &upgradeDefs[i]
			break
		}
	}
	if def == nil {
		return 0, errMissingUpgrade
	}
	var newLevel int
	err := d.db.Update(func(tx *bolt.Tx) error {
		ub, err := tx.CreateBucketIfNotExists(bucketUpgrades)
		if err != nil {
			return err
		}
		sb, err := tx.CreateBucketIfNotExists(bucketSouls)
		if err != nil {
			return err
		}
		levels := make(map[PermaUpgrade]int)
		if v := ub.Get([]byte(fingerprint)); v != nil {
			// Corrupted data must abort the buy: proceeding with zeroed
			// levels would let the player re-buy from level 0 and then
			// overwrite their real progression on Put.
			if err := json.Unmarshal(v, &levels); err != nil {
				return fmt.Errorf("corrupted upgrade data for %s: %w", fingerprint, err)
			}
		}
		if levels[id] >= def.MaxLevel {
			return errAtMaxLevel
		}
		cost := int64(def.BaseCost * (levels[id] + 1))
		var souls int64
		if v := sb.Get([]byte(fingerprint)); v != nil && len(v) == 8 {
			souls = int64(binary.BigEndian.Uint64(v))
		}
		if souls < cost {
			return errNotEnoughSouls
		}
		souls -= cost
		levels[id]++
		newLevel = levels[id]
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(souls))
		if err := sb.Put([]byte(fingerprint), buf); err != nil {
			return err
		}
		data, err := json.Marshal(levels)
		if err != nil {
			return err
		}
		return ub.Put([]byte(fingerprint), data)
	})
	return newLevel, err
}

// UpgradeCost returns the cost to buy the next level of an upgrade.
func UpgradeCost(currentLevel int, def UpgradeDef) int64 {
	return int64(def.BaseCost * (currentLevel + 1))
}
