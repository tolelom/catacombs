package dungeon

import (
	"math/rand"
	"sort"
)

const (
	MapWidth  = 60
	MapHeight = 20
	MinLeafW  = 12
	MinLeafH  = 8
	MinRoomW  = 6
	MinRoomH  = 4
	RoomPad   = 1
)

type bspNode struct {
	x, y, w, h  int
	left, right *bspNode
}

func GenerateFloor(floorNum int, rng *rand.Rand) *Floor {
	tiles := newWallTiles()
	leaves := pickRoomLeaves(rng)
	rooms := placeRooms(leaves, rng)
	assignFixedRoomTypes(rooms, floorNum)
	carveRooms(tiles, rooms)
	connectRoomsInChain(tiles, rooms)
	addExtraConnections(tiles, rooms, rng)

	return &Floor{
		Number:      floorNum,
		Rooms:       rooms,
		CurrentRoom: 0,
		Tiles:       tiles,
		Width:       MapWidth,
		Height:      MapHeight,
	}
}

func newWallTiles() [][]Tile {
	tiles := make([][]Tile, MapHeight)
	for y := 0; y < MapHeight; y++ {
		tiles[y] = make([]Tile, MapWidth)
	}
	return tiles
}

func pickRoomLeaves(rng *rand.Rand) []*bspNode {
	root := &bspNode{x: 0, y: 0, w: MapWidth, h: MapHeight}
	splitBSP(root, 0, rng)

	var leaves []*bspNode
	collectLeaves(root, &leaves)

	targetRooms := 5 + rng.Intn(4)
	if len(leaves) > targetRooms {
		rng.Shuffle(len(leaves), func(i, j int) {
			leaves[i], leaves[j] = leaves[j], leaves[i]
		})
		leaves = leaves[:targetRooms]
	}

	sort.Slice(leaves, func(i, j int) bool {
		ci := leaves[i].y + leaves[i].h/2
		cj := leaves[j].y + leaves[j].h/2
		if ci != cj {
			return ci < cj
		}
		return leaves[i].x+leaves[i].w/2 < leaves[j].x+leaves[j].w/2
	})
	return leaves
}

func placeRooms(leaves []*bspNode, rng *rand.Rand) []*Room {
	rooms := make([]*Room, len(leaves))
	for i, leaf := range leaves {
		rooms[i] = placeRoomInLeaf(leaf, rng)
	}
	return rooms
}

func placeRoomInLeaf(leaf *bspNode, rng *rand.Rand) *Room {
	maxW := leaf.w - 2*RoomPad
	maxH := leaf.h - 2*RoomPad
	if maxW < MinRoomW {
		maxW = MinRoomW
	}
	if maxH < MinRoomH {
		maxH = MinRoomH
	}

	rw := MinRoomW
	if maxW > MinRoomW {
		rw = MinRoomW + rng.Intn(maxW-MinRoomW+1)
	}
	rh := MinRoomH
	if maxH > MinRoomH {
		rh = MinRoomH + rng.Intn(maxH-MinRoomH+1)
	}

	rx := leaf.x + RoomPad
	if leaf.w-2*RoomPad > rw {
		rx += rng.Intn(leaf.w - 2*RoomPad - rw + 1)
	}
	ry := leaf.y + RoomPad
	if leaf.h-2*RoomPad > rh {
		ry += rng.Intn(leaf.h - 2*RoomPad - rh + 1)
	}

	if rx+rw > MapWidth-1 {
		rw = MapWidth - 1 - rx
	}
	if ry+rh > MapHeight-1 {
		rh = MapHeight - 1 - ry
	}
	if rx < 1 {
		rx = 1
	}
	if ry < 1 {
		ry = 1
	}

	return &Room{
		Type:      RandomRoomType(rng),
		X:         rx,
		Y:         ry,
		W:         rw,
		H:         rh,
		Neighbors: []int{},
	}
}

func assignFixedRoomTypes(rooms []*Room, floorNum int) {
	rooms[0].Type = RoomEmpty

	rooms[len(rooms)-1].Type = RoomBoss

	if floorNum == 4 || floorNum == 9 || floorNum == 14 || floorNum == 19 {
		if len(rooms) > 2 {
			rooms[1].Type = RoomMiniBoss
		}
	}
}

