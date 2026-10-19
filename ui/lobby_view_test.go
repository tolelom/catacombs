package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
)

// Pins what each lobby key does before LobbyScreen.Update is split into steps.
// If one breaks, a player cannot create, join or leave rooms as before.

func newLobbyTestContext(t *testing.T) *Context {
	t.Helper()
	cfg, _ := config.Load("")
	lobby := game.NewLobby(cfg)
	lobby.PlayerOnline("fp-me", "Hero")
	return &Context{Lobby: lobby, Fingerprint: "fp-me", PlayerName: "Hero"}
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func pressKeys(t *testing.T, screen Screen, ctx *Context, keys ...tea.KeyMsg) Screen {
	t.Helper()
	for _, key := range keys {
		screen, _ = screen.Update(key, ctx)
	}
	return screen
}

func typeCode(code string) []tea.KeyMsg {
	keys := []tea.KeyMsg{runeKey('j')}
	for _, r := range code {
		keys = append(keys, runeKey(r))
	}
	return keys
}

func TestLobbyJoinByCodeEntersClassSelect(t *testing.T) {
	ctx := newLobbyTestContext(t)
	code, err := ctx.Lobby.CreateRoom("fp-other", "Other")
	if err != nil {
		t.Fatal(err)
	}
	lowered := []rune(code)
	for i, r := range lowered {
		if r >= 'A' && r <= 'Z' {
			lowered[i] = r + ('a' - 'A')
		}
	}

	keys := append(typeCode(string(lowered)), tea.KeyMsg{Type: tea.KeyEnter})
	next := pressKeys(t, NewLobbyScreen(), ctx, keys...)

	if _, ok := next.(*ClassSelectScreen); !ok {
		t.Fatalf("joining by code should open class select, got %T", next)
	}
	if ctx.RoomCode != code {
		t.Errorf("RoomCode = %q, want %q", ctx.RoomCode, code)
	}
}

func TestLobbyJoinByUnknownCodeShowsError(t *testing.T) {
	ctx := newLobbyTestContext(t)
	screen := NewLobbyScreen()

	keys := append(typeCode("ZZZZ"), tea.KeyMsg{Type: tea.KeyEnter})
	next := pressKeys(t, screen, ctx, keys...)

	if next != screen {
		t.Fatalf("a failed join should stay in the lobby, got %T", next)
	}
	if screen.errMsg == "" || screen.joining || screen.codeInput != "" {
		t.Errorf("errMsg=%q joining=%v codeInput=%q, want an error and a reset input", screen.errMsg, screen.joining, screen.codeInput)
	}
}

func TestLobbyCodeInputEditing(t *testing.T) {
	ctx := newLobbyTestContext(t)
	screen := NewLobbyScreen()

	pressKeys(t, screen, ctx, typeCode("abcde")...)
	if screen.codeInput != "ABCD" {
		t.Fatalf("codeInput = %q, want the first four letters upper-cased", screen.codeInput)
	}

	screen.codeInput = "AB"
	pressKeys(t, screen, ctx, tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyBackspace})
	if !screen.joining || screen.codeInput != "A" {
		t.Fatalf("joining=%v codeInput=%q, want Enter ignored before four letters and Backspace to delete", screen.joining, screen.codeInput)
	}

	pressKeys(t, screen, ctx, tea.KeyMsg{Type: tea.KeyEsc})
	if screen.joining || screen.codeInput != "" {
		t.Errorf("Esc should cancel code entry, joining=%v codeInput=%q", screen.joining, screen.codeInput)
	}
}

func TestLobbyEnterJoinsSelectedRoom(t *testing.T) {
	ctx := newLobbyTestContext(t)
	if _, err := ctx.Lobby.CreateRoom("fp-a", "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := ctx.Lobby.CreateRoom("fp-b", "B"); err != nil {
		t.Fatal(err)
	}
	screen := NewLobbyScreen()
	screen.refreshLobby(ctx)
	if len(screen.rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(screen.rooms))
	}

	pressKeys(t, screen, ctx, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown})
	if screen.cursor != 1 {
		t.Fatalf("cursor = %d, want it clamped at the last room", screen.cursor)
	}
	pressKeys(t, screen, ctx, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyUp})
	if screen.cursor != 0 {
		t.Fatalf("cursor = %d, want it clamped at the first room", screen.cursor)
	}

	want := screen.rooms[1].Code
	next := pressKeys(t, screen, ctx, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := next.(*ClassSelectScreen); !ok {
		t.Fatalf("Enter should join the selected room, got %T", next)
	}
	if ctx.RoomCode != want {
		t.Errorf("RoomCode = %q, want %q", ctx.RoomCode, want)
	}
}

