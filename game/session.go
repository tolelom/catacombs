package game

import (
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

type GamePhase int

const (
	PhaseExploring GamePhase = iota
	PhaseCombat
	PhaseShop
	PhaseResult
	PhaseEvent
)

// EventChoiceView is a snapshot-safe view of one event choice for the UI.
// The Resolve function is intentionally omitted — it is non-copyable behavior
// state and lives only on the typed *RandomEvent in the session.
type EventChoiceView struct {
	Label string
	Tags  []ChoiceTag
}

// SoloChainView surfaces the solo-chain sequence state for the HUD.
// Populated only during solo-mode combat; nil otherwise.
type SoloChainView struct {
	LastAction   string
	AttackStreak int
}

// PendingEventView is a snapshot-safe view of the active event, populated
// while Phase == PhaseEvent and nil otherwise.
type PendingEventView struct {
	Name        string
	Description string
	PerPlayer   bool // true for loot picks: each player's own choice applies to them
	Choices     []EventChoiceView
}

type PlayerAction struct {
	Type      ActionType
	TargetIdx int
}

type ActionType int

const (
	ActionAttack ActionType = iota
	ActionSkill
	ActionItem
	ActionFlee
	ActionWait
	ActionCoop
	ActionRelic
)

// RoomPreview is what Foresight reveals to the party about the next room
// before they vote to enter it. Boss rooms hide the monster names but
// expose the room type.
type RoomPreview struct {
	RoomIdx       int
	RoomTypeLabel string
	MonsterNames  []string
	HideMonsters  bool
}

type GameState struct {
	Floor      *dungeon.Floor
	Players    []*entity.Player
	Monsters   []*entity.Monster
	Phase      GamePhase
	FloorNum   int
	TurnNum    int
	CombatTurn int // reset per combat encounter
	SoloMode   bool
	GameOver   bool
	Victory    bool
	ShopItems  []entity.Item
	CombatLog        []string // recent combat messages
	TurnDeadline     time.Time
	SubmittedActions map[string]string // fingerprint -> action description
	PendingLogs      []string          // logs waiting to be revealed one by one
	TurnResolving    bool              // true while logs are being replayed
	BossKilled       bool
	FleeSucceeded    bool
	AnyPlayerDied    bool // true once any player has died this run; powers the no_death achievement
	StartingPartySize int // number of players at StartGame; powers the full_party achievement
	LastEventName    string // name of the most recent random event (for codex)
	MoveVotes        map[string]int // fingerprint -> voted room index (exploration)
	PendingEvent     *PendingEventView // active event during PhaseEvent
	EventVotes       map[string]int    // fingerprint → choice index
	ShopSold         []bool            // parallel to ShopItems; true once a weapon/armor slot is sold (consumables and charms restock freely)
	ShopLeavers      map[string]bool // fingerprint -> wants to leave the shop (UI display only)
	NextRoomPreview  *RoomPreview    // populated when Foresight has been used
	CoopAvailable    map[string]bool // fingerprint → can use coop now (HUD display)
	SoloChain        *SoloChainView  // solo-mode combat chain state (HUD display)
	// Run-scoped event flags (drives polish achievements at GameOver)
	CombosFiredThisRun map[string]bool // combo name → fired at least once this run
	BossPhase2Reached  bool            // any boss reached Phase 2
	MimicKilled        bool            // any mimic monster defeated
	AltarUsedThisRun   int             // cursed altar uses this run
	RevivesThisRun     int             // Divine Grace revives this run
	BossesKilledThisRun map[string]bool // boss name → killed this run
}

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
	moveVotes      map[string]int       // fingerprint -> voted room index
	shopLeavers    map[string]bool      // fingerprint -> wants to leave the shop
	eventVotes     map[string]int       // fingerprint → choice index
	pendingEvent   *RandomEvent         // typed event with Resolve funcs; not snapshotted
	eventPerPlayer bool                 // pendingEvent resolves per player (loot) vs party majority
	rng            *rand.Rand           // session-scoped RNG; deterministic for daily runs

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

// recheckPhaseQuorumLocked runs the per-phase quorum tally and resolves
// the phase if quorum is met. Used by RemovePlayer and the combatLoop
// inactivity sweep so a player leaving (explicit Q or 60s timeout) cannot
// strand the party waiting for a vote they will never cast. Caller holds
// s.mu.
func (s *GameSession) recheckPhaseQuorumLocked() {
	switch s.state.Phase {
	case PhaseExploring:
		if winner, met := s.tallyMoveQuorumLocked(); met {
			s.moveVotes = nil
			s.state.MoveVotes = nil
			s.enterRoomLocked(winner)
		}
	case PhaseShop:
		if !s.tallyShopLeaversLocked() {
			return
		}
		s.shopLeavers = nil
		s.state.Phase = PhaseExploring
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
	case PhaseEvent:
		// Mirror the sweep behavior: prune any stale votes for fingerprints
		// no longer in the alive set (RemovePlayer already deletes from the
		// maps but defense-in-depth keeps the tally correct if a future
		// caller forgets).
		alive := make(map[string]bool, len(s.state.Players))
		for _, p := range s.state.Players {
			if !p.IsOut() && p.Fingerprint != "" {
				alive[p.Fingerprint] = true
			}
		}
		for fp := range s.eventVotes {
			if !alive[fp] {
				delete(s.eventVotes, fp)
				delete(s.state.EventVotes, fp)
			}
		}
		if len(alive) == 0 {
			s.clearPendingEventLocked(true)
			return
		}
		if s.tallyEventQuorumLocked() {
			s.resolveEventVoteLocked()
		}
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

func (s *GameSession) GetState() GameState {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deep copy players. Skip allocating zero-length slices (most players
	// have no Effects between combats, fresh players have no Inventory or
	// Relics). At ~5 Hz polling per client these allocations dominate idle
	// CPU otherwise.
	var players []*entity.Player
	if n := len(s.state.Players); n > 0 {
		players = make([]*entity.Player, n)
		for i, p := range s.state.Players {
			cp := *p
			if len(p.Inventory) > 0 {
				cp.Inventory = make([]entity.Item, len(p.Inventory))
				copy(cp.Inventory, p.Inventory)
			} else {
				cp.Inventory = nil
			}
			if len(p.Relics) > 0 {
				cp.Relics = make([]entity.Relic, len(p.Relics))
				copy(cp.Relics, p.Relics)
			} else {
				cp.Relics = nil
			}
			if len(p.Effects) > 0 {
				cp.Effects = make([]entity.ActiveEffect, len(p.Effects))
				copy(cp.Effects, p.Effects)
			} else {
				cp.Effects = nil
			}
			if p.Skills != nil {
				skillsCopy := *p.Skills
				cp.Skills = &skillsCopy
			}
			players[i] = &cp
		}
	}

	// Deep copy monsters
	var monsters []*entity.Monster
	if n := len(s.state.Monsters); n > 0 {
		monsters = make([]*entity.Monster, n)
		for i, m := range s.state.Monsters {
			cm := *m
			monsters[i] = &cm
		}
	}

	// Deep copy floor
	var floorCopy *dungeon.Floor
	if s.state.Floor != nil {
		fc := *s.state.Floor
		fc.Rooms = make([]*dungeon.Room, len(s.state.Floor.Rooms))
		for i, r := range s.state.Floor.Rooms {
			rc := *r
			if len(r.Neighbors) > 0 {
				rc.Neighbors = make([]int, len(r.Neighbors))
				copy(rc.Neighbors, r.Neighbors)
			} else {
				rc.Neighbors = nil
			}
			fc.Rooms[i] = &rc
		}
		floorCopy = &fc
	}

	// Copy combat log only when non-empty.
	var logCopy []string
	if n := len(s.state.CombatLog); n > 0 {
		logCopy = make([]string, n)
		copy(logCopy, s.state.CombatLog)
	}

	// Copy submitted actions only when non-empty.
	var submittedCopy map[string]string
	if n := len(s.state.SubmittedActions); n > 0 {
		submittedCopy = make(map[string]string, n)
		for k, v := range s.state.SubmittedActions {
			submittedCopy[k] = v
		}
	}

	// Copy move votes
	var moveVotesCopy map[string]int
	if n := len(s.state.MoveVotes); n > 0 {
		moveVotesCopy = make(map[string]int, n)
		for k, v := range s.state.MoveVotes {
			moveVotesCopy[k] = v
		}
	}

	// Copy event votes
	var eventVotesCopy map[string]int
	if s.state.EventVotes != nil {
		n := len(s.state.EventVotes)
		eventVotesCopy = make(map[string]int, n)
		for k, v := range s.state.EventVotes {
			eventVotesCopy[k] = v
		}
	}

	// Copy pending event view (deep — Choices slice + per-Choice Tags slice)
	var pendingEventCopy *PendingEventView
	if src := s.state.PendingEvent; src != nil {
		choices := make([]EventChoiceView, len(src.Choices))
		for i, c := range src.Choices {
			tags := make([]ChoiceTag, len(c.Tags))
			copy(tags, c.Tags)
			choices[i] = EventChoiceView{Label: c.Label, Tags: tags}
		}
		pendingEventCopy = &PendingEventView{
			Name:        src.Name,
			Description: src.Description,
			PerPlayer:   src.PerPlayer,
			Choices:     choices,
		}
	}

	// Mirror shopLeavers for the UI's "N/M ready to leave" display.
	var shopLeaversCopy map[string]bool
	if n := len(s.shopLeavers); n > 0 {
		shopLeaversCopy = make(map[string]bool, n)
		for k, v := range s.shopLeavers {
			shopLeaversCopy[k] = v
		}
	}

	// Copy pending logs
	var pendingCopy []string
	if n := len(s.state.PendingLogs); n > 0 {
		pendingCopy = make([]string, n)
		copy(pendingCopy, s.state.PendingLogs)
	}

	// Copy shop wares
	var shopItemsCopy []entity.Item
	if n := len(s.state.ShopItems); n > 0 {
		shopItemsCopy = make([]entity.Item, n)
		copy(shopItemsCopy, s.state.ShopItems)
	}
	var shopSoldCopy []bool
	if n := len(s.state.ShopSold); n > 0 {
		shopSoldCopy = make([]bool, n)
		copy(shopSoldCopy, s.state.ShopSold)
	}

	return GameState{
		Floor:            floorCopy,
		Players:          players,
		Monsters:         monsters,
		Phase:            s.state.Phase,
		FloorNum:         s.state.FloorNum,
		TurnNum:          s.state.TurnNum,
		CombatTurn:       s.state.CombatTurn,
		SoloMode:         s.state.SoloMode,
		GameOver:         s.state.GameOver,
		Victory:          s.state.Victory,
		ShopItems:        shopItemsCopy,
		ShopSold:         shopSoldCopy,
		CombatLog:        logCopy,
		TurnDeadline:     s.state.TurnDeadline,
		SubmittedActions: submittedCopy,
		PendingLogs:      pendingCopy,
		TurnResolving:    s.state.TurnResolving,
		BossKilled:       s.state.BossKilled,
		FleeSucceeded:    s.state.FleeSucceeded,
		AnyPlayerDied:    s.state.AnyPlayerDied,
		StartingPartySize: s.state.StartingPartySize,
		LastEventName:    s.state.LastEventName,
		MoveVotes:        moveVotesCopy,
		PendingEvent:     pendingEventCopy,
		EventVotes:       eventVotesCopy,
		ShopLeavers:        shopLeaversCopy,
		NextRoomPreview:    s.nextRoomPreview,
		CoopAvailable:      s.computeCoopAvailable(),
		SoloChain:          s.soloChainView(),
		CombosFiredThisRun: copyStringBoolMap(s.state.CombosFiredThisRun),
		BossPhase2Reached:  s.state.BossPhase2Reached,
		MimicKilled:        s.state.MimicKilled,
		AltarUsedThisRun:   s.state.AltarUsedThisRun,
		RevivesThisRun:     s.state.RevivesThisRun,
		BossesKilledThisRun: copyStringBoolMap(s.state.BossesKilledThisRun),
	}
}

// soloChainView builds the HUD snapshot of the solo-chain state, or nil when
// it doesn't apply (party mode or out of combat). Caller holds s.mu.
func (s *GameSession) soloChainView() *SoloChainView {
	if !s.state.SoloMode || s.state.Phase != PhaseCombat {
		return nil
	}
	return &SoloChainView{
		LastAction:   s.soloLastAction,
		AttackStreak: s.soloAttackStreak,
	}
}

func copyStringBoolMap(in map[string]bool) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// computeCoopAvailable returns a map of fingerprint→true when that player can
// fire their coop ability right now. Caller must hold s.mu.
func (s *GameSession) computeCoopAvailable() map[string]bool {
	out := make(map[string]bool, len(s.state.Players))
	for _, p := range s.state.Players {
		if p.IsOut() || p.Fingerprint == "" {
			continue
		}
		switch p.Class {
		case entity.ClassRogue:
			if s.state.Phase == PhaseExploring && !s.scoutUsedThisFloor[p.Fingerprint] {
				out[p.Fingerprint] = true
			}
		default:
			if s.state.Phase == PhaseCombat && !s.coopUsedThisCombat[p.Fingerprint] {
				out[p.Fingerprint] = true
			}
		}
	}
	return out
}

// SubmitForesight is the Rogue's coop ability. Available once per floor while
// exploring. Reveals the next room's type and exposes monster room context for
// informed party voting. Boss rooms hide monster names but show "보스" type.
func (s *GameSession) SubmitForesight(fingerprint string, roomIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseExploring {
		return fmt.Errorf("정찰은 탐험 중에만 사용 가능")
	}
	if s.scoutUsedThisFloor[fingerprint] {
		return fmt.Errorf("정찰은 층당 1회만 사용 가능")
	}
	var rogue *entity.Player
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			rogue = p
			break
		}
	}
	if rogue == nil || rogue.IsOut() || rogue.Class != entity.ClassRogue {
		return fmt.Errorf("정찰은 살아있는 도적만 사용 가능")
	}
	if s.state.Floor == nil || roomIdx < 0 || roomIdx >= len(s.state.Floor.Rooms) {
		return fmt.Errorf("유효하지 않은 방")
	}
	room := s.state.Floor.Rooms[roomIdx]
	preview := &RoomPreview{RoomIdx: roomIdx, RoomTypeLabel: room.Type.String()}
	switch room.Type {
	case dungeon.RoomCombat:
		preview.MonsterNames = []string{"전투방"}
	case dungeon.RoomBoss:
		preview.HideMonsters = true
	}
	s.nextRoomPreview = preview
	s.scoutUsedThisFloor[fingerprint] = true
	s.addLog(fmt.Sprintf("%s 정찰 사용! 다음 방: %s", rogue.Name, room.Type.String()))
	return nil
}

func (s *GameSession) SubmitAction(playerID string, action PlayerAction) {
	s.mu.Lock()
	if !s.isAliveMemberLocked(playerID) {
		s.mu.Unlock()
		return
	}

	// Prevent duplicate submissions in the same turn
	if _, already := s.state.SubmittedActions[playerID]; already {
		s.mu.Unlock()
		return
	}

	desc := ""
	switch action.Type {
	case ActionAttack:
		desc = "공격"
	case ActionSkill:
		desc = "스킬 사용"
	case ActionItem:
		desc = "아이템 사용"
	case ActionFlee:
		desc = "도주"
	case ActionWait:
		desc = "방어"
	}
	if s.state.SubmittedActions == nil {
		s.state.SubmittedActions = make(map[string]string)
	}
	s.state.SubmittedActions[playerID] = desc
	turn := s.state.TurnNum
	s.mu.Unlock()
	s.actionCh <- playerActionMsg{PlayerID: playerID, Action: action, Turn: turn}
}

func (s *GameSession) isAliveMemberLocked(fingerprint string) bool {
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			return !p.IsOut()
		}
	}
	return false
}

