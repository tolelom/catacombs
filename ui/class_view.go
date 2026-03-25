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
						room.Session.HardMode = ctx.HardMode
						room.Session.ApplyWeeklyMutation()
					}
					ctx.Session = room.Session
					player := entity.NewPlayer(ctx.PlayerName, selectedClass)
					player.Fingerprint = ctx.Fingerprint
					ctx.Session.AddPlayer(player)
					if ctx.Lobby != nil {
						ctx.Lobby.RegisterSession(ctx.Fingerprint, ctx.RoomCode)
					}
					ws := NewWaitingScreen()
					return ws, ws.pollWaiting()
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
	{entity.ClassWarrior, "Warrior", "HP:120 ATK:12 DEF:8  스킬: Taunt (적의 공격을 끌어옴)"},
	{entity.ClassMage, "Mage", "HP:70  ATK:20 DEF:3  스킬: Fireball (광역 피해)"},
	{entity.ClassHealer, "Healer", "HP:90  ATK:8  DEF:5  스킬: Heal (HP 30 회복)"},
	{entity.ClassRogue, "Rogue", "HP:85  ATK:15 DEF:4  스킬: Scout (주변 방 탐색)"},
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

	header := headerStyle.Render("── 직업을 선택하세요 ──")
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

	menu := "[Up/Down] 선택  [Enter] 확인"

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		list,
		menu,
	)
}
