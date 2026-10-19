package ui

import (
	"fmt"
	"slices"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/store"
)

var allMonsters = []string{
	"Slime", "Skeleton", "Orc", "Dark Knight", "Mimic",
	"Guardian's Herald", "Warden's Shadow", "Overlord's Lieutenant", "Archlich's Harbinger",
	"Guardian", "Warden", "Overlord", "Archlich",
}

var allItems = []string{
	"Iron Sword", "Bronze Sword", "Steel Sword", "Mythril Blade",
	"Iron Shield", "Bronze Shield", "Steel Shield", "Mythril Shield",
	"HP Potion", "행운 부적",
	"Vampiric Ring", "Power Amulet", "Iron Ward", "Gold Charm",
	"Antidote Charm", "Flame Guard", "Life Siphon",
}

var allEvents = []string{
	"altar", "fountain", "merchant", "trap_room",
	"shrine", "chest", "ghost", "mushroom",
	"gravekeeper", "forge", "toll_bridge", "feast",
}

var bossNames = []string{"Guardian", "Warden", "Overlord", "Archlich"}

var (
	totalMonsters = len(allMonsters)
	totalItems    = len(allItems)
	totalEvents   = len(allEvents)
)

type CodexScreen struct {
	codex     store.Codex
	tab       int
	bossKills map[string]int64
}

func NewCodexScreen(ctx *Context) *CodexScreen {
	var codex store.Codex
	bossKills := make(map[string]int64)
	if ctx.Store != nil {
		codex, _ = ctx.Store.GetCodex(ctx.Fingerprint)
		for _, name := range bossNames {
			n, _ := ctx.Store.GetCounter(ctx.Fingerprint, "boss_kill:"+name)
			bossKills[name] = n
		}
	} else {
		codex = store.Codex{
			Monsters: make(map[string]bool),
			Items:    make(map[string]bool),
			Events:   make(map[string]bool),
		}
	}
	return &CodexScreen{codex: codex, bossKills: bossKills}
}

func (s *CodexScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "esc", "c", "q") || key.Type == tea.KeyEsc {
			return NewTitleScreen(), nil
		}
		if key.Type == tea.KeyTab || isKey(key, "right", "l") || key.Type == tea.KeyRight {
			s.tab = (s.tab + 1) % 4
		}
		if isKey(key, "left", "h") || key.Type == tea.KeyLeft {
			s.tab = (s.tab + 3) % 4
		}
	}
	return s, nil
}

func (s *CodexScreen) View(ctx *Context) string {
	title := styleHeader.Render("-- 도감 --")
	tabBar := renderCodexTabBar(s.tab)

	var completion, entries string
	if s.tab == 3 {
		completion, entries = s.renderBossTrophies()
	} else {
		completion, entries = s.renderDiscoveries()
	}

	footer := styleSystem.Render("[Tab/Left/Right] 탭 전환  [Esc] 뒤로")

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

func renderCodexTabBar(activeTab int) string {
	tabNames := []string{"몬스터", "아이템", "이벤트", "보스 트로피"}
	var tabs []string
	for i, name := range tabNames {
		if i == activeTab {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(colorYellow).Bold(true).
				Render(fmt.Sprintf("[ %s ]", name)))
		} else {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(colorGray).
				Render(fmt.Sprintf("  %s  ", name)))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, tabs...)
}

func (s *CodexScreen) renderBossTrophies() (completion, entries string) {
	killCount := 0
	for _, name := range bossNames {
		n := s.bossKills[name]
		if n > 0 {
			killCount++
			entries += fmt.Sprintf("  🏆 %s — %d회 처치\n", lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render(name), n)
			if lore := monsterLore(name); lore != "" {
				entries += "      " + styleSystem.Render(lore) + "\n"
			}
		} else {
			entries += fmt.Sprintf("  [ ] %s\n", lipgloss.NewStyle().Foreground(colorGray).Render("???"))
		}
	}
	completion = lipgloss.NewStyle().Foreground(colorCyan).Render(fmt.Sprintf("트로피: %d/%d", killCount, len(bossNames)))
	return completion, entries
}

func (s *CodexScreen) renderDiscoveries() (completion, entries string) {
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
	completion = lipgloss.NewStyle().Foreground(colorCyan).
		Render(fmt.Sprintf("발견: %d/%d (%.0f%%)", count, total, pct))

	for _, name := range allNames {
		entries += s.renderKnownEntry(name, discovered[name])
	}
	entries += renderUnlistedDiscoveries(discovered, allNames)
	return completion, entries
}

func (s *CodexScreen) renderKnownEntry(name string, found bool) string {
	if !found {
		return fmt.Sprintf("  [ ] %s\n", lipgloss.NewStyle().Foreground(colorGray).Render("???"))
	}
	entry := fmt.Sprintf("  [x] %s\n", lipgloss.NewStyle().Foreground(colorGreen).Render(name))
	var lore string
	switch s.tab {
	case 0:
		lore = monsterLore(name)
	case 1:
		lore = itemLore(name)
	case 2:
		lore = eventLore(name)
	}
	if lore != "" {
		entry += "      " + styleSystem.Render(lore) + "\n"
	}
	return entry
}

func renderUnlistedDiscoveries(discovered map[string]bool, allNames []string) string {
	discoveredKeys := make([]string, 0, len(discovered))
	for k := range discovered {
		discoveredKeys = append(discoveredKeys, k)
	}
	sort.Strings(discoveredKeys)

	var entries string
	for _, k := range discoveredKeys {
		if !slices.Contains(allNames, k) {
			entries += fmt.Sprintf("  [x] %s\n", lipgloss.NewStyle().Foreground(colorGreen).Render(k))
		}
	}
	return entries
}
