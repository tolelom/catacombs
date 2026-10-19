package game

import (
	"maps"
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

	return GameState{
		Floor:               copyFloor(s.state.Floor),
		Players:             copyPlayers(s.state.Players),
		Monsters:            copyMonsters(s.state.Monsters),
		Phase:               s.state.Phase,
		FloorNum:            s.state.FloorNum,
		TurnNum:             s.state.TurnNum,
		CombatTurn:          s.state.CombatTurn,
		SoloMode:            s.state.SoloMode,
		GameOver:            s.state.GameOver,
		Victory:             s.state.Victory,
		ShopItems:           copySliceOrNil(s.state.ShopItems),
		ShopSold:            copySliceOrNil(s.state.ShopSold),
		CombatLog:           copySliceOrNil(s.state.CombatLog),
		TurnDeadline:        s.state.TurnDeadline,
		SubmittedActions:    copyMapOrNil(s.state.SubmittedActions),
		PendingLogs:         copySliceOrNil(s.state.PendingLogs),
		TurnResolving:       s.state.TurnResolving,
		BossKilled:          s.state.BossKilled,
		FleeSucceeded:       s.state.FleeSucceeded,
		AnyPlayerDied:       s.state.AnyPlayerDied,
		StartingPartySize:   s.state.StartingPartySize,
		LastEventName:       s.state.LastEventName,
		MoveVotes:           copyMapOrNil(s.state.MoveVotes),
		PendingEvent:        copyPendingEvent(s.state.PendingEvent),
		EventVotes:          maps.Clone(s.state.EventVotes),
		ShopLeavers:         copyMapOrNil(s.shopLeavers),
		NextRoomPreview:     s.nextRoomPreview,
		CoopAvailable:       s.computeCoopAvailable(),
		SoloChain:           s.soloChainView(),
		CombosFiredThisRun:  copyMapOrNil(s.state.CombosFiredThisRun),
		BossPhase2Reached:   s.state.BossPhase2Reached,
		MimicKilled:         s.state.MimicKilled,
		AltarUsedThisRun:    s.state.AltarUsedThisRun,
		RevivesThisRun:      s.state.RevivesThisRun,
		BossesKilledThisRun: copyMapOrNil(s.state.BossesKilledThisRun),
	}
}

func copyPlayers(src []*entity.Player) []*entity.Player {
	if len(src) == 0 {
		return nil
	}
	players := make([]*entity.Player, len(src))
	for i, p := range src {
		players[i] = copyPlayer(p)
	}
	return players
}

func copyPlayer(p *entity.Player) *entity.Player {
	cp := *p
	cp.Inventory = copySliceOrNil(p.Inventory)
	cp.Relics = copySliceOrNil(p.Relics)
	cp.Effects = copySliceOrNil(p.Effects)
	if p.Skills != nil {
		skillsCopy := *p.Skills
		cp.Skills = &skillsCopy
	}
	return &cp
}

func copyMonsters(src []*entity.Monster) []*entity.Monster {
	if len(src) == 0 {
		return nil
	}
	monsters := make([]*entity.Monster, len(src))
	for i, m := range src {
		cm := *m
		monsters[i] = &cm
	}
	return monsters
}

func copyFloor(src *dungeon.Floor) *dungeon.Floor {
	if src == nil {
		return nil
	}
	fc := *src
	fc.Rooms = make([]*dungeon.Room, len(src.Rooms))
	for i, r := range src.Rooms {
		rc := *r
		rc.Neighbors = copySliceOrNil(r.Neighbors)
		fc.Rooms[i] = &rc
	}
	return &fc
}

func copyPendingEvent(src *PendingEventView) *PendingEventView {
	if src == nil {
		return nil
	}
	choices := make([]EventChoiceView, len(src.Choices))
	for i, c := range src.Choices {
		tags := make([]ChoiceTag, len(c.Tags))
		copy(tags, c.Tags)
		choices[i] = EventChoiceView{Label: c.Label, Tags: tags}
	}
	return &PendingEventView{
		Name:        src.Name,
		Description: src.Description,
		PerPlayer:   src.PerPlayer,
		Choices:     choices,
	}
}

func copySliceOrNil[T any](src []T) []T {
	if len(src) == 0 {
		return nil
	}
	out := make([]T, len(src))
	copy(out, src)
	return out
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

func copyMapOrNil[K comparable, V any](src map[K]V) map[K]V {
	if len(src) == 0 {
		return nil
	}
	return maps.Clone(src)
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
