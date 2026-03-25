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
	{ID: "first_clear", Name: "던전 탐험가", Description: "처음으로 5층 클리어"},
	{ID: "boss_slayer", Name: "보스 슬레이어", Description: "보스 처치"},
	{ID: "floor10", Name: "심층 탐험가", Description: "10층 도달"},
	{ID: "floor20", Name: "정복자", Description: "카타콤 정복 (20층)"},
	{ID: "solo_clear", Name: "외로운 늑대", Description: "솔로로 5층 클리어"},
	{ID: "gold_hoarder", Name: "골드 수집가", Description: "한 번의 플레이에서 골드 200 이상 모으기"},
	{ID: "no_death", Name: "무적", Description: "아무도 죽지 않고 층 클리어"},
	{ID: "full_party", Name: "동료들", Description: "4명으로 게임 시작"},
	{ID: "relic_collector", Name: "유물 수집가", Description: "한 번의 플레이에서 유물 3개 이상 수집"},
	{ID: "flee_master", Name: "전략적 후퇴", Description: "전투에서 도주 성공"},
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
