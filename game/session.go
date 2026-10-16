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
	actions        map[string]PlayerAction // playerName -> action
	actionCh       chan playerActionMsg
	combatSignal   chan struct{}
	playerLeftCh   chan struct{} // wakes RunTurn's collect loop on mid-turn quit
	done           chan struct{}
	stopOnce       sync.Once
	onAbandoned    func()
	disconnectedAt map[string]time.Time
	clock          func() time.Time
	lastReveal     time.Time
	recorder       RunRecorder
	recordOnce     sync.Once
	recorded       chan struct{}
	moveVotes      map[string]int  // fingerprint -> voted room index
	shopLeavers    map[string]bool // fingerprint -> wants to leave the shop
	eventVotes     map[string]int  // fingerprint → choice index
	pendingEvent   *RandomEvent    // typed event with Resolve funcs; not snapshotted
	eventPerPlayer bool            // pendingEvent resolves per player (loot) vs party majority
	rng            *rand.Rand      // session-scoped RNG; deterministic for daily runs

	// Coop ability state — all maps keyed by player fingerprint.
	coopUsedThisCombat  map[string]bool    // reset on combat start
	healerReviveUsed    map[string]bool    // run-scoped (revive arm of Divine Grace)
	scoutUsedThisFloor  map[string]bool    // reset on floor advance
	soloHealerRevived   map[string]bool    // run-scoped solo auto-revive
	coopDamageReduction map[string]float64 // applies for current turn only
	invincibleNextTurn  map[string]bool    // applies for next turn (Sanctuary combo)
	dodgeBonusThisTurn  map[string]float64 // Smoke Bomb: extra dodge%, fingerprint→bonus (0..1)
	extraActionNext     map[string]bool    // Time Glass: bearer gets a second action next turn
	activeRelicUsed     map[string]bool    // fingerprint → already used active relic this combat
	nextRoomPreview     *RoomPreview       // populated by Foresight, consumed at next room entry
	critGuaranteedNext  bool               // next turn party crit (Expose Weakness combo)
	enemyMissNextTurn   bool               // next turn 50% enemy attacks miss (Mirage)
	poisonNextCombat    bool               // next combat starts enemies poisoned (Venomous Edge)

	// Solo chain state — per-combat action sequence for SoloMode chains
	// (the solo replacement for cross-class party combos).
	soloLastAction   string // previous turn's successful action ("" breaks chains)
	soloAttackStreak int    // consecutive attacks so far this combat

	// The fields below are set during session construction (inside an
	// EnsureSession init callback under Lobby.mu) and are not mutated
	// thereafter. Reading them without s.mu is safe because the lobby-lock
	// release establishes happens-before for every subsequent reader.
	HardMode       bool
	ActiveMutation *Mutation
	DailyMode      bool
	DailyDate      string

	// New Game+ state — same publication invariant as above.
	NGPlusLevel int
	ActiveCurse *Curse

	// Daily challenge modifier (set by lobby when DailyMode = true) — same
	// publication invariant as above.
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

// hasMutation returns true if the session has the given mutation active.
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

// reseedForFloor refreshes the session RNG when entering a new floor. In
// DailyMode the seed is derived from (date + floor) so two players running
// the same daily on the same date face identical monster spawns, item rolls,
// and event outcomes. Outside DailyMode we fold in the floor number so each
// floor still gets fresh randomness rather than reusing the prior floor's
// stream. Callers must hold s.mu.
func (s *GameSession) reseedForFloor() {
	if s.DailyMode {
		s.rng = rand.New(rand.NewSource(DailySeed(s.DailyDate) + int64(s.state.FloorNum)))
		return
	}
	s.rng = rand.New(rand.NewSource(time.Now().UnixNano() + int64(s.state.FloorNum)))
}

// ApplyWeeklyMutation sets the current week's mutation on this session.
func (s *GameSession) ApplyWeeklyMutation() {
	mut := GetWeeklyMutation()
	s.ActiveMutation = &mut
}

// effectiveSkillUses returns the per-combat skill use budget, accounting for
// the active mutation and any NG+ curse. Reading via this helper avoids
// mutating shared config.
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

// effectiveTurnTimeout returns the per-turn timeout in seconds, accounting
// for the active mutation.
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

// StartGame determines solo mode from actual player count at game start
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

// combatLoop continuously runs turns while in combat phase
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
			// Reading state without s.mu here is safe: FloorNum/Victory are
			// only written inside RunTurn (advanceFloor / turn.go), all on
			// this same combatLoop goroutine.
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
			// Wake periodically so the inactivity sweep above runs even
			// during PhaseShop / PhaseExploring. Otherwise a disconnected
			// player can deadlock the shop-leave consensus or move-vote
			// (both require every alive fingerprint to participate) until
			// the next combat — which never arrives without the move.
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

// RemovePlayer removes a player from the session by fingerprint and returns
// the number of remaining players. Returns -1 if the fingerprint was not found
// (so callers can distinguish a no-op from a successful drop to zero).
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
	// The leaver might have been the last voter the party was waiting on —
	// re-evaluate the current phase's quorum so we don't deadlock.
	s.recheckPhaseQuorumLocked()
	// Wake RunTurn's collect loop in case it's blocked waiting for this
	// player's never-arriving action.
	select {
	case s.playerLeftCh <- struct{}{}:
	default:
	}
	return len(s.state.Players)
}

func (s *GameSession) AddPlayer(p *entity.Player) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Dedup by fingerprint — a player who left the waiting room and rejoined
	// must not be added twice to the session's player list.
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

	// Revive dead players at 30% HP
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
