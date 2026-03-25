package ui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

// Total known entries for completion calculation
const (
	totalMonsters = 16 // 8 regular + 4 bosses + 4 mini-bosses
	totalItems    = 15 // weapons + armor + potions + relics
	totalEvents   = 8  // random events from game/random_event.go
)

// All known entry names for display
var allMonsters = []string{
	"Goblin", "Skeleton", "Bat", "Slime", "Zombie", "Spider", "Rat", "Ghost",
	"Dragon", "Lich", "Demon Lord", "Hydra",
	"Troll", "Wraith", "Golem", "Minotaur",
}

var allItems = []string{
	"Iron Sword", "Steel Axe", "Magic Staff", "Shadow Dagger", "Holy Mace",
	"Leather Armor", "Chain Mail", "Plate Armor",
	"Health Potion", "Mana Potion", "Strength Potion",
	"Shield Relic", "Amulet of Life", "Ring of Power", "Boots of Speed",
}

var allEvents = []string{
	"altar", "fountain", "merchant", "trap_room",
	"shrine", "chest", "ghost", "mushroom",
}

// CodexScreen displays the player's codex with discovered entries.
type CodexScreen struct {
	codex store.Codex
	tab   int // 0=monsters, 1=items, 2=events
}

func NewCodexScreen(ctx *Context) *CodexScreen {
	var codex store.Codex
	if ctx.Store != nil {
		codex, _ = ctx.Store.GetCodex(ctx.Fingerprint)
	} else {
		codex = store.Codex{
			Monsters: make(map[string]bool),
			Items:    make(map[string]bool),
			Events:   make(map[string]bool),
		}
	}
	return &CodexScreen{codex: codex}
}

func (s *CodexScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "esc", "c", "q") || key.Type == tea.KeyEsc {
			return NewTitleScreen(), nil
		}
		if key.Type == tea.KeyTab || isKey(key, "right", "l") || key.Type == tea.KeyRight {
			s.tab = (s.tab + 1) % 3
		}
		if isKey(key, "left", "h") || key.Type == tea.KeyLeft {
			s.tab = (s.tab + 2) % 3
		}
	}
	return s, nil
}

func (s *CodexScreen) View(ctx *Context) string {
	title := styleHeader.Render("-- Codex --")

	// Tab headers
	tabNames := []string{"Monsters", "Items", "Events"}
	var tabs []string
	for i, name := range tabNames {
		if i == s.tab {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(colorYellow).Bold(true).
				Render(fmt.Sprintf("[ %s ]", name)))
		} else {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(colorGray).
				Render(fmt.Sprintf("  %s  ", name)))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Center, tabs...)

	// Entries
	var entries string
	var discovered map[string]bool
	var allNames []string
	var total int

	switch s.tab {
	case 0:
		discovered = s.codex.Monsters
		allNames = allMonsters
		total = totalMonsters
	case 1:
		discovered = s.codex.Items
		allNames = allItems
		total = totalItems
	case 2:
		discovered = s.codex.Events
		allNames = allEvents
		total = totalEvents
	}

	count := len(discovered)
	pct := 0.0
	if total > 0 {
		pct = float64(count) / float64(total) * 100
	}

	completion := lipgloss.NewStyle().Foreground(colorCyan).
		Render(fmt.Sprintf("Discovered: %d/%d (%.0f%%)", count, total, pct))

	// Sort discovered keys for consistent display
	discoveredKeys := make([]string, 0, len(discovered))
	for k := range discovered {
		discoveredKeys = append(discoveredKeys, k)
	}
	sort.Strings(discoveredKeys)

	// Build a set of discovered for quick lookup
	discoveredSet := discovered

	for _, name := range allNames {
		if discoveredSet[name] {
			entries += fmt.Sprintf("  [x] %s\n", lipgloss.NewStyle().Foreground(colorGreen).Render(name))
		} else {
			entries += fmt.Sprintf("  [ ] %s\n", lipgloss.NewStyle().Foreground(colorGray).Render("???"))
		}
	}

	// Show any discovered entries not in the known list
	for _, k := range discoveredKeys {
		found := false
		for _, name := range allNames {
			if name == k {
				found = true
				break
			}
		}
		if !found {
			entries += fmt.Sprintf("  [x] %s\n", lipgloss.NewStyle().Foreground(colorGreen).Render(k))
		}
	}

	footer := styleSystem.Render("[Tab/Left/Right] Switch Tab  [Esc] Back")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		tabBar,
		"",
		completion,
		"",
		entries,
		"",
		footer,
	)

	return lipgloss.Place(ctx.Width, ctx.Height, lipgloss.Center, lipgloss.Center, content)
}
