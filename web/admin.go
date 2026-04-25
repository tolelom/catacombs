package web

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// AdminStats is the JSON response for the /admin endpoint.
type AdminStats struct {
	OnlinePlayers int     `json:"online_players"`
	ActiveRooms   int     `json:"active_rooms"`
	TodayRuns     int     `json:"today_runs"`
	AvgFloorReach float64 `json:"avg_floor_reached"`
	UptimeSeconds int64   `json:"uptime_seconds"`
}

// AdminHandler returns an http.Handler for the /admin stats endpoint.
// It requires Basic Auth using credentials from config.
func AdminHandler(lobby *game.Lobby, db *store.DB, startTime time.Time) http.Handler {
	cfg := lobby.Cfg()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r, cfg.Admin) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Catacombs Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		todayRuns, _ := db.GetTodayRunCount()
		avgFloor, _ := db.GetTodayAvgFloor()

		stats := AdminStats{
			OnlinePlayers: len(lobby.ListOnline()),
			ActiveRooms:   len(lobby.ListRooms()),
			TodayRuns:     todayRuns,
			AvgFloorReach: avgFloor,
			UptimeSeconds: int64(time.Since(startTime).Seconds()),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats)
	})
}

func checkAuth(r *http.Request, cfg config.AdminConfig) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	// Constant-time compare to defeat timing-side-channel guesses against
	// the admin credential.
	uOK := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1
	pOK := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) == 1
	return uOK && pOK
}
