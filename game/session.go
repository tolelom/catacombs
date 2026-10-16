package game

import (
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) addLog(msg string) {
	if s.state.TurnResolving {
		s.state.PendingLogs = append(s.state.PendingLogs, msg)
	} else {
		s.state.CombatLog = append(s.state.CombatLog, msg)
		if len(s.state.CombatLog) > 8 {
			s.state.CombatLog = s.state.CombatLog[len(s.state.CombatLog)-8:]
		}
	}
}

func (s *GameSession) clearLog() {
	s.state.CombatLog = nil
}

type GameSession struct {
	mu             sync.Mutex
	cfg            *config.Config
	state          GameState
	started        bool
	actions        map[string]PlayerAction
	actionCh       chan playerActionMsg
	combatSignal   chan struct{}
	playerLeftCh   chan struct{}
	done           chan struct{}
	stopOnce       sync.Once
	onAbandoned    func()
	disconnectedAt map[string]time.Time
	clock          func() time.Time
	lastReveal     time.Time
	recorder       RunRecorder
	recordOnce     sync.Once
	recorded       chan struct{}
	moveVotes      map[string]int
	shopLeavers    map[string]bool
	eventVotes     map[string]int
	pendingEvent   *RandomEvent
	eventPerPlayer bool
	rng            *rand.Rand

	coopUsedThisCombat  map[string]bool
	healerReviveUsed    map[string]bool
	scoutUsedThisFloor  map[string]bool
	soloHealerRevived   map[string]bool
	coopDamageReduction map[string]float64
	invincibleNextTurn  map[string]bool
	dodgeBonusThisTurn  map[string]float64
	extraActionNext     map[string]bool
	activeRelicUsed     map[string]bool
	nextRoomPreview     *RoomPreview
	critGuaranteedNext  bool
	enemyMissNextTurn   bool
	poisonNextCombat    bool

	soloLastAction   string
	soloAttackStreak int

	HardMode       bool
	ActiveMutation *Mutation
	DailyMode      bool
	DailyDate      string

	NGPlusLevel int
	ActiveCurse *Curse

	DailyMod *DailyModifier
}

type playerActionMsg struct {
	PlayerID string
	Action   PlayerAction
	Turn     int
}

const (
	disconnectGrace   = 60 * time.Second
	logRevealInterval = 150 * time.Millisecond
)

func (s *GameSession) hasMutation(id string) bool {
	return s.ActiveMutation != nil && s.ActiveMutation.ID == id
}

func NewGameSession(cfg *config.Config) *GameSession {
	return &GameSession{
		cfg: cfg,
		state: GameState{
			FloorNum: 1,
		},
		actions:             make(map[string]PlayerAction),
		actionCh:            make(chan playerActionMsg, 4),
		combatSignal:        make(chan struct{}, 1),
		playerLeftCh:        make(chan struct{}, 4),
		done:                make(chan struct{}),
		disconnectedAt:      make(map[string]time.Time),
		recorded:            make(chan struct{}),
		clock:               time.Now,
		rng:                 rand.New(rand.NewSource(time.Now().UnixNano())),
		coopUsedThisCombat:  make(map[string]bool),
		healerReviveUsed:    make(map[string]bool),
		scoutUsedThisFloor:  make(map[string]bool),
		soloHealerRevived:   make(map[string]bool),
		coopDamageReduction: make(map[string]float64),
		invincibleNextTurn:  make(map[string]bool),
		dodgeBonusThisTurn:  make(map[string]float64),
		extraActionNext:     make(map[string]bool),
		activeRelicUsed:     make(map[string]bool),
	}
}

func (s *GameSession) reseedForFloor() {
	if s.DailyMode {
		s.rng = rand.New(rand.NewSource(DailySeed(s.DailyDate) + int64(s.state.FloorNum)))
		return
	}
	s.rng = rand.New(rand.NewSource(time.Now().UnixNano() + int64(s.state.FloorNum)))
}

func (s *GameSession) ApplyWeeklyMutation() {
	mut := GetWeeklyMutation()
	s.ActiveMutation = &mut
}

func (s *GameSession) effectiveSkillUses() int {
	if s.hasMutation("no_skills") {
		return 0
	}
	uses := s.cfg.Game.SkillUses
	if s.ActiveCurse != nil {
		uses += s.ActiveCurse.SkillUseDelta
	}
	if s.DailyMod != nil {
		uses += s.DailyMod.SkillUseDelta
	}
	if uses < 0 {
		uses = 0
	}
	return uses
}

func (s *GameSession) effectiveTurnTimeout() int {
	t := s.cfg.Game.TurnTimeoutSec
	if s.hasMutation("speed_run") {
		t = t / 2
		if t < 2 {
			t = 2
		}
	}
	return t
}

func (s *GameSession) Stop() {
	s.stopOnce.Do(func() { close(s.done) })
}

func (s *GameSession) IsOver() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.GameOver
}

func (s *GameSession) setOnAbandoned(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAbandoned = fn
}

