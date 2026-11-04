// Package throttle limits repeated authentication failures per key
// (an IP for the admin endpoint, a nickname for game logins).
package throttle

import (
	"sync"
	"time"
)

type Limiter struct {
	mu       sync.Mutex
	fails    map[string][]time.Time
	window   time.Duration
	max      int
	cooldown time.Duration
}

func New(window time.Duration, max int, cooldown time.Duration) *Limiter {
	return &Limiter{
		fails:    make(map[string][]time.Time),
		window:   window,
		max:      max,
		cooldown: cooldown,
	}
}

// Blocked reports whether key has reached max failures within the window,
// and roughly how long the caller should wait.
func (l *Limiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	keep := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	l.fails[key] = keep
	if len(keep) >= l.max {
		return true, l.cooldown - now.Sub(keep[0])
	}
	return false, 0
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key] = append(l.fails[key], time.Now())
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
