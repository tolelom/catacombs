package ui

import (
	"fmt"
	"strings"

	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

func renderResult(state game.GameState, rankings []store.RunRecord) string {
	var sb strings.Builder

	// Title
	if state.Victory {
		sb.WriteString(styleHeal.Render("  ✦ VICTORY ✦  ") + "\n\n")
		sb.WriteString(styleSystem.Render("  You conquered the Catacombs!") + "\n\n")
	} else {
		sb.WriteString(styleDamage.Render("  ✦ DEFEAT ✦  ") + "\n\n")
		sb.WriteString(styleSystem.Render(fmt.Sprintf("  Fallen on floor B%d", state.FloorNum)) + "\n\n")
	}

	// Player summary
	sb.WriteString(styleHeader.Render("── Party Summary ──") + "\n\n")
	totalGold := 0
	for _, p := range state.Players {
		status := styleHeal.Render("Alive")
		if p.IsDead() {
			status = styleDamage.Render("Dead")
		}
		sb.WriteString(fmt.Sprintf("  %s (%s)  %s  Gold: %d  Items: %d  Relics: %d\n",
			stylePlayer.Render(p.Name), p.Class, status, p.Gold, len(p.Inventory), len(p.Relics)))
		totalGold += p.Gold
	}
	sb.WriteString(fmt.Sprintf("\n  Total Gold: %s\n", styleGold.Render(fmt.Sprintf("%d", totalGold))))

	// Rankings
	if len(rankings) > 0 {
		sb.WriteString("\n" + styleHeader.Render("── Top Runs ──") + "\n\n")
		for i, r := range rankings {
			medal := "  "
			switch i {
			case 0:
				medal = styleGold.Render("🥇")
			case 1:
				medal = styleSystem.Render("🥈")
			case 2:
				medal = styleGold.Render("🥉")
			}
			sb.WriteString(fmt.Sprintf("  %s %s  Floor B%d  Score: %d\n", medal, r.Player, r.Floor, r.Score))
		}
	}

	sb.WriteString("\n" + styleAction.Render("  [Enter] Return to Lobby") + "\n")

	return sb.String()
}
