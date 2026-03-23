package dungeon

import "testing"

func TestGenerateFloor(t *testing.T) {
	floor := GenerateFloor(1)
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
	for i := 0; i < n; i++ {
		counts[RandomRoomType()]++
	}
	combatPct := float64(counts[RoomCombat]) / float64(n) * 100
	if combatPct < 40 || combatPct > 50 {
		t.Errorf("Combat room probability: got %.1f%%, want ~45%%", combatPct)
	}
}