// RevealNextLog moves one log from PendingLogs to CombatLog. Returns true if there was one to reveal.
func (s *GameSession) RevealNextLog() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	if len(s.state.PendingLogs) == 0 || now.Sub(s.lastReveal) < logRevealInterval {
		return false
	}
	s.lastReveal = now
	msg := s.state.PendingLogs[0]
	s.state.PendingLogs = s.state.PendingLogs[1:]
	s.state.CombatLog = append(s.state.CombatLog, msg)
	if len(s.state.CombatLog) > 8 {
		s.state.CombatLog = s.state.CombatLog[len(s.state.CombatLog)-8:]
	}
	return true
}

func (s *GameSession) MarkDisconnected(fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disconnectedAt[fingerprint] = s.clock()
}

func (s *GameSession) MarkConnected(fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.disconnectedAt, fingerprint)
}

func (s *GameSession) sweepDisconnectedLocked(now time.Time) bool {
	remaining := make([]*entity.Player, 0, len(s.state.Players))
	for _, p := range s.state.Players {
		if s.pastDisconnectGraceLocked(p, now) {
			slog.Warn("disconnected player removed", "fingerprint", p.Fingerprint, "name", p.Name)
			s.addLog(fmt.Sprintf("%s 제거됨 (접속 끊김)", p.Name))
			delete(s.disconnectedAt, p.Fingerprint)
			continue
		}
		remaining = append(remaining, p)
	}
	if len(remaining) == len(s.state.Players) {
		return false
	}
	s.state.Players = remaining
	if len(remaining) > 0 {
		s.recheckPhaseQuorumLocked()
	}
	return true
}

