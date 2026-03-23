package game

import (
	"sync"
	"time"

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
	CombatLog    []string // recent combat messages
	TurnDeadline time.Time
}

func (s *GameSession) addLog(msg string) {
	s.state.CombatLog = append(s.state.CombatLog, msg)
	// Keep last 5 messages
	if len(s.state.CombatLog) > 5 {
		s.state.CombatLog = s.state.CombatLog[len(s.state.CombatLog)-5:]
	}
}

func (s *GameSession) clearLog() {
	s.state.CombatLog = nil
}

type GameSession struct {
	mu           sync.Mutex
	state        GameState
	started      bool
	actions      map[string]PlayerAction // playerName -> action
	actionCh     chan playerActionMsg
	combatSignal chan struct{}
}

type playerActionMsg struct {
	PlayerName string
	Action     PlayerAction
}

func NewGameSession() *GameSession {
	return &GameSession{
		state: GameState{
			FloorNum: 1,
		},
		actions:      make(map[string]PlayerAction),
		actionCh:     make(chan playerActionMsg, 4),
		combatSignal: make(chan struct{}, 1),
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
	s.mu.Unlock()
	s.StartFloor()
	go s.combatLoop()
}

// combatLoop continuously runs turns while in combat phase
func (s *GameSession) combatLoop() {
	for {
		s.mu.Lock()
		phase := s.state.Phase
		gameOver := s.state.GameOver
		s.mu.Unlock()

		if gameOver {
			return
		}

		if phase == PhaseCombat {
			s.RunTurn() // blocks until all actions collected or timeout
		} else {
			// Not in combat, wait for an action signal to avoid busy-spinning
			// We'll just sleep briefly and re-check
			select {
			case <-s.combatSignal:
				// Room entered, combat may have started
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
	s.state.Floor = dungeon.GenerateFloor(s.state.FloorNum)
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
	return s.state
}

func (s *GameSession) SubmitAction(playerName string, action PlayerAction) {
	s.actionCh <- playerActionMsg{PlayerName: playerName, Action: action}
}

// BuyItem handles shop purchases
func (s *GameSession) BuyItem(playerName string, itemIdx int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Phase != PhaseShop || itemIdx < 0 || itemIdx >= len(s.state.ShopItems) {
		return false
	}
	item := s.state.ShopItems[itemIdx]
	for _, p := range s.state.Players {
		if p.Name == playerName && p.Gold >= item.Price {
			p.Gold -= item.Price
			p.Inventory = append(p.Inventory, item)
			return true
		}
	}
	return false
}

// LeaveShop exits the shop phase
func (s *GameSession) LeaveShop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Phase = PhaseExploring
	s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
}
