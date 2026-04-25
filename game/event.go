package game

import (
	"fmt"
	"math/rand"
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
		s.grantTreasure()
		room.Cleared = true
	case dungeon.RoomEvent:
		s.triggerEvent()
		room.Cleared = true
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
	}
}

func (s *GameSession) spawnMonsters() {
	count := 1 + rand.Intn(5)
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
		mt := valid[rand.Intn(len(valid))]
		m := entity.NewMonster(mt, floor, s.cfg.Combat.MonsterScaling)
		if s.state.SoloMode {
			m.HP = int(float64(m.HP) * s.cfg.Combat.SoloHPReduction)
			if m.HP < 1 {
				m.HP = 1
			}
			m.MaxHP = m.HP
			m.DEF = int(float64(m.DEF) * s.cfg.Combat.SoloHPReduction)
		}
		if s.hasMutation("elite_flood") || rand.Float64() < 0.20 {
			entity.ApplyPrefix(m, entity.RandomPrefix())
		}
		if s.HardMode {
			mult := s.cfg.Difficulty.HardModeMonsterMult
			m.HP = int(float64(m.HP) * mult)
			m.MaxHP = m.HP
			m.ATK = int(float64(m.ATK) * mult)
		}
		s.state.Monsters[i] = m
	}

	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
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
	if s.state.SoloMode {
		boss.HP = int(float64(boss.HP) * s.cfg.Combat.SoloHPReduction)
		if boss.HP < 1 {
			boss.HP = 1
		}
		boss.MaxHP = boss.HP
		boss.DEF = int(float64(boss.DEF) * s.cfg.Combat.SoloHPReduction)
	}
	if s.HardMode {
		mult := s.cfg.Difficulty.HardModeMonsterMult
		boss.HP = int(float64(boss.HP) * mult)
		boss.MaxHP = boss.HP
		boss.ATK = int(float64(boss.ATK) * mult)
	}
	s.state.Monsters = []*entity.Monster{boss}

	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
}

func (s *GameSession) grantTreasure() {
	floor := s.state.FloorNum
	for _, p := range s.state.Players {
		if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
			s.addLog(fmt.Sprintf("%s의 인벤토리가 가득 찼습니다!", p.Name))
			continue
		}
		if rand.Float64() < 0.5 {
			bonus := 3 + rand.Intn(6) + floor/3
			item := entity.Item{
				Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus,
			}
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
		} else {
			bonus := 2 + rand.Intn(4) + floor/4
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
	weaponBonus := 3 + rand.Intn(6) + floor/3
	// Armor bonus scales: base 2-5 + floor/4
	armorBonus := 2 + rand.Intn(4) + floor/4
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

func (s *GameSession) triggerEvent() {
	event := PickRandomEvent()
	s.state.LastEventName = event.Name
	s.addLog(fmt.Sprintf("이벤트: %s — %s", event.Name, event.Description))

	// Auto-resolve with a random choice
	choice := event.Choices[rand.Intn(len(event.Choices))]
	outcome := choice.Resolve(s.state.FloorNum)
	s.addLog(fmt.Sprintf("  → %s: %s", choice.Label, outcome.Description))

	// Pick a random available player to apply the outcome (excludes fled).
	var available []*entity.Player
	for _, p := range s.state.Players {
		if !p.IsOut() {
			available = append(available, p)
		}
	}
	if len(available) == 0 {
		return
	}
	target := available[rand.Intn(len(available))]

	if outcome.HPChange > 0 {
		before := target.HP
		target.Heal(outcome.HPChange)
		s.addLog(fmt.Sprintf("  %s HP %d 회복", target.Name, target.HP-before))
	} else if outcome.HPChange < 0 {
		target.TakeDamage(-outcome.HPChange)
		s.addLog(fmt.Sprintf("  %s %d 피해를 받음", target.Name, -outcome.HPChange))
	}

	if outcome.GoldChange != 0 {
		target.Gold += outcome.GoldChange
		if target.Gold < 0 {
			target.Gold = 0
		}
		if outcome.GoldChange > 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 획득", target.Name, outcome.GoldChange))
		} else {
			s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", target.Name, -outcome.GoldChange))
		}
	}

	if outcome.ItemDrop {
		if len(target.Inventory) < s.cfg.Game.InventoryLimit {
			floor := s.state.FloorNum
			if rand.Float64() < 0.5 {
				bonus := 3 + rand.Intn(6) + floor/3
				item := entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus}
				target.Inventory = append(target.Inventory, item)
				s.addLog(fmt.Sprintf("  %s %s 발견 (ATK+%d)", target.Name, item.Name, item.Bonus))
			} else {
				bonus := 2 + rand.Intn(4) + floor/4
				item := entity.Item{Name: armorName(floor), Type: entity.ItemArmor, Bonus: bonus}
				target.Inventory = append(target.Inventory, item)
				s.addLog(fmt.Sprintf("  %s %s 발견 (DEF+%d)", target.Name, item.Name, item.Bonus))
			}
		} else {
			s.addLog(fmt.Sprintf("  %s의 인벤토리가 가득 찼습니다!", target.Name))
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
			if rand.Float64() < 0.5 {
				bonus := 3 + rand.Intn(6) + floor/3
				item := entity.Item{
					Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus,
				}
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("%s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
			} else {
				bonus := 2 + rand.Intn(4) + floor/4
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

	if s.state.SoloMode {
		miniBoss.HP = int(float64(miniBoss.HP) * s.cfg.Combat.SoloHPReduction)
		if miniBoss.HP < 1 {
			miniBoss.HP = 1
		}
		miniBoss.MaxHP = miniBoss.HP
		miniBoss.DEF = int(float64(miniBoss.DEF) * s.cfg.Combat.SoloHPReduction)
	}
	if s.HardMode {
		mult := s.cfg.Difficulty.HardModeMonsterMult
		miniBoss.HP = int(float64(miniBoss.HP) * mult)
		miniBoss.MaxHP = miniBoss.HP
		miniBoss.ATK = int(float64(miniBoss.ATK) * mult)
	}
	s.state.Monsters = []*entity.Monster{miniBoss}
	s.addLog(fmt.Sprintf("미니보스 등장: %s!", miniBoss.Name))

	// Reset skill uses for all players at combat start
	for _, p := range s.state.Players {
		p.SkillUses = s.effectiveSkillUses()
	}
}
