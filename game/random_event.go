package game

import (
	"math/rand"

	"github.com/tolelom/catacombs/entity"
)

// ChoiceTag classifies an event choice's possible outcomes for UI hints.
// One choice can carry multiple tags (e.g. altar pray = Heal + Harm + Gamble).
type ChoiceTag int

const (
	TagHeal     ChoiceTag = iota // ❤  HP recovery possible
	TagHarm                      // 🔥 HP loss risk
	TagGoldGain                  // 💰+ gold gain
	TagGoldCost                  // 💰- gold cost
	TagItem                      // ✨ item drop
	TagGamble                    // 🎲 outcome branches on RNG
	TagSkip                      // ⤳ no effect (safe pass)
)

// EventOutcome describes the result of choosing an event option.
type EventOutcome struct {
	HPChange    int
	GoldChange  int
	ItemDrop    bool         // roll a random gear item per recipient
	Item        *entity.Item // grant this specific item (loot picks reveal values up front)
	Gambled     bool         // outcome came from a TagGamble choice — lucky charms block its losses
	Description string
}

// tagsContain reports whether want appears in tags.
func tagsContain(tags []ChoiceTag, want ChoiceTag) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// EventChoice represents a single choice the player can make during an event.
// Resolve receives the session RNG so daily-challenge runs stay deterministic;
// callers must not use the package-level rand inside Resolve.
type EventChoice struct {
	Label   string
	Tags    []ChoiceTag
	Resolve func(floor int, rng *rand.Rand) EventOutcome
}

// RandomEvent represents a random event with multiple choices.
type RandomEvent struct {
	Name        string
	Description string
	Choices     []EventChoice
}

