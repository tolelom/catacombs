package dungeon

type Visibility int

const (
	Hidden Visibility = iota
	Visited
	Visible
)

func UpdateVisibility(floor *Floor) {
	if floor == nil {
		return
	}
	idx := floor.CurrentRoom
	if idx < 0 || idx >= len(floor.Rooms) {
		return
	}
	floor.Rooms[idx].Visited = true
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
	if floor.CurrentRoom >= 0 && floor.CurrentRoom < len(floor.Rooms) {
		for _, n := range floor.Rooms[floor.CurrentRoom].Neighbors {
			if n == roomIdx {
				return Visited
			}
		}
	}
	return Hidden
}
