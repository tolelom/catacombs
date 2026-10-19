package dungeon

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Golden traces pinning floor generation and map rendering before a refactor.
// The trace records every room, the tile map, the next RNG value (same value
// means the same number and order of RNG draws) and the rendered map with and
// without fog. Regenerate: go test ./dungeon/ -run Pin -update

var updateGolden = flag.Bool("update", false, "rewrite golden files")

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Errorf("%s differs from golden; rerun with -update only if the change is intended", name)
	}
}

func describeFloor(b *strings.Builder, floor *Floor) {
	for i, r := range floor.Rooms {
		fmt.Fprintf(b, "room %d type=%d x=%d y=%d w=%d h=%d neighbors=%v\n", i, r.Type, r.X, r.Y, r.W, r.H, r.Neighbors)
	}
	for _, row := range floor.Tiles {
		for _, tile := range row {
			b.WriteString(string("#.+/"[tile]))
		}
		b.WriteString("\n")
	}
}

func describeRender(b *strings.Builder, label, rendered string) {
	fmt.Fprintf(b, "-- %s\n", label)
	for _, line := range strings.Split(rendered, "\n") {
		fmt.Fprintf(b, "%q\n", line)
	}
}

// TestPinGenerateAndRenderFloor pins the dungeon layout and map a player sees.
// If it breaks, the same daily seed builds a different dungeon or the map
// draws differently.
func TestPinGenerateAndRenderFloor(t *testing.T) {
	var b strings.Builder
	for _, seed := range []int64{42} {
		for _, floorNum := range []int{1, 4, 9, 14, 20} {
			rng := rand.New(rand.NewSource(seed))
			floor := GenerateFloor(floorNum, rng)
			fmt.Fprintf(&b, "== seed=%d floor=%d next=%d\n", seed, floorNum, rng.Int63())
			describeFloor(&b, floor)

			describeRender(&b, "no fog", RenderFloor(floor, floor.CurrentRoom, false))
			UpdateVisibility(floor)
			describeRender(&b, "fog at start", RenderFloor(floor, floor.CurrentRoom, true))

			floor.Rooms[0].Cleared = true
			floor.CurrentRoom = len(floor.Rooms) - 1
			UpdateVisibility(floor)
			describeRender(&b, "fog at last room", RenderFloor(floor, floor.CurrentRoom, true))
			describeRender(&b, "fog with no current room", RenderFloor(floor, -1, true))
		}
	}
	checkGolden(t, "generate_and_render_floor", b.String())
}
