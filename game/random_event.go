package game

import "math/rand"

// EventOutcome describes the result of choosing an event option.
type EventOutcome struct {
	HPChange    int
	GoldChange  int
	ItemDrop    bool
	Description string
}

// EventChoice represents a single choice the player can make during an event.
type EventChoice struct {
	Label   string
	Resolve func(floor int) EventOutcome
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
			Description: "You discover an ancient altar glowing with strange energy.",
			Choices: []EventChoice{
				{
					Label: "Pray at the altar",
					Resolve: func(floor int) EventOutcome {
						if rand.Float64() < 0.6 {
							heal := 15 + floor*2
							return EventOutcome{HPChange: heal, Description: "The altar blesses you with healing light."}
						}
						dmg := 10 + floor
						return EventOutcome{HPChange: -dmg, Description: "The altar's energy lashes out at you!"}
					},
				},
				{
					Label: "Offer gold",
					Resolve: func(floor int) EventOutcome {
						cost := 10 + floor
						return EventOutcome{GoldChange: -cost, ItemDrop: true, Description: "You offer gold and receive a divine gift."}
					},
				},
				{
					Label: "Walk away",
					Resolve: func(floor int) EventOutcome {
						return EventOutcome{Description: "You leave the altar undisturbed."}
					},
				},
			},
		},
		{
			Name:        "fountain",
			Description: "A shimmering fountain bubbles in the center of the room.",
			Choices: []EventChoice{
				{
					Label: "Drink from the fountain",
					Resolve: func(floor int) EventOutcome {
						heal := 20 + floor*2
						return EventOutcome{HPChange: heal, Description: "The water rejuvenates you!"}
					},
				},
				{
					Label: "Toss a coin",
					Resolve: func(floor int) EventOutcome {
						if rand.Float64() < 0.5 {
							gold := 15 + floor*3
							return EventOutcome{GoldChange: gold, Description: "The fountain rewards your generosity!"}
						}
						return EventOutcome{GoldChange: -5, Description: "The coin sinks and nothing happens."}
					},
				},
			},
		},
		{
			Name:        "merchant",
			Description: "A hooded merchant appears from the shadows.",
			Choices: []EventChoice{
				{
					Label: "Trade gold for healing",
					Resolve: func(floor int) EventOutcome {
						cost := 15 + floor
						heal := 25 + floor*2
						return EventOutcome{HPChange: heal, GoldChange: -cost, Description: "The merchant sells you a healing draught."}
					},
				},
				{
					Label: "Buy a mystery item",
					Resolve: func(floor int) EventOutcome {
						cost := 20 + floor*2
						return EventOutcome{GoldChange: -cost, ItemDrop: true, Description: "The merchant hands you a wrapped package."}
					},
				},
				{
					Label: "Decline",
					Resolve: func(floor int) EventOutcome {
						return EventOutcome{Description: "The merchant vanishes into the shadows."}
					},
				},
			},
		},
		{
			Name:        "trap_room",
			Description: "The floor is covered with suspicious pressure plates.",
			Choices: []EventChoice{
				{
					Label: "Carefully navigate",
					Resolve: func(floor int) EventOutcome {
						if rand.Float64() < 0.5 {
							return EventOutcome{Description: "You skillfully avoid all the traps!"}
						}
						dmg := 8 + floor
						return EventOutcome{HPChange: -dmg, Description: "You trigger a trap and take damage!"}
					},
				},
				{
					Label: "Rush through",
					Resolve: func(floor int) EventOutcome {
						dmg := 5 + floor/2
						gold := 10 + floor*2
						return EventOutcome{HPChange: -dmg, GoldChange: gold, Description: "You take minor damage but find hidden gold!"}
					},
				},
			},
		},
		{
			Name:        "shrine",
			Description: "A glowing shrine hums with divine power.",
			Choices: []EventChoice{
				{
					Label: "Kneel and pray",
					Resolve: func(floor int) EventOutcome {
						heal := 30 + floor*2
						return EventOutcome{HPChange: heal, Description: "The shrine fills you with renewed vigor!"}
					},
				},
				{
					Label: "Take the offering",
					Resolve: func(floor int) EventOutcome {
						gold := 20 + floor*3
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, GoldChange: gold, Description: "You steal the offering but anger the spirits!"}
					},
				},
			},
		},
		{
			Name:        "chest",
			Description: "An ornate chest sits in the corner of the room.",
			Choices: []EventChoice{
				{
					Label: "Open carefully",
					Resolve: func(floor int) EventOutcome {
						if rand.Float64() < 0.7 {
							gold := 15 + floor*2
							return EventOutcome{GoldChange: gold, Description: "The chest contains a pile of gold!"}
						}
						dmg := 12 + floor
						return EventOutcome{HPChange: -dmg, Description: "The chest was a mimic! It bites you!"}
					},
				},
				{
					Label: "Smash it open",
					Resolve: func(floor int) EventOutcome {
						return EventOutcome{ItemDrop: true, Description: "You smash the chest and find equipment inside!"}
					},
				},
				{
					Label: "Leave it",
					Resolve: func(floor int) EventOutcome {
						return EventOutcome{Description: "Better safe than sorry."}
					},
				},
			},
		},
		{
			Name:        "ghost",
			Description: "A spectral figure materializes before you.",
			Choices: []EventChoice{
				{
					Label: "Speak with the ghost",
					Resolve: func(floor int) EventOutcome {
						gold := 10 + floor*2
						return EventOutcome{GoldChange: gold, Description: "The ghost thanks you for listening and rewards you."}
					},
				},
				{
					Label: "Attack the ghost",
					Resolve: func(floor int) EventOutcome {
						if rand.Float64() < 0.4 {
							return EventOutcome{ItemDrop: true, Description: "The ghost drops a spectral weapon as it fades!"}
						}
						dmg := 15 + floor
						return EventOutcome{HPChange: -dmg, Description: "The ghost retaliates with ghostly fury!"}
					},
				},
			},
		},
		{
			Name:        "mushroom",
			Description: "Strange glowing mushrooms grow in clusters here.",
			Choices: []EventChoice{
				{
					Label: "Eat a mushroom",
					Resolve: func(floor int) EventOutcome {
						r := rand.Float64()
						if r < 0.33 {
							heal := 20 + floor*2
							return EventOutcome{HPChange: heal, Description: "The mushroom tastes great and heals you!"}
						} else if r < 0.66 {
							dmg := 10 + floor
							return EventOutcome{HPChange: -dmg, Description: "The mushroom was poisonous!"}
						}
						gold := 10 + floor
						return EventOutcome{GoldChange: gold, Description: "The mushroom gives you strange visions... and gold falls from above!"}
					},
				},
				{
					Label: "Collect and sell",
					Resolve: func(floor int) EventOutcome {
						gold := 8 + floor
						return EventOutcome{GoldChange: gold, Description: "You carefully harvest the mushrooms for sale."}
					},
				},
				{
					Label: "Ignore them",
					Resolve: func(floor int) EventOutcome {
						return EventOutcome{Description: "You wisely avoid the mysterious fungi."}
					},
				},
			},
		},
	}
}

// PickRandomEvent returns a random event from the list.
func PickRandomEvent() RandomEvent {
	events := GetRandomEvents()
	return events[rand.Intn(len(events))]
}
