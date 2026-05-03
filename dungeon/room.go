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
	RoomSecret
	RoomMiniBoss
	RoomMimic
	RoomGamble
	RoomCursedAltar
	RoomWanderingMerchant
)

func (r RoomType) String() string {
	return [...]string{"전투", "보물", "상점", "이벤트", "빈 방", "보스", "비밀", "미니보스", "미믹", "도박장", "저주제단", "방랑상인"}[r]
}

type Tile int

const (
	TileWall Tile = iota
	TileFloor
	TileCorridor
	TileDoor
)

type Room struct {
	Type      RoomType
	X, Y      int // top-left in tile space
	W, H      int // dimensions in tiles
	Visited   bool
	Cleared   bool
	Neighbors []int
}

type Floor struct {
	Number      int
	Rooms       []*Room
	CurrentRoom int
	Tiles       [][]Tile
	Width       int
	Height      int
}

func RandomRoomType(rng *rand.Rand) RoomType {
	r := rng.Float64() * 100
	switch {
	case r < 5:
		return RoomSecret
	case r < 42:
		return RoomCombat
	case r < 55:
		return RoomTreasure
	case r < 65:
		return RoomShop
	case r < 78:
		return RoomEvent
	case r < 83:
		return RoomMimic
	case r < 88:
		return RoomGamble
	case r < 92:
		return RoomCursedAltar
	case r < 96:
		return RoomWanderingMerchant
	default:
		return RoomEmpty
	}
}
