package game

import (
	"errors"
	"fmt"
	"log/slog"
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
	Class       string
	Fingerprint string
	Ready       bool
}

type LobbyRoom struct {
	Code    string
	Name    string
	Owner   string
	Private bool
	Players []LobbyPlayer
	Status  RoomStatus
	Session *GameSession
}

const maxRooms = 100

var (
	ErrOwnerHasWaitingRoom = errors.New("이미 대기 중인 방이 있습니다")
	ErrTooManyRooms        = errors.New("방이 너무 많습니다. 잠시 후 다시 시도해 주세요")
)

type OnlinePlayer struct {
	Name        string
	Fingerprint string
	InRoom      string
}

type Lobby struct {
	mu             sync.RWMutex
	cfg            *config.Config
	rooms          map[string]*LobbyRoom
	online         map[string]*OnlinePlayer
	activeSessions map[string]string
	recorder       RunRecorder
}

func NewLobby(cfg *config.Config) *Lobby {
	return &Lobby{
		cfg:            cfg,
		rooms:          make(map[string]*LobbyRoom),
		online:         make(map[string]*OnlinePlayer),
		activeSessions: make(map[string]string),
	}
}

func (l *Lobby) SetRecorder(r RunRecorder) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recorder = r
}

func (l *Lobby) Cfg() *config.Config {
	return l.cfg
}

func (l *Lobby) RegisterSession(fingerprint, roomCode string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.activeSessions[fingerprint] = roomCode
}

