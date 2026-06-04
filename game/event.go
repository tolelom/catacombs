package game

import (
	"fmt"
	"time"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) EnterRoom(roomIdx int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, p := range s.state.Players {
		if p.Fingerprint != "" {
			s.lastActivity[p.Fingerprint] = now
		}
	}

	s.enterRoomLocked(roomIdx)
}

// enterRoomLocked performs room entry logic. Caller must hold s.mu.
func (s *GameSession) enterRoomLocked(roomIdx int) {
	if s.state.Floor == nil || roomIdx < 0 || roomIdx >= len(s.state.Floor.Rooms) {
		return
	}
	s.nextRoomPreview = nil
	s.state.Floor.CurrentRoom = roomIdx
	dungeon.UpdateVisibility(s.state.Floor)
	room := s.state.Floor.Rooms[roomIdx]

	if room.Cleared {
		return
	}

	switch room.Type {
	case dungeon.RoomCombat:
		s.spawnMonsters()
		s.state.Phase = PhaseCombat
		s.state.CombatTurn = 0
		s.signalCombat()
	case dungeon.RoomBoss:
		s.spawnBoss()
		s.state.Phase = PhaseCombat
		s.state.CombatTurn = 0
		s.signalCombat()
	case dungeon.RoomShop:
		if s.hasMutation("no_shop") {
			s.addLog("상점이 닫혔습니다! (주간 변이)")
			room.Cleared = true
			return
		}
		s.generateShopItems()
		s.state.Phase = PhaseShop
	case dungeon.RoomTreasure:
		s.triggerLoot()
		// room.Cleared flipped after all picks resolve in resolveEventVote
	case dungeon.RoomEvent:
		s.triggerEvent()
		// room.Cleared flipped after vote quorum in resolveEventVote
	case dungeon.RoomSecret:
		s.grantSecretTreasure()
		room.Cleared = true
	case dungeon.RoomMiniBoss:
		s.spawnMiniBoss()
		s.state.Phase = PhaseCombat
		s.state.CombatTurn = 0
		s.signalCombat()
	case dungeon.RoomEmpty:
		room.Cleared = true
	case dungeon.RoomMimic:
		s.handleMimic(room)
	case dungeon.RoomGamble:
		s.handleGamble()
		room.Cleared = true
	case dungeon.RoomCursedAltar:
		s.handleCursedAltar()
		room.Cleared = true
	case dungeon.RoomWanderingMerchant:
		s.handleWanderingMerchant()
		s.state.Phase = PhaseShop
	}
}

// handleMimic: 50% it's actually a treasure, 50% an ambush by a tough monster
// (1.5x stat scaling). The room is marked Cleared on the treasure path; combat
// path defers to combat clear.
func (s *GameSession) handleMimic(room *dungeon.Room) {
	if s.rng.Float64() < 0.5 {
		s.addLog("미믹! 보물 상자가 깨어났습니다!")
		floor := s.state.FloorNum
		mimic := entity.NewMonster(entity.MonsterDarkKnight, floor, s.cfg.Combat.MonsterScaling)
		mimic.Name = "Mimic"
		mimic.HP = int(float64(mimic.HP) * 1.5)
		mimic.MaxHP = mimic.HP
		mimic.ATK = int(float64(mimic.ATK) * 1.3)
		s.applyDifficultyMods(mimic)
		s.state.Monsters = []*entity.Monster{mimic}
		s.state.Phase = PhaseCombat
		s.state.CombatTurn = 0
		// Match other combat-start sites: reset every per-combat map and
		// apply pending Venomous Edge poison. Without this, Time Glass's
		// extraActionNext would carry over from the prior combat into the
		// mimic fight (free attack), and Venomous Edge would skip the mimic.
		s.coopUsedThisCombat = make(map[string]bool)
		s.activeRelicUsed = make(map[string]bool)
		s.extraActionNext = make(map[string]bool)
		s.applyPoisonNextCombatIfPending()
		s.signalCombat()
		for _, p := range s.state.Players {
			p.SkillUses = s.effectiveSkillUses()
		}
	} else {
		s.addLog("진짜 보물 상자였습니다!")
		s.grantTreasure()
		room.Cleared = true
	}
}