// GetRandomEvents returns all 8 defined random events.
func GetRandomEvents() []RandomEvent {
	return []RandomEvent{
		{
			Name:        "altar",
			Description: "이상한 에너지로 빛나는 고대 제단을 발견했습니다.",
			Choices: []EventChoice{
				{
					Label: "제단에서 기도하기",
					Tags:  []ChoiceTag{TagHeal, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.6 {
							heal := 15 + floor*2
							return EventOutcome{HPChange: heal, Description: "제단이 치유의 빛으로 축복합니다."}
						}
						dmg := 10 + floor
						return EventOutcome{HPChange: -dmg, Description: "제단의 에너지가 당신을 공격합니다!"}
					},
				},
				{
					Label: "골드 바치기",
					Tags:  []ChoiceTag{TagGoldCost, TagItem},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						cost := 10 + floor
						return EventOutcome{GoldChange: -cost, ItemDrop: true, Description: "골드를 바치고 신성한 선물을 받았습니다."}
					},
				},
				{
					Label: "그냥 지나가기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "제단을 건드리지 않고 떠납니다."}
					},
				},
			},
		},
		{
			Name:        "fountain",
			Description: "방 중앙에서 빛나는 분수가 솟아오릅니다.",
			Choices: []EventChoice{
				{
					Label: "분수의 물 마시기",
					Tags:  []ChoiceTag{TagHeal},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						heal := 20 + floor*2
						return EventOutcome{HPChange: heal, Description: "물이 당신을 활기차게 합니다!"}
					},
				},
				{
					Label: "동전 던지기",
					Tags:  []ChoiceTag{TagGoldGain, TagGoldCost, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.5 {
							gold := 15 + floor*3
							return EventOutcome{GoldChange: gold, Description: "분수가 당신의 관대함에 보답합니다!"}
						}
						return EventOutcome{GoldChange: -5, Description: "동전이 가라앉고 아무 일도 일어나지 않습니다."}
					},
				},
			},
		},
		{
			Name:        "merchant",
			Description: "두건을 쓴 상인이 어둠 속에서 나타납니다.",
			Choices: []EventChoice{
				{
					Label: "골드로 치료 거래",
					Tags:  []ChoiceTag{TagHeal, TagGoldCost},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						cost := 15 + floor
						heal := 25 + floor*2
						return EventOutcome{HPChange: heal, GoldChange: -cost, Description: "상인이 치유의 물약을 팝니다."}
					},
				},
				{
					Label: "미스터리 아이템 구매",
					Tags:  []ChoiceTag{TagGoldCost, TagItem},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						cost := 20 + floor*2
						return EventOutcome{GoldChange: -cost, ItemDrop: true, Description: "상인이 포장된 꾸러미를 건넵니다."}
					},
				},
				{
					Label: "거절하기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "상인이 어둠 속으로 사라집니다."}
					},
				},
			},
		},
		{
			Name:        "trap_room",
			Description: "바닥이 수상한 압력판으로 덮여 있습니다.",
			Choices: []EventChoice{
				{
					Label: "조심히 지나가기",
					Tags:  []ChoiceTag{TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.5 {
							return EventOutcome{Description: "능숙하게 모든 함정을 피했습니다!"}
						}
						dmg := 8 + floor
						return EventOutcome{HPChange: -dmg, Description: "함정을 밟아 피해를 입었습니다!"}
					},
				},
				{
					Label: "돌진하기",
					Tags:  []ChoiceTag{TagHarm, TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						dmg := 5 + floor/2
						gold := 10 + floor*2
						return EventOutcome{HPChange: -dmg, GoldChange: gold, Description: "약간의 피해를 입었지만 숨겨진 골드를 발견했습니다!"}
					},
				},
			},
		},
		{
			Name:        "shrine",
			Description: "신성한 힘으로 울리는 빛나는 성소가 있습니다.",
			Choices: []EventChoice{
				{
					Label: "무릎 꿇고 기도하기",
					Tags:  []ChoiceTag{TagHeal},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						heal := 30 + floor*2
						return EventOutcome{HPChange: heal, Description: "성소가 새로운 활력으로 가득 채워줍니다!"}
					},
				},
				{
					Label: "제물 가져가기",
					Tags:  []ChoiceTag{TagHarm, TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 20 + floor*3
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, GoldChange: gold, Description: "제물을 훔쳤지만 영혼들이 분노합니다!"}
					},
				},
			},
		},
		{
			Name:        "chest",
			Description: "방 구석에 화려한 상자가 놓여 있습니다.",
			Choices: []EventChoice{
				{
					Label: "조심히 열기",
					Tags:  []ChoiceTag{TagGoldGain, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.7 {
							gold := 15 + floor*2
							return EventOutcome{GoldChange: gold, Description: "상자 안에 골드 더미가 있습니다!"}
						}
						dmg := 12 + floor
						return EventOutcome{HPChange: -dmg, Description: "상자가 미믹이었습니다! 물어뜯깁니다!"}
					},
				},
				{
					Label: "부수어 열기",
					Tags:  []ChoiceTag{TagItem},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{ItemDrop: true, Description: "상자를 부수고 안에서 장비를 발견했습니다!"}
					},
				},
				{
					Label: "그냥 두기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "안전한 게 최고입니다."}
					},
				},
			},
		},
		{
			Name:        "ghost",
			Description: "유령 같은 형체가 눈앞에 나타납니다.",
			Choices: []EventChoice{
				{
					Label: "유령과 대화하기",
					Tags:  []ChoiceTag{TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 10 + floor*2
						return EventOutcome{GoldChange: gold, Description: "유령이 들어줘서 감사하며 보상합니다."}
					},
				},
				{
					Label: "유령 공격하기",
					Tags:  []ChoiceTag{TagItem, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.4 {
							return EventOutcome{ItemDrop: true, Description: "유령이 사라지며 유령 무기를 떨어뜨립니다!"}
						}
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, Description: "유령이 분노하여 반격합니다!"}
					},
				},
			},
		},
		{
			Name:        "mushroom",
			Description: "이상하게 빛나는 버섯들이 무리 지어 자라고 있습니다.",
			Choices: []EventChoice{
				{
					Label: "버섯 먹기",
					Tags:  []ChoiceTag{TagHeal, TagHarm, TagGoldGain, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						r := rng.Float64()
						if r < 0.33 {
							heal := 20 + floor*2
							return EventOutcome{HPChange: heal, Description: "버섯이 맛있고 치유 효과가 있습니다!"}
						} else if r < 0.66 {
							dmg := 10 + floor
							return EventOutcome{HPChange: -dmg, Description: "독버섯이었습니다!"}
						}
						gold := 10 + floor
						return EventOutcome{GoldChange: gold, Description: "버섯이 이상한 환각을 보여주고... 위에서 골드가 떨어집니다!"}
					},
				},
				{
					Label: "채집하여 팔기",
					Tags:  []ChoiceTag{TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 8 + floor
						return EventOutcome{GoldChange: gold, Description: "조심히 버섯을 채집하여 판매합니다."}
					},
				},
				{
					Label: "무시하기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "의문의 균류를 현명하게 피합니다."}
					},
				},
			},
		},
		{
			Name:        "gravekeeper",
			Description: "이끼 낀 무덤들 사이에 늙은 묘지기가 서 있습니다.",
			Choices: []EventChoice{
				{
					Label: "무덤 파헤치기",
					Tags:  []ChoiceTag{TagItem, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.5 {
							item := randomSetItem(rng, floor)
							return EventOutcome{Item: &item, Description: "관 속에서 세트 아이템을 발견했습니다!"}
						}
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, Description: "분노한 영혼이 당신을 할퀴고 갑니다!"}
					},
				},
				{
					Label: "묘비 정리하기",
					Tags:  []ChoiceTag{TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 8 + floor
						return EventOutcome{GoldChange: gold, Description: "묘지기가 수고비를 건넵니다."}
					},
				},
				{
					Label: "명복 빌기",
					Tags:  []ChoiceTag{TagHeal},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						heal := 12 + floor
						return EventOutcome{HPChange: heal, Description: "고요한 평화가 상처를 어루만집니다."}
					},
				},
			},
		},
		{
			Name:        "forge",
			Description: "버려진 대장간. 용광로에 아직 잔불이 남아 있습니다.",
			Choices: []EventChoice{
				{
					Label: "용광로 뒤지기",
					Tags:  []ChoiceTag{TagItem, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.6 {
							// Premium roll: base bonus floor +2 above the
							// regular ItemDrop weapon formula (3+...).
							item := entity.Item{
								Name:  "대장장이의 " + weaponName(floor),
								Type:  entity.ItemWeapon,
								Bonus: 5 + rng.Intn(6) + floor/3,
							}
							return EventOutcome{Item: &item, Description: "재 속에서 명품 무기를 건졌습니다!"}
						}
						dmg := 12 + floor
						return EventOutcome{HPChange: -dmg, Description: "잔불에 손을 데였습니다!"}
					},
				},
				{
					Label: "고철 모아 팔기",
					Tags:  []ChoiceTag{TagGoldGain},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						gold := 12 + floor*2
						return EventOutcome{GoldChange: gold, Description: "쓸 만한 고철을 챙겼습니다."}
					},
				},
				{
					Label: "지나가기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "식은 용광로를 뒤로하고 떠납니다."}
					},
				},
			},
		},
		{
			Name:        "toll_bridge",
			Description: "낡은 다리 앞을 트롤이 가로막고 있습니다. \"통행료!\"",
			Choices: []EventChoice{
				{
					Label: "통행료 내기",
					Tags:  []ChoiceTag{TagGoldCost},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						cost := 15 + floor
						return EventOutcome{GoldChange: -cost, Description: "트롤이 흡족하게 길을 비킵니다."}
					},
				},
				{
					Label: "몰래 건너기",
					Tags:  []ChoiceTag{TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.6 {
							return EventOutcome{Description: "트롤이 조는 사이 무사히 건넜습니다!"}
						}
						dmg := 20 + floor
						return EventOutcome{HPChange: -dmg, Description: "들켰습니다! 트롤이 다리 밑으로 내던집니다!"}
					},
				},
				{
					Label: "다리 밑 뒤지기",
					Tags:  []ChoiceTag{TagGoldGain, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.5 {
							gold := 30 + floor*2
							return EventOutcome{GoldChange: gold, Description: "트롤이 모아둔 통행료 더미를 찾았습니다!"}
						}
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, Description: "트롤의 몽둥이에 얻어맞았습니다!"}
					},
				},
			},
		},
		{
			Name:        "feast",
			Description: "아무도 없는 방에 김이 나는 만찬이 차려져 있습니다.",
			Choices: []EventChoice{
				{
					Label: "마음껏 먹기",
					Tags:  []ChoiceTag{TagHeal, TagHarm, TagGamble},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						if rng.Float64() < 0.7 {
							heal := 25 + floor*2
							return EventOutcome{HPChange: heal, Description: "놀랍도록 맛있고 든든합니다!"}
						}
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, Description: "식중독입니다! 배를 움켜쥡니다!"}
					},
				},
				{
					Label: "음식 싸가기",
					Tags:  []ChoiceTag{TagItem},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						// Humble snack tier: below the shop potion (30+floor) on
						// every floor so a free pick never beats a bought one.
						item := entity.Item{Name: "여행 도시락", Type: entity.ItemConsumable, Bonus: 20 + floor/2}
						return EventOutcome{Item: &item, Description: "음식을 포장해 챙겼습니다."}
					},
				},
				{
					Label: "의심하며 떠나기",
					Tags:  []ChoiceTag{TagSkip},
					Resolve: func(floor int, rng *rand.Rand) EventOutcome {
						return EventOutcome{Description: "공짜 만찬은 없는 법입니다."}
					},
				},
			},
		},
	}
}

// PickRandomEvent returns a random event from the list using the provided RNG.
func PickRandomEvent(rng *rand.Rand) RandomEvent {
	events := GetRandomEvents()
	return events[rng.Intn(len(events))]
}