func (s *GameSession) pastDisconnectGraceLocked(p *entity.Player, now time.Time) bool {
	since, ok := s.disconnectedAt[p.Fingerprint]
	return ok && now.Sub(since) > disconnectGrace
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

// AllocateSkillPoint spends one skill point into the given branch for the player.
func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int, choice byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			if p.Skills == nil || p.Skills.Points <= p.Skills.Allocated {
				return fmt.Errorf("스킬 포인트가 없습니다")
			}
			before := p.Skills.GetMaxHPBonus(p.Class)
			if err := p.Skills.Allocate(branchIdx, p.Class, choice); err != nil {
				return err
			}
			// MaxHPBoost nodes raise both MaxHP and current HP by the delta.
			delta := p.Skills.GetMaxHPBonus(p.Class) - before
			if delta > 0 {
				p.MaxHP += delta
				p.HP += delta
			}
			return nil
		}
	}
	return fmt.Errorf("플레이어를 찾을 수 없습니다")
}

// BuyResult describes the outcome of a shop purchase attempt.
type BuyResult int

const (
	BuyOK BuyResult = iota
	BuyNoGold
	BuyInventoryFull
	BuyFailed
)

// BuyItem handles shop purchases
func (s *GameSession) BuyItem(playerID string, itemIdx int) BuyResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop || itemIdx < 0 || itemIdx >= len(s.state.ShopItems) {
		return BuyFailed
	}
	if itemIdx < len(s.state.ShopSold) && s.state.ShopSold[itemIdx] {
		return BuyFailed
	}
	item := s.state.ShopItems[itemIdx]
	for _, p := range s.state.Players {
		if p.Fingerprint == playerID {
			if p.Gold < item.Price {
				return BuyNoGold
			}
			if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
				return BuyInventoryFull
			}
			p.Gold -= item.Price
			p.Inventory = append(p.Inventory, item)
			// Only gear sells out — potions and charms restock freely.
			if (item.Type == entity.ItemWeapon || item.Type == entity.ItemArmor) && itemIdx < len(s.state.ShopSold) {
				s.state.ShopSold[itemIdx] = true
			}
			return BuyOK
		}
	}
	return BuyFailed
}

