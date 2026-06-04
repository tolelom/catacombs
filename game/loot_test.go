package game

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

// --- BuildLootEvent generation ---

// TestBuildLootEventHasThreeChoices enforces the loot table shape: weapon,
// armor, and one rotating third slot.
func TestBuildLootEventHasThreeChoices(t *testing.T) {
	ev := BuildLootEvent(5, rand.New(rand.NewSource(1)))
	if len(ev.Choices) != 3 {
		t.Fatalf("loot event should have 3 choices, got %d", len(ev.Choices))
	}
	for i, c := range ev.Choices {
		if len(c.Tags) == 0 {
			t.Errorf("choice %d (%q) has no Tags", i, c.Label)
		}
		if c.Resolve == nil {
			t.Errorf("choice %d (%q) has nil Resolve", i, c.Label)
		}
	}
}

// TestBuildLootEventRevealsValues — the weapon and armor labels must expose
// the exact bonus so picking is an informed decision, not a slot machine.
func TestBuildLootEventRevealsValues(t *testing.T) {
	ev := BuildLootEvent(8, rand.New(rand.NewSource(7)))

	// Weapon/armor picks are pre-rolled at build time; Resolve must not
	// depend on this rng (fresh throwaway proves it).
	rng := rand.New(rand.NewSource(0))
	weapon := ev.Choices[0].Resolve(8, rng)
	if weapon.Item == nil || weapon.Item.Type != entity.ItemWeapon {
		t.Fatal("choice 0 should grant a weapon")
	}
	if want := fmt.Sprintf("ATK+%d", weapon.Item.Bonus); !strings.Contains(ev.Choices[0].Label, want) {
		t.Errorf("weapon label %q should contain %q", ev.Choices[0].Label, want)
	}

	armor := ev.Choices[1].Resolve(8, rng)
	if armor.Item == nil || armor.Item.Type != entity.ItemArmor {
		t.Fatal("choice 1 should grant an armor")
	}
	if want := fmt.Sprintf("DEF+%d", armor.Item.Bonus); !strings.Contains(ev.Choices[1].Label, want) {
		t.Errorf("armor label %q should contain %q", ev.Choices[1].Label, want)
	}
}

// TestBuildLootEventDeterministic — same seed, same floor → identical labels.
// Daily-challenge runs depend on this.
func TestBuildLootEventDeterministic(t *testing.T) {
	a := BuildLootEvent(10, rand.New(rand.NewSource(42)))
	b := BuildLootEvent(10, rand.New(rand.NewSource(42)))
	for i := range a.Choices {
		if a.Choices[i].Label != b.Choices[i].Label {
			t.Errorf("choice %d labels differ: %q vs %q", i, a.Choices[i].Label, b.Choices[i].Label)
		}
	}
}

// TestBuildLootEventRotatingSlotVariants — over many seeds the third slot
// must produce all four variants (gold / heal / cursed / mystery).
func TestBuildLootEventRotatingSlotVariants(t *testing.T) {
	seen := make(map[string]bool)
	for seed := int64(0); seed < 200; seed++ {
		ev := BuildLootEvent(5, rand.New(rand.NewSource(seed)))
		c := ev.Choices[2]
		switch {
		case hasTag(c.Tags, TagGoldGain):
			seen["gold"] = true
		case hasTag(c.Tags, TagHeal):
			seen["heal"] = true
		case hasTag(c.Tags, TagHarm):
			seen["cursed"] = true
		case hasTag(c.Tags, TagGamble):
			seen["mystery"] = true
		}
	}
	for _, variant := range []string{"gold", "heal", "cursed", "mystery"} {
		if !seen[variant] {
			t.Errorf("rotating slot variant %q never generated in 200 seeds", variant)
		}
	}
}

func hasTag(tags []ChoiceTag, want ChoiceTag) bool {
	for _, tg := range tags {
		if tg == want {
			return true
		}
	}
	return false
}

// TestLootOutcomesNeverDeferItemRolls — every loot Resolve must return a
// concrete Item (or none), never a deferred ItemDrop roll: the apply step
// must not consume RNG conditionally on inventory state (daily determinism),
// and concrete items share the full-inventory gold-conversion path.
func TestLootOutcomesNeverDeferItemRolls(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		rng := rand.New(rand.NewSource(seed))
		ev := BuildLootEvent(5, rng)
		for i, c := range ev.Choices {
			for trial := 0; trial < 10; trial++ {
				if out := c.Resolve(5, rng); out.ItemDrop {
					t.Fatalf("seed %d choice %d (%q) returned deferred ItemDrop", seed, i, c.Label)
				}
			}
		}
	}
}

