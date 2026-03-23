package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type lobbyState struct {
	rooms     []roomInfo
	input     string
	cursor    int
	creating  bool
	roomName  string
	joining   bool
	codeInput string
}

type roomInfo struct {
	Code    string
	Name    string
	Players int
	Status  string
}

func renderLobby(state lobbyState, width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	roomStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	header := headerStyle.Render("── Lobby ──")
	menu := "[C] Create Room  [J] Join by Code  [Up/Down] Select  [Enter] Join  [Q] Back"

	roomList := ""
	for i, r := range state.rooms {
		marker := "  "
		if i == state.cursor {
			marker = "> "
		}
		roomList += fmt.Sprintf("%s%s [%s] (%d/4) %s\n",
			marker, r.Name, r.Code, r.Players, r.Status)
	}
	if roomList == "" {
		roomList = "  No rooms available. Create one!"
	}
	if state.joining {
		inputStr := state.codeInput + strings.Repeat("_", 4-len(state.codeInput))
		roomList += fmt.Sprintf("\n  Enter room code: [%s]  (Esc to cancel)\n", inputStr)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		roomStyle.Render(roomList),
		"",
		menu,
	)
}
