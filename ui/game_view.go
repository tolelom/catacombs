package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/game"
)

func renderGame(state game.GameState, width, height int) string {
	mapView := renderMap(state.Floor)
	hudView := renderHUD(state)

	return lipgloss.JoinVertical(lipgloss.Left,
		mapView,
		hudView,
	)
}

func renderMap(floor *dungeon.Floor) string {
	if floor == nil {
		return ""
	}

	var sb strings.Builder
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	sb.WriteString(headerStyle.Render(fmt.Sprintf("── Catacombs B%d ──", floor.Number)))
	sb.WriteString("\n\n")

	roomStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	hiddenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	for i, room := range floor.Rooms {
		vis := dungeon.GetRoomVisibility(floor, i)
		symbol := roomTypeSymbol(room.Type)
		label := fmt.Sprintf("[%d] %s %s", i, symbol, room.Type.String())

		if i == floor.CurrentRoom {
			label = ">> " + label + " <<"
		}

		switch vis {
		case dungeon.Visible:
			sb.WriteString(roomStyle.Render(label))
		case dungeon.Visited:
			sb.WriteString(dimStyle.Render(label))
		case dungeon.Hidden:
			sb.WriteString(hiddenStyle.Render("[?] ???"))
		}

		// Show connections
		for _, n := range room.Neighbors {
			if n > i {
				sb.WriteString(" ─── ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderHUD(state game.GameState) string {
	var sb strings.Builder
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1)

	for _, p := range state.Players {
		hpBar := renderHPBar(p.HP, p.MaxHP, 20)
		status := ""
		if p.IsDead() {
			status = " [DEAD]"
		}
		sb.WriteString(fmt.Sprintf("%s (%s) %s %d/%d%s  Gold: %d\n",
			p.Name, p.Class, hpBar, p.HP, p.MaxHP, status, p.Gold))
	}

	if state.Phase == game.PhaseCombat {
		sb.WriteString("\n")
		for i, m := range state.Monsters {
			if !m.IsDead() {
				mhpBar := renderHPBar(m.HP, m.MaxHP, 15)
				sb.WriteString(fmt.Sprintf("  [%d] %s %s %d/%d\n", i, m.Name, mhpBar, m.HP, m.MaxHP))
			}
		}
		sb.WriteString("\n[1]Attack [2]Skill [3]Item [4]Flee [5]Wait")
	} else if state.Phase == game.PhaseExploring {
		sb.WriteString("\nChoose a room to enter (number) or [Q] quit")
	}

	return border.Render(sb.String())
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
