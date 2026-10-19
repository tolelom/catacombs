package game

import (
	"fmt"

	"github.com/tolelom/catacombs/entity"
)

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
