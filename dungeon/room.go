package dungeon

import "math/rand"

type RoomType int

const (
	RoomCombat RoomType = iota
	RoomTreasure
	RoomShop
	RoomEvent
	RoomEmpty
	RoomBoss
)

func (r RoomType) String() string {
	return [...]string{"Combat", "Treasure", "Shop", "Event", "Empty", "Boss"}[r]
}

type Room struct {
	Type      RoomType
	X, Y      int
	Width, Height int
	Visited   bool
	Cleared   bool
	Neighbors []int
}

func RandomRoomType() RoomType {
	r := rand.Float64() * 100
	switch {
	case r < 45:
		return RoomCombat
	case r < 60:
		return RoomTreasure
	case r < 70:
		return RoomShop
	case r < 85:
		return RoomEvent
	default:
		return RoomEmpty
	}
}
