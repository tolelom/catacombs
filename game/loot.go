package game

import (
	"fmt"
	"math/rand"

	"github.com/tolelom/catacombs/entity"
)

const (
	mysteryBoxSetChance  = 0.35
	mysteryBoxGearChance = 0.40
	mysteryBoxTrapBase   = 12
	cursedWeaponBonusUp  = 5
)

func BuildLootEvent(floor int, rng *rand.Rand) RandomEvent {
	weapon := entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: 3 + rng.Intn(6) + floor/3}
	armor := entity.Item{Name: armorName(floor), Type: entity.ItemArmor, Bonus: 2 + rng.Intn(4) + floor/4}

	choices := []EventChoice{
		{
			Label: fmt.Sprintf("%s (ATK+%d)", weapon.Name, weapon.Bonus),
			Tags:  []ChoiceTag{TagItem},
			Resolve: func(floor int, rng *rand.Rand) EventOutcome {
				item := weapon
				return EventOutcome{Item: &item, Description: "무기를 챙겼습니다."}
			},
		},
		{
			Label: fmt.Sprintf("%s (DEF+%d)", armor.Name, armor.Bonus),
			Tags:  []ChoiceTag{TagItem},
			Resolve: func(floor int, rng *rand.Rand) EventOutcome {
				item := armor
				return EventOutcome{Item: &item, Description: "방어구를 챙겼습니다."}
			},
		},
		buildRotatingLootChoice(floor, rng),
	}

	return RandomEvent{
		Name:        "treasure",
		Description: "보물 더미를 발견했습니다. 하나만 가져갈 수 있습니다.",
		Choices:     choices,
	}
}

func buildRotatingLootChoice(floor int, rng *rand.Rand) EventChoice {
	switch rng.Intn(4) {
	case 0:
		gold := 25 + floor*3
		return EventChoice{
			Label: fmt.Sprintf("골드 주머니 (+%dG)", gold),
			Tags:  []ChoiceTag{TagGoldGain},
			Resolve: func(floor int, rng *rand.Rand) EventOutcome {
				return EventOutcome{GoldChange: gold, Description: "골드 주머니를 챙겼습니다."}
			},
		}
	case 1:
		heal := 20 + floor*2
		return EventChoice{
			Label: fmt.Sprintf("치유의 성수 (HP+%d)", heal),
			Tags:  []ChoiceTag{TagHeal},
			Resolve: func(floor int, rng *rand.Rand) EventOutcome {
				return EventOutcome{HPChange: heal, Description: "성수가 상처를 치유합니다."}
			},
		}
	case 2:
		cursed := entity.Item{
			Name:  "저주받은 " + weaponName(floor),
			Type:  entity.ItemWeapon,
			Bonus: 3 + rng.Intn(6) + floor/3 + cursedWeaponBonusUp,
		}
		cost := 10 + floor*2
		return EventChoice{
			Label: fmt.Sprintf("%s (ATK+%d, HP-%d)", cursed.Name, cursed.Bonus, cost),
			Tags:  []ChoiceTag{TagItem, TagHarm},
			Resolve: func(floor int, rng *rand.Rand) EventOutcome {
				item := cursed
				return EventOutcome{Item: &item, HPChange: -cost, Description: "저주가 생명력을 갉아먹습니다!"}
			},
		}
	default:
		return EventChoice{
			Label: "미확인 상자 (???)",
			Tags:  []ChoiceTag{TagItem, TagGamble},
			Resolve: func(floor int, rng *rand.Rand) EventOutcome {
				r := rng.Float64()
				switch {
				case r < mysteryBoxSetChance:
					item := randomSetItem(rng, floor)
					return EventOutcome{Item: &item, Description: "상자에서 세트 아이템이 나왔습니다!"}
				case r < mysteryBoxSetChance+mysteryBoxGearChance:
					item := rollGearItem(floor, rng)
					return EventOutcome{Item: &item, Description: "상자에서 장비가 나왔습니다."}
				default:
					return EventOutcome{HPChange: -(mysteryBoxTrapBase + floor), Description: "함정 상자였습니다!"}
				}
			},
		}
	}
}

func rollGearItem(floor int, rng *rand.Rand) entity.Item {
	if rng.Float64() < 0.5 {
		return entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: 3 + rng.Intn(6) + floor/3}
	}
	return entity.Item{Name: armorName(floor), Type: entity.ItemArmor, Bonus: 2 + rng.Intn(4) + floor/4}
}