// handleGamble: each player can bet 50 gold. Outcome:
// - 50%: lose the 50 gold
// - 30%: win 100 gold (+50 net)
// - 20%: win a random item
func (s *GameSession) handleGamble() {
	s.addLog("도박장 발견! 골드 50을 베팅하면 운에 따라 보상.")
	for _, p := range s.state.Players {
		if p.IsOut() || p.Gold < 50 {
			continue
		}
		p.Gold -= 50
		roll := s.rng.Float64()
		switch {
		case roll < 0.50:
			s.addLog(fmt.Sprintf("%s 베팅 실패! -50G", p.Name))
		case roll < 0.80:
			p.Gold += 100
			s.addLog(fmt.Sprintf("%s 베팅 성공! +50G", p.Name))
		default:
			if len(p.Inventory) < s.cfg.Game.InventoryLimit {
				floor := s.state.FloorNum
				bonus := 4 + s.rng.Intn(6) + floor/3
				item := entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("%s 잭팟! %s 획득 (ATK+%d)", p.Name, item.Name, item.Bonus))
			} else {
				p.Gold += 150
				s.addLog(fmt.Sprintf("%s 잭팟! 인벤 가득 → +150G", p.Name))
			}
		}
	}
}

// handleCursedAltar: each player loses 30% MaxHP, gains random permanent
// stat boost (ATK+5 / DEF+3 / Gold+100).
func (s *GameSession) handleCursedAltar() {
	s.state.AltarUsedThisRun++
	s.addLog("저주받은 제단! 생명력을 바쳐 힘을 얻습니다.")
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		cost := p.MaxHP * 30 / 100
		p.HP -= cost
		if p.HP < 1 {
			p.HP = 1
		}
		switch s.rng.Intn(3) {
		case 0:
			p.ATK += 5
			s.addLog(fmt.Sprintf("%s -%dHP, +5 ATK 영구!", p.Name, cost))
		case 1:
			p.DEF += 3
			s.addLog(fmt.Sprintf("%s -%dHP, +3 DEF 영구!", p.Name, cost))
		case 2:
			p.Gold += 100
			s.addLog(fmt.Sprintf("%s -%dHP, +100G!", p.Name, cost))
		}
	}
}

// handleWanderingMerchant: rare items at 2x normal price (one-time stock).
func (s *GameSession) handleWanderingMerchant() {
	floor := s.state.FloorNum
	weaponBonus := 6 + s.rng.Intn(6) + floor/3
	armorBonus := 4 + s.rng.Intn(4) + floor/4
	weaponPrice := (40 + weaponBonus*5) * 2
	armorPrice := (30 + armorBonus*5) * 2
	potionPrice := (20 + floor/2) * 2

	s.state.ShopItems = []entity.Item{
		{Name: "Greater HP Potion", Type: entity.ItemConsumable, Bonus: 60 + floor*2, Price: potionPrice},
		{Name: "Rare " + weaponName(floor), Type: entity.ItemWeapon, Bonus: weaponBonus, Price: weaponPrice},
		{Name: "Rare " + armorName(floor), Type: entity.ItemArmor, Bonus: armorBonus, Price: armorPrice},
	}
	s.state.ShopSold = make([]bool, len(s.state.ShopItems))
	s.addLog("방랑상인이 희귀 아이템을 들고 나타났습니다! (가격 2배)")
}

// applySoloScaling weakens a monster for single-player runs: HP and DEF are
// reduced by SoloHPReduction (squishier) and ATK by SoloATKReduction (hits
// softer). The ATK cut compensates for a solo party having no healer sustain
// or coop combos — without it, fragile classes cannot survive late bosses long
// enough to kill them. No-op outside solo mode.
func (s *GameSession) applySoloScaling(m *entity.Monster) {
	if !s.state.SoloMode {
		return
	}
	m.HP = int(float64(m.HP) * s.cfg.Combat.SoloHPReduction)
	if m.HP < 1 {
		m.HP = 1
	}
	m.MaxHP = m.HP
	m.DEF = int(float64(m.DEF) * s.cfg.Combat.SoloHPReduction)
	m.ATK = int(float64(m.ATK) * s.cfg.Combat.SoloATKReduction)
	if m.ATK < 1 {
		m.ATK = 1
	}
}

