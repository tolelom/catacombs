package game

import (
	"fmt"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) killGoldMultiplier() float64 {
	goldMult := 1.0
	if s.NGPlusLevel > 0 {
		goldMult += 0.30 * float64(s.NGPlusLevel)
	}
	if s.ActiveCurse != nil && s.ActiveCurse.GoldMult > 0 {
		goldMult *= s.ActiveCurse.GoldMult
	}
	if s.DailyMod != nil && s.DailyMod.GoldMult > 0 {
		goldMult *= s.DailyMod.GoldMult
	}
	return goldMult
}

func (s *GameSession) awardKills(aliveBefore map[int]bool) {
	goldMult := s.killGoldMultiplier()
	for i, m := range s.state.Monsters {
		if m.IsDead() && aliveBefore[i] {
			s.awardKill(m, goldMult)
		}
	}
}

func (s *GameSession) awardKill(m *entity.Monster, goldMult float64) {
	if m.Name == "Mimic" {
		s.state.MimicKilled = true
	}
	goldReward := int(float64(5+s.state.FloorNum*2) * goldMult)
	for _, p := range s.state.Players {
		if !p.IsOut() {
			s.payKillReward(p, goldReward)
		}
	}
	s.addLog(fmt.Sprintf("%s 처치! +%d 골드", m.Name, goldReward))
	if m.IsBoss {
		s.state.BossKilled = true
		if s.state.BossesKilledThisRun == nil {
			s.state.BossesKilledThisRun = make(map[string]bool)
		}
		s.state.BossesKilledThisRun[m.Name] = true
		s.grantBossRelic()
	}
}

func (s *GameSession) payKillReward(p *entity.Player, goldReward int) {
	bonus := 0
	for _, r := range p.Relics {
		if r.Effect == entity.RelicGoldBoost {
			bonus += r.Value
		}
		if r.Effect == entity.RelicHealOnKill {
			p.Heal(r.Value)
			s.addLog(fmt.Sprintf("%s의 유물로 HP %d 회복", p.Name, r.Value))
		}
	}
	p.Gold += goldReward + bonus
}

func (s *GameSession) removeDeadMonsters() {
	alive := make([]*entity.Monster, 0)
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			alive = append(alive, m)
		}
	}
	s.state.Monsters = alive
}

func (s *GameSession) finishRoomIfCleared() {
	if len(s.state.Monsters) != 0 {
		return
	}
	s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
	s.addLog("방 클리어!")
	for _, p := range s.state.Players {
		p.Fled = false
	}
	if s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Type == dungeon.RoomBoss {
		s.advanceFloor()
	} else {
		s.state.Phase = PhaseExploring
	}
}

func (s *GameSession) advanceFloor() {
	if s.state.FloorNum >= s.cfg.Game.MaxFloors {
		s.state.Phase = PhaseResult
		s.state.Victory = true
		s.state.GameOver = true
		s.addLog("카타콤을 정복했습니다!")
		return
	}

	for _, p := range s.state.Players {
		if p.Skills == nil {
			p.Skills = &entity.PlayerSkills{BranchIndex: -1}
		}
		p.Skills.Points++
	}
	s.state.FloorNum++
	s.reseedForFloor()
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, s.rng)
	s.state.Phase = PhaseExploring
	s.state.CombatTurn = 0
	s.scoutUsedThisFloor = make(map[string]bool)
	s.nextRoomPreview = nil
	s.addLog(fmt.Sprintf("B%d층으로 내려갑니다...", s.state.FloorNum))
	for _, p := range s.state.Players {
		if p.IsDead() {
			p.Revive(0.30)
			s.addLog(fmt.Sprintf("✦ %s HP %d로 부활!", p.Name, p.HP))
		}
		p.Fled = false
	}
}

func (s *GameSession) grantBossRelic() {
	relics := []entity.Relic{
		{Name: "Vampiric Ring", Effect: entity.RelicHealOnKill, Value: 5, Price: 100},
		{Name: "Power Amulet", Effect: entity.RelicATKBoost, Value: 3, Price: 120},
		{Name: "Iron Ward", Effect: entity.RelicDEFBoost, Value: 2, Price: 100},
		{Name: "Gold Charm", Effect: entity.RelicGoldBoost, Value: 5, Price: 150},
		{Name: "Antidote Charm", Effect: entity.RelicPoisonImmunity, Value: 0, Price: 100},
		{Name: "Flame Guard", Effect: entity.RelicBurnResist, Value: 0, Price: 120},
		{Name: "Life Siphon", Effect: entity.RelicLifeSteal, Value: 10, Price: 150},
		{Name: "Smoke Bomb", Effect: entity.RelicActiveSmokeBomb, Value: 0, Price: 130},
		{Name: "Bloodstone", Effect: entity.RelicActiveBloodstone, Value: 30, Price: 130},
		{Name: "Time Glass", Effect: entity.RelicActiveTimeGlass, Value: 0, Price: 180},
	}
	for _, p := range s.state.Players {
		if !p.IsOut() {
			r := relics[s.rng.Intn(len(relics))]
			p.Relics = append(p.Relics, r)
			s.addLog(fmt.Sprintf("%s 유물 획득: %s", p.Name, r.Name))

			if len(p.Inventory) < s.cfg.Game.InventoryLimit {
				item := randomSetItem(s.rng, s.state.FloorNum)
				p.Inventory = append(p.Inventory, item)
				s.addLog(fmt.Sprintf("%s 세트 아이템: %s", p.Name, item.Name))
			}
		}
	}
}

func randomSetItem(rng interface{ Intn(int) int }, floor int) entity.Item {
	sets := []entity.ItemSet{entity.SetFlame, entity.SetFrost, entity.SetHoly, entity.SetShadow, entity.SetNature}
	set := sets[rng.Intn(len(sets))]
	if rng.Intn(2) == 0 {
		bonus := 4 + rng.Intn(5) + floor/3
		return entity.Item{Name: entity.SetName(set) + " 무기", Type: entity.ItemWeapon, Bonus: bonus, Set: set}
	}
	bonus := 3 + rng.Intn(4) + floor/4
	return entity.Item{Name: entity.SetName(set) + " 갑옷", Type: entity.ItemArmor, Bonus: bonus, Set: set}
}
