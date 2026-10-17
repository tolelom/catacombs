package game

import (
	"fmt"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) EnterRoom(roomIdx int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enterRoomLocked(roomIdx)
}

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

	case dungeon.RoomEvent:
		s.triggerEvent()

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
		s.resetPerCombatStateLocked()
		s.signalCombat()
	} else {
		s.addLog("진짜 보물 상자였습니다!")
		s.grantTreasure()
		room.Cleared = true
	}
}

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
			if p.ConsumeCharm() {
				p.Gold += 50
				s.addLog(fmt.Sprintf("🍀 %s의 행운 부적이 베팅 금액을 지켜냈다!", p.Name))
			} else {
				s.addLog(fmt.Sprintf("%s 베팅 실패! -50G", p.Name))
			}
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

func (s *GameSession) handleWanderingMerchant() {
	floor := s.state.FloorNum
	weaponBonus := 6 + s.rng.Intn(6) + floor/3
	armorBonus := 4 + s.rng.Intn(4) + floor/4
	weaponPrice := (40 + weaponBonus*5) * 2
	armorPrice := (30 + armorBonus*5) * 2
	potionPrice := (20 + floor/2) * 2

	charm := entity.NewLuckyCharm(floor)
	charm.Price *= 2

	s.state.ShopItems = []entity.Item{
		{Name: "Greater HP Potion", Type: entity.ItemConsumable, Bonus: 60 + floor*2, Price: potionPrice},
		{Name: "Rare " + weaponName(floor), Type: entity.ItemWeapon, Bonus: weaponBonus, Price: weaponPrice},
		{Name: "Rare " + armorName(floor), Type: entity.ItemArmor, Bonus: armorBonus, Price: armorPrice},
		charm,
	}
	s.state.ShopSold = make([]bool, len(s.state.ShopItems))
	s.addLog("방랑상인이 희귀 아이템을 들고 나타났습니다! (가격 2배)")
}

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

func (s *GameSession) earlyThreatMult(floor int) float64 {
	const fadeFloor = 10
	steps := floor - 2
	if fade := fadeFloor - floor; fade < steps {
		steps = fade
	}
	if steps <= 0 {
		return 1.0
	}
	return 1.0 + s.cfg.Combat.EarlyThreatRamp*float64(steps)
}

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

func (s *GameSession) resetPerCombatStateLocked() {
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
	s.coopUsedThisCombat = make(map[string]bool)
	s.activeRelicUsed = make(map[string]bool)
	s.extraActionNext = make(map[string]bool)
	s.soloLastAction = ""
	s.soloAttackStreak = 0
	s.applyPoisonNextCombatIfPending()
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

	s.resetPerCombatStateLocked()
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
		boss.Pattern = entity.PatternPoison
	case entity.MonsterBoss10:
		boss.Pattern = entity.PatternBurn
	case entity.MonsterBoss15:
		boss.Pattern = entity.PatternFreeze
	case entity.MonsterBoss20:
		boss.Pattern = entity.PatternHeal
	}
	s.applySoloScaling(boss)
	s.applyDifficultyMods(boss)
	s.state.Monsters = []*entity.Monster{boss}

	s.resetPerCombatStateLocked()
}

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

	weaponBonus := 3 + s.rng.Intn(6) + floor/3

	armorBonus := 2 + s.rng.Intn(4) + floor/4

	weaponPrice := 40 + weaponBonus*5
	armorPrice := 30 + armorBonus*5

	potionHeal := 30 + floor
	potionPrice := 20 + floor/2
	charm := entity.NewLuckyCharm(floor)

	if s.HardMode {
		mult := s.cfg.Difficulty.HardModeShopMult
		potionPrice = int(float64(potionPrice) * mult)
		weaponPrice = int(float64(weaponPrice) * mult)
		armorPrice = int(float64(armorPrice) * mult)
		charm.Price = int(float64(charm.Price) * mult)
	}
	s.state.ShopItems = []entity.Item{
		{Name: "HP Potion", Type: entity.ItemConsumable, Bonus: potionHeal, Price: potionPrice},
		{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: weaponBonus, Price: weaponPrice},
		{Name: armorName(floor), Type: entity.ItemArmor, Bonus: armorBonus, Price: armorPrice},
		charm,
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

func (s *GameSession) applyEventOutcome(o EventOutcome) {
	alive := s.activePlayersLocked()
	if len(alive) == 0 {
		return
	}
	charmProtected := s.consumeCharmsAgainstLossLocked(o, alive)
	s.applyEventHPChangeLocked(o.HPChange, alive, charmProtected)
	s.applyEventGoldChangeLocked(o.GoldChange, alive, charmProtected)
	s.applyEventItemsLocked(o, alive)
}

func (s *GameSession) activePlayersLocked() []*entity.Player {
	alive := make([]*entity.Player, 0, len(s.state.Players))
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive = append(alive, p)
		}
	}
	return alive
}

func (s *GameSession) consumeCharmsAgainstLossLocked(o EventOutcome, alive []*entity.Player) map[string]bool {
	charmProtected := make(map[string]bool)
	if !o.Gambled || (o.HPChange >= 0 && o.GoldChange >= 0) {
		return charmProtected
	}
	for _, p := range alive {
		if p.ConsumeCharm() {
			charmProtected[p.Fingerprint] = true
			s.addLog(fmt.Sprintf("  🍀 %s의 행운 부적이 불운을 막아냈다!", p.Name))
		}
	}
	return charmProtected
}