// SendChat appends a chat message to the combat log
func (s *GameSession) SendChat(playerName, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if emoteText, ok := ParseEmote(message); ok {
		s.addLog(fmt.Sprintf("✨ %s %s", playerName, emoteText))
	} else {
		s.addLog(fmt.Sprintf("[%s] %s", playerName, message))
	}
}

// tallyShopLeaversLocked returns true when every available (non-out)
// player has signaled they are ready to leave the shop. Treats a zero-
// available party as ready so a wipe in the shop doesn't deadlock.
// Empty-fingerprint players are counted through the "" key rather than
// skipped, so their quorum slot stays fillable. Caller holds s.mu.
func (s *GameSession) tallyShopLeaversLocked() bool {
	available := 0
	ready := 0
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		available++
		if s.shopLeavers[p.Fingerprint] {
			ready++
		}
	}
	if available == 0 {
		return true
	}
	return ready >= available
}

// LeaveShop marks the calling player as ready to leave the shop. The phase
// only flips back to PhaseExploring once every available (non-out) player
// has pressed leave, so one impatient player can't yank others out mid-browse.
// Returns true if the call triggered the actual transition.
func (s *GameSession) LeaveShop(fingerprint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop {
		return false
	}
	if s.shopLeavers == nil {
		s.shopLeavers = make(map[string]bool)
	}
	// Empty-fingerprint players (legacy/test path) signal under the "" key,
	// mirroring move votes — excluding them entirely would make their quorum
	// slot unfillable and deadlock the shop for the whole party.
	s.shopLeavers[fingerprint] = true
	if !s.tallyShopLeaversLocked() {
		return false
	}
	s.shopLeavers = nil
	s.state.Phase = PhaseExploring
	if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
		s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
	}
	return true
}

