package entity

type ItemType int

const (
	ItemWeapon ItemType = iota
	ItemArmor
	ItemConsumable
	ItemCharm // one-shot insurance: auto-blocks one gambled loss while held
)

// ItemSet identifies a 2-piece set; 0 = no set.
type ItemSet int

const (
	SetNone ItemSet = iota
	SetFlame
	SetFrost
	SetHoly
	SetShadow
	SetNature
)

type Item struct {
	Name  string
	Type  ItemType
	Bonus int
	Price int
	Set   ItemSet
}

// SetBonusATK returns the cumulative ATK bonus from completed 2-piece sets in
// inventory. Each completed set adds +5 ATK.
func SetBonusATK(inv []Item) int {
	weapons := make(map[ItemSet]int)
	armors := make(map[ItemSet]int)
	for _, it := range inv {
		if it.Set == SetNone {
			continue
		}
		switch it.Type {
		case ItemWeapon:
			weapons[it.Set]++
		case ItemArmor:
			armors[it.Set]++
		}
	}
	bonus := 0
	for set := range weapons {
		if armors[set] > 0 {
			bonus += setATKBonus(set)
		}
	}
	return bonus
}

// SetBonusDEF returns the cumulative DEF bonus from completed 2-piece sets.
func SetBonusDEF(inv []Item) int {
	weapons := make(map[ItemSet]int)
	armors := make(map[ItemSet]int)
	for _, it := range inv {
		if it.Set == SetNone {
			continue
		}
		switch it.Type {
		case ItemWeapon:
			weapons[it.Set]++
		case ItemArmor:
			armors[it.Set]++
		}
	}
	bonus := 0
	for set := range weapons {
		if armors[set] > 0 {
			bonus += setDEFBonus(set)
		}
	}
	return bonus
}

// CompletedSets returns the set IDs whose 2-piece bonus is active.
func CompletedSets(inv []Item) []ItemSet {
	weapons := make(map[ItemSet]bool)
	armors := make(map[ItemSet]bool)
	for _, it := range inv {
		if it.Set == SetNone {
			continue
		}
		switch it.Type {
		case ItemWeapon:
			weapons[it.Set] = true
		case ItemArmor:
			armors[it.Set] = true
		}
	}
	var out []ItemSet
	for set := range weapons {
		if armors[set] {
			out = append(out, set)
		}
	}
	return out
}

func setATKBonus(s ItemSet) int {
	switch s {
	case SetFlame, SetShadow:
		return 8
	case SetFrost, SetNature:
		return 4
	case SetHoly:
		return 5
	}
	return 0
}

func setDEFBonus(s ItemSet) int {
	switch s {
	case SetFlame, SetShadow:
		return 0
	case SetFrost, SetNature:
		return 5
	case SetHoly:
		return 6
	}
	return 0
}

// SetName returns the human-readable name of an ItemSet.
func SetName(s ItemSet) string {
	switch s {
	case SetFlame:
		return "불꽃"
	case SetFrost:
		return "얼음"
	case SetHoly:
		return "신성"
	case SetShadow:
		return "그림자"
	case SetNature:
		return "자연"
	}
	return ""
}

type RelicEffect int

const (
	RelicHealOnKill RelicEffect = iota
	RelicATKBoost
	RelicDEFBoost
	RelicGoldBoost
	RelicPoisonImmunity // immune to poison
	RelicBurnResist     // halve burn damage
	RelicLifeSteal      // heal 10% of damage dealt
	// Active relics — triggered by player via ActionUseRelic, consumed once per combat
	RelicActiveSmokeBomb // 1턴 파티 회피 +30%
	RelicActiveBloodstone // 즉시 30% MaxHP 회복
	RelicActiveTimeGlass  // 자신 다음 턴 추가 1회 행동
)

// IsActive returns true for relics that are triggered by the player rather
// than auto-applied. Active relics have a per-combat cooldown.
func (e RelicEffect) IsActive() bool {
	switch e {
	case RelicActiveSmokeBomb, RelicActiveBloodstone, RelicActiveTimeGlass:
		return true
	}
	return false
}

type Relic struct {
	Name   string
	Effect RelicEffect
	Value  int
	Price  int
}

func NewHPPotion() Item {
	return Item{Name: "HP Potion", Type: ItemConsumable, Bonus: 30, Price: 20}
}

// NewLuckyCharm rolls the shop's RNG-hedge consumable: while held, it
// auto-blocks one gambled loss (TagGamble event outcomes, gambling-den bets)
// and is consumed. Fixed trade-off costs are prices, not gambles — the charm
// ignores them.
func NewLuckyCharm(floor int) Item {
	return Item{Name: "행운 부적", Type: ItemCharm, Price: 30 + floor}
}
