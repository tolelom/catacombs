package dungeon

import (
	"math/rand"
	"testing"
)

func newTestRng() *rand.Rand {
	return rand.New(rand.NewSource(rand.Int63()))
}

func TestGenerateFloor(t *testing.T) {
	floor := GenerateFloor(1, newTestRng())
	if len(floor.Rooms) < 5 || len(floor.Rooms) > 8 {
		t.Errorf("Room count: got %d, want 5~8", len(floor.Rooms))
	}
	bossCount := 0
	for _, r := range floor.Rooms {
		if r.Type == RoomBoss {
			bossCount++
		}
	}
	if bossCount != 1 {
		t.Errorf("Boss rooms: got %d, want 1", bossCount)
	}
	visited := make(map[int]bool)
	var dfs func(int)
	dfs = func(idx int) {
		if visited[idx] { return }
		visited[idx] = true
		for _, n := range floor.Rooms[idx].Neighbors { dfs(n) }
	}
	dfs(0)
	if len(visited) != len(floor.Rooms) {
		t.Errorf("Not all rooms connected: reachable %d / %d", len(visited), len(floor.Rooms))
	}
}

func TestRoomTypeProbability(t *testing.T) {
	counts := make(map[RoomType]int)
	n := 10000
	rng := rand.New(rand.NewSource(12345))
	for i := 0; i < n; i++ {
		counts[RandomRoomType(rng)]++
	}
	combatPct := float64(counts[RoomCombat]) / float64(n) * 100
	if combatPct < 32 || combatPct > 42 {
		t.Errorf("Combat room probability: got %.1f%%, want ~37%% (range 5-42)", combatPct)
	}
}

func TestSecretRoomInRandomType(t *testing.T) {
	counts := make(map[RoomType]int)
	n := 10000
	rng := rand.New(rand.NewSource(12345))
	for i := 0; i < n; i++ {
		counts[RandomRoomType(rng)]++
	}
	secretPct := float64(counts[RoomSecret]) / float64(n) * 100
	if secretPct < 2 || secretPct > 8 {
		t.Errorf("Secret room probability: got %.1f%%, want ~5%%", secretPct)
	}
	// Verify RoomMiniBoss is never returned by RandomRoomType
	if counts[RoomMiniBoss] > 0 {
		t.Errorf("MiniBoss rooms should not be generated randomly, got %d", counts[RoomMiniBoss])
	}
}

func TestMiniBossRoomPlacement(t *testing.T) {
	for _, floorNum := range []int{4, 9, 14, 19} {
		floor := GenerateFloor(floorNum, newTestRng())
		found := false
		for _, r := range floor.Rooms {
			if r.Type == RoomMiniBoss {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Floor %d should have a mini-boss room", floorNum)
		}
	}
	// Non-miniboss floors should not have mini-boss rooms
	for _, floorNum := range []int{1, 3, 5, 10} {
		floor := GenerateFloor(floorNum, newTestRng())
		for _, r := range floor.Rooms {
			if r.Type == RoomMiniBoss {
				t.Errorf("Floor %d should not have a mini-boss room", floorNum)
				break
			}
		}
	}
}

func TestFloorHasTileMap(t *testing.T) {
	floor := GenerateFloor(1, newTestRng())
	if floor.Tiles == nil {
		t.Fatal("Floor should have tile map")
	}
	if floor.Width != 60 || floor.Height != 20 {
		t.Errorf("Map size: got %dx%d, want 60x20", floor.Width, floor.Height)
	}
	// Current room should have floor tiles
	room := floor.Rooms[0]
	centerTile := floor.Tiles[room.Y+room.H/2][room.X+room.W/2]
	if centerTile != TileFloor {
		t.Errorf("Room center should be floor tile, got %d", centerTile)
	}
}

func TestDeterministicGeneration(t *testing.T) {
	rng1 := rand.New(rand.NewSource(42))
	rng2 := rand.New(rand.NewSource(42))
	f1 := GenerateFloor(5, rng1)
	f2 := GenerateFloor(5, rng2)
	if len(f1.Rooms) != len(f2.Rooms) {
		t.Fatalf("room counts differ: %d vs %d", len(f1.Rooms), len(f2.Rooms))
	}
	for i, r := range f1.Rooms {
		if r.Type != f2.Rooms[i].Type || r.X != f2.Rooms[i].X || r.Y != f2.Rooms[i].Y {
			t.Errorf("room %d differs: type=%v/%v x=%d/%d y=%d/%d",
				i, r.Type, f2.Rooms[i].Type, r.X, f2.Rooms[i].X, r.Y, f2.Rooms[i].Y)
		}
	}
	// Also verify tile maps match
	for y := 0; y < f1.Height; y++ {
		for x := 0; x < f1.Width; x++ {
			if f1.Tiles[y][x] != f2.Tiles[y][x] {
				t.Errorf("tile at (%d,%d) differs: %d vs %d", x, y, f1.Tiles[y][x], f2.Tiles[y][x])
			}
		}
	}
}