// tallyMoveQuorumLocked returns the winning room index and true if every
// alive player has cast a move vote. Returns (-1, false) when quorum is
// not met or no alive players remain. Tie-break: lowest room index.
// Caller holds s.mu.
func (s *GameSession) tallyMoveQuorumLocked() (int, bool) {
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		return -1, false
	}
	voteCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.moveVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}
	if voteCount < aliveCount {
		return -1, false
	}
	tally := make(map[int]int)
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if room, ok := s.moveVotes[p.Fingerprint]; ok {
				tally[room]++
			}
		}
	}
	bestRoom := -1
	bestCount := 0
	for room, count := range tally {
		if count > bestCount || (count == bestCount && (bestRoom < 0 || room < bestRoom)) {
			bestRoom = room
			bestCount = count
		}
	}
	if bestRoom < 0 {
		return -1, false
	}
	return bestRoom, true
}

// SubmitMoveVote records a player's room choice during exploration.
// When all alive players have voted, the majority choice wins and the party moves.
// Returns true if the vote triggered a move (all votes collected).
func (s *GameSession) SubmitMoveVote(fingerprint string, roomIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Phase != PhaseExploring {
		return false
	}

	if s.moveVotes == nil {
		s.moveVotes = make(map[string]int)
	}
	s.moveVotes[fingerprint] = roomIdx

	// Mirror to snapshot map for UI display.
	s.state.MoveVotes = make(map[string]int, len(s.moveVotes))
	for k, v := range s.moveVotes {
		s.state.MoveVotes[k] = v
	}

	winner, met := s.tallyMoveQuorumLocked()
	if !met {
		return false
	}

	s.moveVotes = nil
	s.state.MoveVotes = nil
	s.enterRoomLocked(winner)
	return true
}

