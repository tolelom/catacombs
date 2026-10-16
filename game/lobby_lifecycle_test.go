package game

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var roomOwnerSeq atomic.Int64

// mustCreateRoom creates a room owned by a fresh fingerprint so tests that only
// care about rooms are not tripped by the one-waiting-room-per-owner rule.
func mustCreateRoom(t *testing.T, l *Lobby, name string) string {
	t.Helper()
	code, err := l.CreateRoom(fmt.Sprintf("fp-owner-%d", roomOwnerSeq.Add(1)), name)
	if err != nil {
		t.Fatalf("create room %q: %v", name, err)
	}
	return code
}

// seatInWaitingRoom mirrors what the UI does from lobby to class selection:
// join the room, ensure the session, add the player, register for reconnect.
func seatInWaitingRoom(t *testing.T, l *Lobby, code, name, fp string) *GameSession {
	t.Helper()
	if err := l.JoinRoom(code, name, fp); err != nil {
		t.Fatalf("join %s: %v", name, err)
	}
	cfg := l.Cfg()
	session := l.EnsureSession(code, func() *GameSession { return NewGameSession(cfg) })
	session.AddPlayer(newPlayerWithFP(name, fp))
	l.RegisterSession(fp, code)
	return session
}

func startPlaying(t *testing.T, l *Lobby, code string, session *GameSession) {
	t.Helper()
	l.StartRoom(code)
	session.StartGame()
	t.Cleanup(session.Stop)
}

func waitFor(t *testing.T, within time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func TestDisconnectFromLobbyMarksPlayerOffline(t *testing.T) {
	l := NewLobby(testConfig(t))
	l.PlayerOnline("fp-a", "A")

	l.Disconnect("fp-a")

	if n := len(l.ListOnline()); n != 0 {
		t.Fatalf("online players after disconnect: got %d, want 0", n)
	}
}

func TestDisconnectFromWaitingRoomRemovesAbandonedRoom(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := mustCreateRoom(t, l, "A의 방")
	seatInWaitingRoom(t, l, code, "A", "fp-a")

	l.Disconnect("fp-a")

	if l.GetRoom(code) != nil {
		t.Fatal("waiting room of the only player survived the disconnect")
	}
}

func TestDisconnectFromWaitingRoomKeepsRoomForOthers(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := mustCreateRoom(t, l, "shared")
	seatInWaitingRoom(t, l, code, "A", "fp-a")
	session := seatInWaitingRoom(t, l, code, "B", "fp-b")

	l.Disconnect("fp-a")

	snap, ok := l.RoomSnapshot(code)
	if !ok || len(snap.Players) != 1 || snap.Players[0].Fingerprint != "fp-b" {
		t.Fatalf("room should keep only B, got %+v (exists=%v)", snap.Players, ok)
	}
	if players := session.GetState().Players; len(players) != 1 || players[0].Fingerprint != "fp-b" {
		t.Fatalf("session should keep only B, got %+v", players)
	}
}

func TestDisconnectDuringGameKeepsReconnect(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := mustCreateRoom(t, l, "run")
	session := seatInWaitingRoom(t, l, code, "A", "fp-a")
	startPlaying(t, l, code, session)

	l.Disconnect("fp-a")

	if gotCode, gotSession := l.GetActiveSession("fp-a"); gotCode != code || gotSession != session {
		t.Fatalf("reconnect lost after disconnect mid-game: code=%q session=%p", gotCode, gotSession)
	}
}

func TestDisconnectAfterGameOverRemovesRoom(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := mustCreateRoom(t, l, "finished")
	session := seatInWaitingRoom(t, l, code, "A", "fp-a")
	startPlaying(t, l, code, session)
	session.mu.Lock()
	session.state.GameOver = true
	session.mu.Unlock()

	l.Disconnect("fp-a")

	if l.GetRoom(code) != nil {
		t.Fatal("finished room survived after its last player disconnected")
	}
}

func TestSessionAbandonedAfterDisconnectRemovesRoom(t *testing.T) {
	l := NewLobby(testConfig(t))
	code := mustCreateRoom(t, l, "abandoned")
	session := seatInWaitingRoom(t, l, code, "A", "fp-a")
	startPlaying(t, l, code, session)

	l.Disconnect("fp-a")
	setClock(session, time.Now().Add(2*time.Minute))
	session.signalCombat()

	if !waitFor(t, 3*time.Second, func() bool { return l.GetRoom(code) == nil }) {
		t.Fatal("room of a session abandoned after its last player disconnected was not reclaimed")
	}
}

func TestOwnerCanHoldOnlyOneWaitingRoom(t *testing.T) {
	l := NewLobby(testConfig(t))
	if _, err := l.CreateRoom("fp-a", "first"); err != nil {
		t.Fatalf("first room: %v", err)
	}

	if _, err := l.CreateRoom("fp-a", "second"); err == nil {
		t.Fatal("same owner created a second waiting room")
	}
	if n := len(l.ListRooms()); n != 1 {
		t.Fatalf("rooms after rejected create: got %d, want 1", n)
	}
}

func TestOwnerCanCreateAgainAfterLeaving(t *testing.T) {
	l := NewLobby(testConfig(t))
	code, err := l.CreateRoom("fp-a", "first")
	if err != nil {
		t.Fatalf("first room: %v", err)
	}
	seatInWaitingRoom(t, l, code, "A", "fp-a")
	l.Disconnect("fp-a")

	if _, err := l.CreateRoom("fp-a", "again"); err != nil {
		t.Fatalf("owner could not create a room after leaving the old one: %v", err)
	}
}

func TestRoomCountIsCapped(t *testing.T) {
	l := NewLobby(testConfig(t))
	for i := 0; i < maxRooms; i++ {
		if _, err := l.CreateRoom(fmt.Sprintf("fp-%d", i), "room"); err != nil {
			t.Fatalf("room %d of %d: %v", i+1, maxRooms, err)
		}
	}

	if _, err := l.CreateRoom("fp-extra", "room"); err == nil {
		t.Fatalf("created room beyond the cap of %d", maxRooms)
	}
}

func TestConcurrentStopDoesNotPanic(t *testing.T) {
	session := NewGameSession(testConfig(t))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.Stop()
		}()
	}
	wg.Wait()

	select {
	case <-session.done:
	default:
		t.Fatal("session not stopped")
	}
}

type countingRecorder struct {
	mu   sync.Mutex
	runs []RunResult
}

func (r *countingRecorder) RecordRun(result RunResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, result)
}

func (r *countingRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.runs)
}

func TestFinishedRunIsRecordedOnceWithoutAnyClient(t *testing.T) {
	l := NewLobby(testConfig(t))
	rec := &countingRecorder{}
	l.SetRecorder(rec)
	code := mustCreateRoom(t, l, "run")
	session := seatInWaitingRoom(t, l, code, "A", "fp-a")
	startPlaying(t, l, code, session)

	session.mu.Lock()
	session.state.GameOver = true
	session.mu.Unlock()
	session.signalCombat()

	if !waitFor(t, 3*time.Second, func() bool { return rec.count() == 1 }) {
		t.Fatalf("finished run recorded %d times, want 1", rec.count())
	}
	session.signalCombat()
	time.Sleep(100 * time.Millisecond)
	if n := rec.count(); n != 1 {
		t.Fatalf("finished run recorded %d times, want exactly 1", n)
	}
	if !session.WaitRecorded(time.Second) {
		t.Fatal("WaitRecorded did not report the recording")
	}
	if got := rec.runs[0].State.Players; len(got) != 1 || got[0].Fingerprint != "fp-a" {
		t.Fatalf("recorded members: %+v", got)
	}
}
