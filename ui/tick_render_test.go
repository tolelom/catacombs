package ui

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// tickCountingScreen schedules a fresh tick on every key, like the combat
// screen does, and counts how many ticks actually reach it.
type tickCountingScreen struct {
	ticks int
}

func (s *tickCountingScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		s.ticks++
	case tea.KeyMsg:
		return s, scheduleTick(ctx, time.Millisecond)
	}
	return s, nil
}

func (s *tickCountingScreen) View(*Context) string { return "" }

func runCmds(cmds []tea.Cmd) []tea.Msg {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var msgs []tea.Msg
	for _, c := range cmds {
		if c == nil {
			continue
		}
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			msg := c()
			mu.Lock()
			msgs = append(msgs, msg)
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	return msgs
}

func TestOnlyTheLatestTickReachesTheScreen(t *testing.T) {
	screen := &tickCountingScreen{}
	m := Model{currentScreen: screen, ctx: &Context{Width: 80, Height: 24}}

	var cmds []tea.Cmd
	for i := 0; i < 30; i++ {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		m = next.(Model)
		cmds = append(cmds, cmd)
	}
	for _, msg := range runCmds(cmds) {
		next, _ := m.Update(msg)
		m = next.(Model)
	}

	if screen.ticks != 1 {
		t.Fatalf("screen handled %d ticks after 30 key presses, want 1", screen.ticks)
	}
}

func newStoreWithRun(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "ui.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.SaveRun("ranked-player", 7, 1234, "warrior", nil); err != nil {
		t.Fatalf("save run: %v", err)
	}
	return db
}

// renderBeforeAndAfterNewRun renders once, records a new top run, and renders
// again; a screen that loads once on entry must not pick the new run up.
func renderBeforeAndAfterNewRun(t *testing.T, s Screen, ctx *Context) string {
	t.Helper()
	_ = s.View(ctx)
	if err := ctx.Store.SaveRun("newcomer", 19, 99999, "mage", nil); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := ctx.Store.SaveRun(ctx.PlayerName, 19, 99999, "mage", nil); err != nil {
		t.Fatalf("save run: %v", err)
	}
	return s.View(ctx)
}

func TestResultScreenLoadsRankingsOnce(t *testing.T) {
	ctx := &Context{Width: 120, Height: 40, PlayerName: "viewer", Store: newStoreWithRun(t)}
	view := renderBeforeAndAfterNewRun(t, NewResultScreen(game.GameState{}, true), ctx)

	if !strings.Contains(view, "ranked-player") || strings.Contains(view, "newcomer") {
		t.Fatal("result screen re-queried rankings on render")
	}
}

func TestLeaderboardLoadsOnce(t *testing.T) {
	ctx := &Context{Width: 120, Height: 40, PlayerName: "viewer", Store: newStoreWithRun(t)}
	view := renderBeforeAndAfterNewRun(t, NewLeaderboardScreen(), ctx)

	if !strings.Contains(view, "ranked-player") || strings.Contains(view, "newcomer") {
		t.Fatal("leaderboard re-queried the store on render")
	}
}

func TestStatsLoadOnce(t *testing.T) {
	ctx := &Context{Width: 120, Height: 40, PlayerName: "ranked-player", Store: newStoreWithRun(t)}
	screen := NewStatsScreen()
	first := screen.View(ctx)
	view := renderBeforeAndAfterNewRun(t, screen, ctx)

	if view != first {
		t.Fatal("stats screen re-queried the store on render")
	}
}

func newExploringSession(t *testing.T) *game.GameSession {
	t.Helper()
	cfg, _ := config.Load("")
	s := game.NewGameSession(cfg)
	p := entity.NewPlayer("p", entity.ClassWarrior)
	p.Fingerprint = "fp-p"
	s.AddPlayer(p)
	s.StartGame()
	t.Cleanup(s.Stop)
	return s
}

func TestStaleMoveCursorDoesNotPanic(t *testing.T) {
	session := newExploringSession(t)
	if session.GetState().Phase != game.PhaseExploring {
		t.Skip("first room did not start in exploration")
	}
	ctx := &Context{Width: 120, Height: 40, Fingerprint: "fp-p", PlayerName: "p", Session: session}
	gs := NewGameScreen()
	gs.gameState = session.GetState()
	gs.moveCursor = 99

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Enter with a stale move cursor panicked: %v", r)
		}
	}()
	gs.Update(tea.KeyMsg{Type: tea.KeyEnter}, ctx)
}

func TestChatBackspaceRemovesAWholeCharacter(t *testing.T) {
	gs := NewGameScreen()
	gs.chatting = true
	gs.chatInput = "안녕"

	gs.Update(tea.KeyMsg{Type: tea.KeyBackspace}, &Context{Width: 120, Height: 40})

	if gs.chatInput != "안" {
		t.Fatalf("backspace left %q, want %q", gs.chatInput, "안")
	}
}

func TestChatAcceptsFortyKoreanCharacters(t *testing.T) {
	gs := NewGameScreen()
	gs.chatting = true
	ctx := &Context{Width: 120, Height: 40}

	for i := 0; i < 45; i++ {
		gs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'가'}}, ctx)
	}

	if got := len([]rune(gs.chatInput)); got != 40 {
		t.Fatalf("chat holds %d characters, want 40", got)
	}
}