// ClearMoveVotes resets any pending move votes (e.g. when phase changes).
func (s *GameSession) ClearMoveVotes() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveVotes = nil
	s.state.MoveVotes = nil
}

// tallyEventQuorumLocked returns true if every alive player has cast an
// event vote. Does NOT compute a winner — winner selection lives in
// resolveEventVoteLocked. Returns false when no alive players remain.
// The anonymous-solo special case stays in SubmitEventVote because it
// depends on the caller's fingerprint context. Caller holds s.mu.
func (s *GameSession) tallyEventQuorumLocked() bool {
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		return false
	}
	voteCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			if _, ok := s.eventVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}
	return voteCount >= aliveCount
}

// SubmitEventVote records a player's event choice during PhaseEvent. When
// all alive players have voted, the majority choice (ties broken by lowest
// index) is resolved and applied to the party. Returns true if this call
// triggered resolution.
func (s *GameSession) SubmitEventVote(fingerprint string, choiceIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Phase != PhaseEvent || s.pendingEvent == nil {
		return false
	}
	if choiceIdx < 0 || choiceIdx >= len(s.pendingEvent.Choices) {
		return false
	}
	if s.eventVotes == nil {
		s.eventVotes = make(map[string]int)
	}
	if fingerprint != "" {
		s.eventVotes[fingerprint] = choiceIdx
	}

	// Mirror to snapshot map
	s.state.EventVotes = make(map[string]int, len(s.eventVotes))
	for k, v := range s.eventVotes {
		s.state.EventVotes[k] = v
	}

	// Defensive: nobody alive to receive the outcome. Clear and bail.
	aliveCount := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		s.clearPendingEventLocked(true)
		return true
	}
	// Anonymous single-player test path: the caller passed no fingerprint so
	// the vote did not land in s.eventVotes — treat this call as the sole
	// alive vote.
	if aliveCount == 1 && fingerprint == "" && len(s.eventVotes) == 0 {
		s.resolveEventVoteLocked()
		return true
	}
	if !s.tallyEventQuorumLocked() {
		return false
	}
	s.resolveEventVoteLocked()
	return true
}

// clearPendingEventLocked clears all pending-event state and optionally marks
// the current event room as cleared. Caller holds s.mu.
func (s *GameSession) clearPendingEventLocked(markRoomCleared bool) {
	s.pendingEvent = nil
	s.eventPerPlayer = false
	s.state.PendingEvent = nil
	s.state.EventVotes = nil
	s.eventVotes = nil
	if markRoomCleared && s.state.Floor != nil {
		idx := s.state.Floor.CurrentRoom
		if idx >= 0 && idx < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[idx].Cleared = true
		}
	}
	s.state.Phase = PhaseExploring
}