func (l *Lobby) GetActiveSession(fingerprint string) (string, *GameSession) {
	l.mu.RLock()
	code, ok := l.activeSessions[fingerprint]
	if !ok {
		l.mu.RUnlock()
		return "", nil
	}
	room, ok := l.rooms[code]
	if !ok || room.Session == nil {
		l.mu.RUnlock()
		return "", nil
	}
	session := room.Session
	l.mu.RUnlock()
	for _, p := range session.GetState().Players {
		if p.Fingerprint == fingerprint {
			return code, session
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

func (l *Lobby) Disconnect(fingerprint string) {
	l.PlayerOffline(fingerprint)
	code, ok := l.roomOf(fingerprint)
	if !ok {
		return
	}
	if l.releasesOnDisconnect(code) {
		l.LeaveRoom(code, fingerprint)
		l.LeaveSession(code, fingerprint)
		return
	}
	l.markDisconnectedInSession(code, fingerprint)
}

func (l *Lobby) markDisconnectedInSession(code, fingerprint string) {
	l.mu.RLock()
	room, ok := l.rooms[code]
	var session *GameSession
	if ok {
		session = room.Session
	}
	l.mu.RUnlock()
	if session != nil {
		session.MarkDisconnected(fingerprint)
	}
}

func (l *Lobby) roomOf(fingerprint string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if code, ok := l.activeSessions[fingerprint]; ok {
		if _, exists := l.rooms[code]; exists {
			return code, true
		}
	}
	for code, r := range l.rooms {
		for _, p := range r.Players {
			if p.Fingerprint == fingerprint {
				return code, true
			}
		}
	}
	return "", false
}

func (l *Lobby) releasesOnDisconnect(code string) bool {
	l.mu.RLock()
	room, ok := l.rooms[code]
	if !ok {
		l.mu.RUnlock()
		return false
	}
	status, session := room.Status, room.Session
	l.mu.RUnlock()
	if status == RoomWaiting || session == nil {
		return true
	}
	return session.IsOver()
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

func (l *Lobby) CreateRoom(owner, name string) (string, error) {
	return l.createRoom(owner, name, false)
}

func (l *Lobby) CreatePrivateRoom(owner, name string) (string, error) {
	return l.createRoom(owner, name, true)
}

func (l *Lobby) createRoom(owner, name string, private bool) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.checkRoomQuotaLocked(owner); err != nil {
		return "", err
	}
	code := l.unusedCodeLocked()
	l.rooms[code] = &LobbyRoom{
		Code:    code,
		Name:    name,
		Owner:   owner,
		Private: private,
		Status:  RoomWaiting,
	}
	slog.Info("room created", "code", code, "name", name)
	return code, nil
}

func (l *Lobby) checkRoomQuotaLocked(owner string) error {
	if len(l.rooms) >= maxRooms {
		return ErrTooManyRooms
	}
	for _, r := range l.rooms {
		if r.Owner == owner && r.Status == RoomWaiting {
			return ErrOwnerHasWaitingRoom
		}
	}
	return nil
}

func (l *Lobby) unusedCodeLocked() string {
	code := generateCode()
	for l.rooms[code] != nil {
		code = generateCode()
	}
	return code
}

func (l *Lobby) JoinRoom(code, playerName, fingerprint string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	room, ok := l.rooms[code]
	if !ok {
		return fmt.Errorf("방 %s을(를) 찾을 수 없습니다", code)
	}
	if room.Private && fingerprint != room.Owner {
		return fmt.Errorf("방 %s은(는) 비공개 방입니다", code)
	}
	if room.Status != RoomWaiting {
		return fmt.Errorf("방 %s이(가) 이미 진행 중입니다", code)
	}
	for _, p := range room.Players {
		if p.Fingerprint == fingerprint {
			return fmt.Errorf("이미 방 %s에 입장해 있습니다", code)
		}
	}
	if len(room.Players) >= l.cfg.Game.MaxPlayers {
		return fmt.Errorf("방 %s이(가) 가득 찼습니다", code)
	}
	room.Players = append(room.Players, LobbyPlayer{Name: playerName, Fingerprint: fingerprint})
	slog.Info("player joined", "room", code, "player", playerName)
	return nil
}

func (l *Lobby) LeaveRoom(code, fingerprint string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	room, ok := l.rooms[code]
	if !ok {
		return
	}
	for i, p := range room.Players {
		if p.Fingerprint == fingerprint {
			room.Players = append(room.Players[:i], room.Players[i+1:]...)
			break
		}
	}
	if len(room.Players) == 0 && room.Status == RoomWaiting {
		delete(l.rooms, code)
	}
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

type RoomSnapshot struct {
	Code    string
	Name    string
	Players []LobbyPlayer
	Status  RoomStatus
	Session *GameSession
}

func (l *Lobby) RoomSnapshot(code string) (RoomSnapshot, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	room, ok := l.rooms[code]
	if !ok {
		return RoomSnapshot{}, false
	}
	return RoomSnapshot{
		Code:    room.Code,
		Name:    room.Name,
		Players: append([]LobbyPlayer(nil), room.Players...),
		Status:  room.Status,
		Session: room.Session,
	}, true
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

func (l *Lobby) ListRoomSnapshots() []RoomSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]RoomSnapshot, 0, len(l.rooms))
	for _, r := range l.rooms {
		if r.Private {
			continue
		}
		result = append(result, RoomSnapshot{
			Code:    r.Code,
			Name:    r.Name,
			Players: append([]LobbyPlayer(nil), r.Players...),
			Status:  r.Status,
			Session: r.Session,
		})
	}
	return result
}

func (l *Lobby) LeaveSession(code, fingerprint string) int {
	l.mu.Lock()
	room, ok := l.rooms[code]
	if l.activeSessions[fingerprint] == code {
		delete(l.activeSessions, fingerprint)
	}
	if !ok || room.Session == nil {
		l.mu.Unlock()
		return 0
	}
	session := room.Session
	l.mu.Unlock()

	remaining := session.RemovePlayer(fingerprint)
	if remaining > 0 {
		return remaining
	}

	l.removeRoomIfSession(code, session)
	return 0
}

func (l *Lobby) removeRoomIfSession(code string, session *GameSession) {
	l.mu.Lock()
	cur, ok := l.rooms[code]
	removed := ok && cur.Session == session
	if removed {
		delete(l.rooms, code)
	}
	l.mu.Unlock()
	if removed {
		session.Stop()
	}
}

func (l *Lobby) EnsureSession(code string, init func() *GameSession) *GameSession {
	l.mu.Lock()
	defer l.mu.Unlock()
	room, ok := l.rooms[code]
	if !ok {
		return nil
	}
	if room.Session == nil {
		session := init()
		session.setOnAbandoned(func() { l.removeRoomIfSession(code, session) })
		session.setRecorder(l.recorder)
		room.Session = session
	}
	return room.Session
}

func (l *Lobby) StartRoom(code string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room, ok := l.rooms[code]; ok {
		room.Status = RoomPlaying
		slog.Info("game started", "room", code, "players", len(room.Players))
	}
}

func (l *Lobby) StopAllSessions() {
	l.mu.Lock()
	sessions := make([]*GameSession, 0, len(l.rooms))
	for _, r := range l.rooms {
		if r.Session != nil {
			sessions = append(sessions, r.Session)
		}
	}
	l.mu.Unlock()
	for _, s := range sessions {
		s.Stop()
	}
}

func generateCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 4)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
