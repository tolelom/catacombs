package ui

import (
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/throttle"
)

type Context struct {
	Width       int
	Height      int
	Fingerprint string
	PlayerName  string

	Lobby *game.Lobby
	Store *store.DB
	// LoginLimiter is shared by all sessions; nil disables login throttling.
	LoginLimiter *throttle.Limiter
	Session      *game.GameSession
	RoomCode     string
	HardMode     bool

	tickGen int
}
