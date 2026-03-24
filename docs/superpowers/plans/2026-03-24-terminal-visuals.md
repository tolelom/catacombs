# Terminal Visuals Enhancement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance Catacombs terminal visuals — monster ASCII art, combat layout with box-drawing panels, semantic color effects, and improved title screen.

**Architecture:** New files for styles and ASCII art data. Rewrite of `game_view.go` render functions. All changes are UI-only — no game logic modifications.

**Tech Stack:** Go, charmbracelet/lipgloss (box-drawing, colors, layout)

**Spec:** `docs/superpowers/specs/2026-03-24-terminal-visuals-design.md`

---

## File Map

```
ui/styles.go             — NEW: shared lipgloss style constants
ui/ascii_art.go          — NEW: monster ASCII art data + Art() function
ui/game_view.go          — REWRITE: combat layout, colored log, HP bar 3-color
ui/title.go              — MODIFY: gradient logo, centered layout
```

---

### Task 1: Shared Styles (ui/styles.go)

**Files:**
- Create: `ui/styles.go`

- [ ] **Step 1: Create styles.go with color constants and reusable styles**

```go
package ui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	colorRed     = lipgloss.Color("196")
	colorGreen   = lipgloss.Color("46")
	colorYellow  = lipgloss.Color("226")
	colorCyan    = lipgloss.Color("51")
	colorMagenta = lipgloss.Color("201")
	colorWhite   = lipgloss.Color("255")
	colorGray    = lipgloss.Color("240")
	colorOrange  = lipgloss.Color("208")
	colorPink    = lipgloss.Color("205")
)

// Text styles
var (
	styleDamage  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleHeal    = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleCoop    = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleFlee    = lipgloss.NewStyle().Foreground(colorCyan)
	styleStatus  = lipgloss.NewStyle().Foreground(colorMagenta)
	styleGold    = lipgloss.NewStyle().Foreground(colorYellow)
	styleSystem  = lipgloss.NewStyle().Foreground(colorGray).Italic(true)
	styleEnemy   = lipgloss.NewStyle().Foreground(colorRed)
	stylePlayer  = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	styleHeader  = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	styleAction  = lipgloss.NewStyle().Bold(true)
	styleTimer   = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
)

// Panel styles
var (
	panelBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorGray).
		Padding(0, 1)

	panelHeader = lipgloss.NewStyle().
		Foreground(colorPink).
		Bold(true).
		Align(lipgloss.Center)
)
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add ui/styles.go
git commit -m "feat: add shared lipgloss styles for terminal visuals"
```

---

### Task 2: Monster ASCII Art (ui/ascii_art.go)

**Files:**
- Create: `ui/ascii_art.go`

- [ ] **Step 1: Create ascii_art.go with art data**

```go
package ui

import "github.com/tolelom/catacombs/entity"

// MonsterArt returns ASCII art lines for a monster type.
func MonsterArt(mt entity.MonsterType) []string {
	switch mt {
	case entity.MonsterSlime:
		return []string{
			` /\OO/\ `,
			` \    / `,
			`  |__| `,
		}
	case entity.MonsterSkeleton:
		return []string{
			` ,--. `,
			` |oo| `,
			` /||\ `,
		}
	case entity.MonsterOrc:
		return []string{
			` .---. `,
			`/o   o\`,
			`| --- |`,
		}
	case entity.MonsterDarkKnight:
		return []string{
			` /|||\ `,
			` |===| `,
			` |   | `,
		}
	case entity.MonsterBoss5:
		return []string{
			` /\  /\ `,
			`| @  @ |`,
			`|  ||  |`,
			`| \__/ |`,
			` \    / `,
		}
	case entity.MonsterBoss10:
		return []string{
			`  __|__  `,
			` /|o o|\ `,
			` | === | `,
			` |\___/| `,
			`  |___| `,
		}
	case entity.MonsterBoss15:
		return []string{
			` ,=====. `,
			`/ \   / \`,
			`|  (O)  |`,
			` \ |=| / `,
			`  '===' `,
		}
	case entity.MonsterBoss20:
		return []string{
			` ___/\___ `,
			`|  x  x  |`,
			`|  ===   |`,
			`|\_____/|`,
			`|_|   |_|`,
		}
	default:
		return []string{` ??? `}
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add ui/ascii_art.go
git commit -m "feat: add monster ASCII art data"
```

---

### Task 3: HP Bar 3-Color + Colored Combat Log (ui/game_view.go)

**Files:**
- Modify: `ui/game_view.go`

- [ ] **Step 1: Rewrite renderHPBar with 3-color thresholds**

Replace the `renderHPBar` function:
```go
func renderHPBar(current, max, width int) string {
	if max == 0 {
		return ""
	}
	filled := current * width / max
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	pct := float64(current) / float64(max)
	var barStyle lipgloss.Style
	switch {
	case pct > 0.5:
		barStyle = lipgloss.NewStyle().Foreground(colorGreen)
	case pct > 0.25:
		barStyle = lipgloss.NewStyle().Foreground(colorYellow)
	default:
		barStyle = lipgloss.NewStyle().Foreground(colorRed)
	}
	emptyStyle := lipgloss.NewStyle().Foreground(colorGray)

	return barStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))
}
```

