package dungeon

import "math/rand"

type Floor struct {
	Number    int
	Rooms     []*Room
	CurrentRoom int
}

func GenerateFloor(floorNum int) *Floor {
	numRooms := 5 + rand.Intn(4)
	rooms := make([]*Room, numRooms)
	for i := 0; i < numRooms; i++ {
		rt := RandomRoomType()
		rooms[i] = &Room{
			Type:      rt,
			X:         (i % 3) * 20,
			Y:         (i / 3) * 10,
			Width:     12 + rand.Intn(6),
			Height:    6 + rand.Intn(4),
			Neighbors: []int{},
		}
	}
	rooms[numRooms-1].Type = RoomBoss
	for i := 0; i < numRooms-1; i++ {
		rooms[i].Neighbors = append(rooms[i].Neighbors, i+1)
		rooms[i+1].Neighbors = append(rooms[i+1].Neighbors, i)
	}
	extras := 1 + rand.Intn(2)
	for e := 0; e < extras; e++ {
		a := rand.Intn(numRooms)
		b := rand.Intn(numRooms)
		if a != b && !hasNeighbor(rooms[a], b) {
			rooms[a].Neighbors = append(rooms[a].Neighbors, b)
			rooms[b].Neighbors = append(rooms[b].Neighbors, a)
		}
	}
	return &Floor{Number: floorNum, Rooms: rooms, CurrentRoom: 0}
}

func hasNeighbor(r *Room, idx int) bool {
	for _, n := range r.Neighbors {
		if n == idx {
			return true
		}
	}
	return false
}
