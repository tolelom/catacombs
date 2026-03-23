package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/entity"
)

type classSelectState struct {
	cursor int
}

var classOptions = []struct {
	class entity.Class
	name  string
	desc  string
}{
	{entity.ClassWarrior, "Warrior", "HP:120 ATK:12 DEF:8  Skill: Taunt (draw enemy fire)"},
	{entity.ClassMage, "Mage", "HP:70  ATK:20 DEF:3  Skill: Fireball (AoE damage)"},
	{entity.ClassHealer, "Healer", "HP:90  ATK:8  DEF:5  Skill: Heal (restore 30 HP)"},
	{entity.ClassRogue, "Rogue", "HP:85  ATK:15 DEF:4  Skill: Scout (reveal rooms)"},
}

func renderClassSelect(state classSelectState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	header := headerStyle.Render("── Choose Your Class ──")
	list := ""
	for i, opt := range classOptions {
		marker := "  "
		style := normalStyle
		if i == state.cursor {
			marker = "> "
			style = selectedStyle
		}
		list += fmt.Sprintf("%s%s\n   %s\n\n",
			marker, style.Render(opt.name), descStyle.Render(opt.desc))
	}

	menu := "[Up/Down] Select  [Enter] Confirm"

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		list,
		menu,
	)
}