// TestCursedWeaponOutcome — the cursed weapon must grant the item AND cost HP.
func TestCursedWeaponOutcome(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		ev := BuildLootEvent(5, rng)
		c := ev.Choices[2]
		if !hasTag(c.Tags, TagHarm) || !hasTag(c.Tags, TagItem) {
			continue
		}
		out := c.Resolve(5, rng)
		if out.Item == nil {
			t.Fatal("cursed weapon outcome should carry an item")
		}
		if out.HPChange >= 0 {
			t.Fatalf("cursed weapon must cost HP, got HPChange=%d", out.HPChange)
		}
		return
	}
	t.Skip("no cursed variant in 200 seeds — covered by rotating-slot test")
}

// --- Treasure room session flow ---

func enterTreasureRoom(t *testing.T, s *GameSession) {
	t.Helper()
	s.mu.Lock()
	s.state.Floor = &dungeon.Floor{
		Rooms:       []*dungeon.Room{{Type: dungeon.RoomTreasure}},
		CurrentRoom: 0,
	}
	s.state.FloorNum = 3
	s.state.Phase = PhaseExploring
	s.mu.Unlock()
	s.EnterRoom(0)
}

// TestTreasureRoomEntersPhaseEventPerPlayer — entering a treasure room must
// park the session in PhaseEvent with PerPlayer set instead of auto-granting.
func TestTreasureRoomEntersPhaseEventPerPlayer(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	s.AddPlayer(p)

	enterTreasureRoom(t, s)

	st := s.GetState()
	if st.Phase != PhaseEvent {
		t.Fatalf("phase should be PhaseEvent, got %v", st.Phase)
	}
	if st.PendingEvent == nil || !st.PendingEvent.PerPlayer {
		t.Fatal("PendingEvent should be populated with PerPlayer=true")
	}
	if len(st.PendingEvent.Choices) != 3 {
		t.Errorf("treasure should offer 3 choices, got %d", len(st.PendingEvent.Choices))
	}
	if st.Floor.Rooms[0].Cleared {
		t.Error("room should not be cleared before picks resolve")
	}
	if len(st.Players[0].Inventory) != 0 {
		t.Error("no items should be auto-granted on entry")
	}
}

// TestTreasureRoomZeroAliveCleared — mirroring the event-room guard, a
// treasure room entered with no alive players clears immediately.
func TestTreasureRoomZeroAliveCleared(t *testing.T) {
	s := NewGameSession(testCfg(t))
	enterTreasureRoom(t, s)
	st := s.GetState()
	if st.Phase == PhaseEvent {
		t.Error("zero-alive entry should not enter PhaseEvent")
	}
	if !st.Floor.Rooms[0].Cleared {
		t.Error("zero-alive treasure room should clear immediately")
	}
}

// TestLootPicksApplyIndividually — two players picking different slots each
// receive only their own outcome.
func TestLootPicksApplyIndividually(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	enterTreasureRoom(t, s)

	if s.SubmitEventVote("fp-a", 0) {
		t.Fatal("first pick should not resolve while second player pending")
	}
	if !s.SubmitEventVote("fp-b", 1) {
		t.Fatal("second pick should trigger resolution")
	}

	st := s.GetState()
	if st.Phase != PhaseExploring {
		t.Fatalf("phase should return to PhaseExploring, got %v", st.Phase)
	}
	if !st.Floor.Rooms[0].Cleared {
		t.Error("room should be cleared after all picks resolve")
	}

	// A picked slot 0 (weapon), B picked slot 1 (armor).
	if len(st.Players[0].Inventory) != 1 || st.Players[0].Inventory[0].Type != entity.ItemWeapon {
		t.Errorf("A should hold exactly the weapon, got %+v", st.Players[0].Inventory)
	}
	if len(st.Players[1].Inventory) != 1 || st.Players[1].Inventory[0].Type != entity.ItemArmor {
		t.Errorf("B should hold exactly the armor, got %+v", st.Players[1].Inventory)
	}
	if st.PendingEvent != nil || st.EventVotes != nil {
		t.Error("pending state must be cleared after resolution")
	}
}

// TestLootSamePickBothReceive — both players choosing the same item slot each
// get their own copy.
func TestLootSamePickBothReceive(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	enterTreasureRoom(t, s)
	_ = s.SubmitEventVote("fp-a", 0)
	if !s.SubmitEventVote("fp-b", 0) {
		t.Fatal("second pick should resolve")
	}
	st := s.GetState()
	for i, p := range st.Players {
		if len(p.Inventory) != 1 || p.Inventory[0].Type != entity.ItemWeapon {
			t.Errorf("player %d should hold the weapon copy, got %+v", i, p.Inventory)
		}
	}
}

