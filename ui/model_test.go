package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

func testDB(t *testing.T) *store.DB {
	db, err := store.Open("test_ui.db")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTitleToLobby(t *testing.T) {
	lobby := game.NewLobby(func() *config.Config { c, _ := config.Load(""); return c }())
	db := testDB(t)
	defer func() { _ = db.Close(); _ = os.Remove("test_ui.db") }()

	m := NewModel(80, 24, "testfp", lobby, db)

	if m.screenType() != screenTitle {
		t.Fatalf("initial screen: got %d, want screenTitle(0)", m.screenType())
	}

	// First-time player: Enter goes to nickname screen
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)

	if m2.screenType() != screenNickname {
		t.Errorf("after Enter (first time): screen=%d, want screenNickname(%d)", m2.screenType(), screenNickname)
	}

	// Type a name
	for _, ch := range "Hero" {
		result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m2 = result.(Model)
	}

	// Confirm nickname
	result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := result.(Model)

	if m3.screenType() != screenLobby {
		t.Errorf("after nickname Enter: screen=%d, want screenLobby(1)", m3.screenType())
	}
	if m3.playerName() == "" {
		t.Error("playerName should be set")
	}
}

func TestLobbyCreateRoom(t *testing.T) {
	lobby := game.NewLobby(func() *config.Config { c, _ := config.Load(""); return c }())
	db := testDB(t)
	defer func() { _ = db.Close(); _ = os.Remove("test_ui.db") }()

	m := NewModel(80, 24, "testfp2", lobby, db)

	// Go to nickname screen (first-time player)
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)

	// Type name and confirm
	for _, ch := range "Hero" {
		result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m2 = result.(Model)
	}
	result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 = result.(Model)

	// Press 'c' to create room
	result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m3 := result.(Model)

	if m3.screenType() != screenClassSelect {
		t.Errorf("after 'c': screen=%d, want screenClassSelect(2)", m3.screenType())
	}
	if m3.roomCode() == "" {
		t.Error("roomCode should be set")
	}
}

func TestClassSelectToGame(t *testing.T) {
	lobby := game.NewLobby(func() *config.Config { c, _ := config.Load(""); return c }())
	db := testDB(t)
	defer func() { _ = db.Close(); _ = os.Remove("test_ui.db") }()

	m := NewModel(80, 24, "testfp3", lobby, db)

	// Title -> Nickname -> Lobby
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)
	for _, ch := range "Hero" {
		result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m2 = result.(Model)
	}
	result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 = result.(Model)

	// Lobby -> Class Select
	result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m3 := result.(Model)

	if m3.screenType() != screenClassSelect {
		t.Fatalf("should be at class select, got %d", m3.screenType())
	}

	// Press Enter to select Warrior (default cursor=0) → WaitingScreen
	result, _ = m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m4 := result.(Model)

	if m4.screenType() != screenWaiting {
		t.Fatalf("after class select Enter: screen=%d, want screenWaiting(%d)", m4.screenType(), screenWaiting)
	}

	// Press Enter to ready up (solo room → immediately starts game)
	result, _ = m4.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m5 := result.(Model)

	if m5.screenType() != screenGame {
		t.Errorf("after ready Enter: screen=%d, want screenGame(%d)", m5.screenType(), screenGame)
	}
	if m5.session() == nil {
		t.Error("session should be set")
	}
}

// TestFindPlayerRankAndMissing verifies the result-screen rank lookup:
// returns the 1-indexed position in the top-N or 0 when the player isn't
// listed.
func TestFindPlayerRankAndMissing(t *testing.T) {
	rankings := []store.RunRecord{
		{Player: "Top",    Floor: 20},
		{Player: "Second", Floor: 18},
		{Player: "Me",     Floor: 12},
	}
	if got := findPlayerRank(rankings, "Me"); got != 3 {
		t.Errorf("expected rank 3 for Me, got %d", got)
	}
	if got := findPlayerRank(rankings, "Nobody"); got != 0 {
		t.Errorf("expected 0 (not listed) for Nobody, got %d", got)
	}
	if got := findPlayerRank(nil, "Me"); got != 0 {
		t.Errorf("expected 0 on empty rankings, got %d", got)
	}
}

// TestRenderHelpDynamic verifies that the help screen pulls live numbers
// from the active config rather than hardcoding "10초" / "3회" etc., so
// admin overrides in config.yaml stay in sync with what users read.
func TestRenderHelpDynamic(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Game.TurnTimeoutSec = 7
	cfg.Game.SkillUses = 5
	cfg.Game.MaxPlayers = 6
	// Dynamic numbers live on specific pages now: timeout on the controls
	// page, skill uses and player cap on the combat page.
	out := renderHelp(80, 24, cfg, 0)
	if !strings.Contains(out, "턴당 7초") {
		t.Error("expected controls page to contain 턴당 7초")
	}
	out = renderHelp(80, 24, cfg, 1)
	for _, want := range []string{"5회 사용", "최대 6명"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected combat page to contain %q", want)
		}
	}
	// Nil cfg should still produce something sensible (defaults).
	out = renderHelp(80, 24, nil, 0)
	if !strings.Contains(out, "턴당 10초") {
		t.Error("nil cfg should fall back to default 10초")
	}
}

// TestIsWebUserRecognizesAnonPrefixes pins the web/SSH discrimination so
// the password flow always runs for anonymous (web) connections and never
// for real SSH key holders.
func TestIsWebUserRecognizesAnonPrefixes(t *testing.T) {
	cases := []struct {
		fp   string
		want bool
		desc string
	}{
		{"", true, "empty fingerprint"},
		{"anon:abcdef0123456789", true, "anon: prefix"},
		{"SHA256:abcdef", false, "real SSH fingerprint"},
		{"SHA256:web:nick", false, "deterministic web login fingerprint"},
	}
	for _, tc := range cases {
		ctx := &Context{Fingerprint: tc.fp}
		if got := isWebUser(ctx); got != tc.want {
			t.Errorf("%s (fp=%q): got %v, want %v", tc.desc, tc.fp, got, tc.want)
		}
	}
}

func TestKeyHelpers(t *testing.T) {
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	if !isEnter(enter) {
		t.Error("isEnter should match KeyEnter type")
	}

	enterStr := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\r'}}
	_ = enterStr // might not match, that's ok

	up := tea.KeyMsg{Type: tea.KeyUp}
	if !isUp(up) {
		t.Error("isUp should match KeyUp type")
	}

	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	if !isQuit(q) {
		t.Error("isQuit should match 'q'")
	}
}
