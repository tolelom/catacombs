package entity

type ItemType int

const (
	ItemWeapon ItemType = iota
	ItemArmor
	ItemConsumable
	ItemCharm
)

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
	RelicPoisonImmunity
	RelicBurnResist
	RelicLifeSteal
	RelicActiveSmokeBomb
	RelicActiveBloodstone
	RelicActiveTimeGlass
)

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

func NewLuckyCharm(floor int) Item {
	return Item{Name: "행운 부적", Type: ItemCharm, Price: 30 + floor}
}
