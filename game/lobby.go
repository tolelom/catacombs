package game

import (
	"fmt"
	"math/rand"
	"sync"
)

type RoomStatus int

const (
	RoomWaiting RoomStatus = iota
	RoomPlaying
)

type LobbyRoom struct {
	Code    string
	Name    string
	Players []string
	Status  RoomStatus
	Session *GameSession
}

type Lobby struct {
	mu    sync.RWMutex
	rooms map[string]*LobbyRoom
}

func NewLobby() *Lobby {
	return &Lobby{rooms: make(map[string]*LobbyRoom)}
}

func (l *Lobby) CreateRoom(name string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	code := generateCode()
	for l.rooms[code] != nil {
		code = generateCode()
	}
	l.rooms[code] = &LobbyRoom{
		Code:   code,
		Name:   name,
		Status: RoomWaiting,
	}
	return code
}

func (l *Lobby) JoinRoom(code, playerName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	room, ok := l.rooms[code]
	if !ok {
		return fmt.Errorf("room %s not found", code)
	}
	if len(room.Players) >= 4 {
		return fmt.Errorf("room %s is full", code)
	}
	if room.Status != RoomWaiting {
		return fmt.Errorf("room %s already in progress", code)
	}
	room.Players = append(room.Players, playerName)
	return nil
}

func (l *Lobby) GetRoom(code string) *LobbyRoom {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rooms[code]
}

func (l *Lobby) ListRooms() []*LobbyRoom {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*LobbyRoom, 0, len(l.rooms))
	for _, r := range l.rooms {
		result = append(result, r)
	}
	return result
}

func (l *Lobby) RemoveRoom(code string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.rooms, code)
}

func generateCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 4)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
