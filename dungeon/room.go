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
)

func (r RoomType) String() string {
	return [...]string{"전투", "보물", "상점", "이벤트", "빈 방", "보스", "비밀", "미니보스"}[r]
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
	case r < 50:
		return RoomCombat
	case r < 65:
		return RoomTreasure
	case r < 75:
		return RoomShop
	case r < 90:
		return RoomEvent
	default:
		return RoomEmpty
	}
}
