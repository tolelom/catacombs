package game

import (
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
	PhaseEvent
)

type EventChoiceView struct {
	Label string
	Tags  []ChoiceTag
}

type SoloChainView struct {
	LastAction   string
	AttackStreak int
}

type PendingEventView struct {
	Name        string
	Description string
	PerPlayer   bool
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

type RoomPreview struct {
	RoomIdx       int
	RoomTypeLabel string
	MonsterNames  []string
	HideMonsters  bool
}

type GameState struct {
	Floor             *dungeon.Floor
	Players           []*entity.Player
	Monsters          []*entity.Monster
	Phase             GamePhase
	FloorNum          int
	TurnNum           int
	CombatTurn        int
	SoloMode          bool
	GameOver          bool
	Victory           bool
	ShopItems         []entity.Item
	CombatLog         []string
	TurnDeadline      time.Time
	SubmittedActions  map[string]string
	PendingLogs       []string
	TurnResolving     bool
	BossKilled        bool
	FleeSucceeded     bool
	AnyPlayerDied     bool
	StartingPartySize int
	LastEventName     string
	MoveVotes         map[string]int
	PendingEvent      *PendingEventView
	EventVotes        map[string]int
	ShopSold          []bool
	ShopLeavers       map[string]bool
	NextRoomPreview   *RoomPreview
	CoopAvailable     map[string]bool
	SoloChain         *SoloChainView

	CombosFiredThisRun  map[string]bool
	BossPhase2Reached   bool
	MimicKilled         bool
	AltarUsedThisRun    int
	RevivesThisRun      int
	BossesKilledThisRun map[string]bool
}

func (s *GameSession) GetState() GameState {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	var monsters []*entity.Monster
	if n := len(s.state.Monsters); n > 0 {
		monsters = make([]*entity.Monster, n)
		for i, m := range s.state.Monsters {
			cm := *m
			monsters[i] = &cm
		}
	}

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

	var logCopy []string
	if n := len(s.state.CombatLog); n > 0 {
		logCopy = make([]string, n)
		copy(logCopy, s.state.CombatLog)
	}

	var submittedCopy map[string]string
	if n := len(s.state.SubmittedActions); n > 0 {
		submittedCopy = make(map[string]string, n)
		for k, v := range s.state.SubmittedActions {
			submittedCopy[k] = v
		}
	}

	var moveVotesCopy map[string]int
	if n := len(s.state.MoveVotes); n > 0 {
		moveVotesCopy = make(map[string]int, n)
		for k, v := range s.state.MoveVotes {
			moveVotesCopy[k] = v
		}
	}

	var eventVotesCopy map[string]int
	if s.state.EventVotes != nil {
		n := len(s.state.EventVotes)
		eventVotesCopy = make(map[string]int, n)
		for k, v := range s.state.EventVotes {
			eventVotesCopy[k] = v
		}
	}

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

	var shopLeaversCopy map[string]bool
	if n := len(s.shopLeavers); n > 0 {
		shopLeaversCopy = make(map[string]bool, n)
		for k, v := range s.shopLeavers {
			shopLeaversCopy[k] = v
		}
	}

	var pendingCopy []string
	if n := len(s.state.PendingLogs); n > 0 {
		pendingCopy = make([]string, n)
		copy(pendingCopy, s.state.PendingLogs)
	}

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
		Floor:               floorCopy,
		Players:             players,
		Monsters:            monsters,
		Phase:               s.state.Phase,
		FloorNum:            s.state.FloorNum,
		TurnNum:             s.state.TurnNum,
		CombatTurn:          s.state.CombatTurn,
		SoloMode:            s.state.SoloMode,
		GameOver:            s.state.GameOver,
		Victory:             s.state.Victory,
		ShopItems:           shopItemsCopy,
		ShopSold:            shopSoldCopy,
		CombatLog:           logCopy,
		TurnDeadline:        s.state.TurnDeadline,
		SubmittedActions:    submittedCopy,
		PendingLogs:         pendingCopy,
		TurnResolving:       s.state.TurnResolving,
		BossKilled:          s.state.BossKilled,
		FleeSucceeded:       s.state.FleeSucceeded,
		AnyPlayerDied:       s.state.AnyPlayerDied,
		StartingPartySize:   s.state.StartingPartySize,
		LastEventName:       s.state.LastEventName,
		MoveVotes:           moveVotesCopy,
		PendingEvent:        pendingEventCopy,
		EventVotes:          eventVotesCopy,
		ShopLeavers:         shopLeaversCopy,
		NextRoomPreview:     s.nextRoomPreview,
		CoopAvailable:       s.computeCoopAvailable(),
		SoloChain:           s.soloChainView(),
		CombosFiredThisRun:  copyStringBoolMap(s.state.CombosFiredThisRun),
		BossPhase2Reached:   s.state.BossPhase2Reached,
		MimicKilled:         s.state.MimicKilled,
		AltarUsedThisRun:    s.state.AltarUsedThisRun,
		RevivesThisRun:      s.state.RevivesThisRun,
		BossesKilledThisRun: copyStringBoolMap(s.state.BossesKilledThisRun),
	}
}

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