// earlyThreatMult returns a HP/ATK multiplier that compresses the trivial
// early-game "warmup". It is a smooth bump: 1.0 on the onboarding floors (1-2),
// rising to a mid-early peak (~floor 6), then tapering back to 1.0 by floor 10
// where the exponential min-floor scaling takes over. This brings tension
// forward to ~floor 4-5 without creating a difficulty discontinuity at floor 10.
// EarlyThreatRamp == 0 disables it entirely.
func (s *GameSession) earlyThreatMult(floor int) float64 {
	const fadeFloor = 10 // ramp returns to 1.0 here, merging into base scaling
	steps := floor - 2
	if fade := fadeFloor - floor; fade < steps {
		steps = fade
	}
	if steps <= 0 {
		return 1.0
	}
	return 1.0 + s.cfg.Combat.EarlyThreatRamp*float64(steps)
}

// applyEarlyThreat scales a regular monster's HP and ATK (not DEF — the
// subtractive damage formula makes DEF lifts disproportionately punishing) by
// earlyThreatMult. No-op once the multiplier is 1.0.
func (s *GameSession) applyEarlyThreat(m *entity.Monster, floor int) {
	mult := s.earlyThreatMult(floor)
	if mult == 1.0 {
		return
	}
	m.HP = int(float64(m.HP) * mult)
	if m.HP < 1 {
		m.HP = 1
	}
	m.MaxHP = m.HP
	m.ATK = int(float64(m.ATK) * mult)
	if m.ATK < 1 {
		m.ATK = 1
	}
}

func (s *GameSession) spawnMonsters() {
	count := 1 + s.rng.Intn(5)
	floor := s.state.FloorNum
	s.state.Monsters = make([]*entity.Monster, count)

	type floorRange struct {
		mt       entity.MonsterType
		minFloor int
		maxFloor int
	}
	ranges := []floorRange{
		{entity.MonsterSlime, 1, 5},
		{entity.MonsterSkeleton, 3, 10},
		{entity.MonsterOrc, 6, 14},
		{entity.MonsterDarkKnight, 12, 20},
	}
	var valid []entity.MonsterType
	for _, r := range ranges {
		if floor >= r.minFloor && floor <= r.maxFloor {
			valid = append(valid, r.mt)
		}
	}
	if len(valid) == 0 {
		valid = []entity.MonsterType{entity.MonsterSlime}
	}

	for i := 0; i < count; i++ {
		mt := valid[s.rng.Intn(len(valid))]
		m := entity.NewMonster(mt, floor, s.cfg.Combat.MonsterScaling)
		s.applyEarlyThreat(m, floor)
		s.applySoloScaling(m)
		if s.hasMutation("elite_flood") || s.rng.Float64() < 0.20 {
			entity.ApplyPrefix(m, entity.RandomPrefix(s.rng))
		}
		s.applyDifficultyMods(m)
		s.state.Monsters[i] = m
	}

	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.applyPoisonNextCombatIfPending()
}

func (s *GameSession) spawnBoss() {
	var mt entity.MonsterType
	switch s.state.FloorNum {
	case 5:
		mt = entity.MonsterBoss5
	case 10:
		mt = entity.MonsterBoss10
	case 15:
		mt = entity.MonsterBoss15
	case 20:
		mt = entity.MonsterBoss20
	default:
		mt = entity.MonsterBoss5
	}
	boss := entity.NewMonster(mt, s.state.FloorNum, s.cfg.Combat.MonsterScaling)
	switch mt {
	case entity.MonsterBoss5:
		boss.Pattern = entity.PatternPoison // Swamp theme
	case entity.MonsterBoss10:
		boss.Pattern = entity.PatternBurn // Volcano theme
	case entity.MonsterBoss15:
		boss.Pattern = entity.PatternFreeze // Glacier theme
	case entity.MonsterBoss20:
		boss.Pattern = entity.PatternHeal // Inferno theme (+ natural AoE every 3 turns)
	}
	s.applySoloScaling(boss)
	s.applyDifficultyMods(boss)
	s.state.Monsters = []*entity.Monster{boss}

	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.applyPoisonNextCombatIfPending()
}

