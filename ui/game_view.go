package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

func renderGame(state game.GameState, width, height int, targetCursor int, moveCursor int, chatting bool, chatInput string) string {
	mapView := renderMap(state.Floor)
	hudView := renderHUD(state, targetCursor, moveCursor)
	logView := renderCombatLog(state.CombatLog)

	if chatting {
		chatStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
		chatView := chatStyle.Render(fmt.Sprintf("> %s_", chatInput))
		return lipgloss.JoinVertical(lipgloss.Left, mapView, hudView, logView, chatView)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		mapView,
		hudView,
		logView,
	)
}

func renderMap(floor *dungeon.Floor) string {
	if floor == nil {
		return ""
	}
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	// Count explored rooms
	explored := 0
	for _, r := range floor.Rooms {
		if r.Visited || r.Cleared {
			explored++
		}
	}
	total := len(floor.Rooms)

	header := headerStyle.Render(fmt.Sprintf("── Catacombs B%d ── %d/%d Rooms ──", floor.Number, explored, total))
	return header + "\n" + dungeon.RenderFloor(floor, floor.CurrentRoom, true)
}

func renderHUD(state game.GameState, targetCursor int, moveCursor int) string {
	var sb strings.Builder
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1)

	// Player info
	for _, p := range state.Players {
		hpBar := renderHPBar(p.HP, p.MaxHP, 20)
		status := ""
		if p.IsDead() {
			status = " [DEAD]"
		}
		sb.WriteString(fmt.Sprintf("%s (%s) %s %d/%d%s  Gold: %d",
			p.Name, p.Class, hpBar, p.HP, p.MaxHP, status, p.Gold))

		// Show inventory count
		itemCount := len(p.Inventory)
		relicCount := len(p.Relics)
		if itemCount > 0 || relicCount > 0 {
			sb.WriteString(fmt.Sprintf("  Items:%d Relics:%d", itemCount, relicCount))
		}
		sb.WriteString("\n")
	}

	if state.Phase == game.PhaseCombat {
		// Two-panel layout: PARTY | ENEMIES
		partyContent := renderPartyPanel(state.Players, state.SubmittedActions)
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

		// Skill description for first alive player only
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
				skillDesc += fmt.Sprintf(" (%d uses left)", p.SkillUses)
				sb.WriteString(styleSystem.Render(skillDesc))
				sb.WriteString("\n")
				break
			}
		}
	} else if state.Phase == game.PhaseExploring {
		if state.Floor != nil && state.Floor.CurrentRoom >= 0 && state.Floor.CurrentRoom < len(state.Floor.Rooms) {
			current := state.Floor.Rooms[state.Floor.CurrentRoom]
			if len(current.Neighbors) > 0 {
				sb.WriteString("\n")
				selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
				normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
				for i, n := range current.Neighbors {
					if n >= 0 && n < len(state.Floor.Rooms) {
						r := state.Floor.Rooms[n]
						status := r.Type.String()
						if r.Cleared {
							status = "Cleared"
						}
						marker := "  "
						style := normalStyle
						if i == moveCursor {
							marker = "> "
							style = selectedStyle
						}
						sb.WriteString(style.Render(fmt.Sprintf("%sRoom %d: %s", marker, n, status)))
						sb.WriteString("\n")
					}
				}
			}
		}
		sb.WriteString("[Up/Down] Select  [Enter] Move  [Q] Quit")
	}

	if state.Phase == game.PhaseCombat {
		return sb.String()
	}
	return border.Render(sb.String())
}

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

func renderPartyPanel(players []*entity.Player, submittedActions map[string]string) string {
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

		if len(p.Effects) > 0 {
			var effects []string
			for _, e := range p.Effects {
				switch e.Type {
				case entity.StatusPoison:
					effects = append(effects, styleHeal.Render(fmt.Sprintf("☠Poison(%dt)", e.Duration)))
				case entity.StatusBurn:
					effects = append(effects, styleDamage.Render(fmt.Sprintf("🔥Burn(%dt)", e.Duration)))
				case entity.StatusFreeze:
					effects = append(effects, styleFlee.Render(fmt.Sprintf("❄Freeze(%dt)", e.Duration)))
				}
			}
			sb.WriteString("    " + strings.Join(effects, " ") + "\n")
		}

		sb.WriteString(fmt.Sprintf("    ATK:%-3d DEF:%-3d  ", p.EffectiveATK(), p.EffectiveDEF()))
		sb.WriteString(styleGold.Render(fmt.Sprintf("G:%d", p.Gold)))
		sb.WriteString("\n")

		if action, ok := submittedActions[p.Fingerprint]; ok {
			sb.WriteString(styleHeal.Render(fmt.Sprintf("    ✓ %s", action)))
			sb.WriteString("\n")
		} else if !p.IsOut() {
			sb.WriteString(styleSystem.Render("    ... Waiting"))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

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
			sb.WriteString(styleEnemy.Render("  "+line) + "\n")
		}

		// Name + HP
		marker := "  "
		if i == targetCursor {
			marker = "> "
		}
		hpBar := renderHPBar(m.HP, m.MaxHP, 12)
		taunt := ""
		if m.TauntTarget {
			taunt = styleStatus.Render(fmt.Sprintf(" [TAUNTED %dt]", m.TauntTurns))
		}
		sb.WriteString(fmt.Sprintf("  %s[%d] %s %s %d/%d%s\n\n",
			marker, i, styleEnemy.Render(m.Name), hpBar, m.HP, m.MaxHP, taunt))
	}
	return sb.String()
}