- [ ] **Step 2: Rewrite renderCombatLog with pattern-based coloring**

Replace `renderCombatLog`:
```go
func renderCombatLog(log []string) string {
	if len(log) == 0 {
		return ""
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorGray).
		Padding(0, 1)

	var sb strings.Builder
	for _, msg := range log {
		colored := colorizeLog(msg)
		sb.WriteString("  > " + colored + "\n")
	}
	return border.Render(sb.String())
}

func colorizeLog(msg string) string {
	// Apply semantic colors based on keywords
	switch {
	case strings.Contains(msg, "fled"):
		return styleFlee.Render(msg)
	case strings.Contains(msg, "co-op"):
		return styleCoop.Render(msg)
	case strings.Contains(msg, "healed") || strings.Contains(msg, "Heal") || strings.Contains(msg, "Blessing"):
		return styleHeal.Render(msg)
	case strings.Contains(msg, "dmg") || strings.Contains(msg, "hit") || strings.Contains(msg, "attacks") || strings.Contains(msg, "Trap"):
		return styleDamage.Render(msg)
	case strings.Contains(msg, "Taunt") || strings.Contains(msg, "scouted"):
		return styleStatus.Render(msg)
	case strings.Contains(msg, "gold") || strings.Contains(msg, "Gold") || strings.Contains(msg, "found"):
		return styleGold.Render(msg)
	case strings.Contains(msg, "defeated") || strings.Contains(msg, "cleared") || strings.Contains(msg, "Descending"):
		return styleSystem.Render(msg)
	default:
		return msg
	}
}
```

- [ ] **Step 3: Build and test**

```bash
go build ./...
go test ./ui/ -timeout 15s
```

- [ ] **Step 4: Commit**

```bash
git add ui/game_view.go
git commit -m "feat: 3-color HP bar and semantic combat log coloring"
```

---

### Task 4: Combat Layout Redesign (ui/game_view.go)

**Files:**
- Modify: `ui/game_view.go`

This is the largest task. Rewrite `renderHUD` to use a 2-panel layout in combat mode.

- [ ] **Step 1: Add renderPartyPanel helper**

Add to `ui/game_view.go`:
```go
func renderPartyPanel(players []*entity.Player) string {
	var sb strings.Builder
	sb.WriteString(styleHeader.Render("  PARTY") + "\n\n")

	for _, p := range players {
		nameStr := stylePlayer.Render(fmt.Sprintf("  ♦ %s", p.Name))
		classStr := styleSystem.Render(fmt.Sprintf(" (%s)", p.Class))
		status := ""
		if p.IsDead() {
			status = styleDamage.Render(" [DEAD]")
		}
		sb.WriteString(nameStr + classStr + status + "\n")

		hpBar := renderHPBar(p.HP, p.MaxHP, 16)
		sb.WriteString(fmt.Sprintf("    %s %d/%d\n", hpBar, p.HP, p.MaxHP))

		sb.WriteString(fmt.Sprintf("    ATK:%-3d DEF:%-3d  ", p.EffectiveATK(), p.EffectiveDEF()))
		sb.WriteString(styleGold.Render(fmt.Sprintf("G:%d", p.Gold)))
		sb.WriteString("\n\n")
	}
	return sb.String()
}
```

- [ ] **Step 2: Add renderEnemyPanel helper**

```go
func renderEnemyPanel(monsters []*entity.Monster, targetCursor int) string {
	var sb strings.Builder
	sb.WriteString(styleHeader.Render("  ENEMIES") + "\n\n")

	for i, m := range monsters {
		if m.IsDead() {
			continue
		}
		// ASCII art
		art := MonsterArt(m.Type)
		for _, line := range art {
			sb.WriteString(styleEnemy.Render("  " + line) + "\n")
		}

		// Name + HP
		marker := "  "
		if i == targetCursor {
			marker = "> "
		}
		hpBar := renderHPBar(m.HP, m.MaxHP, 12)
		taunt := ""
		if m.TauntTarget {
			taunt = styleStatus.Render(" [TAUNTED]")
		}
		sb.WriteString(fmt.Sprintf("  %s[%d] %s %s %d/%d%s\n\n",
			marker, i, styleEnemy.Render(m.Name), hpBar, m.HP, m.MaxHP, taunt))
	}
	return sb.String()
}
```

- [ ] **Step 3: Rewrite renderHUD for combat mode**

