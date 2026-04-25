package game

import (
	"math/rand"
	"testing"
)

func testRng() *rand.Rand { return rand.New(rand.NewSource(42)) }

func TestGetRandomEvents(t *testing.T) {
	events := GetRandomEvents()
	if len(events) < 8 {
		t.Errorf("Expected at least 8 random events, got %d", len(events))
	}
	for _, e := range events {
		if len(e.Choices) < 2 {
			t.Errorf("Event %q should have at least 2 choices, got %d", e.Name, len(e.Choices))
		}
		if e.Name == "" {
			t.Error("Event name should not be empty")
		}
		if e.Description == "" {
			t.Errorf("Event %q description should not be empty", e.Name)
		}
	}
}

func TestResolveChoice(t *testing.T) {
	events := GetRandomEvents()

	// Test altar "Walk away" — always safe
	var altar RandomEvent
	for _, e := range events {
		if e.Name == "altar" {
			altar = e
			break
		}
	}
	if altar.Name == "" {
		t.Fatal("altar event not found")
	}
	// The third choice (index 2) is "Walk away" — should have no HP/gold change
	outcome := altar.Choices[2].Resolve(5, testRng())
	if outcome.HPChange != 0 || outcome.GoldChange != 0 {
		t.Errorf("Walk away should have no changes, got HP=%d Gold=%d", outcome.HPChange, outcome.GoldChange)
	}

	// Test fountain "Drink" — always heals
	var fountain RandomEvent
	for _, e := range events {
		if e.Name == "fountain" {
			fountain = e
			break
		}
	}
	if fountain.Name == "" {
		t.Fatal("fountain event not found")
	}
	drinkOutcome := fountain.Choices[0].Resolve(10, testRng())
	if drinkOutcome.HPChange <= 0 {
		t.Errorf("Drinking from fountain should heal, got HP=%d", drinkOutcome.HPChange)
	}

	// Test shrine "Kneel and pray" — always heals
	var shrine RandomEvent
	for _, e := range events {
		if e.Name == "shrine" {
			shrine = e
			break
		}
	}
	if shrine.Name == "" {
		t.Fatal("shrine event not found")
	}
	prayOutcome := shrine.Choices[0].Resolve(5, testRng())
	if prayOutcome.HPChange <= 0 {
		t.Errorf("Praying at shrine should heal, got HP=%d", prayOutcome.HPChange)
	}

	// Test chest "Smash it open" — always gives item
	var chest RandomEvent
	for _, e := range events {
		if e.Name == "chest" {
			chest = e
			break
		}
	}
	if chest.Name == "" {
		t.Fatal("chest event not found")
	}
	smashOutcome := chest.Choices[1].Resolve(5, testRng())
	if !smashOutcome.ItemDrop {
		t.Error("Smashing chest should always give an item drop")
	}
}

func TestPickRandomEvent(t *testing.T) {
	event := PickRandomEvent(testRng())
	if event.Name == "" {
		t.Error("PickRandomEvent should return a valid event")
	}
	if len(event.Choices) < 2 {
		t.Errorf("Picked event should have at least 2 choices, got %d", len(event.Choices))
	}
}
