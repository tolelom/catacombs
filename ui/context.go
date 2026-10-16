package ui

import (
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

type Context struct {
	Width       int
	Height      int
	Fingerprint string
	PlayerName  string

	Lobby    *game.Lobby
	Store    *store.DB
	Session  *game.GameSession
	RoomCode string
	HardMode bool

	tickGen int
}
