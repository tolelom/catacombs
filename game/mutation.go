package game

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/tolelom/catacombs/config"
)

// Mutation represents a weekly gameplay modifier.
type Mutation struct {
	ID          string
	Name        string
	Description string
	Apply       func(cfg *config.GameConfig)
}

// Mutations is the list of all available mutations.
var Mutations = []Mutation{
	{ID: "no_skills", Name: "스킬 봉인", Description: "직업 스킬 사용 불가",
		Apply: func(cfg *config.GameConfig) { cfg.SkillUses = 0 }},
	{ID: "speed_run", Name: "스피드 런", Description: "턴 제한 시간 절반",
		Apply: func(cfg *config.GameConfig) { cfg.TurnTimeoutSec = max(cfg.TurnTimeoutSec/2, 2) }},
	{ID: "no_shop", Name: "상점 폐쇄", Description: "상점 이용 불가",
		Apply: func(cfg *config.GameConfig) {}}, // handled at runtime in EnterRoom
	{ID: "glass_cannon", Name: "유리 대포", Description: "피해 2배, HP 절반",
		Apply: func(cfg *config.GameConfig) {}}, // handled at runtime in AddPlayer/spawnMonsters
	{ID: "elite_flood", Name: "엘리트 범람", Description: "모든 몬스터가 엘리트",
		Apply: func(cfg *config.GameConfig) {}}, // handled at runtime in spawnMonsters
}

// GetWeeklyMutation returns the mutation for the current week,
// determined by a SHA-256 hash of the year and ISO week number.
func GetWeeklyMutation() Mutation {
	year, week := time.Now().ISOWeek()
	h := sha256.Sum256([]byte(fmt.Sprintf("mutation:%d:%d", year, week)))
	idx := int(binary.BigEndian.Uint64(h[:8]) % uint64(len(Mutations)))
	return Mutations[idx]
}