Replace the combat section of `renderHUD` (the `if state.Phase == game.PhaseCombat` block):
```go
	if state.Phase == game.PhaseCombat {
		// Two-panel layout: PARTY | ENEMIES
		partyContent := renderPartyPanel(state.Players)
		enemyContent := renderEnemyPanel(state.Monsters, targetCursor)

		partyPanel := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGray).
			Width(35).
			Padding(0, 1).
			Render(partyContent)

		enemyPanel := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGray).
			Width(38).
			Padding(0, 1).
			Render(enemyContent)

		panels := lipgloss.JoinHorizontal(lipgloss.Top, partyPanel, enemyPanel)
		sb.WriteString(panels)
		sb.WriteString("\n")

		// Action bar
		sb.WriteString(styleAction.Render("[1]Attack [2]Skill [3]Item [4]Flee [5]Wait [Tab]Target [/]Chat"))
		sb.WriteString("\n")

		// Timer
		if !state.TurnDeadline.IsZero() {
			remaining := time.Until(state.TurnDeadline)
			if remaining < 0 {
				remaining = 0
			}
			sb.WriteString(styleTimer.Render(fmt.Sprintf("  Timer: %.1fs", remaining.Seconds())))
			sb.WriteString("\n")
		}

		// Skill description for current player (first alive)
		for _, p := range state.Players {
			if !p.IsDead() {
				var skillDesc string
				switch p.Class {
				case entity.ClassWarrior:
					skillDesc = "Skill: Taunt — enemies attack you for 2 turns"
				case entity.ClassMage:
					skillDesc = "Skill: Fireball — AoE 0.8x dmg to all enemies"
				case entity.ClassHealer:
					skillDesc = "Skill: Heal — restore 30 HP to an ally"
				case entity.ClassRogue:
					skillDesc = "Skill: Scout — reveal neighboring rooms"
				}
				sb.WriteString(styleSystem.Render(skillDesc))
				sb.WriteString("\n")
				break // show only current player's skill
			}
		}
	}
```

- [ ] **Step 4: Update renderHUD to not wrap combat in border (panels have their own)**

The exploration mode still uses the old `border` style. Change the function to only apply `border.Render()` for exploration, not combat:

Restructure `renderHUD` so that:
- Combat: return `sb.String()` directly (panels already have borders)
- Exploring: wrap in `border.Render(sb.String())`

Replace the return at the end of `renderHUD`:
```go
	if state.Phase == game.PhaseCombat {
		return sb.String()
	}
	return border.Render(sb.String())
```

- [ ] **Step 5: Remove the unused roomTypeSymbol function**

Delete `roomTypeSymbol` (lines ~185-202) — it's dead code.

- [ ] **Step 6: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 30s
```

- [ ] **Step 7: Commit**

```bash
git add ui/game_view.go
git commit -m "feat: two-panel combat layout with monster ASCII art"
```

---

### Task 5: Title Screen Enhancement (ui/title.go)

**Files:**
- Modify: `ui/title.go`

- [ ] **Step 1: Rewrite renderTitle with gradient colors and centered layout**

Replace `ui/title.go` entirely:
```go
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var titleLines = []string{
	` ██████╗ █████╗ ████████╗ █████╗  ██████╗ ██████╗ ███╗   ███╗██████╗ ███████╗`,
	`██╔════╝██╔══██╗╚══██╔══╝██╔══██╗██╔════╝██╔═══██╗████╗ ████║██╔══██╗██╔════╝`,
	`██║     ███████║   ██║   ███████║██║     ██║   ██║██╔████╔██║██████╔╝███████╗`,
	`██║     ██╔══██║   ██║   ██╔══██║██║     ██║   ██║██║╚██╔╝██║██╔══██╗╚════██║`,
	`╚██████╗██║  ██║   ██║   ██║  ██║╚██████╗╚██████╔╝██║ ╚═╝ ██║██████╔╝███████║`,
	`╚═════╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚══════╝`,
}

// gradient from red → orange → yellow
var titleColors = []lipgloss.Color{
	lipgloss.Color("196"), // red
	lipgloss.Color("202"), // orange-red
	lipgloss.Color("208"), // orange
	lipgloss.Color("214"), // yellow-orange
	lipgloss.Color("220"), // yellow
	lipgloss.Color("226"), // bright yellow
}

func renderTitle(width, height int) string {
	// Render logo with gradient
	var logoLines []string
	for i, line := range titleLines {
		color := titleColors[i%len(titleColors)]
		style := lipgloss.NewStyle().Foreground(color).Bold(true)
		logoLines = append(logoLines, style.Render(line))
	}
	logo := strings.Join(logoLines, "\n")

	subtitle := lipgloss.NewStyle().
		Foreground(colorGray).
		Render("⚔  A Cooperative Dungeon Crawler  ⚔")

	server := lipgloss.NewStyle().
		Foreground(colorCyan).
		Render("ssh catacombs.tolelom.xyz")

	menu := lipgloss.NewStyle().
		Foreground(colorWhite).
		Bold(true).
		Render("[Enter] Start    [Q] Quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		subtitle,
		server,
		"",
		"",
		menu,
	)

	// Center on screen
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}
```

- [ ] **Step 2: Build and test**

```bash
go build ./...
go test ./ui/ -timeout 15s
```

- [ ] **Step 3: Commit**

```bash
git add ui/title.go
git commit -m "feat: gradient title screen with centered layout"
```

---

### Task 6: Final integration test

- [ ] **Step 1: Build and run all tests**

```bash
go build ./...
go test ./... -timeout 30s
```

- [ ] **Step 2: Verify no dead code or unused imports**

```bash
go vet ./...
```
