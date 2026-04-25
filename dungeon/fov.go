package dungeon

type Visibility int

const (
	Hidden  Visibility = iota
	Visited
	Visible
)

// UpdateVisibility marks the floor's current room as visited. Callers can
// pass a freshly-generated floor where CurrentRoom may be -1 or out of
// range; in that case this is a no-op.
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
