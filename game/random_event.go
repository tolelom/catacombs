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
	}
}

// PickRandomEvent returns a random event from the list using the provided RNG.
func PickRandomEvent(rng *rand.Rand) RandomEvent {
	events := GetRandomEvents()
	return events[rng.Intn(len(events))]
}
