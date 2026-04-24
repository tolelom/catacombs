package game

import (
	"fmt"
	"testing"

	"github.com/tolelom/catacombs/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, _ := config.Load("")
	return cfg
}

func TestCreateRoom(t *testing.T) {
	lobby := NewLobby(testConfig(t))
	code := lobby.CreateRoom("Test Room")
	if len(code) != 4 {
		t.Errorf("Room code length: got %d, want 4", len(code))
	}
	rooms := lobby.ListRooms()
	if len(rooms) != 1 {
		t.Errorf("Room count: got %d, want 1", len(rooms))
	}
}

func TestJoinRoom(t *testing.T) {
	lobby := NewLobby(testConfig(t))
	code := lobby.CreateRoom("Test Room")
	err := lobby.JoinRoom(code, "player1", "fp-player1")
	if err != nil {
		t.Errorf("Join failed: %v", err)
	}
	room := lobby.GetRoom(code)
	if len(room.Players) != 1 {
		t.Errorf("Player count: got %d, want 1", len(room.Players))
	}
}

func TestRoomStatusTransition(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := l.CreateRoom("Test")
	_ = l.JoinRoom(code, "Alice", "fp-alice")
	r := l.GetRoom(code)
	if r.Status != RoomWaiting {
		t.Errorf("new room should be Waiting, got %d", r.Status)
	}
	l.StartRoom(code)
	r = l.GetRoom(code)
	if r.Status != RoomPlaying {
		t.Errorf("started room should be Playing, got %d", r.Status)
	}
	err := l.JoinRoom(code, "Bob", "fp-bob")
	if err == nil {
		t.Error("should not be able to join a Playing room")
	}
}

func TestJoinRoomFull(t *testing.T) {
	lobby := NewLobby(testConfig(t))
	code := lobby.CreateRoom("Test Room")
	for i := 0; i < 4; i++ {
		if err := lobby.JoinRoom(code, fmt.Sprintf("player%d", i), fmt.Sprintf("fp-player%d", i)); err != nil {
			t.Fatalf("unexpected join error: %v", err)
		}
	}
	err := lobby.JoinRoom(code, "player5", "fp-player5")
	if err == nil {
		t.Error("Should reject 5th player")
	}
}

func TestJoinRoomDuplicateFingerprint(t *testing.T) {
	lobby := NewLobby(testConfig(t))
	code := lobby.CreateRoom("Test Room")
	if err := lobby.JoinRoom(code, "Alice", "fp-alice"); err != nil {
		t.Fatalf("first join should succeed: %v", err)
	}
	if err := lobby.JoinRoom(code, "Alice", "fp-alice"); err == nil {
		t.Error("duplicate fingerprint should be rejected")
	}
	room := lobby.GetRoom(code)
	if len(room.Players) != 1 {
		t.Errorf("expected 1 player, got %d", len(room.Players))
	}
}

func TestSetPlayerClass(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := l.CreateRoom("Test")
	_ = l.JoinRoom(code, "Alice", "fp-alice")
	l.SetPlayerClass(code, "fp-alice", "Warrior")
	room := l.GetRoom(code)
	if room.Players[0].Class != "Warrior" {
		t.Errorf("expected class Warrior, got %s", room.Players[0].Class)
	}
}

func TestAllReady(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := l.CreateRoom("Test")
	_ = l.JoinRoom(code, "Alice", "fp-alice")
	_ = l.JoinRoom(code, "Bob", "fp-bob")

	if l.AllReady(code) {
		t.Error("no one ready yet, should return false")
	}

	l.SetPlayerReady(code, "fp-alice", true)
	if l.AllReady(code) {
		t.Error("only Alice ready, should return false")
	}

	l.SetPlayerReady(code, "fp-bob", true)
	if !l.AllReady(code) {
		t.Error("both ready, should return true")
	}
}

func TestAllReadyEmptyRoom(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := l.CreateRoom("Test")
	if l.AllReady(code) {
		t.Error("empty room should not be all ready")
	}
}