// applyPoisonNextCombatIfPending applies the Venomous Edge combo's effect at
// combat start: every spawned monster gets a 3-turn poison stack.
// Caller holds s.mu.
func (s *GameSession) applyPoisonNextCombatIfPending() {
	if !s.poisonNextCombat {
		return
	}
	for _, m := range s.state.Monsters {
		poisonValue := m.MaxHP * 5 / 100
		if poisonValue < 2 {
			poisonValue = 2
		}
		m.AddEffect(entity.ActiveEffect{Type: entity.StatusPoison, Duration: 3, Value: poisonValue})
	}
	s.addLog("독칼: 모든 적이 3턴간 독에 중독!")
	s.poisonNextCombat = false
}

func (s *GameSession) grantTreasure() {
	floor := s.state.FloorNum
	for _, p := range s.state.Players {
		if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
			s.addLog(fmt.Sprintf("%s의 인벤토리가 가득 찼습니다!", p.Name))
			continue
		}
		// 10% chance: drop a set item instead of regular gear.
		if s.rng.Float64() < 0.10 {
			item := randomSetItem(s.rng, floor)
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s 희귀 발견: %s (Bonus+%d)", p.Name, item.Name, item.Bonus))
			continue
		}
		if s.rng.Float64() < 0.5 {
			bonus := 3 + s.rng.Intn(6) + floor/3
			item := entity.Item{
				Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus,
			}
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
		} else {
			bonus := 2 + s.rng.Intn(4) + floor/4
			item := entity.Item{
				Name: armorName(floor), Type: entity.ItemArmor, Bonus: bonus,
			}
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s %s 발견 (DEF+%d)", p.Name, item.Name, item.Bonus))
		}
	}
}

func (s *GameSession) generateShopItems() {
	floor := s.state.FloorNum
	// Weapon bonus scales: base 3-8 + floor/3
	weaponBonus := 3 + s.rng.Intn(6) + floor/3
	// Armor bonus scales: base 2-5 + floor/4
	armorBonus := 2 + s.rng.Intn(4) + floor/4
	// Prices scale with power
	weaponPrice := 40 + weaponBonus*5
	armorPrice := 30 + armorBonus*5
	// Potion heals more on higher floors
	potionHeal := 30 + floor
	potionPrice := 20 + floor/2

	if s.HardMode {
		mult := s.cfg.Difficulty.HardModeShopMult
		potionPrice = int(float64(potionPrice) * mult)
		weaponPrice = int(float64(weaponPrice) * mult)
		armorPrice = int(float64(armorPrice) * mult)
	}
	s.state.ShopItems = []entity.Item{
		{Name: "HP Potion", Type: entity.ItemConsumable, Bonus: potionHeal, Price: potionPrice},
		{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: weaponBonus, Price: weaponPrice},
		{Name: armorName(floor), Type: entity.ItemArmor, Bonus: armorBonus, Price: armorPrice},
	}
	s.state.ShopSold = make([]bool, len(s.state.ShopItems))
}

func weaponName(floor int) string {
	switch {
	case floor >= 15:
		return "Mythril Blade"
	case floor >= 10:
		return "Steel Sword"
	case floor >= 5:
		return "Bronze Sword"
	default:
		return "Iron Sword"
	}
}

func armorName(floor int) string {
	switch {
	case floor >= 15:
		return "Mythril Shield"
	case floor >= 10:
		return "Steel Shield"
	case floor >= 5:
		return "Bronze Shield"
	default:
		return "Iron Shield"
	}
}

