package game

import "testing"

func TestCreateRoom(t *testing.T) {
	lobby := NewLobby()
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
	lobby := NewLobby()
	code := lobby.CreateRoom("Test Room")
	err := lobby.JoinRoom(code, "player1")
	if err != nil {
		t.Errorf("Join failed: %v", err)
	}
	room := lobby.GetRoom(code)
	if len(room.Players) != 1 {
		t.Errorf("Player count: got %d, want 1", len(room.Players))
	}
}

func TestJoinRoomFull(t *testing.T) {
	lobby := NewLobby()
	code := lobby.CreateRoom("Test Room")
	for i := 0; i < 4; i++ {
		lobby.JoinRoom(code, "player")
	}
	err := lobby.JoinRoom(code, "player5")
	if err == nil {
		t.Error("Should reject 5th player")
	}
}