func carveRooms(tiles [][]Tile, rooms []*Room) {
	for _, room := range rooms {
		for dy := 0; dy < room.H; dy++ {
			for dx := 0; dx < room.W; dx++ {
				ty := room.Y + dy
				tx := room.X + dx
				if ty >= 0 && ty < MapHeight && tx >= 0 && tx < MapWidth {
					tiles[ty][tx] = TileFloor
				}
			}
		}
	}
}

func connectRoomsInChain(tiles [][]Tile, rooms []*Room) {
	for i := 0; i < len(rooms)-1; i++ {
		rooms[i].Neighbors = append(rooms[i].Neighbors, i+1)
		rooms[i+1].Neighbors = append(rooms[i+1].Neighbors, i)
		carveCorridor(tiles, rooms[i], rooms[i+1])
	}
}

func addExtraConnections(tiles [][]Tile, rooms []*Room, rng *rand.Rand) {
	extras := 1 + rng.Intn(2)
	for e := 0; e < extras; e++ {
		a := rng.Intn(len(rooms))
		b := rng.Intn(len(rooms))
		if a != b && !hasNeighbor(rooms[a], b) {
			rooms[a].Neighbors = append(rooms[a].Neighbors, b)
			rooms[b].Neighbors = append(rooms[b].Neighbors, a)
			carveCorridor(tiles, rooms[a], rooms[b])
		}
	}
}

func splitBSP(node *bspNode, depth int, rng *rand.Rand) {
	if depth > 4 {
		return
	}
	if node.w < MinLeafW*2 && node.h < MinLeafH*2 {
		return
	}

	if depth > 2 && rng.Float64() < 0.3 {
		return
	}

	horizontal := rng.Float64() < 0.5
	if node.w < MinLeafW*2 {
		horizontal = true
	}
	if node.h < MinLeafH*2 {
		horizontal = false
	}

	if horizontal {
		if node.h < MinLeafH*2 {
			return
		}
		split := MinLeafH + rng.Intn(node.h-MinLeafH*2+1)
		node.left = &bspNode{x: node.x, y: node.y, w: node.w, h: split}
		node.right = &bspNode{x: node.x, y: node.y + split, w: node.w, h: node.h - split}
	} else {
		if node.w < MinLeafW*2 {
			return
		}
		split := MinLeafW + rng.Intn(node.w-MinLeafW*2+1)
		node.left = &bspNode{x: node.x, y: node.y, w: split, h: node.h}
		node.right = &bspNode{x: node.x + split, y: node.y, w: node.w - split, h: node.h}
	}

	splitBSP(node.left, depth+1, rng)
	splitBSP(node.right, depth+1, rng)
}

func collectLeaves(node *bspNode, leaves *[]*bspNode) {
	if node == nil {
		return
	}
	if node.left == nil && node.right == nil {
		*leaves = append(*leaves, node)
		return
	}
	collectLeaves(node.left, leaves)
	collectLeaves(node.right, leaves)
}

func carveCorridor(tiles [][]Tile, a, b *Room) {
	ax := a.X + a.W/2
	ay := a.Y + a.H/2
	bx := b.X + b.W/2
	by := b.Y + b.H/2

	x := ax
	for x != bx {
		if y := ay; y >= 0 && y < MapHeight && x >= 0 && x < MapWidth {
			if tiles[y][x] == TileWall {
				tiles[y][x] = TileCorridor
			}
		}
		if x < bx {
			x++
		} else {
			x--
		}
	}
	y := ay
	for y != by {
		if x >= 0 && x < MapWidth && y >= 0 && y < MapHeight {
			if tiles[y][x] == TileWall {
				tiles[y][x] = TileCorridor
			}
		}
		if y < by {
			y++
		} else {
			y--
		}
	}
	if bx >= 0 && bx < MapWidth && by >= 0 && by < MapHeight {
		if tiles[by][bx] == TileWall {
			tiles[by][bx] = TileCorridor
		}
	}
}

func hasNeighbor(r *Room, idx int) bool {
	for _, n := range r.Neighbors {
		if n == idx {
			return true
		}
	}
	return false
}