// applyEventOutcome applies an event outcome to the party per the rules in
// the design spec: HPChange to each alive player, GoldChange split evenly
// (remainder to the first alive player), ItemDrop one rolled item per alive
// player (skip if inventory full). Caller holds s.mu.
func (s *GameSession) applyEventOutcome(o EventOutcome) {
	alive := make([]*entity.Player, 0, len(s.state.Players))
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive = append(alive, p)
		}
	}
	if len(alive) == 0 {
		return
	}

	// HP change: each alive player heals/takes damage of the full amount.
	if o.HPChange > 0 {
		for _, p := range alive {
			before := p.HP
			p.Heal(o.HPChange)
			s.addLog(fmt.Sprintf("  %s HP %d 회복", p.Name, p.HP-before))
		}
	} else if o.HPChange < 0 {
		dmg := -o.HPChange
		for _, p := range alive {
			p.TakeDamage(dmg)
			if p.Dead {
				s.state.AnyPlayerDied = true
			}
			s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, dmg))
		}
	}

	// Gold change: split evenly across alive; remainder to alive[0].
	if o.GoldChange != 0 {
		total := o.GoldChange
		share := total / len(alive)
		remainder := total - share*len(alive)
		for i, p := range alive {
			delta := share
			if i == 0 {
				delta += remainder
			}
			p.Gold += delta
			if p.Gold < 0 {
				p.Gold = 0
			}
			if delta > 0 {
				s.addLog(fmt.Sprintf("  %s 골드 %d 획득", p.Name, delta))
			} else if delta < 0 {
				s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", p.Name, -delta))
			}
		}
	}

	// Item drop: one rolled item per alive; skip if their inventory is full.
	if o.ItemDrop {
		floor := s.state.FloorNum
		for _, p := range alive {
			if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
				s.addLog(fmt.Sprintf("  %s의 인벤토리가 가득 찼습니다!", p.Name))
				continue
			}
			if s.rng.Float64() < 0.5 {
				bonus := 3 + s.rng.Intn(6) + floor/3
				item := entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("  %s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
			} else {
				bonus := 2 + s.rng.Intn(4) + floor/4
				item := entity.Item{Name: armorName(floor), Type: entity.ItemArmor, Bonus: bonus}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("  %s %s 발견 (DEF+%d)", p.Name, item.Name, item.Bonus))
			}
		}
	}
}

// resolveEventVoteLocked tallies eventVotes, picks the winning choice (lowest
// index breaks ties), runs Resolve, applies the outcome party-wide, and
// returns the session to PhaseExploring with the room cleared. Per-player
// pending events (loot picks) skip the tally and resolve each player's own
// choice individually. Caller holds s.mu and has verified quorum.
func (s *GameSession) resolveEventVoteLocked() {
	if s.pendingEvent == nil {
		s.clearPendingEventLocked(true)
		return
	}
	if s.eventPerPlayer {
		s.resolveLootPicksLocked()
		return
	}
	// Tally
	tally := make(map[int]int, len(s.pendingEvent.Choices))
	for _, idx := range s.eventVotes {
		tally[idx]++
	}
	winnerIdx := -1
	bestCount := -1
	for idx, count := range tally {
		if count > bestCount || (count == bestCount && idx < winnerIdx) {
			bestCount = count
			winnerIdx = idx
		}
	}
	if winnerIdx < 0 || winnerIdx >= len(s.pendingEvent.Choices) {
		s.clearPendingEventLocked(true)
		return
	}
	choice := s.pendingEvent.Choices[winnerIdx]
	outcome := choice.Resolve(s.state.FloorNum, s.rng)
	s.addLog(fmt.Sprintf("  → %s: %s", choice.Label, outcome.Description))

	s.applyEventOutcome(outcome)
	s.clearPendingEventLocked(true)
}

// triggerEvent picks a random event and parks the session in PhaseEvent
// awaiting player votes. The actual outcome runs later via resolveEventVote.
// Caller holds s.mu.
func (s *GameSession) triggerEvent() {
	event := PickRandomEvent(s.rng)
	s.state.LastEventName = event.Name
	s.addLog(fmt.Sprintf("이벤트: %s — %s", event.Name, event.Description))

	// Edge case: nobody alive to vote (post-wipe race). Skip + clear.
	alive := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive++
		}
	}
	if alive == 0 {
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
		return
	}

	s.startEventPhaseLocked(event, false)
}

