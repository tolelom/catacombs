package game

import (
	"reflect"
	"testing"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

// populatedSnapshotSession fills every GameState field GetState copies with
// non-empty values, so a snapshot that drops or aliases one shows up.
func populatedSnapshotSession(t *testing.T) *GameSession {
	t.Helper()
	s := NewGameSession(testCfg(t))
	p := entity.NewPlayer("Snap", entity.ClassHealer)
	p.Fingerprint = "fp-snap"
	p.Inventory = []entity.Item{{Name: "Sword", Type: entity.ItemWeapon, Bonus: 5}}
	p.Relics = []entity.Relic{{Name: "Stone", Effect: entity.RelicATKBoost, Value: 3}}
	p.Effects = []entity.ActiveEffect{{Type: entity.StatusPoison, Duration: 2, Value: 4}}
	s.AddPlayer(p)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Phase = PhaseEvent
	s.state.Floor = &dungeon.Floor{
		Number: 3,
		Rooms: []*dungeon.Room{
			{Type: dungeon.RoomEmpty, Neighbors: []int{1}},
			{Type: dungeon.RoomCombat, Neighbors: []int{0}},
		},
	}
	s.state.Monsters = []*entity.Monster{{Name: "Slime", HP: 10, MaxHP: 10}}
	s.state.CombatLog = []string{"log"}
	s.state.PendingLogs = []string{"pending"}
	s.state.SubmittedActions = map[string]string{"fp-snap": "attack"}
	s.state.MoveVotes = map[string]int{"fp-snap": 1}
	s.state.EventVotes = map[string]int{"fp-snap": 0}
	s.state.PendingEvent = &PendingEventView{
		Name:    "altar",
		Choices: []EventChoiceView{{Label: "pray", Tags: []ChoiceTag{TagHeal}}},
	}
	s.state.ShopItems = []entity.Item{{Name: "Potion", Type: entity.ItemConsumable, Bonus: 30}}
	s.state.ShopSold = []bool{false}
	s.state.CombosFiredThisRun = map[string]bool{"combo": true}
	s.state.BossesKilledThisRun = map[string]bool{"boss": true}
	s.shopLeavers = map[string]bool{"fp-snap": true}
	return s
}

// TestGetStateCopiesEveryField pins that the UI snapshot shows the session's
// current values. If it breaks, a screen renders stale or missing state.
func TestGetStateCopiesEveryField(t *testing.T) {
	s := populatedSnapshotSession(t)
	snapshot := s.GetState()

	s.mu.Lock()
	want := s.state
	want.ShopLeavers = s.shopLeavers
	s.mu.Unlock()
	want.NextRoomPreview = snapshot.NextRoomPreview
	want.CoopAvailable = snapshot.CoopAvailable
	want.SoloChain = snapshot.SoloChain

	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("snapshot differs from session state\n got: %+v\nwant: %+v", snapshot, want)
	}
}

// TestGetStateSnapshotIsIndependent pins that mutating a snapshot never
// reaches the live session. If it breaks, rendering one player's screen can
// corrupt the shared game (and races the turn loop).
func TestGetStateSnapshotIsIndependent(t *testing.T) {
	s := populatedSnapshotSession(t)
	before := s.GetState()
	snapshot := s.GetState()

	player := snapshot.Players[0]
	player.HP = -1
	player.Inventory[0].Bonus = 99
	player.Relics[0].Value = 99
	player.Effects[0].Duration = 99
	player.Skills.Points = 99
	snapshot.Monsters[0].HP = 99
	snapshot.Floor.Rooms[0].Visited = true
	snapshot.Floor.Rooms[0].Neighbors[0] = 99
	snapshot.CombatLog[0] = "changed"
	snapshot.PendingLogs[0] = "changed"
	snapshot.SubmittedActions["fp-snap"] = "changed"
	snapshot.MoveVotes["fp-snap"] = 99
	snapshot.EventVotes["fp-snap"] = 99
	snapshot.PendingEvent.Choices[0].Tags[0] = TagSkip
	snapshot.PendingEvent.Choices[0].Label = "changed"
	snapshot.ShopItems[0].Bonus = 99
	snapshot.ShopSold[0] = true
	snapshot.ShopLeavers["other"] = true
	snapshot.CombosFiredThisRun["other"] = true
	snapshot.BossesKilledThisRun["other"] = true

	if after := s.GetState(); !reflect.DeepEqual(after, before) {
		t.Fatalf("mutating a snapshot changed the session\n got: %+v\nwant: %+v", after, before)
	}
}
