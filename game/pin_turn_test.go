package game

import (
	"fmt"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

// 1a. resolveMonsterActions: 광역/단일, 보스 패턴, 엘리트 타격 효과, 환영진 회피, 도발 틱,
// 피해 보정(무적·협동 감소·회피 보너스), 솔로 힐러 부활과 사망 로그를 고정 시드로 기록한다.

type monsterScenario struct {
	name    string
	classes []entity.Class
	setup   func(s *GameSession)
}

func monsterScenarios() []monsterScenario {
	party := []entity.Class{entity.ClassWarrior, entity.ClassMage, entity.ClassHealer, entity.ClassRogue}
	regular := func() *entity.Monster {
		return &entity.Monster{Name: "Goblin", HP: 80, MaxHP: 80, ATK: 25, DEF: 2}
	}
	boss := func(p entity.BossPattern) *entity.Monster {
		return &entity.Monster{Name: "Boss", HP: 400, MaxHP: 800, ATK: 30, DEF: 5, IsBoss: true, Pattern: p}
	}
	elite := func(prefix entity.ElitePrefixType) *entity.Monster {
		m := regular()
		m.Name = "Elite"
		m.IsElite = true
		m.ElitePrefix = prefix
		return m
	}
	scenarios := []monsterScenario{
		{"regular vs party", party, func(s *GameSession) {
			s.state.Monsters = []*entity.Monster{regular(), regular()}
		}},
		{"dead monster is skipped", party, func(s *GameSession) {
			dead := regular()
			dead.HP = 0
			s.state.Monsters = []*entity.Monster{dead, regular()}
		}},
		{"taunt targets warrior", party, func(s *GameSession) {
			m := regular()
			m.TauntTarget = true
			m.TauntTurns = 2
			s.state.Monsters = []*entity.Monster{m}
		}},
		{"taunt with no warrior", []entity.Class{entity.ClassMage, entity.ClassRogue}, func(s *GameSession) {
			m := regular()
			m.TauntTarget = true
			m.TauntTurns = 1
			s.state.Monsters = []*entity.Monster{m}
		}},
		{"mini boss", party, func(s *GameSession) {
			m := regular()
			m.IsMiniBoss = true
			m.Pattern = entity.PatternBurn
			s.state.Monsters = []*entity.Monster{m}
		}},
		{"boss phase 2", party, func(s *GameSession) {
			m := boss(entity.PatternPoison)
			m.Phase = 2
			s.state.Monsters = []*entity.Monster{m}
		}},
		{"illusion miss", party, func(s *GameSession) {
			s.enemyMissNextTurn = true
			s.state.Monsters = []*entity.Monster{boss(entity.PatternFreeze), regular(), regular()}
		}},
		{"damage mods", party, func(s *GameSession) {
			s.invincibleNextTurn = map[string]bool{"fp-0": true}
			s.coopDamageReduction = map[string]float64{"fp-1": 0.5}
			s.dodgeBonusThisTurn = map[string]float64{"fp-3": 0.4}
			s.state.Monsters = []*entity.Monster{boss(entity.PatternNone), regular(), regular()}
		}},
		{"lethal hits wipe party", []entity.Class{entity.ClassMage, entity.ClassRogue}, func(s *GameSession) {
			for _, p := range s.state.Players {
				p.HP = 1
			}
			strong := boss(entity.PatternNone)
			strong.ATK = 500
			s.state.Monsters = []*entity.Monster{strong, regular()}
		}},
		{"solo healer revives once", []entity.Class{entity.ClassHealer}, func(s *GameSession) {
			s.state.Players[0].HP = 1
			strong := regular()
			strong.ATK = 500
			s.state.Monsters = []*entity.Monster{strong, strong}
		}},
		{"out players are skipped", party, func(s *GameSession) {
			s.state.Players[0].Dead = true
			s.state.Players[0].HP = 0
			s.state.Players[1].Fled = true
			s.state.Monsters = []*entity.Monster{boss(entity.PatternBurn), regular()}
		}},
		{"not in combat", party, func(s *GameSession) {
			s.state.Phase = PhaseExploring
			s.state.Monsters = []*entity.Monster{regular()}
		}},
	}
	for _, p := range []entity.BossPattern{entity.PatternNone, entity.PatternPoison, entity.PatternBurn, entity.PatternHeal, entity.PatternFreeze} {
		p := p
		scenarios = append(scenarios, monsterScenario{fmt.Sprintf("boss pattern %v", p), party, func(s *GameSession) {
			s.state.Monsters = []*entity.Monster{boss(p)}
		}})
	}
	for _, prefix := range []entity.ElitePrefixType{entity.PrefixVenomous, entity.PrefixBurning, entity.PrefixFreezing, entity.PrefixBleeding, entity.PrefixVampiric} {
		prefix := prefix
		scenarios = append(scenarios, monsterScenario{fmt.Sprintf("elite prefix %v", prefix), party, func(s *GameSession) {
			m := elite(prefix)
			m.HP = 40
			s.state.Monsters = []*entity.Monster{m, elite(prefix)}
		}})
	}
	return scenarios
}

func TestPinResolveMonsterActions(t *testing.T) {
	var tw traceWriter
	for _, sc := range monsterScenarios() {
		for _, turn := range []int{1, 2, 3, 4, 6} {
			for _, seed := range []int64{1, 2, 3} {
				s := pinSession(t, seed, sc.classes...)
				s.state.CombatTurn = turn
				sc.setup(s)
				s.resolveMonsterActions()
				tw.line("== %s turn=%d seed=%d", sc.name, turn, seed)
				tw.session(s)
			}
		}
	}
	checkGolden(t, "resolve_monster_actions", tw.sb.String())
}

// 1b. checkBossPhaseTransitions: 층별(5/10/15/20/기타) 2페이즈 효과, cruel 저주의 70% 임계값,
// 이미 전환된 보스와 일반 몬스터는 건너뛰는 것을 기록한다.

func TestPinCheckBossPhaseTransitions(t *testing.T) {
	var tw traceWriter
	party := []entity.Class{entity.ClassWarrior, entity.ClassMage}
	for _, floor := range []int{3, 5, 10, 15, 20} {
		for _, hp := range []int{100, 51, 50, 70, 10} {
			for _, curse := range []string{"", "cruel"} {
				for _, hard := range []bool{false, true} {
					s := pinSession(t, 7, party...)
					s.state.FloorNum = floor
					s.HardMode = hard
					if curse != "" {
						c := Curse{ID: curse}
						s.ActiveCurse = &c
					}
					s.state.Players[1].Dead = true
					s.state.Players[1].HP = 0
					s.state.Monsters = []*entity.Monster{
						{Name: "Boss", HP: hp, MaxHP: 100, ATK: 40, DEF: 10, IsBoss: true, Pattern: entity.PatternHeal},
						{Name: "Done", HP: 10, MaxHP: 100, ATK: 40, DEF: 10, IsBoss: true, Phase: 2, PhaseTransitioned: true},
						{Name: "Grunt", HP: 10, MaxHP: 100, ATK: 10, DEF: 1},
					}
					s.checkBossPhaseTransitions()
					tw.line("== floor=%d hp=%d curse=%q hard=%v", floor, hp, curse, hard)
					tw.session(s)
				}
			}
		}
	}
	checkGolden(t, "check_boss_phase_transitions", tw.sb.String())
}

// applyEventOutcome도 단계로 나눌 대상이라 같은 방식으로 고정한다(부적, HP, 골드 분배, 아이템).

func TestPinApplyEventOutcome(t *testing.T) {
	var tw traceWriter
	sword := entity.Item{Name: "Pin Sword", Type: entity.ItemWeapon, Bonus: 7}
	outcomes := []EventOutcome{
		{HPChange: 25},
		{HPChange: -30},
		{HPChange: -500},
		{HPChange: -30, Gambled: true},
		{GoldChange: 101},
		{GoldChange: -101},
		{GoldChange: -101, Gambled: true},
		{GoldChange: 7},
		{ItemDrop: true},
		{Item: &sword},
		{Item: &sword, ItemDrop: true},
		{HPChange: -40, GoldChange: -50, ItemDrop: true, Gambled: true},
	}
	setups := []struct {
		name    string
		classes []entity.Class
		setup   func(s *GameSession)
	}{
		{"party", []entity.Class{entity.ClassWarrior, entity.ClassMage, entity.ClassRogue}, func(s *GameSession) {
			s.state.Players[0].Gold = 30
			s.state.Players[1].HP = 20
		}},
		{"charm and full inventory", []entity.Class{entity.ClassWarrior, entity.ClassMage}, func(s *GameSession) {
			s.state.Players[0].Gold = 200
			s.state.Players[0].Inventory = append(s.state.Players[0].Inventory, entity.Item{Name: "행운 부적", Type: entity.ItemCharm})
			for len(s.state.Players[1].Inventory) < s.cfg.Game.InventoryLimit {
				s.state.Players[1].Inventory = append(s.state.Players[1].Inventory, entity.Item{Name: "Rock", Type: entity.ItemArmor, Bonus: 1})
			}
		}},
		{"one out player", []entity.Class{entity.ClassWarrior, entity.ClassHealer}, func(s *GameSession) {
			s.state.Players[0].Dead = true
			s.state.Players[0].HP = 0
		}},
		{"solo healer", []entity.Class{entity.ClassHealer}, func(s *GameSession) {}},
		{"everyone out", []entity.Class{entity.ClassMage}, func(s *GameSession) {
			s.state.Players[0].Fled = true
		}},
	}
	for _, st := range setups {
		for i, o := range outcomes {
			for _, floor := range []int{2, 12} {
				s := pinSession(t, int64(11+i), st.classes...)
				s.state.FloorNum = floor
				st.setup(s)
				s.applyEventOutcome(o)
				tw.line("== %s outcome=%d floor=%d", st.name, i, floor)
				tw.session(s)
			}
		}
	}
	checkGolden(t, "apply_event_outcome", tw.sb.String())
}