// triggerLoot presents the treasure-room loot table and parks the session in
// PhaseEvent with per-player resolution: each player's own pick applies to
// them only. Mirrors triggerEvent's zero-alive guard. Caller holds s.mu.
func (s *GameSession) triggerLoot() {
	alive := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive++
		}
	}
	if alive == 0 {
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
		return
	}

	event := BuildLootEvent(s.state.FloorNum, s.rng)
	s.addLog("보물 발견! 각자 하나를 선택하세요.")
	s.startEventPhaseLocked(event, true)
}

// startEventPhaseLocked stashes the typed event, builds the snapshot-safe
// view, and parks the session in PhaseEvent awaiting player input. perPlayer
// selects individual resolution (loot picks) over party majority (random
// events). Caller holds s.mu and has verified at least one alive player.
func (s *GameSession) startEventPhaseLocked(event RandomEvent, perPlayer bool) {
	ev := event
	s.pendingEvent = &ev
	s.eventPerPlayer = perPlayer

	view := &PendingEventView{
		Name:        event.Name,
		Description: event.Description,
		PerPlayer:   perPlayer,
		Choices:     make([]EventChoiceView, 0, len(event.Choices)),
	}
	for _, c := range event.Choices {
		tagsCopy := make([]ChoiceTag, len(c.Tags))
		copy(tagsCopy, c.Tags)
		view.Choices = append(view.Choices, EventChoiceView{Label: c.Label, Tags: tagsCopy})
	}
	s.state.PendingEvent = view
	s.state.EventVotes = make(map[string]int)
	s.eventVotes = make(map[string]int)
	s.state.Phase = PhaseEvent
}

// resolveLootPicksLocked applies each alive player's own pick. Iterates
// s.state.Players (not the vote map) so RNG consumption order is deterministic
// for daily runs. Voteless alive players (disconnect-sweep edge) are skipped.
// Caller holds s.mu.
func (s *GameSession) resolveLootPicksLocked() {
	floor := s.state.FloorNum
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		idx, ok := s.eventVotes[p.Fingerprint]
		if !ok || idx < 0 || idx >= len(s.pendingEvent.Choices) {
			continue
		}
		choice := s.pendingEvent.Choices[idx]
		outcome := choice.Resolve(floor, s.rng)
		s.addLog(fmt.Sprintf("  %s → %s", p.Name, outcome.Description))
		s.applyOutcomeToPlayer(p, outcome)
	}
	s.clearPendingEventLocked(true)
}

// applyOutcomeToPlayer applies a loot outcome to a single player. Unlike
// applyEventOutcome (party-wide), gold goes entirely to the picker, and a
// concrete Item grant converts to bonus*5 gold when the inventory is full
// (mirrors handleGamble's jackpot fallback; consistent with the shop price
// formula 40+bonus*5). Loot outcomes always carry concrete items (o.Item),
// never deferred ItemDrop rolls — see buildRotatingLootChoice. A player
// killed by the HP cost still receives the item/gold: pick and price are one
// transaction, and a revive keeps the loot. Caller holds s.mu.
func (s *GameSession) applyOutcomeToPlayer(p *entity.Player, o EventOutcome) {
	if o.HPChange > 0 {
		before := p.HP
		p.Heal(o.HPChange)
		s.addLog(fmt.Sprintf("  %s HP %d 회복", p.Name, p.HP-before))
	} else if o.HPChange < 0 {
		p.TakeDamage(-o.HPChange)
		if p.Dead {
			s.state.AnyPlayerDied = true
		}
		s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, -o.HPChange))
	}

	if o.GoldChange != 0 {
		p.Gold += o.GoldChange
		if p.Gold < 0 {
			p.Gold = 0
		}
		if o.GoldChange > 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 획득", p.Name, o.GoldChange))
		} else {
			s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", p.Name, -o.GoldChange))
		}
	}

	if o.Item != nil {
		if len(p.Inventory) < s.cfg.Game.InventoryLimit {
			p.Inventory = append(p.Inventory, *o.Item)
			s.addLog(fmt.Sprintf("  %s %s 획득", p.Name, o.Item.Name))
		} else {
			comp := o.Item.Bonus * 5
			p.Gold += comp
			s.addLog(fmt.Sprintf("  %s 인벤토리 가득 → +%dG로 전환", p.Name, comp))
		}
	}
}

