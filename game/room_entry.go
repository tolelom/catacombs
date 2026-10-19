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
