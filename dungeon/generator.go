package dungeon

import "math/rand"

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
	x, y, w, h int
	left, right *bspNode
	room        *Room
	roomIdx     int
}

func GenerateFloor(floorNum int) *Floor {
	// Create tile map filled with walls
	tiles := make([][]Tile, MapHeight)
	for y := 0; y < MapHeight; y++ {
		tiles[y] = make([]Tile, MapWidth)
		// TileWall is 0, so already initialized
	}

	// BSP tree
	root := &bspNode{x: 0, y: 0, w: MapWidth, h: MapHeight}
	splitBSP(root, 0)

	// Collect leaf nodes
	var leaves []*bspNode
	collectLeaves(root, &leaves)

	// Shuffle leaves so room assignment is varied
	rand.Shuffle(len(leaves), func(i, j int) {
		leaves[i], leaves[j] = leaves[j], leaves[i]
	})

	// We want 5-8 rooms. If we have more leaves, merge some; if fewer, accept it.
	// Ensure at least 5 leaves by re-generating if needed (BSP should produce enough).
	// Cap at 8 rooms.
	targetRooms := 5 + rand.Intn(4) // 5..8
	if len(leaves) > targetRooms {
		leaves = leaves[:targetRooms]
	}
	// If we somehow have fewer than 5, that's fine — the BSP with 60x20 and min 12x8 gives ~5-8 naturally.

	// Place rooms inside each leaf
	rooms := make([]*Room, len(leaves))
	for i, leaf := range leaves {
		// Room with padding inside the leaf
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
			rw = MinRoomW + rand.Intn(maxW-MinRoomW+1)
		}
		rh := MinRoomH
		if maxH > MinRoomH {
			rh = MinRoomH + rand.Intn(maxH-MinRoomH+1)
		}

		// Position room within the leaf
		rx := leaf.x + RoomPad
		if leaf.w-2*RoomPad > rw {
			rx += rand.Intn(leaf.w - 2*RoomPad - rw + 1)
		}
		ry := leaf.y + RoomPad
		if leaf.h-2*RoomPad > rh {
			ry += rand.Intn(leaf.h - 2*RoomPad - rh + 1)
		}

		// Clamp to map bounds
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

		rt := RandomRoomType()
		rooms[i] = &Room{
			Type:      rt,
			X:         rx,
			Y:         ry,
			W:         rw,
			H:         rh,
			Neighbors: []int{},
		}
		leaf.room = rooms[i]
		leaf.roomIdx = i
	}

	// Last room is boss
	rooms[len(rooms)-1].Type = RoomBoss

	// Carve rooms into tile map
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

	// Connect rooms: linear chain for guaranteed connectivity
	for i := 0; i < len(rooms)-1; i++ {
		rooms[i].Neighbors = append(rooms[i].Neighbors, i+1)
		rooms[i+1].Neighbors = append(rooms[i+1].Neighbors, i)
		carveCorridor(tiles, rooms[i], rooms[i+1])
	}

	// Add 1-2 extra connections
	extras := 1 + rand.Intn(2)
	for e := 0; e < extras; e++ {
		a := rand.Intn(len(rooms))
		b := rand.Intn(len(rooms))
		if a != b && !hasNeighbor(rooms[a], b) {
			rooms[a].Neighbors = append(rooms[a].Neighbors, b)
			rooms[b].Neighbors = append(rooms[b].Neighbors, a)
			carveCorridor(tiles, rooms[a], rooms[b])
		}
	}

	return &Floor{
		Number:      floorNum,
		Rooms:       rooms,
		CurrentRoom: 0,
		Tiles:       tiles,
		Width:       MapWidth,
		Height:      MapHeight,
	}
}

func splitBSP(node *bspNode, depth int) {
	// Stop conditions
	if depth > 4 {
		return
	}
	if node.w < MinLeafW*2 && node.h < MinLeafH*2 {
		return
	}

	// Random chance to stop splitting (more likely at deeper levels)
	if depth > 2 && rand.Float64() < 0.3 {
		return
	}

	// Decide split direction
	horizontal := rand.Float64() < 0.5
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
		split := MinLeafH + rand.Intn(node.h-MinLeafH*2+1)
		node.left = &bspNode{x: node.x, y: node.y, w: node.w, h: split}
		node.right = &bspNode{x: node.x, y: node.y + split, w: node.w, h: node.h - split}
	} else {
		if node.w < MinLeafW*2 {
			return
		}
		split := MinLeafW + rand.Intn(node.w-MinLeafW*2+1)
		node.left = &bspNode{x: node.x, y: node.y, w: split, h: node.h}
		node.right = &bspNode{x: node.x + split, y: node.y, w: node.w - split, h: node.h}
	}

	splitBSP(node.left, depth+1)
	splitBSP(node.right, depth+1)
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
	// L-shaped corridor from center of a to center of b
	ax := a.X + a.W/2
	ay := a.Y + a.H/2
	bx := b.X + b.W/2
	by := b.Y + b.H/2

	// Go horizontal first, then vertical
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
	// Place final tile
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
