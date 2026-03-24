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
	header := headerStyle.Render(fmt.Sprintf("── Catacombs B%d ──", floor.Number))
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
		sb.WriteString("\n")
		// Enemies
		enemyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		for i, m := range state.Monsters {
			if !m.IsDead() {
				mhpBar := renderHPBar(m.HP, m.MaxHP, 15)
				taunt := ""
				if m.TauntTarget {
					taunt = " [TAUNTED]"
				}
				marker := "  "
			if i == targetCursor {
				marker = "> "
			}
			sb.WriteString(enemyStyle.Render(fmt.Sprintf("%s[%d] %s %s %d/%d%s", marker, i, m.Name, mhpBar, m.HP, m.MaxHP, taunt)))
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
		// Actions with skill description
		actionStyle := lipgloss.NewStyle().Bold(true)
		sb.WriteString(actionStyle.Render("[1]Attack  [2]Skill  [3]Item  [4]Flee  [5]Wait  [Tab]Target"))
		sb.WriteString("\n")
		if !state.TurnDeadline.IsZero() {
			remaining := time.Until(state.TurnDeadline)
			if remaining < 0 {
				remaining = 0
			}
			timerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
			sb.WriteString(timerStyle.Render(fmt.Sprintf("  Timer: %.1fs", remaining.Seconds())))
			sb.WriteString("\n")
		}

		// Skill description per class
		skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
		for _, p := range state.Players {
			if !p.IsDead() {
				var skillDesc string
				switch p.Class {
				case entity.ClassWarrior:
					skillDesc = "Skill: Taunt - enemies attack you for 2 turns"
				case entity.ClassMage:
					skillDesc = "Skill: Fireball - AoE 0.8x dmg to all enemies"
				case entity.ClassHealer:
					skillDesc = "Skill: Heal - restore 30 HP to an ally"
				case entity.ClassRogue:
					skillDesc = "Skill: Scout - reveal neighboring rooms"
				}
				sb.WriteString(skillStyle.Render(skillDesc))
				sb.WriteString("\n")
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

	return border.Render(sb.String())
}

func renderCombatLog(log []string) string {
	if len(log) == 0 {
		return ""
	}
	logStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		PaddingLeft(1)

	var sb strings.Builder
	for _, msg := range log {
		sb.WriteString("  > " + msg + "\n")
	}
	return logStyle.Render(sb.String())
}

func renderHPBar(current, max, width int) string {
	if max == 0 {
		return ""
	}
	filled := current * width / max
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	bar := greenStyle.Render(strings.Repeat("█", filled)) +
		redStyle.Render(strings.Repeat("░", empty))
	return bar
}

func roomTypeSymbol(rt dungeon.RoomType) string {
	switch rt {
	case dungeon.RoomCombat:
		return "D"
	case dungeon.RoomTreasure:
		return "$"
	case dungeon.RoomShop:
		return "S"
	case dungeon.RoomEvent:
		return "?"
	case dungeon.RoomEmpty:
		return "."
	case dungeon.RoomBoss:
		return "B"
	default:
		return " "
	}
}