func TestLobbyEnterOnStartedRoomShowsError(t *testing.T) {
	ctx := newLobbyTestContext(t)
	code, err := ctx.Lobby.CreateRoom("fp-a", "A")
	if err != nil {
		t.Fatal(err)
	}
	screen := NewLobbyScreen()
	screen.refreshLobby(ctx)
	ctx.Lobby.StartRoom(code)

	next := pressKeys(t, screen, ctx, tea.KeyMsg{Type: tea.KeyEnter})
	if next != screen || screen.errMsg == "" {
		t.Errorf("joining a started room should stay with an error, got %T errMsg=%q", next, screen.errMsg)
	}
}

func TestLobbyDailyStartsDailySession(t *testing.T) {
	ctx := newLobbyTestContext(t)

	next := pressKeys(t, NewLobbyScreen(), ctx, runeKey('d'))

	if _, ok := next.(*ClassSelectScreen); !ok {
		t.Fatalf("'d' should open class select, got %T", next)
	}
	if ctx.Session == nil || !ctx.Session.DailyMode {
		t.Fatalf("'d' should attach a daily session, got %+v", ctx.Session)
	}
	if want := time.Now().Format("2006-01-02"); ctx.Session.DailyDate != want {
		t.Errorf("DailyDate = %q, want %q", ctx.Session.DailyDate, want)
	}
	if room := ctx.Lobby.GetRoom(ctx.RoomCode); room == nil || !room.Private {
		t.Errorf("daily room should be private, got %+v", room)
	}
}

func TestLobbyCreateRoomQuotaShowsError(t *testing.T) {
	ctx := newLobbyTestContext(t)
	if _, err := ctx.Lobby.CreateRoom("fp-me", "Mine"); err != nil {
		t.Fatal(err)
	}
	screen := NewLobbyScreen()

	next := pressKeys(t, screen, ctx, runeKey('c'))

	if next != screen {
		t.Fatalf("a refused create should stay in the lobby, got %T", next)
	}
	if screen.errMsg != game.ErrOwnerHasWaitingRoom.Error() {
		t.Errorf("errMsg = %q, want %q", screen.errMsg, game.ErrOwnerHasWaitingRoom.Error())
	}
}

func TestLobbyHardModeToggleNeedsUnlock(t *testing.T) {
	ctx := newLobbyTestContext(t)
	screen := NewLobbyScreen()

	pressKeys(t, screen, ctx, runeKey('h'))
	if screen.hardMode || ctx.HardMode {
		t.Fatal("'h' must not toggle hard mode while it is locked")
	}

	screen.hardUnlocked = true
	pressKeys(t, screen, ctx, runeKey('h'))
	if !screen.hardMode || !ctx.HardMode {
		t.Fatal("'h' should turn hard mode on once unlocked")
	}
	pressKeys(t, screen, ctx, runeKey('h'))
	if screen.hardMode || ctx.HardMode {
		t.Fatal("'h' should turn hard mode back off")
	}
}

func TestLobbySoulsAndQuitKeys(t *testing.T) {
	ctx := newLobbyTestContext(t)

	if next := pressKeys(t, NewLobbyScreen(), ctx, runeKey('s')); !isSoulsScreen(next) {
		t.Errorf("'s' should open the souls screen, got %T", next)
	}

	next := pressKeys(t, NewLobbyScreen(), ctx, runeKey('q'))
	if _, ok := next.(*TitleScreen); !ok {
		t.Errorf("'q' should return to the title, got %T", next)
	}
	if n := len(ctx.Lobby.ListOnline()); n != 0 {
		t.Errorf("'q' should mark the player offline, %d still online", n)
	}
}

func isSoulsScreen(screen Screen) bool {
	_, ok := screen.(*SoulsScreen)
	return ok
}
