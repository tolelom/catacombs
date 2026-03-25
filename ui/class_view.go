package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

// ClassSelectScreen lets the player choose a class before entering the game.
type ClassSelectScreen struct {
	cursor int
}

func NewClassSelectScreen() *ClassSelectScreen {
	return &ClassSelectScreen{}
}

func (s *ClassSelectScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isUp(key) {
			if s.cursor > 0 {
				s.cursor--
			}
		} else if isDown(key) {
			if s.cursor < len(classOptions)-1 {
				s.cursor++
			}
		} else if isEnter(key) {
			if ctx.Lobby != nil {
				selectedClass := classOptions[s.cursor].class
				ctx.Lobby.SetPlayerClass(ctx.RoomCode, ctx.Fingerprint, selectedClass.String())
				room := ctx.Lobby.GetRoom(ctx.RoomCode)
				if room != nil {
					if room.Session == nil {
						room.Session = game.NewGameSession(ctx.Lobby.Cfg())
					}
					ctx.Session = room.Session
					player := entity.NewPlayer(ctx.PlayerName, selectedClass)
					player.Fingerprint = ctx.Fingerprint
					ctx.Session.AddPlayer(player)
					if ctx.Lobby != nil {
						ctx.Lobby.RegisterSession(ctx.Fingerprint, ctx.RoomCode)
					}
					ctx.Session.StartGame()
					ctx.Lobby.StartRoom(ctx.RoomCode)
					gs := NewGameScreen()
					gs.gameState = ctx.Session.GetState()
					return gs, gs.pollState()
				}
			}
		}
	}
	return s, nil
}

func (s *ClassSelectScreen) View(ctx *Context) string {
	state := classSelectState{cursor: s.cursor}
	return renderClassSelect(state, ctx.Width, ctx.Height)
}

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