func (s *GameSession) applyEventHPChangeLocked(change int, alive []*entity.Player, charmProtected map[string]bool) {
	if change > 0 {
		for _, p := range alive {
			before := p.HP
			p.Heal(change)
			s.addLog(fmt.Sprintf("  %s HP %d 회복", p.Name, p.HP-before))
		}
		return
	}
	if change == 0 {
		return
	}
	dmg := -change
	for _, p := range alive {
		if charmProtected[p.Fingerprint] {
			continue
		}
		p.TakeDamage(dmg)
		s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, dmg))
		if p.IsDead() {
			s.tryReviveSoloHealerLocked(p)
		}
	}
}

func (s *GameSession) applyEventGoldChangeLocked(total int, alive []*entity.Player, charmProtected map[string]bool) {
	if total == 0 {
		return
	}
	share := total / len(alive)
	remainder := total - share*len(alive)
	for i, p := range alive {
		delta := share
		if i == 0 {
			delta += remainder
		}
		if delta < 0 && charmProtected[p.Fingerprint] {
			continue
		}
		p.Gold = max(p.Gold+delta, 0)
		if delta > 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 획득", p.Name, delta))
		} else if delta < 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", p.Name, -delta))
		}
	}
}

func (s *GameSession) applyEventItemsLocked(o EventOutcome, alive []*entity.Player) {
	if o.Item != nil {
		for _, p := range alive {
			s.grantEventItemLocked(p, *o.Item)
		}
		return
	}
	if o.ItemDrop {
		for _, p := range alive {
			s.rollEventItemDropLocked(p)
		}
	}
}

func (s *GameSession) grantEventItemLocked(p *entity.Player, item entity.Item) {
	if len(p.Inventory) < s.cfg.Game.InventoryLimit {
		p.Inventory = append(p.Inventory, item)
		s.addLog(fmt.Sprintf("  %s %s 획득", p.Name, item.Name))
		return
	}
	comp := item.Bonus * 5
	p.Gold += comp
	s.addLog(fmt.Sprintf("  %s 인벤토리 가득 → +%dG로 전환", p.Name, comp))
}

func (s *GameSession) rollEventItemDropLocked(p *entity.Player) {
	if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
		s.addLog(fmt.Sprintf("  %s의 인벤토리가 가득 찼습니다!", p.Name))
		return
	}
	floor := s.state.FloorNum
	if s.rng.Float64() < 0.5 {
		bonus := 3 + s.rng.Intn(6) + floor/3
		item := entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus}
		p.Inventory = append(p.Inventory, item)
		s.addLog(fmt.Sprintf("  %s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
		return
	}
	bonus := 2 + s.rng.Intn(4) + floor/4
	item := entity.Item{Name: armorName(floor), Type: entity.ItemArmor, Bonus: bonus}
	p.Inventory = append(p.Inventory, item)
	s.addLog(fmt.Sprintf("  %s %s 발견 (DEF+%d)", p.Name, item.Name, item.Bonus))
}

func (s *GameSession) resolveEventVoteLocked() {
	if s.pendingEvent == nil {
		s.clearPendingEventLocked(true)
		return
	}
	if s.eventPerPlayer {
		s.resolveLootPicksLocked()
		return
	}

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
	outcome.Gambled = tagsContain(choice.Tags, TagGamble)
	s.addLog(fmt.Sprintf("  → %s: %s", choice.Label, outcome.Description))

	s.applyEventOutcome(outcome)
	s.clearPendingEventLocked(true)
	s.endRunIfPartyWipedLocked()
}

func (s *GameSession) triggerEvent() {
	event := PickRandomEvent(s.rng)
	s.state.LastEventName = event.Name
	s.addLog(fmt.Sprintf("이벤트: %s — %s", event.Name, event.Description))

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
		outcome.Gambled = tagsContain(choice.Tags, TagGamble)
		s.addLog(fmt.Sprintf("  %s → %s", p.Name, outcome.Description))
		s.applyOutcomeToPlayer(p, outcome)
	}
	s.clearPendingEventLocked(true)
	s.endRunIfPartyWipedLocked()
}

func (s *GameSession) applyOutcomeToPlayer(p *entity.Player, o EventOutcome) {
	hpChange, goldChange := o.HPChange, o.GoldChange

	if o.Gambled && (hpChange < 0 || goldChange < 0) && p.ConsumeCharm() {
		s.addLog(fmt.Sprintf("  🍀 %s의 행운 부적이 불운을 막아냈다!", p.Name))
		if hpChange < 0 {
			hpChange = 0
		}
		if goldChange < 0 {
			goldChange = 0
		}
	}

	if hpChange > 0 {
		before := p.HP
		p.Heal(hpChange)
		s.addLog(fmt.Sprintf("  %s HP %d 회복", p.Name, p.HP-before))
	} else if hpChange < 0 {
		p.TakeDamage(-hpChange)
		if p.Dead {
			s.state.AnyPlayerDied = true
		}
		s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, -hpChange))
	}

	if goldChange != 0 {
		p.Gold += goldChange
		if p.Gold < 0 {
			p.Gold = 0
		}
		if goldChange > 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 획득", p.Name, goldChange))
		} else {
			s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", p.Name, -goldChange))
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

	s.resetPerCombatStateLocked()
}

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
