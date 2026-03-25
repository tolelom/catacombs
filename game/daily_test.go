package game

import (
	"math/rand"
	"testing"

	"github.com/tolelom/catacombs/dungeon"
)

func TestDailySeed(t *testing.T) {
	// Same date produces the same seed
	seed1 := DailySeed("2026-03-25")
	seed2 := DailySeed("2026-03-25")
	if seed1 != seed2 {
		t.Errorf("same date should produce same seed: got %d and %d", seed1, seed2)
	}

	// Different date produces a different seed
	seed3 := DailySeed("2026-03-26")
	if seed1 == seed3 {
		t.Errorf("different dates should produce different seeds: both got %d", seed1)
	}
}

func TestDailyFloorDeterminism(t *testing.T) {
	seed := DailySeed("2026-03-25")

	// Generate floor twice with the same seed
	floor1 := dungeon.GenerateFloor(1, rand.New(rand.NewSource(seed)))
	floor2 := dungeon.GenerateFloor(1, rand.New(rand.NewSource(seed)))

	if len(floor1.Rooms) != len(floor2.Rooms) {
		t.Fatalf("room count mismatch: %d vs %d", len(floor1.Rooms), len(floor2.Rooms))
	}

	for i, r1 := range floor1.Rooms {
		r2 := floor2.Rooms[i]
		if r1.Type != r2.Type {
			t.Errorf("room %d type mismatch: %d vs %d", i, r1.Type, r2.Type)
		}
	}
}
