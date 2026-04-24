package dungeon

type Visibility int

const (
	Hidden  Visibility = iota
	Visited
	Visible
)

func UpdateVisibility(floor *Floor) {
	for i, room := range floor.Rooms {
		if i == floor.CurrentRoom {
			room.Visited = true
		}
	}
}

func GetRoomVisibility(floor *Floor, roomIdx int) Visibility {
	if floor == nil || roomIdx < 0 || roomIdx >= len(floor.Rooms) {
		return Hidden
	}
	if roomIdx == floor.CurrentRoom {
		return Visible
	}
	if floor.Rooms[roomIdx].Visited {
		return Visited
	}
	// Neighbors of current room are dimly visible (so player can see where to go)
	if floor.CurrentRoom >= 0 && floor.CurrentRoom < len(floor.Rooms) {
		for _, n := range floor.Rooms[floor.CurrentRoom].Neighbors {
			if n == roomIdx {
				return Visited
			}
		}
	}
	return Hidden
}
