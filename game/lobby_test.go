package game

import (
	"fmt"
	"testing"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/entity"
)

func newPlayerWithFP(name, fp string) *entity.Player {
	p := entity.NewPlayer(name, entity.ClassWarrior)
	p.Fingerprint = fp
	return p
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, _ := config.Load("")
	return cfg
}

func TestCreateRoom(t *testing.T) {
	lobby := NewLobby(testConfig(t))
	code := mustCreateRoom(t, lobby, "Test Room")
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
	code := mustCreateRoom(t, lobby, "Test Room")
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
	code := mustCreateRoom(t, l, "Test")
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
	code := mustCreateRoom(t, lobby, "Test Room")
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
	code := mustCreateRoom(t, lobby, "Test Room")
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
	code := mustCreateRoom(t, l, "Test")
	_ = l.JoinRoom(code, "Alice", "fp-alice")
	l.SetPlayerClass(code, "fp-alice", "Warrior")
	room := l.GetRoom(code)
	if room.Players[0].Class != "Warrior" {
		t.Errorf("expected class Warrior, got %s", room.Players[0].Class)
	}
}

func TestAllReady(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := mustCreateRoom(t, l, "Test")
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
	code := mustCreateRoom(t, l, "Test")
	if l.AllReady(code) {
		t.Error("empty room should not be all ready")
	}
}

// TestLeaveSessionRemovesGhostFromWaitingRoom regresses the ghost-player
// bug: a player who picked a class and then Esc'd out of the waiting room
// would remain in session.state.Players, so when the rest of the party
// readied up StartGame would carry the absent player into the run.
func TestLeaveSessionRemovesGhostFromWaitingRoom(t *testing.T) {
	cfg := testConfig(t)
	l := NewLobby(cfg)
	code := mustCreateRoom(t, l, "Pre-game")

	// Two players join the lobby, pick classes, get added to the session.
	if err := l.JoinRoom(code, "A", "fp-a"); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if err := l.JoinRoom(code, "B", "fp-b"); err != nil {
		t.Fatalf("join B: %v", err)
	}
	session := l.EnsureSession(code, func() *GameSession { return NewGameSession(cfg) })
	session.AddPlayer(newPlayerWithFP("A", "fp-a"))
	session.AddPlayer(newPlayerWithFP("B", "fp-b"))
	l.RegisterSession("fp-a", code)
	l.RegisterSession("fp-b", code)

	// A backs out of the waiting room.
	l.LeaveRoom(code, "fp-a")
	l.LeaveSession(code, "fp-a")

	players := session.GetState().Players
	if len(players) != 1 || players[0].Fingerprint != "fp-b" {
		t.Errorf("session should hold only B, got %+v", players)
	}
	if _, sess := l.GetActiveSession("fp-a"); sess != nil {
		t.Error("activeSessions should no longer contain fp-a")
	}
}

// TestStopAllSessionsHaltsCombatLoops verifies graceful shutdown closes the
// done channel of every active session so combatLoop goroutines can exit.
func TestStopAllSessionsHaltsCombatLoops(t *testing.T) {
	cfg := testConfig(t)
	l := NewLobby(cfg)
	codeA := mustCreateRoom(t, l, "A")
	codeB := mustCreateRoom(t, l, "B")
	sessA := l.EnsureSession(codeA, func() *GameSession { return NewGameSession(cfg) })
	sessB := l.EnsureSession(codeB, func() *GameSession { return NewGameSession(cfg) })
	if sessA == nil || sessB == nil {
		t.Fatal("EnsureSession returned nil")
	}

	l.StopAllSessions()
	for name, s := range map[string]*GameSession{"A": sessA, "B": sessB} {
		select {
		case <-s.done:
		default:
			t.Errorf("session %s should have been stopped", name)
		}
	}
}

// TestLeaveSessionKeepsRoomForRemainingPlayers verifies that one player's
// quit does not terminate the shared session for the rest of the party.
// Regression for: leaveGame previously called Stop+RemoveRoom unilaterally.
func TestLeaveSessionKeepsRoomForRemainingPlayers(t *testing.T) {
	cfg := testConfig(t)
	l := NewLobby(cfg)
	code := mustCreateRoom(t, l, "Test")

	session := l.EnsureSession(code, func() *GameSession { return NewGameSession(cfg) })
	if session == nil {
		t.Fatal("EnsureSession returned nil")
	}
	a := newPlayerWithFP("A", "fp-a")
	b := newPlayerWithFP("B", "fp-b")
	session.AddPlayer(a)
	session.AddPlayer(b)

	remaining := l.LeaveSession(code, "fp-a")
	if remaining != 1 {
		t.Errorf("expected 1 remaining after A leaves, got %d", remaining)
	}
	if l.GetRoom(code) == nil {
		t.Error("room should still exist while one player remains")
	}
	select {
	case <-session.done:
		t.Error("session was stopped while a player remained")
	default:
	}

	remaining = l.LeaveSession(code, "fp-b")
	if remaining != 0 {
		t.Errorf("expected 0 remaining after last player leaves, got %d", remaining)
	}
	if l.GetRoom(code) != nil {
		t.Error("room should be removed once last player leaves")
	}
	select {
	case <-session.done:
	default:
		t.Error("session should be stopped after last player leaves")
	}
}

// TestLeaveSessionPreservesUnrelatedActiveSession guards against a stale
// LeaveSession (e.g., a delayed UI cmd from the prior room) wiping the
// reconnect mapping for a room the player has since moved to.
func TestLeaveSessionPreservesUnrelatedActiveSession(t *testing.T) {
	l := NewLobby(testConfig(t))
	codeA := mustCreateRoom(t, l, "RoomA")
	codeB := mustCreateRoom(t, l, "RoomB")
	l.RegisterSession("fp-alice", codeA)

	// Stale leave for room B must not touch the mapping for room A.
	l.LeaveSession(codeB, "fp-alice")

	l.mu.RLock()
	gotCode := l.activeSessions["fp-alice"]
	l.mu.RUnlock()
	if gotCode != codeA {
		t.Errorf("expected reconnect mapping to %q, got %q", codeA, gotCode)
	}
}
