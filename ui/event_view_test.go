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
