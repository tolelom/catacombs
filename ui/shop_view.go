package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

func itemTypeLabel(item entity.Item) string {
	switch item.Type {
	case entity.ItemWeapon:
		return fmt.Sprintf("[ATK+%d]", item.Bonus)
	case entity.ItemArmor:
		return fmt.Sprintf("[DEF+%d]", item.Bonus)
	case entity.ItemConsumable:
		return fmt.Sprintf("[HP+%d]", item.Bonus)
	default:
		return fmt.Sprintf("[+%d]", item.Bonus)
	}
}

func renderShop(state game.GameState, width, height int, shopMsg string) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
	goldStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("220"))
	msgStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	header := headerStyle.Render("── Shop ──")

	// Show current player's gold
	goldLine := ""
	for _, p := range state.Players {
		inventoryCount := len(p.Inventory)
		goldLine += goldStyle.Render(fmt.Sprintf("  %s — Gold: %d  Items: %d/10", p.Name, p.Gold, inventoryCount))
		goldLine += "\n"
	}

	items := ""
	for i, item := range state.ShopItems {
		label := itemTypeLabel(item)
		items += fmt.Sprintf("  [%d] %s %s — %d gold\n", i+1, item.Name, label, item.Price)
	}

	menu := "[1-3] Buy  [Q] Leave Shop"

	parts := []string{header, "", goldLine, items, "", menu}
	if shopMsg != "" {
		parts = append(parts, "", msgStyle.Render(shopMsg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
