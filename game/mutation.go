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
	{ID: "no_skills", Name: "Skill Lockout", Description: "Class skills are disabled",
		Apply: func(cfg *config.GameConfig) { cfg.SkillUses = 0 }},
	{ID: "speed_run", Name: "Speed Run", Description: "Turn timeout halved",
		Apply: func(cfg *config.GameConfig) { cfg.TurnTimeoutSec = max(cfg.TurnTimeoutSec/2, 2) }},
	{ID: "no_shop", Name: "Shop Closed", Description: "Shops are unavailable",
		Apply: func(cfg *config.GameConfig) {}},
	{ID: "glass_cannon", Name: "Glass Cannon", Description: "Double damage, half HP",
		Apply: func(cfg *config.GameConfig) {}},
	{ID: "elite_flood", Name: "Elite Flood", Description: "All monsters are elite",
		Apply: func(cfg *config.GameConfig) {}},
}

// GetWeeklyMutation returns the mutation for the current week,
// determined by a SHA-256 hash of the year and ISO week number.
func GetWeeklyMutation() Mutation {
	year, week := time.Now().ISOWeek()
	h := sha256.Sum256([]byte(fmt.Sprintf("mutation:%d:%d", year, week)))
	idx := int(binary.BigEndian.Uint64(h[:8]) % uint64(len(Mutations)))
	return Mutations[idx]
}
