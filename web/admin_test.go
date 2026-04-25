package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

func TestAdminEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := store.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	cfg, _ := config.Load("")
	cfg.Admin = config.AdminConfig{Username: "admin", Password: "secret"}
	lobby := game.NewLobby(cfg)

	handler := AdminHandler(lobby, db, time.Now())

	// Test without auth → 401
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Test with wrong auth → 401
	req = httptest.NewRequest("GET", "/admin", nil)
	req.SetBasicAuth("admin", "wrong")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Test with correct auth → 200 + JSON
	req = httptest.NewRequest("GET", "/admin", nil)
	req.SetBasicAuth("admin", "secret")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats AdminStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if stats.OnlinePlayers != 0 {
		t.Fatalf("expected 0 online, got %d", stats.OnlinePlayers)
	}
}

// TestAdminThrottlesRepeatedFailures regresses the brute-force-deterrent
// rate limit: more than 5 failed Basic Auth attempts from the same IP
// inside the rolling window must return 429 Too Many Requests.
func TestAdminThrottlesRepeatedFailures(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := store.Open(filepath.Join(tmpDir, "test_throttle.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	cfg, _ := config.Load("")
	cfg.Admin = config.AdminConfig{Username: "admin", Password: "right"}
	lobby := game.NewLobby(cfg)
	handler := AdminHandler(lobby, db, time.Now())

	attempt := func(user, pass, fromIP string) int {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.SetBasicAuth(user, pass)
		req.RemoteAddr = fromIP + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	// First five wrong attempts → 401 each.
	for i := 0; i < 5; i++ {
		if got := attempt("admin", "nope", "10.0.0.1"); got != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, got)
		}
	}
	// Sixth should be throttled → 429.
	if got := attempt("admin", "nope", "10.0.0.1"); got != http.StatusTooManyRequests {
		t.Errorf("expected 429 after 5 failures, got %d", got)
	}
	// A different IP should still be free to try.
	if got := attempt("admin", "right", "10.0.0.2"); got != http.StatusOK {
		t.Errorf("unrelated IP should not be throttled, got %d", got)
	}
}

// TestAdminThrottleResetsOnSuccess ensures a successful auth wipes the
// per-IP failure counter so a legitimate operator with a typo isn't
// locked out for five minutes.
func TestAdminThrottleResetsOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := store.Open(filepath.Join(tmpDir, "test_throttle_reset.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	cfg, _ := config.Load("")
	cfg.Admin = config.AdminConfig{Username: "admin", Password: "right"}
	lobby := game.NewLobby(cfg)
	handler := AdminHandler(lobby, db, time.Now())

	attempt := func(pass, fromIP string) int {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.SetBasicAuth("admin", pass)
		req.RemoteAddr = fromIP + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	// Four wrong attempts.
	for i := 0; i < 4; i++ {
		_ = attempt("nope", "10.0.0.3")
	}
	// One success — should clear the counter.
	if got := attempt("right", "10.0.0.3"); got != http.StatusOK {
		t.Fatalf("expected success, got %d", got)
	}
	// Now four more wrongs should still fall under the threshold.
	for i := 0; i < 4; i++ {
		if got := attempt("nope", "10.0.0.3"); got != http.StatusUnauthorized {
			t.Errorf("post-success attempt %d: got %d, want 401", i+1, got)
		}
	}
}
