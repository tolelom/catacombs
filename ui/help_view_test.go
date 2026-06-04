package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHelpPageNavigationClamps — arrow keys move between pages and clamp at
// both edges instead of wrapping or panicking.
func TestHelpPageNavigationClamps(t *testing.T) {
	s := NewHelpScreen()
	ctx := &Context{Width: 80, Height: 24}

	left := tea.KeyMsg{Type: tea.KeyLeft}
	right := tea.KeyMsg{Type: tea.KeyRight}

	s.Update(left, ctx)
	if s.page != 0 {
		t.Errorf("left at first page should clamp to 0, got %d", s.page)
	}
	for i := 0; i < helpPageCount+2; i++ {
		s.Update(right, ctx)
	}
	if s.page != helpPageCount-1 {
		t.Errorf("right past last page should clamp to %d, got %d", helpPageCount-1, s.page)
	}
	s.Update(left, ctx)
	if s.page != helpPageCount-2 {
		t.Errorf("left should step back one page, got %d", s.page)
	}
}

// TestHelpPagesDocumentNewSystems — each page must mention its headline
// systems so the new mechanics are discoverable in-game.
func TestHelpPagesDocumentNewSystems(t *testing.T) {
	wantByPage := map[int][]string{
		0: {"공격", "Warrior", "Healer"},                  // controls + classes
		1: {"잔향 일격", "반격 태세", "연속 공세", "캡스톤", "협동 보너스"}, // combat systems
		2: {"투표", "보물", "행운 부적", "도박"},                   // exploration + economy
	}
	for page, wants := range wantByPage {
		out := renderHelp(120, 60, nil, page)
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("help page %d should contain %q", page+1, want)
			}
		}
		pageMarker := fmt.Sprintf("(%d/%d)", page+1, helpPageCount)
		if !strings.Contains(out, pageMarker) {
			t.Errorf("help page %d should show indicator %q", page+1, pageMarker)
		}
	}
}

// TestHelpExitKeysReturnToTitle — H/Enter/Q still leave the help screen.
func TestHelpExitKeysReturnToTitle(t *testing.T) {
	ctx := &Context{Width: 80, Height: 24}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'h'}},
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	} {
		s := NewHelpScreen()
		next, _ := s.Update(key, ctx)
		if _, ok := next.(*TitleScreen); !ok {
			t.Errorf("key %v should return to title, got %T", key, next)
		}
	}
}

// TestCodexIncludesLuckyCharm — the charm is purchasable in every shop, so it
// must be a discoverable codex entry with lore.
func TestCodexIncludesLuckyCharm(t *testing.T) {
	found := false
	for _, name := range allItems {
		if name == "행운 부적" {
			found = true
			break
		}
	}
	if !found {
		t.Error("allItems should include 행운 부적")
	}
	if itemLore("행운 부적") == "" {
		t.Error("행운 부적 should have lore text")
	}
}

// TestNoOrphanMonsterLore — every monster with lore must exist in the codex
// list; orphaned lore (e.g. the removed Mimic) is dead content.
func TestNoOrphanMonsterLore(t *testing.T) {
	if monsterLore("Mimic") != "" {
		t.Error("Mimic lore should be removed (not in allMonsters)")
	}
	for _, name := range allMonsters {
		if monsterLore(name) == "" {
			t.Errorf("monster %q in codex has no lore", name)
		}
	}
}
