package game

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/tolelom/catacombs/config"
)

type RoomStatus int

const (
	RoomWaiting RoomStatus = iota
	RoomPlaying
)

type LobbyPlayer struct {
	Name        string
	Class       string // empty until class selected
	Fingerprint string
	Ready       bool
}

type LobbyRoom struct {
	Code    string
	Name    string
	Players []LobbyPlayer
	Status  RoomStatus
	Session *GameSession
}

type OnlinePlayer struct {
	Name        string
	Fingerprint string
	InRoom      string // room code, empty if in lobby
}

type Lobby struct {
	mu             sync.RWMutex
	cfg            *config.Config
	rooms          map[string]*LobbyRoom
	online         map[string]*OnlinePlayer // fingerprint -> player
	activeSessions map[string]string        // fingerprint -> room code (for reconnect)
}

func NewLobby(cfg *config.Config) *Lobby {
	return &Lobby{
		cfg:            cfg,
		rooms:          make(map[string]*LobbyRoom),
		online:         make(map[string]*OnlinePlayer),
		activeSessions: make(map[string]string),
	}
}

func (l *Lobby) Cfg() *config.Config {
	return l.cfg
}

func (l *Lobby) RegisterSession(fingerprint, roomCode string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.activeSessions[fingerprint] = roomCode
}

func (l *Lobby) UnregisterSession(fingerprint string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.activeSessions, fingerprint)
}

func (l *Lobby) GetActiveSession(fingerprint string) (string, *GameSession) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	code, ok := l.activeSessions[fingerprint]
	if !ok {
		return "", nil
	}
	room, ok := l.rooms[code]
	if !ok || room.Session == nil {
		return "", nil
	}
	// Check if this player is still in the session
	for _, p := range room.Session.GetState().Players {
		if p.Fingerprint == fingerprint {
			return code, room.Session
		}
	}
	return "", nil
}

func (l *Lobby) PlayerOnline(fingerprint, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.online[fingerprint] = &OnlinePlayer{Name: name, Fingerprint: fingerprint}
}

func (l *Lobby) PlayerOffline(fingerprint string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.online, fingerprint)
}

func (l *Lobby) ListOnline() []*OnlinePlayer {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*OnlinePlayer, 0, len(l.online))
	for _, p := range l.online {
		result = append(result, p)
	}
	return result
}

func (l *Lobby) InvitePlayer(roomCode, fingerprint string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.online[fingerprint]
	if !ok {
		return fmt.Errorf("player not online")
	}
	if p.InRoom != "" {
		return fmt.Errorf("player already in a room")
	}
	// Store the invite as a pending field
	p.InRoom = "invited:" + roomCode
	return nil
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

func (l *Lobby) JoinRoom(code, playerName, fingerprint string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	room, ok := l.rooms[code]
	if !ok {
		return fmt.Errorf("room %s not found", code)
	}
	if len(room.Players) >= l.cfg.Game.MaxPlayers {
		return fmt.Errorf("room %s is full", code)
	}
	if room.Status != RoomWaiting {
		return fmt.Errorf("room %s already in progress", code)
	}
	room.Players = append(room.Players, LobbyPlayer{Name: playerName, Fingerprint: fingerprint})
	return nil
}

func (l *Lobby) SetPlayerClass(code, fingerprint, class string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room, ok := l.rooms[code]; ok {
		for i := range room.Players {
			if room.Players[i].Fingerprint == fingerprint {
				room.Players[i].Class = class
			}
		}
	}
}

func (l *Lobby) SetPlayerReady(code, fingerprint string, ready bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room, ok := l.rooms[code]; ok {
		for i := range room.Players {
			if room.Players[i].Fingerprint == fingerprint {
				room.Players[i].Ready = ready
			}
		}
	}
}

func (l *Lobby) AllReady(code string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	room, ok := l.rooms[code]
	if !ok || len(room.Players) == 0 {
		return false
	}
	for _, p := range room.Players {
		if !p.Ready {
			return false
		}
	}
	return true
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

func (l *Lobby) StartRoom(code string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room, ok := l.rooms[code]; ok {
		room.Status = RoomPlaying
	}
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
