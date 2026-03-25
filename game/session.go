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
	mu           sync.Mutex
	cfg          *config.Config
	state        GameState
	started      bool
	actions      map[string]PlayerAction // playerName -> action
	actionCh     chan playerActionMsg
	combatSignal chan struct{}
	done         chan struct{}
	lastActivity map[string]time.Time // fingerprint -> last activity time
}

type playerActionMsg struct {
	PlayerID string
	Action   PlayerAction
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
						s.addLog(fmt.Sprintf("%s removed (disconnected)", p.Name))
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
	s.state.Players = append(s.state.Players, p)
}

func (s *GameSession) StartFloor() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum, rand.New(rand.NewSource(time.Now().UnixNano())))
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
	}
}

func (s *GameSession) SubmitAction(playerID string, action PlayerAction) {
	s.mu.Lock()
	s.lastActivity[playerID] = time.Now()
	desc := ""
	switch action.Type {
	case ActionAttack:
		desc = "Attacking"
	case ActionSkill:
		desc = "Using Skill"
	case ActionItem:
		desc = "Using Item"
	case ActionFlee:
		desc = "Fleeing"
	case ActionWait:
		desc = "Defending"
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

// BuyItem handles shop purchases
func (s *GameSession) BuyItem(playerID string, itemIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop || itemIdx < 0 || itemIdx >= len(s.state.ShopItems) {
		return false
	}
	item := s.state.ShopItems[itemIdx]
	for _, p := range s.state.Players {
		if p.Fingerprint == playerID && p.Gold >= item.Price {
			if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
				return false
			}
			p.Gold -= item.Price
			p.Inventory = append(p.Inventory, item)
			return true
		}
	}
	return false
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