func (s *GameSession) StartGame() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.state.SoloMode = len(s.state.Players) == 1
	s.state.StartingPartySize = len(s.state.Players)
	s.mu.Unlock()
	s.StartFloor()
	go s.combatLoop()
}

func (s *GameSession) combatLoop() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.mu.Lock()
		phase := s.state.Phase
		gameOver := s.state.GameOver
		s.mu.Unlock()

		if gameOver {

			slog.Info("game over", "floor", s.state.FloorNum, "victory", s.state.Victory)
			s.recordRun()
			return
		}

		s.mu.Lock()
		if s.sweepDisconnectedLocked(s.clock()) && len(s.state.Players) == 0 {
			s.state.GameOver = true
			onAbandoned := s.onAbandoned
			s.mu.Unlock()
			if onAbandoned != nil {
				onAbandoned()
			}
			return
		}
		s.mu.Unlock()

		if phase == PhaseCombat {
			s.RunTurn()
		} else {

			select {
			case <-s.combatSignal:
			case <-time.After(10 * time.Second):
			case <-s.done:
				return
			}
		}
	}
}

func (s *GameSession) signalCombat() {
	select {
	case s.combatSignal <- struct{}{}:
	default:
	}
}

func (s *GameSession) RemovePlayer(fingerprint string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fingerprint == "" {
		return len(s.state.Players)
	}
	idx := -1
	for i, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			idx = i
			break
		}
	}
	if idx < 0 {
		return -1
	}
	s.state.Players = append(s.state.Players[:idx], s.state.Players[idx+1:]...)
	delete(s.disconnectedAt, fingerprint)
	if s.moveVotes != nil {
		delete(s.moveVotes, fingerprint)
	}
	if s.shopLeavers != nil {
		delete(s.shopLeavers, fingerprint)
	}
	if s.state.MoveVotes != nil {
		delete(s.state.MoveVotes, fingerprint)
	}
	if s.state.SubmittedActions != nil {
		delete(s.state.SubmittedActions, fingerprint)
	}
	if s.actions != nil {
		delete(s.actions, fingerprint)
	}
	if s.eventVotes != nil {
		delete(s.eventVotes, fingerprint)
	}
	if s.state.EventVotes != nil {
		delete(s.state.EventVotes, fingerprint)
	}
	delete(s.coopUsedThisCombat, fingerprint)
	delete(s.healerReviveUsed, fingerprint)
	delete(s.scoutUsedThisFloor, fingerprint)
	delete(s.soloHealerRevived, fingerprint)
	delete(s.coopDamageReduction, fingerprint)
	delete(s.invincibleNextTurn, fingerprint)
	delete(s.dodgeBonusThisTurn, fingerprint)
	delete(s.extraActionNext, fingerprint)
	delete(s.activeRelicUsed, fingerprint)

	s.recheckPhaseQuorumLocked()

	select {
	case s.playerLeftCh <- struct{}{}:
	default:
	}
	return len(s.state.Players)
}

func (s *GameSession) AddPlayer(p *entity.Player) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.Fingerprint != "" {
		for _, existing := range s.state.Players {
			if existing.Fingerprint == p.Fingerprint {
				return
			}
		}
	}
	if p.Skills == nil {
		p.Skills = &entity.PlayerSkills{BranchIndex: -1}
	}
	if s.hasMutation("glass_cannon") {
		p.ATK *= 2
		p.MaxHP /= 2
		if p.MaxHP < 1 {
			p.MaxHP = 1
		}
		p.HP = p.MaxHP
	}
	if s.DailyMod != nil && s.DailyMod.StartGoldBonus > 0 {
		p.Gold += s.DailyMod.StartGoldBonus
	}
	if s.ActiveCurse != nil {
		c := s.ActiveCurse
		if c.HPMult != 0 && c.HPMult != 1.0 {
			p.MaxHP = int(float64(p.MaxHP) * c.HPMult)
			if p.MaxHP < 1 {
				p.MaxHP = 1
			}
			p.HP = p.MaxHP
		}
		if c.ATKMult != 0 && c.ATKMult != 1.0 {
			p.ATK = int(float64(p.ATK) * c.ATKMult)
			if p.ATK < 1 {
				p.ATK = 1
			}
		}
		if c.DEFMult != 0 && c.DEFMult != 1.0 {
			p.DEF = int(float64(p.DEF) * c.DEFMult)
			if p.DEF < 0 {
				p.DEF = 0
			}
		}
	}
	s.state.Players = append(s.state.Players, p)
}

func (s *GameSession) StartFloor() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reseedForFloor()
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, s.rng)
	s.state.Phase = PhaseExploring
	s.state.TurnNum = 0

	for _, p := range s.state.Players {
		if p.IsDead() {
			p.Revive(0.30)
		}
	}
}

func (s *GameSession) endRunIfPartyWipedLocked() {
	for _, p := range s.state.Players {
		if !p.IsOut() {
			return
		}
	}
	s.state.Phase = PhaseResult
	s.state.GameOver = true
	s.addLog("파티가 전멸했습니다!")
	s.signalCombat()
}
