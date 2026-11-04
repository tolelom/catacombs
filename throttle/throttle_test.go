package throttle

import (
	"testing"
	"time"
)

func TestBlocksAfterMaxFailures(t *testing.T) {
	l := New(time.Minute, 3, 5*time.Minute)
	for i := 0; i < 3; i++ {
		if blocked, _ := l.Blocked("alice"); blocked {
			t.Fatalf("blocked after %d failures, want 3", i)
		}
		l.Fail("alice")
	}
	blocked, retry := l.Blocked("alice")
	if !blocked || retry <= 0 {
		t.Fatalf("Blocked = %v, %v; want true with positive retry", blocked, retry)
	}
	if blocked, _ := l.Blocked("bob"); blocked {
		t.Fatal("other keys must not be blocked")
	}
}

func TestResetClearsFailures(t *testing.T) {
	l := New(time.Minute, 2, 5*time.Minute)
	l.Fail("alice")
	l.Fail("alice")
	l.Reset("alice")
	if blocked, _ := l.Blocked("alice"); blocked {
		t.Fatal("Reset must clear failures")
	}
}
