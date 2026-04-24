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
)

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
)

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
	LastEventName    string // name of the most recent random event (for codex)
	MoveVotes        map[string]int // fingerprint -> voted room index (exploration)
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
	done           chan struct{}
	lastActivity   map[string]time.Time // fingerprint -> last activity time
	moveVotes      map[string]int       // fingerprint -> voted room index
	HardMode       bool
	ActiveMutation *Mutation
	DailyMode      bool
	DailyDate      string
}

type playerActionMsg struct {
	PlayerID string
	Action   PlayerAction
}

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
		actions:      make(map[string]PlayerAction),
		actionCh:     make(chan playerActionMsg, 4),
		combatSignal: make(chan struct{}, 1),
		done:         make(chan struct{}),
		lastActivity: make(map[string]time.Time),
	}
}

// ApplyWeeklyMutation sets the current week's mutation on this session.
func (s *GameSession) ApplyWeeklyMutation() {
	mut := GetWeeklyMutation()
	s.ActiveMutation = &mut
	mut.Apply(&s.cfg.Game)
}

func (s *GameSession) Stop() {
	select {
	case <-s.done:
		// already stopped
	default:
		close(s.done)
	}
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
	now := time.Now()
	for _, p := range s.state.Players {
		s.lastActivity[p.Fingerprint] = now
	}
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
			slog.Info("game over", "floor", s.state.FloorNum, "victory", s.state.Victory)
			return
		}

		// Remove players inactive for >60 seconds
		s.mu.Lock()
		now := time.Now()
		changed := false
		remaining := make([]*entity.Player, 0, len(s.state.Players))
		for _, p := range s.state.Players {
			if p.Fingerprint != "" && !p.IsOut() {
				if last, ok := s.lastActivity[p.Fingerprint]; ok {
					if now.Sub(last) > 60*time.Second {
						slog.Warn("player inactive removed", "fingerprint", p.Fingerprint, "name", p.Name)
						s.addLog(fmt.Sprintf("%s 제거됨 (접속 끊김)", p.Name))
						changed = true
						continue
					}
				}
			}
			remaining = append(remaining, p)
		}
		if changed {
			s.state.Players = remaining
			if len(s.state.Players) == 0 {
				s.state.GameOver = true
				s.mu.Unlock()
				return
			}
		}
		s.mu.Unlock()

		if phase == PhaseCombat {
			s.RunTurn()
		} else {
			select {
			case <-s.combatSignal:
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

func (s *GameSession) AddPlayer(p *entity.Player) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.state.Players = append(s.state.Players, p)
}

func (s *GameSession) StartFloor() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.DailyMode {
		seed := DailySeed(s.DailyDate) + int64(s.state.FloorNum)
		s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, rand.New(rand.NewSource(seed)))
	} else {
		s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, rand.New(rand.NewSource(time.Now().UnixNano())))
	}
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

	// Deep copy players
	players := make([]*entity.Player, len(s.state.Players))
	for i, p := range s.state.Players {
		cp := *p
		cp.Inventory = make([]entity.Item, len(p.Inventory))
		copy(cp.Inventory, p.Inventory)
		cp.Relics = make([]entity.Relic, len(p.Relics))
		copy(cp.Relics, p.Relics)
		cp.Effects = make([]entity.ActiveEffect, len(p.Effects))
		copy(cp.Effects, p.Effects)
		if p.Skills != nil {
			skillsCopy := *p.Skills
			cp.Skills = &skillsCopy
		}
		players[i] = &cp
	}

	// Deep copy monsters
	monsters := make([]*entity.Monster, len(s.state.Monsters))
	for i, m := range s.state.Monsters {
		cm := *m
		monsters[i] = &cm
	}

	// Deep copy floor
	var floorCopy *dungeon.Floor
	if s.state.Floor != nil {
		fc := *s.state.Floor
		fc.Rooms = make([]*dungeon.Room, len(s.state.Floor.Rooms))
		for i, r := range s.state.Floor.Rooms {
			rc := *r
			rc.Neighbors = make([]int, len(r.Neighbors))
			copy(rc.Neighbors, r.Neighbors)
			fc.Rooms[i] = &rc
		}
		floorCopy = &fc
	}

	// Copy combat log
	logCopy := make([]string, len(s.state.CombatLog))
	copy(logCopy, s.state.CombatLog)

	// Copy submitted actions
	submittedCopy := make(map[string]string, len(s.state.SubmittedActions))
	for k, v := range s.state.SubmittedActions {
		submittedCopy[k] = v
	}

	// Copy move votes
	var moveVotesCopy map[string]int
	if s.state.MoveVotes != nil {
		moveVotesCopy = make(map[string]int, len(s.state.MoveVotes))
		for k, v := range s.state.MoveVotes {
			moveVotesCopy[k] = v
		}
	}

	// Copy pending logs
	pendingCopy := make([]string, len(s.state.PendingLogs))
	copy(pendingCopy, s.state.PendingLogs)

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
		ShopItems:        append([]entity.Item{}, s.state.ShopItems...),
		CombatLog:        logCopy,
		TurnDeadline:     s.state.TurnDeadline,
		SubmittedActions: submittedCopy,
		PendingLogs:      pendingCopy,
		TurnResolving:    s.state.TurnResolving,
		BossKilled:       s.state.BossKilled,
		FleeSucceeded:    s.state.FleeSucceeded,
		LastEventName:    s.state.LastEventName,
		MoveVotes:        moveVotesCopy,
	}
}

