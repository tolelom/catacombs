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
	if roomIdx == floor.CurrentRoom {
		return Visible
	}
	if floor.Rooms[roomIdx].Visited {
		return Visited
	}
	return Hidden
}
