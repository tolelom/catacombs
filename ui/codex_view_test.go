package ui

import (
	"testing"

	"github.com/tolelom/catacombs/game"
)

// TestAllEventsMatchesGameEventPool — the codex event list must stay in sync
// with game.GetRandomEvents: codex completion counts len(allEvents) and
// entries are recorded by event name, so a divergence silently makes the
// codex uncompletable (or under-counted).
func TestAllEventsMatchesGameEventPool(t *testing.T) {
	pool := game.GetRandomEvents()
	if len(pool) != len(allEvents) {
		t.Errorf("allEvents has %d entries but game pool has %d", len(allEvents), len(pool))
	}
	inCodex := make(map[string]bool, len(allEvents))
	for _, name := range allEvents {
		inCodex[name] = true
	}
	for _, ev := range pool {
		if !inCodex[ev.Name] {
			t.Errorf("event %q missing from codex allEvents", ev.Name)
		}
	}
	// Every codex event must have lore.
	for _, name := range allEvents {
		if eventLore(name) == "" {
			t.Errorf("event %q has no lore text", name)
		}
	}
}
