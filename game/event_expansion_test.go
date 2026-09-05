package game

import (
	"math/rand"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

// TestTwelveUniqueEvents — the pool grew to 12 with unique names.
func TestTwelveUniqueEvents(t *testing.T) {
	events := GetRandomEvents()
	if len(events) != 12 {
		t.Fatalf("expected 12 events, got %d", len(events))
	}
	seen := make(map[string]bool)
	for _, ev := range events {
		if seen[ev.Name] {
			t.Errorf("duplicate event name %q", ev.Name)
		}
		seen[ev.Name] = true
	}
	for _, want := range []string{"gravekeeper", "forge", "toll_bridge", "feast"} {
		if !seen[want] {
			t.Errorf("new event %q missing", want)
		}
	}
}

func findEvent(t *testing.T, name string) RandomEvent {
	t.Helper()
	for _, ev := range GetRandomEvents() {
		if ev.Name == name {
			return ev
		}
	}
	t.Fatalf("event %q not found", name)
	return RandomEvent{}
}

// TestGravekeeperDigBothBranches — the dig gamble yields a set item on
// success and HP damage on failure, across seeds.
func TestGravekeeperDigBothBranches(t *testing.T) {
	ev := findEvent(t, "gravekeeper")
	dig := ev.Choices[0]
	if !tagsContain(dig.Tags, TagGamble) {
		t.Fatal("dig must be tagged TagGamble (charm coverage)")
	}
	gotItem, gotHarm := false, false
	for seed := int64(0); seed < 60; seed++ {
		out := dig.Resolve(5, rand.New(rand.NewSource(seed)))
		if out.Item != nil {
			if out.Item.Set == entity.SetNone {
				t.Error("dig success should yield a SET item")
			}
			gotItem = true
		} else if out.HPChange < 0 {
			gotHarm = true
		} else {
			t.Errorf("seed %d: dig produced neither item nor harm: %+v", seed, out)
		}
	}
	if !gotItem || !gotHarm {
		t.Errorf("both branches should be reachable; item=%v harm=%v", gotItem, gotHarm)
	}
}

// TestForgePremiumWeapon — forge success yields a weapon strictly above the
// regular roll ceiling is not guaranteed, but min must exceed regular min.
func TestForgePremiumWeapon(t *testing.T) {
	ev := findEvent(t, "forge")
	rummage := ev.Choices[0]
	for seed := int64(0); seed < 60; seed++ {
		out := rummage.Resolve(5, rand.New(rand.NewSource(seed)))
		if out.Item == nil {
			continue
		}
		if out.Item.Type != entity.ItemWeapon {
			t.Fatal("forge should yield a weapon")
		}
		// premium floor: 5 + floor/3 (regular is 3 + floor/3)
		if min := 5 + 5/3; out.Item.Bonus < min {
			t.Errorf("forge weapon bonus %d below premium floor %d", out.Item.Bonus, min)
		}
		return
	}
	t.Fatal("no forge success in 60 seeds")
}

// TestFeastTakeawayGrantsPotionPartyWide — the concrete-Item grant works
// through the PARTY apply path: every alive player receives the potion.
func TestFeastTakeawayGrantsPotionPartyWide(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		s.AddPlayer(p)
	}
	ev := findEvent(t, "feast")
	takeaway := ev.Choices[1]
	out := takeaway.Resolve(5, rand.New(rand.NewSource(1)))
	if out.Item == nil || out.Item.Type != entity.ItemConsumable {
		t.Fatalf("takeaway should grant a potion, got %+v", out)
	}

	s.mu.Lock()
	s.applyEventOutcome(out)
	s.mu.Unlock()

	for i, p := range s.state.Players {
		if len(p.Inventory) != 1 || p.Inventory[0].Type != entity.ItemConsumable {
			t.Errorf("player %d should hold the potion, got %+v", i, p.Inventory)
		}
	}
}

// TestPartyItemGrantFullInventoryConvertsToGold — party-path Item grants use
// the same bonus*5 fallback as the per-player path.
func TestPartyItemGrantFullInventoryConvertsToGold(t *testing.T) {
	cfg := testCfg(t)
	s := NewGameSession(cfg)
	p := entity.NewPlayer("full", entity.ClassWarrior)
	p.Fingerprint = "fp-full"
	for i := 0; i < cfg.Game.InventoryLimit; i++ {
		p.Inventory = append(p.Inventory, entity.Item{Name: "junk", Type: entity.ItemConsumable, Bonus: 1})
	}
	p.Gold = 0
	s.AddPlayer(p)

	item := entity.Item{Name: "thing", Type: entity.ItemWeapon, Bonus: 6}
	s.mu.Lock()
	s.applyEventOutcome(EventOutcome{Item: &item})
	s.mu.Unlock()

	// Assert via the session's player list, not the local pointer, so the
	// test stays valid if AddPlayer ever copies.
	got := s.state.Players[0]
	if len(got.Inventory) != cfg.Game.InventoryLimit {
		t.Error("full inventory must not grow")
	}
	if got.Gold != 30 {
		t.Errorf("full-inventory grant should convert to bonus*5=30G, got %d", got.Gold)
	}
}

// TestCharmBlocksGravekeeperWrath — end-to-end: a charm holder digging the
// grave and rolling the wrath branch takes no damage.
func TestCharmBlocksGravekeeperWrath(t *testing.T) {
	ev := findEvent(t, "gravekeeper")
	dig := ev.Choices[0]
	for seed := int64(0); seed < 60; seed++ {
		out := dig.Resolve(5, rand.New(rand.NewSource(seed)))
		if out.HPChange >= 0 {
			continue
		}
		out.Gambled = tagsContain(dig.Tags, TagGamble)

		s := NewGameSession(testCfg(t))
		p := entity.NewPlayer("A", entity.ClassWarrior)
		p.Fingerprint = "fp-a"
		p.Inventory = append(p.Inventory, entity.NewLuckyCharm(1))
		s.AddPlayer(p)
		before := p.HP

		s.mu.Lock()
		s.applyEventOutcome(out)
		s.mu.Unlock()

		if p.HP != before {
			t.Errorf("charm should block the wrath; HP %d → %d", before, p.HP)
		}
		if charmCount(p) != 0 {
			t.Error("charm should be consumed")
		}
		return
	}
	t.Fatal("no wrath roll in 60 seeds")
}

// TestTollBridgeShape — three choices with the expected risk profile.
func TestTollBridgeShape(t *testing.T) {
	ev := findEvent(t, "toll_bridge")
	if len(ev.Choices) != 3 {
		t.Fatalf("toll_bridge should have 3 choices, got %d", len(ev.Choices))
	}
	// Choice 0: paying the toll always costs gold, never HP.
	for seed := int64(0); seed < 20; seed++ {
		out := ev.Choices[0].Resolve(5, rand.New(rand.NewSource(seed)))
		if out.GoldChange >= 0 || out.HPChange != 0 {
			t.Fatalf("toll should be a pure gold cost, got %+v", out)
		}
	}
	// Choice 1: sneaking either passes clean or hurts.
	clean, hurt := false, false
	for seed := int64(0); seed < 60; seed++ {
		out := ev.Choices[1].Resolve(5, rand.New(rand.NewSource(seed)))
		if out.HPChange < 0 {
			hurt = true
		} else if out.HPChange == 0 && out.GoldChange == 0 {
			clean = true
		}
	}
	if !clean || !hurt {
		t.Errorf("sneak branches unreachable; clean=%v hurt=%v", clean, hurt)
	}
}