// TestLootInventoryFullConvertsToGold — a concrete item pick with a full
// inventory converts to bonus*5 gold (handleGamble jackpot fallback pattern).
func TestLootInventoryFullConvertsToGold(t *testing.T) {
	cfg := testCfg(t)
	s := NewGameSession(cfg)
	p := entity.NewPlayer("full", entity.ClassWarrior)
	p.Fingerprint = "fp-full"
	for i := 0; i < cfg.Game.InventoryLimit; i++ {
		p.Inventory = append(p.Inventory, entity.Item{Name: "junk", Type: entity.ItemConsumable, Bonus: 1})
	}
	p.Gold = 0
	s.AddPlayer(p)

	enterTreasureRoom(t, s)
	st := s.GetState()
	if st.PendingEvent == nil {
		t.Fatal("expected pending loot event")
	}
	if !s.SubmitEventVote("fp-full", 0) {
		t.Fatal("solo pick should resolve immediately")
	}

	st = s.GetState()
	if len(st.Players[0].Inventory) != cfg.Game.InventoryLimit {
		t.Errorf("full inventory must not grow, got %d items", len(st.Players[0].Inventory))
	}
	if st.Players[0].Gold <= 0 {
		t.Errorf("full-inventory pick should convert to gold, got %dG", st.Players[0].Gold)
	}
}

// TestLootVotelessAlivePlayerSkipped — an alive player with no recorded pick
// (disconnect-sweep edge) is skipped during per-player resolution.
func TestLootVotelessAlivePlayerSkipped(t *testing.T) {
	s := NewGameSession(testCfg(t))
	a := entity.NewPlayer("A", entity.ClassWarrior)
	a.Fingerprint = "fp-a"
	b := entity.NewPlayer("B", entity.ClassMage)
	b.Fingerprint = "fp-b"
	s.AddPlayer(a)
	s.AddPlayer(b)

	enterTreasureRoom(t, s)
	_ = s.SubmitEventVote("fp-a", 0)

	// Force-resolve with only one vote recorded (sweep path calls
	// resolveEventVoteLocked directly after cleaning stale votes).
	s.mu.Lock()
	s.resolveEventVoteLocked()
	s.mu.Unlock()

	st := s.GetState()
	if len(st.Players[0].Inventory) != 1 {
		t.Errorf("voter should receive their pick, got %d items", len(st.Players[0].Inventory))
	}
	if len(st.Players[1].Inventory) != 0 {
		t.Errorf("voteless player should be skipped, got %d items", len(st.Players[1].Inventory))
	}
	if st.Phase != PhaseExploring || !st.Floor.Rooms[0].Cleared {
		t.Error("session should exit PhaseEvent with the room cleared")
	}
}

// TestLootLethalOutcomeSetsAnyPlayerDied — a loot pick that kills (cursed
// weapon / mystery-box trap) must flip AnyPlayerDied for the no_death
// achievement.
func TestLootLethalOutcomeSetsAnyPlayerDied(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("fragile", entity.ClassWarrior)
	p.Fingerprint = "fp-fragile"
	p.HP = 1
	s.AddPlayer(p)

	s.mu.Lock()
	s.applyOutcomeToPlayer(p, EventOutcome{HPChange: -100})
	died := s.state.AnyPlayerDied
	s.mu.Unlock()

	if !died {
		t.Error("AnyPlayerDied should be set after lethal loot damage")
	}
}

// TestApplyOutcomeToPlayerGoldClampsToZero mirrors the party-wide clamp.
func TestApplyOutcomeToPlayerGoldClampsToZero(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("broke", entity.ClassWarrior)
	p.Fingerprint = "fp-broke"
	p.Gold = 3
	s.AddPlayer(p)

	s.mu.Lock()
	s.applyOutcomeToPlayer(p, EventOutcome{GoldChange: -50})
	s.mu.Unlock()

	if p.Gold != 0 {
		t.Errorf("gold must clamp to 0, got %d", p.Gold)
	}
}

// TestEventVotesStillResolveByMajority — regression guard: the party-vote
// path (random events) must keep majority semantics, not per-player.
func TestEventVotesStillResolveByMajority(t *testing.T) {
	s := NewGameSession(testCfg(t))
	for _, fp := range []string{"fp-a", "fp-b", "fp-c"} {
		p := entity.NewPlayer(fp, entity.ClassWarrior)
		p.Fingerprint = fp
		p.HP = p.MaxHP - 40
		s.AddPlayer(p)
	}
	enterDeterministicEvent(t, s, "shrine")

	_ = s.SubmitEventVote("fp-a", 0) // heal
	_ = s.SubmitEventVote("fp-b", 0) // heal
	if !s.SubmitEventVote("fp-c", 1) {
		t.Fatal("3rd vote should resolve")
	}
	// Majority picked heal → ALL players (including fp-c) healed.
	for _, p := range s.GetState().Players {
		if p.HP <= p.MaxHP-40 {
			t.Errorf("party-wide heal should apply to %s, HP=%d", p.Fingerprint, p.HP)
		}
	}
}
