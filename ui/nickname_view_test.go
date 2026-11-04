package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/throttle"
)

func nicknameTestContext(t *testing.T, db *store.DB, limiter *throttle.Limiter) *Context {
	t.Helper()
	cfg, _ := config.Load("")
	return &Context{Width: 80, Height: 24, Lobby: game.NewLobby(cfg), Store: db, LoginLimiter: limiter}
}

func typeText(t *testing.T, s Screen, ctx *Context, text string) Screen {
	t.Helper()
	for _, r := range text {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}, ctx)
	}
	return s
}

func pressEnter(s Screen, ctx *Context) Screen {
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter}, ctx)
	return s
}

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/ui.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Two web sessions both see the nickname as free and reach the confirm step.
// Only the first may claim it; the second must not log in as that player.
func TestPasswordCreateDoesNotTakeOverClaimedNickname(t *testing.T) {
	db := openTestDB(t)

	first, second := NewNicknameScreen(), NewNicknameScreen()
	ctxA, ctxB := nicknameTestContext(t, db, nil), nicknameTestContext(t, db, nil)
	var a, b Screen = first, second
	for _, step := range []struct {
		s   *Screen
		ctx *Context
		pw  string
	}{{&a, ctxA, "owner-pw"}, {&b, ctxB, "attack-pw"}} {
		*step.s = pressEnter(typeText(t, *step.s, step.ctx, "alice"), step.ctx)
		*step.s = pressEnter(typeText(t, *step.s, step.ctx, step.pw), step.ctx)
		*step.s = typeText(t, *step.s, step.ctx, step.pw)
	}

	a = pressEnter(a, ctxA)
	if ctxA.PlayerName != "alice" {
		t.Fatalf("first session PlayerName = %q, want alice", ctxA.PlayerName)
	}
	b = pressEnter(b, ctxB)
	if ctxB.PlayerName != "" || ctxB.Fingerprint != "" {
		t.Fatalf("second session logged in as %q (%q); want refused", ctxB.PlayerName, ctxB.Fingerprint)
	}
	if ns, ok := b.(*NicknameScreen); !ok || ns.phase != phaseNickname || ns.error == "" {
		t.Fatalf("second session should be back at nickname input with an error, got %#v", b)
	}
	if ok, _ := db.CheckPassword("alice", "owner-pw"); !ok {
		t.Fatal("owner password must be unchanged")
	}
}

func TestPasswordLoginIsThrottledAfterRepeatedFailures(t *testing.T) {
	db := openTestDB(t)
	if err := db.CreatePassword("alice", "right-pw"); err != nil {
		t.Fatal(err)
	}
	limiter := throttle.New(time.Minute, 3, 5*time.Minute)
	ctx := nicknameTestContext(t, db, limiter)

	var s Screen = NewNicknameScreen()
	s = pressEnter(typeText(t, s, ctx, "alice"), ctx)
	for i := 0; i < 3; i++ {
		s = pressEnter(typeText(t, s, ctx, "wrong-pw"), ctx)
	}
	s = pressEnter(typeText(t, s, ctx, "right-pw"), ctx)
	if ctx.PlayerName != "" {
		t.Fatal("login must be refused while throttled, even with the right password")
	}
	if ns, ok := s.(*NicknameScreen); !ok || ns.error == "" {
		t.Fatalf("throttled login should show an error, got %#v", s)
	}
}
