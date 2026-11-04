package web

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/throttle"
)

type AdminStats struct {
	OnlinePlayers int     `json:"online_players"`
	ActiveRooms   int     `json:"active_rooms"`
	TodayRuns     int     `json:"today_runs"`
	AvgFloorReach float64 `json:"avg_floor_reached"`
	UptimeSeconds int64   `json:"uptime_seconds"`
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.LastIndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[i+1:])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func AdminHandler(creds config.AdminConfig, lobby *game.Lobby, db *store.DB, startTime time.Time) http.Handler {
	throttle := throttle.New(time.Minute, 5, 5*time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if blocked, remaining := throttle.Blocked(ip); blocked {
			retry := int(remaining.Seconds())
			if retry < 1 {
				retry = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		if !checkAuth(r, creds) {
			throttle.Fail(ip)
			w.Header().Set("WWW-Authenticate", `Basic realm="Catacombs Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		throttle.Reset(ip)

		todayRuns, err1 := db.GetTodayRunCount()
		avgFloor, err2 := db.GetTodayAvgFloor()
		if err1 != nil || err2 != nil {
			http.Error(w, "stats unavailable", http.StatusServiceUnavailable)
			return
		}

		stats := AdminStats{
			OnlinePlayers: len(lobby.ListOnline()),
			ActiveRooms:   len(lobby.ListRooms()),
			TodayRuns:     todayRuns,
			AvgFloorReach: avgFloor,
			UptimeSeconds: int64(time.Since(startTime).Seconds()),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error("admin stats encode failed", "error", err)
		}
	})
}

func checkAuth(r *http.Request, cfg config.AdminConfig) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	uOK := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1
	pOK := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) == 1
	return uOK && pOK
}
