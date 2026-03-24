package entity

type ItemType int

const (
	ItemWeapon ItemType = iota
	ItemArmor
	ItemConsumable
)

type Item struct {
	Name  string
	Type  ItemType
	Bonus int
	Price int
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
)

type Relic struct {
	Name   string
	Effect RelicEffect
	Value  int
	Price  int
}

func NewHPPotion() Item {
	return Item{Name: "HP Potion", Type: ItemConsumable, Bonus: 30, Price: 20}
}
