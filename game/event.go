package game

import (
	"fmt"
	"math/rand"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) EnterRoom(roomIdx int) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
		s.generateShopItems()
		s.state.Phase = PhaseShop
	case dungeon.RoomTreasure:
		s.grantTreasure()
		room.Cleared = true
	case dungeon.RoomEvent:
		s.triggerEvent()
		room.Cleared = true
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
		m := entity.NewMonster(mt, floor)
		if s.state.SoloMode {
			m.HP = m.HP / 2
			if m.HP < 1 {
				m.HP = 1
			}
			m.MaxHP = m.HP
		}
		s.state.Monsters[i] = m
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
	boss := entity.NewMonster(mt, s.state.FloorNum)
	if s.state.SoloMode {
		boss.HP = boss.HP / 2
		boss.MaxHP = boss.HP
	}
	s.state.Monsters = []*entity.Monster{boss}
}

func (s *GameSession) grantTreasure() {
	for _, p := range s.state.Players {
		if rand.Float64() < 0.5 {
			item := entity.Item{
				Name: "Iron Sword", Type: entity.ItemWeapon, Bonus: 3 + rand.Intn(6),
			}
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s found %s (ATK+%d)", p.Name, item.Name, item.Bonus))
		} else {
			item := entity.Item{
				Name: "Iron Shield", Type: entity.ItemArmor, Bonus: 2 + rand.Intn(4),
			}
			p.Inventory = append(p.Inventory, item)
			s.addLog(fmt.Sprintf("%s found %s (DEF+%d)", p.Name, item.Name, item.Bonus))
		}
	}
}

func (s *GameSession) generateShopItems() {
	s.state.ShopItems = []entity.Item{
		{Name: "HP Potion", Type: entity.ItemConsumable, Bonus: 30, Price: 20},
		{Name: "Iron Sword", Type: entity.ItemWeapon, Bonus: 3 + rand.Intn(6), Price: 40 + rand.Intn(41)},
		{Name: "Iron Shield", Type: entity.ItemArmor, Bonus: 2 + rand.Intn(4), Price: 30 + rand.Intn(31)},
	}
}

func (s *GameSession) triggerEvent() {
	for _, p := range s.state.Players {
		if p.IsDead() {
			continue
		}
		if rand.Float64() < 0.5 {
			dmg := 10 + rand.Intn(11)
			p.TakeDamage(dmg)
			s.addLog(fmt.Sprintf("Trap! %s takes %d damage", p.Name, dmg))
		} else {
			heal := 15 + rand.Intn(11)
			before := p.HP
			p.Heal(heal)
			s.addLog(fmt.Sprintf("Blessing! %s heals %d HP", p.Name, p.HP-before))
		}
	}
}
