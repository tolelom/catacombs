package ui

import (
	"github.com/tolelom/catacombs/game"
)

type screen int

const (
	screenTitle screen = iota
	screenLobby
	screenClassSelect
	screenWaiting
	screenGame
	screenShop
	screenResult
	screenHelp
	screenStats
	screenAchievements
	screenLeaderboard
	screenNickname
)

func (m Model) screenType() screen {
	switch m.currentScreen.(type) {
	case *TitleScreen:
		return screenTitle
	case *LobbyScreen:
		return screenLobby
	case *ClassSelectScreen:
		return screenClassSelect
	case *WaitingScreen:
		return screenWaiting
	case *GameScreen:
		return screenGame
	case *ShopScreen:
		return screenShop
	case *ResultScreen:
		return screenResult
	case *HelpScreen:
		return screenHelp
	case *StatsScreen:
		return screenStats
	case *AchievementsScreen:
		return screenAchievements
	case *LeaderboardScreen:
		return screenLeaderboard
	case *NicknameScreen:
		return screenNickname
	}
	return screenTitle
}

func (m Model) playerName() string {
	return m.ctx.PlayerName
}

func (m Model) roomCode() string {
	return m.ctx.RoomCode
}

func (m Model) session() *game.GameSession {
	return m.ctx.Session
}