func (s *GameSession) grantSecretTreasure() {
	s.addLog("보물로 가득 찬 비밀의 방을 발견했습니다!")
	floor := s.state.FloorNum
	// Double treasure: grant two items per player
	for _, p := range s.state.Players {
		for i := 0; i < 2; i++ {
			if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
				s.addLog(fmt.Sprintf("%s의 인벤토리가 가득 찼습니다!", p.Name))
				break
			}
			if s.rng.Float64() < 0.5 {
				bonus := 3 + s.rng.Intn(6) + floor/3
				item := entity.Item{
					Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus,
				}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("%s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
			} else {
				bonus := 2 + s.rng.Intn(4) + floor/4
				item := entity.Item{
					Name: armorName(floor), Type: entity.ItemArmor, Bonus: bonus,
				}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("%s %s 발견 (DEF+%d)", p.Name, item.Name, item.Bonus))
			}
		}
	}
}

func (s *GameSession) spawnMiniBoss() {
	var mt entity.MonsterType
	floor := s.state.FloorNum
	switch {
	case floor <= 4:
		mt = entity.MonsterMiniBoss5
	case floor <= 9:
		mt = entity.MonsterMiniBoss10
	case floor <= 14:
		mt = entity.MonsterMiniBoss15
	default:
		mt = entity.MonsterMiniBoss20
	}
	miniBoss := entity.NewMonster(mt, floor, s.cfg.Combat.MonsterScaling)

	// Use same pattern as the subsequent boss
	switch mt {
	case entity.MonsterMiniBoss5:
		miniBoss.Pattern = entity.PatternPoison
	case entity.MonsterMiniBoss10:
		miniBoss.Pattern = entity.PatternBurn
	case entity.MonsterMiniBoss15:
		miniBoss.Pattern = entity.PatternFreeze
	case entity.MonsterMiniBoss20:
		miniBoss.Pattern = entity.PatternHeal
	}

	s.applySoloScaling(miniBoss)
	s.applyDifficultyMods(miniBoss)
	s.state.Monsters = []*entity.Monster{miniBoss}
	s.addLog(fmt.Sprintf("미니보스 등장: %s!", miniBoss.Name))

	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.applyPoisonNextCombatIfPending()
}

// applyDifficultyMods scales a monster's HP/ATK by HardMode, NG+ level, and
// the active daily challenge modifier. SoloMode is intentionally NOT applied
// here — solo balance is handled per-spawn-context (regular spawns and bosses
// reduce HP for solo, but B10 phase-2 summons stay tough by design). Caller
// holds s.mu.
func (s *GameSession) applyDifficultyMods(m *entity.Monster) {
	if s.HardMode {
		mult := s.cfg.Difficulty.HardModeMonsterMult
		m.HP = int(float64(m.HP) * mult)
		m.MaxHP = m.HP
		m.ATK = int(float64(m.ATK) * mult)
	}
	if s.NGPlusLevel > 0 {
		ng := 1.0 + 0.25*float64(s.NGPlusLevel)
		m.HP = int(float64(m.HP) * ng)
		m.MaxHP = m.HP
		m.ATK = int(float64(m.ATK) * ng)
	}
	if s.DailyMod != nil {
		if s.DailyMod.MonsterHPMult > 0 && s.DailyMod.MonsterHPMult != 1.0 {
			m.HP = int(float64(m.HP) * s.DailyMod.MonsterHPMult)
			if m.HP < 1 {
				m.HP = 1
			}
			m.MaxHP = m.HP
		}
		if s.DailyMod.MonsterATKMult > 0 && s.DailyMod.MonsterATKMult != 1.0 {
			m.ATK = int(float64(m.ATK) * s.DailyMod.MonsterATKMult)
			if m.ATK < 1 {
				m.ATK = 1
			}
		}
	}
}
