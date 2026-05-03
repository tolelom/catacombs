package store

import "fmt"

// MasteryMilestones lists the run-count thresholds at which a mastery reward
// fires for a given class. Cumulative — beating tier 3 gives all 3 rewards.
var MasteryMilestones = []int{5, 10, 20}

// MasteryReward describes the per-class permanent boost at each milestone.
// Each class gets a different stat focused on their identity.
type MasteryReward struct {
	HPDelta  int
	ATKDelta int
	DEFDelta int
}

// ClassMasteryRewards returns the cumulative reward applied at the player's
// current mastery tier (0..3) for a given class string ("Warrior" etc).
func ClassMasteryRewards(class string, runsCompleted int) MasteryReward {
	tier := 0
	for _, t := range MasteryMilestones {
		if runsCompleted >= t {
			tier++
		}
	}
	if tier == 0 {
		return MasteryReward{}
	}
	switch class {
	case "Warrior":
		return MasteryReward{HPDelta: 10 * tier, DEFDelta: tier}
	case "Mage":
		return MasteryReward{ATKDelta: 2 * tier}
	case "Healer":
		return MasteryReward{HPDelta: 8 * tier, DEFDelta: tier}
	case "Rogue":
		return MasteryReward{ATKDelta: tier, HPDelta: 5 * tier}
	}
	return MasteryReward{}
}

// MasteryCounterKey returns the store-counter name for a class's mastery.
func MasteryCounterKey(class string) string {
	return fmt.Sprintf("mastery:%s", class)
}
