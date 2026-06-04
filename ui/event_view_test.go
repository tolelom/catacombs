package ui

import (
	"strings"
	"testing"

	"github.com/tolelom/catacombs/game"
)

// TestEventScreenViewIncludesChoices renders an EventScreen and asserts the
// choice labels appear in the output.
func TestEventScreenViewIncludesChoices(t *testing.T) {
	state := game.GameState{
		Phase: game.PhaseEvent,
		PendingEvent: &game.PendingEventView{
			Name:        "altar",
			Description: "이상한 에너지로 빛나는 고대 제단을 발견했습니다.",
			Choices: []game.EventChoiceView{
				{Label: "제단에서 기도하기", Tags: []game.ChoiceTag{game.TagHeal, game.TagHarm}},
				{Label: "골드 바치기", Tags: []game.ChoiceTag{game.TagGoldCost}},
				{Label: "그냥 지나가기", Tags: []game.ChoiceTag{game.TagSkip}},
			},
		},
	}
	es := NewEventScreen(state)
	ctx := &Context{Width: 80, Height: 24}
	out := es.View(ctx)
	for _, want := range []string{"제단에서 기도하기", "골드 바치기", "그냥 지나가기"} {
		if !strings.Contains(out, want) {
			t.Errorf("View should contain %q", want)
		}
	}
}

// TestEventScreenPerPlayerWording — loot picks (PerPlayer) swap the party-vote
// wording for individual-choice wording.
func TestEventScreenPerPlayerWording(t *testing.T) {
	state := game.GameState{
		Phase: game.PhaseEvent,
		PendingEvent: &game.PendingEventView{
			Name:      "treasure",
			PerPlayer: true,
			Choices: []game.EventChoiceView{
				{Label: "Iron Sword (ATK+5)", Tags: []game.ChoiceTag{game.TagItem}},
				{Label: "Iron Shield (DEF+3)", Tags: []game.ChoiceTag{game.TagItem}},
			},
		},
	}
	es := NewEventScreen(state)
	ctx := &Context{Width: 80, Height: 24}
	out := es.View(ctx)
	if !strings.Contains(out, "보물 발견!") {
		t.Error("PerPlayer view should use the treasure header")
	}
	if !strings.Contains(out, "선택 현황:") {
		t.Error("PerPlayer view should label picks, not votes")
	}
	if strings.Contains(out, "투표 현황:") {
		t.Error("PerPlayer view must not use party-vote wording")
	}
}
