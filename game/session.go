package game

import (
	"sync"

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
}

type GameSession struct {
	mu       sync.Mutex
	state    GameState
	actions  map[string]PlayerAction // playerName -> action
	actionCh chan playerActionMsg
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
		actions:  make(map[string]PlayerAction),
		actionCh: make(chan playerActionMsg, 4),
	}
}

// StartGame determines solo mode from actual player count at game start
func (s *GameSession) StartGame() {
	s.mu.Lock()
	s.state.SoloMode = len(s.state.Players) == 1
	s.mu.Unlock()
	s.StartFloor()
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
