package game

import (
	"math/rand"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func charmCount(p *entity.Player) int {
	n := 0
	for _, it := range p.Inventory {
		if it.Type == entity.ItemCharm {
			n++
		}
	}
	return n
}

// TestCharmBlocksGambledHPDamage — a gamble-tagged negative outcome consumes
// the charm and leaves HP untouched.
func TestCharmBlocksGambledHPDamage(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Inventory = append(p.Inventory, entity.NewLuckyCharm(1))
	s.AddPlayer(p)

	before := p.HP
	s.mu.Lock()
	s.applyOutcomeToPlayer(p, EventOutcome{HPChange: -30, Gambled: true})
	s.mu.Unlock()

	if p.HP != before {
		t.Errorf("charm should block gambled damage; HP %d → %d", before, p.HP)
	}
	if charmCount(p) != 0 {
		t.Error("charm should be consumed")
	}
}

// TestCharmBlocksGambledGoldLoss — gold losses from gambles are also covered.
func TestCharmBlocksGambledGoldLoss(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Gold = 100
	p.Inventory = append(p.Inventory, entity.NewLuckyCharm(1))
	s.AddPlayer(p)

	s.mu.Lock()
	s.applyOutcomeToPlayer(p, EventOutcome{GoldChange: -50, Gambled: true})
	s.mu.Unlock()

	if p.Gold != 100 {
		t.Errorf("charm should block gambled gold loss, got %d", p.Gold)
	}
	if charmCount(p) != 0 {
		t.Error("charm should be consumed")
	}
}

// TestCharmIgnoresFixedCosts — TagGamble-free losses (cursed weapon price,
// fixed trades) are prices, not gambles: the charm must not trigger.
func TestCharmIgnoresFixedCosts(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Inventory = append(p.Inventory, entity.NewLuckyCharm(1))
	s.AddPlayer(p)

	before := p.HP
	s.mu.Lock()
	s.applyOutcomeToPlayer(p, EventOutcome{HPChange: -20, Gambled: false})
	s.mu.Unlock()

	if p.HP != before-20 {
		t.Errorf("fixed cost should apply normally; HP %d → %d", before, p.HP)
	}
	if charmCount(p) != 1 {
		t.Error("charm must not be consumed by fixed costs")
	}
}

// TestCharmNotConsumedOnGoodOutcome — winning a gamble leaves the charm alone.
func TestCharmNotConsumedOnGoodOutcome(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Inventory = append(p.Inventory, entity.NewLuckyCharm(1))
	s.AddPlayer(p)

	s.mu.Lock()
	s.applyOutcomeToPlayer(p, EventOutcome{GoldChange: 50, Gambled: true})
	s.mu.Unlock()

	if charmCount(p) != 1 {
		t.Error("charm must survive a winning gamble")
	}
}

// TestCharmPartyEventProtectsOnlyHolders — in a party-wide gambled outcome,
// each victim's own charm (and only theirs) protects them.
func TestCharmPartyEventProtectsOnlyHolders(t *testing.T) {
	s := NewGameSession(testCfg(t))
	holder := entity.NewPlayer("H", entity.ClassWarrior)
	holder.Fingerprint = "fp-h"
	holder.Inventory = append(holder.Inventory, entity.NewLuckyCharm(1))
	bare := entity.NewPlayer("B", entity.ClassMage)
	bare.Fingerprint = "fp-b"
	s.AddPlayer(holder)
	s.AddPlayer(bare)

	holderHP, bareHP := holder.HP, bare.HP
	s.mu.Lock()
	s.applyEventOutcome(EventOutcome{HPChange: -15, Gambled: true})
	s.mu.Unlock()

	if holder.HP != holderHP {
		t.Errorf("holder should be protected; HP %d → %d", holderHP, holder.HP)
	}
	if bare.HP != bareHP-15 {
		t.Errorf("non-holder should take damage; HP %d → %d", bareHP, bare.HP)
	}
	if charmCount(holder) != 0 {
		t.Error("holder's charm should be consumed")
	}
}

// TestCharmPartyDualLossConsumesOneCharm — an outcome with BOTH HP and gold
// losses is one insurance claim: a single charm blocks both, in the party
// path too (regression guard for the double-consumption bug).
func TestCharmPartyDualLossConsumesOneCharm(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Gold = 100
	p.Inventory = append(p.Inventory, entity.NewLuckyCharm(1), entity.NewLuckyCharm(1))
	s.AddPlayer(p)

	hpBefore := p.HP
	s.mu.Lock()
	s.applyEventOutcome(EventOutcome{HPChange: -20, GoldChange: -30, Gambled: true})
	s.mu.Unlock()

	if p.HP != hpBefore {
		t.Errorf("charm should block the HP loss; HP %d → %d", hpBefore, p.HP)
	}
	if p.Gold != 100 {
		t.Errorf("the SAME charm should block the gold loss, got %d", p.Gold)
	}
	if charmCount(p) != 1 {
		t.Errorf("exactly one charm should be consumed for one outcome, %d remain", charmCount(p))
	}
}

// TestCharmGoldSplitRemainderVoided — when the first alive player (who bears
// the remainder) holds a charm, their whole share including the remainder is
// voided, not redistributed to others.
func TestCharmGoldSplitRemainderVoided(t *testing.T) {
	s := NewGameSession(testCfg(t))
	holder := entity.NewPlayer("H", entity.ClassWarrior)
	holder.Fingerprint = "fp-h"
	holder.Gold = 100
	holder.Inventory = append(holder.Inventory, entity.NewLuckyCharm(1))
	bare := entity.NewPlayer("B", entity.ClassMage)
	bare.Fingerprint = "fp-b"
	bare.Gold = 100
	s.AddPlayer(holder)
	s.AddPlayer(bare)

	s.mu.Lock()
	// -25 across 2 alive → share -12, remainder -1 on holder (alive[0]).
	s.applyEventOutcome(EventOutcome{GoldChange: -25, Gambled: true})
	s.mu.Unlock()

	if holder.Gold != 100 {
		t.Errorf("holder's full share incl. remainder should be voided, got %d", holder.Gold)
	}
	if bare.Gold != 88 {
		t.Errorf("non-holder should still pay their -12 share, got %d", bare.Gold)
	}
}

// TestCharmRefundsGambleBet — a failed gambling-den bet refunds the 50G stake.
func TestCharmRefundsGambleBet(t *testing.T) {
	// The gamble outcome is RNG-driven; scan seeds for a losing roll, then
	// verify the charm refunded the stake.
	for seed := int64(0); seed < 100; seed++ {
		s := NewGameSession(testCfg(t))
		s.rng = rand.New(rand.NewSource(seed))
		p := entity.NewPlayer("A", entity.ClassWarrior)
		p.Fingerprint = "fp-a"
		p.Gold = 50
		p.Inventory = append(p.Inventory, entity.NewLuckyCharm(1))
		s.AddPlayer(p)

		s.mu.Lock()
		s.handleGamble()
		s.mu.Unlock()

		if p.Gold == 50 && charmCount(p) == 0 {
			return // losing roll happened and the charm refunded the bet
		}
	}
	t.Fatal("no losing gamble roll found in 100 seeds (or refund failed)")
}

// TestGambledFlagSetFromTags — event resolution marks outcomes from
// TagGamble choices so the charm layer can distinguish gambles from prices.
func TestGambledFlagSetFromTags(t *testing.T) {
	if !tagsContain([]ChoiceTag{TagItem, TagGamble}, TagGamble) {
		t.Error("tagsContain should find TagGamble")
	}
	if tagsContain([]ChoiceTag{TagItem, TagHarm}, TagGamble) {
		t.Error("tagsContain must not find absent tags")
	}
}

// TestShopSellsCharm — the shop stocks a charm in slot 4, repeatedly buyable.
func TestShopSellsCharm(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Gold = 1000
	s.AddPlayer(p)

	s.mu.Lock()
	s.state.FloorNum = 3
	s.generateShopItems()
	s.state.Phase = PhaseShop
	s.mu.Unlock()

	st := s.GetState()
	if len(st.ShopItems) != 4 {
		t.Fatalf("shop should stock 4 items, got %d", len(st.ShopItems))
	}
	charmIdx := -1
	for i, it := range st.ShopItems {
		if it.Type == entity.ItemCharm {
			charmIdx = i
		}
	}
	if charmIdx < 0 {
		t.Fatal("shop should stock a lucky charm")
	}

	if r := s.BuyItem("fp-a", charmIdx); r != BuyOK {
		t.Fatalf("first charm purchase should succeed, got %v", r)
	}
	if r := s.BuyItem("fp-a", charmIdx); r != BuyOK {
		t.Fatalf("charm must be repeatedly buyable, got %v", r)
	}
	if charmCount(s.state.Players[0]) != 2 {
		t.Errorf("player should own 2 charms, got %d", charmCount(s.state.Players[0]))
	}
}

// TestShopWeaponStillSellsOut — regression: gear keeps its one-per-shop rule
// after the sold-out policy change.
func TestShopWeaponStillSellsOut(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	p.Gold = 10000
	s.AddPlayer(p)

	s.mu.Lock()
	s.state.FloorNum = 3
	s.generateShopItems()
	s.state.Phase = PhaseShop
	s.mu.Unlock()

	weaponIdx := -1
	for i, it := range s.GetState().ShopItems {
		if it.Type == entity.ItemWeapon {
			weaponIdx = i
		}
	}
	if weaponIdx < 0 {
		t.Fatal("shop should stock a weapon")
	}
	if r := s.BuyItem("fp-a", weaponIdx); r != BuyOK {
		t.Fatalf("first weapon purchase should succeed, got %v", r)
	}
	if r := s.BuyItem("fp-a", weaponIdx); r == BuyOK {
		t.Error("weapon must sell out after one purchase")
	}
}

// TestWanderingMerchantStocksCharm — the rare merchant carries a charm at 2x.
func TestWanderingMerchantStocksCharm(t *testing.T) {
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("A", entity.ClassWarrior)
	p.Fingerprint = "fp-a"
	s.AddPlayer(p)

	s.mu.Lock()
	s.state.FloorNum = 5
	s.handleWanderingMerchant()
	s.mu.Unlock()

	found := false
	for _, it := range s.GetState().ShopItems {
		if it.Type == entity.ItemCharm {
			found = true
			if want := (30 + 5) * 2; it.Price != want {
				t.Errorf("merchant charm should cost %d (2x), got %d", want, it.Price)
			}
		}
	}
	if !found {
		t.Error("wandering merchant should stock a charm")
	}
}