func (s *GameSession) SubmitAction(playerID string, action PlayerAction) {
	s.mu.Lock()
	s.lastActivity[playerID] = time.Now()

	// Block dead/out players from submitting
	for _, p := range s.state.Players {
		if p.Fingerprint == playerID && p.IsOut() {
			s.mu.Unlock()
			return
		}
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
	s.mu.Unlock()
	s.actionCh <- playerActionMsg{PlayerID: playerID, Action: action}
}

// RevealNextLog moves one log from PendingLogs to CombatLog. Returns true if there was one to reveal.
func (s *GameSession) RevealNextLog() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.PendingLogs) == 0 {
		return false
	}
	msg := s.state.PendingLogs[0]
	s.state.PendingLogs = s.state.PendingLogs[1:]
	s.state.CombatLog = append(s.state.CombatLog, msg)
	if len(s.state.CombatLog) > 8 {
		s.state.CombatLog = s.state.CombatLog[len(s.state.CombatLog)-8:]
	}
	return true
}

func (s *GameSession) TouchActivity(fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity[fingerprint] = time.Now()
}

// AllocateSkillPoint spends one skill point into the given branch for the player.
func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			if p.Skills == nil || p.Skills.Points <= p.Skills.Allocated {
				return fmt.Errorf("스킬 포인트가 없습니다")
			}
			return p.Skills.Allocate(branchIdx, p.Class)
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

// LeaveShop exits the shop phase
func (s *GameSession) LeaveShop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Phase = PhaseExploring
	s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
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

	s.lastActivity[fingerprint] = time.Now()

	if s.moveVotes == nil {
		s.moveVotes = make(map[string]int)
	}
	s.moveVotes[fingerprint] = roomIdx

	// Copy votes to state for UI display
	s.state.MoveVotes = make(map[string]int, len(s.moveVotes))
	for k, v := range s.moveVotes {
		s.state.MoveVotes[k] = v
	}

	// Check if all alive players have voted
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
			if _, ok := s.moveVotes[p.Fingerprint]; ok {
				voteCount++
			}
		}
	}

	if voteCount < aliveCount {
		return false
	}

	// All voted — resolve by majority
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
		return false
	}

	// Clear votes
	s.moveVotes = nil
	s.state.MoveVotes = nil

	// Execute the move (inline EnterRoom logic since we already hold the lock)
	s.enterRoomLocked(bestRoom)
	return true
}

// ClearMoveVotes resets any pending move votes (e.g. when phase changes).
func (s *GameSession) ClearMoveVotes() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveVotes = nil
	s.state.MoveVotes = nil
}
