package combat

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

// Regenerate the golden file: go test ./combat/ -run Pin -update
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

func pinMonsters() []*entity.Monster {
	slime := entity.NewMonster(entity.MonsterSlime, 3, 1.15)
	orc := entity.NewMonster(entity.MonsterOrc, 3, 1.15)
	boss := entity.NewMonster(entity.MonsterBoss15, 15, 1.15)
	boss.Vulnerable = true
	miniBoss := entity.NewMonster(entity.MonsterMiniBoss5, 4, 1.15)
	corpse := entity.NewMonster(entity.MonsterSkeleton, 3, 1.15)
	corpse.HP = 0
	return []*entity.Monster{slime, orc, boss, miniBoss, corpse}
}

func pinIntents() []AttackIntent {
	return []AttackIntent{
		{PlayerATK: 20, TargetIdx: 0, Multiplier: 1.0},
		{PlayerATK: 25, TargetIdx: 0, Multiplier: 1.2},
		{PlayerATK: 30, TargetIdx: 1, Multiplier: 1.0},
		{PlayerATK: 18, TargetIdx: 2, Multiplier: 1.5},
		{PlayerATK: 22, TargetIdx: 2, Multiplier: 1.0},
		{PlayerATK: 15, TargetIdx: 3, Multiplier: 1.0},
		{PlayerATK: 12, TargetIdx: 4, Multiplier: 1.0},
		{PlayerATK: 12, TargetIdx: 9, Multiplier: 1.0},
		{PlayerATK: 10, TargetIdx: -1, Multiplier: 0.8, IsAoE: true},
	}
}

// TestPinResolveAttacks pins the damage every player attack deals, including
// dodge/glance rolls, the coop bonus, vulnerable bosses and AoE. If it breaks,
// the same daily seed plays out a different fight.
func TestPinResolveAttacks(t *testing.T) {
	var b strings.Builder
	for seed := int64(1); seed <= 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		monsters := pinMonsters()
		results := ResolveAttacks(rng, pinIntents(), monsters, 0.10)
		fmt.Fprintf(&b, "seed=%d next=%d\n", seed, rng.Int63())
		for i, r := range results {
			fmt.Fprintf(&b, "  result %d %+v\n", i, r)
		}
		for i, m := range monsters {
			fmt.Fprintf(&b, "  monster %d hp=%d\n", i, m.HP)
		}
	}
	checkGolden(t, "resolve_attacks", b.String())
}
