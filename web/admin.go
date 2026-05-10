package web

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

// authThrottle rate-limits admin login failures to deter Basic-Auth
// brute force. A successful auth resets the per-IP counter.
type authThrottle struct {
	mu      sync.Mutex
	fails   map[string][]time.Time
	window  time.Duration
	max     int
	cooldown time.Duration
}

func newAuthThrottle() *authThrottle {
	return &authThrottle{
		fails:    make(map[string][]time.Time),
		window:   1 * time.Minute,
		max:      5,
		cooldown: 5 * time.Minute,
	}
}

// blocked reports whether the given IP has exceeded the failure threshold
// inside the current window. Returns the time until the block lifts.
func (a *authThrottle) blocked(ip string) (bool, time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-a.window)
	keep := a.fails[ip][:0]
	for _, t := range a.fails[ip] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	a.fails[ip] = keep
	if len(keep) >= a.max {
		oldest := keep[0]
		return true, a.cooldown - now.Sub(oldest)
	}
	return false, 0
}

func (a *authThrottle) recordFail(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fails[ip] = append(a.fails[ip], time.Now())
}

func (a *authThrottle) recordSuccess(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.fails, ip)
}

// clientIP returns the IP address used for throttling. X-Forwarded-For is a
// list of intermediaries appended in order (client, proxy1, proxy2, ...);
// when a request reaches us through Caddy the LAST entry is the hop Caddy
// added — i.e., the actual client as seen by Caddy. Reading the FIRST entry
// would let any attacker bypass throttling by setting their own
// X-Forwarded-For header. Falls back to RemoteAddr if no XFF is set.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the rightmost (last) entry — set by our trusted proxy.
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

// AdminHandler returns an http.Handler for the /admin stats endpoint.
// It requires Basic Auth using credentials from config and throttles
// repeated failures from the same IP.
func AdminHandler(lobby *game.Lobby, db *store.DB, startTime time.Time) http.Handler {
	cfg := lobby.Cfg()
	throttle := newAuthThrottle()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if blocked, remaining := throttle.blocked(ip); blocked {
			retry := int(remaining.Seconds())
			if retry < 1 {
				retry = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		if !checkAuth(r, cfg.Admin) {
			throttle.recordFail(ip)
			w.Header().Set("WWW-Authenticate", `Basic realm="Catacombs Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		throttle.recordSuccess(ip)

		todayRuns, err1 := db.GetTodayRunCount()
		avgFloor, err2 := db.GetTodayAvgFloor()
		if err1 != nil || err2 != nil {
			// Don't show synthesized zeros that an operator would mistake
			// for "everything is fine"; surface the failure so the dashboard
			// shows red.
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
