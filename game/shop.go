package game

import (
	"github.com/tolelom/catacombs/entity"
)

// BuyResult describes the outcome of a shop purchase attempt.
type BuyResult int

const (
	BuyOK BuyResult = iota
	BuyNoGold
	BuyInventoryFull
	BuyFailed
)

// BuyItem handles shop purchases
func (s *GameSession) BuyItem(playerID string, itemIdx int) BuyResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop || itemIdx < 0 || itemIdx >= len(s.state.ShopItems) {
		return BuyFailed
	}
	if itemIdx < len(s.state.ShopSold) && s.state.ShopSold[itemIdx] {
		return BuyFailed
	}
	item := s.state.ShopItems[itemIdx]
	for _, p := range s.state.Players {
		if p.Fingerprint == playerID {
			if p.Gold < item.Price {
				return BuyNoGold
			}
			if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
				return BuyInventoryFull
			}
			p.Gold -= item.Price
			p.Inventory = append(p.Inventory, item)
			// Only gear sells out — potions and charms restock freely.
			if (item.Type == entity.ItemWeapon || item.Type == entity.ItemArmor) && itemIdx < len(s.state.ShopSold) {
				s.state.ShopSold[itemIdx] = true
			}
			return BuyOK
		}
	}
	return BuyFailed
}

// tallyShopLeaversLocked returns true when every available (non-out)
// player has signaled they are ready to leave the shop. Treats a zero-
// available party as ready so a wipe in the shop doesn't deadlock.
// Empty-fingerprint players are counted through the "" key rather than
// skipped, so their quorum slot stays fillable. Caller holds s.mu.
func (s *GameSession) tallyShopLeaversLocked() bool {
	available := 0
	ready := 0
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		available++
		if s.shopLeavers[p.Fingerprint] {
			ready++
		}
	}
	if available == 0 {
		return true
	}
	return ready >= available
}

// LeaveShop marks the calling player as ready to leave the shop. The phase
// only flips back to PhaseExploring once every available (non-out) player
// has pressed leave, so one impatient player can't yank others out mid-browse.
// Returns true if the call triggered the actual transition.
func (s *GameSession) LeaveShop(fingerprint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop {
		return false
	}
	if s.shopLeavers == nil {
		s.shopLeavers = make(map[string]bool)
	}
	// Empty-fingerprint players (legacy/test path) signal under the "" key,
	// mirroring move votes — excluding them entirely would make their quorum
	// slot unfillable and deadlock the shop for the whole party.
	s.shopLeavers[fingerprint] = true
	if !s.tallyShopLeaversLocked() {
		return false
	}
	s.shopLeavers = nil
	s.state.Phase = PhaseExploring
	if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
		s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
	}
	return true
}
