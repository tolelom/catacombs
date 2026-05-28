package game

import "testing"

// TestEventChoiceTagsPresent enforces every choice in every event has at
// least one Tag — UI relies on Tags to render category icons.
func TestEventChoiceTagsPresent(t *testing.T) {
	for _, ev := range GetRandomEvents() {
		for i, ch := range ev.Choices {
			if len(ch.Tags) == 0 {
				t.Errorf("event %q choice %d (%q) has no Tags", ev.Name, i, ch.Label)
			}
		}
	}
}
