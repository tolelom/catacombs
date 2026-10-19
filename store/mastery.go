package store

import "fmt"

var MasteryMilestones = []int{5, 10, 20}

type MasteryReward struct {
	HPDelta  int
	ATKDelta int
	DEFDelta int
}

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

func MasteryCounterKey(class string) string {
	return fmt.Sprintf("mastery:%s", class)
}
