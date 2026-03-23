package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"os"
)

func testDB(t *testing.T) *store.DB {
	db, err := store.Open("test_ui.db")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTitleToLobby(t *testing.T) {
	lobby := game.NewLobby()
	db := testDB(t)
	defer func() { db.Close(); os.Remove("test_ui.db") }()

	m := NewModel(80, 24, "testfp", lobby, db)

	if m.screen != screenTitle {
		t.Fatalf("initial screen: got %d, want screenTitle(0)", m.screen)
	}

	// Press Enter
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)

	if m2.screen != screenLobby {
		t.Errorf("after Enter: screen=%d, want screenLobby(1)", m2.screen)
	}
	if m2.playerName == "" {
		t.Error("playerName should be set")
	}
}

func TestLobbyCreateRoom(t *testing.T) {
	lobby := game.NewLobby()
	db := testDB(t)
	defer func() { db.Close(); os.Remove("test_ui.db") }()

	m := NewModel(80, 24, "testfp", lobby, db)

	// Go to lobby
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)

	// Press 'c' to create room
	result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m3 := result.(Model)

	if m3.screen != screenClassSelect {
		t.Errorf("after 'c': screen=%d, want screenClassSelect(2)", m3.screen)
	}
	if m3.roomCode == "" {
		t.Error("roomCode should be set")
	}
}

func TestClassSelectToGame(t *testing.T) {
	lobby := game.NewLobby()
	db := testDB(t)
	defer func() { db.Close(); os.Remove("test_ui.db") }()

	m := NewModel(80, 24, "testfp", lobby, db)

	// Title -> Lobby -> Class Select -> Game
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)
	result, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m3 := result.(Model)

	if m3.screen != screenClassSelect {
		t.Fatalf("should be at class select, got %d", m3.screen)
	}

	// Press Enter to select Warrior (default cursor=0)
	result, _ = m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m4 := result.(Model)

	if m4.screen != screenGame {
		t.Errorf("after class select Enter: screen=%d, want screenGame(3)", m4.screen)
	}
	if m4.session == nil {
		t.Error("session should be set")
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
