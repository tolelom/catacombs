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
	Floor             *dungeon.Floor
	Players           []*entity.Player
	Monsters          []*entity.Monster
	Phase             GamePhase
	FloorNum          int
	TurnNum           int
	CombatTurn        int // reset per combat encounter
	SoloMode          bool
	GameOver          bool
	Victory           bool
	ShopItems         []entity.Item
	CombatLog         []string // recent combat messages
	TurnDeadline      time.Time
	SubmittedActions  map[string]string // fingerprint -> action description
	PendingLogs       []string          // logs waiting to be revealed one by one
	TurnResolving     bool              // true while logs are being replayed
	BossKilled        bool
	FleeSucceeded     bool
	AnyPlayerDied     bool              // true once any player has died this run; powers the no_death achievement
	StartingPartySize int               // number of players at StartGame; powers the full_party achievement
	LastEventName     string            // name of the most recent random event (for codex)
	MoveVotes         map[string]int    // fingerprint -> voted room index (exploration)
	PendingEvent      *PendingEventView // active event during PhaseEvent
	EventVotes        map[string]int    // fingerprint → choice index
	ShopSold          []bool            // parallel to ShopItems; true once a weapon/armor slot is sold (consumables and charms restock freely)
	ShopLeavers       map[string]bool   // fingerprint -> wants to leave the shop (UI display only)
	NextRoomPreview   *RoomPreview      // populated when Foresight has been used
	CoopAvailable     map[string]bool   // fingerprint → can use coop now (HUD display)
	SoloChain         *SoloChainView    // solo-mode combat chain state (HUD display)
	// Run-scoped event flags (drives polish achievements at GameOver)
	CombosFiredThisRun  map[string]bool // combo name → fired at least once this run
	BossPhase2Reached   bool            // any boss reached Phase 2
	MimicKilled         bool            // any mimic monster defeated
	AltarUsedThisRun    int             // cursed altar uses this run
	RevivesThisRun      int             // Divine Grace revives this run
	BossesKilledThisRun map[string]bool // boss name → killed this run
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
