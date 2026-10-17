package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/game"
)

// 결과 화면에서 [n]으로 NG+를 시작할 때의 동작을 고정한다.
// 저주는 PickCurse(nil)이 전역 rand로 뽑으므로 값 대신 "목록 안의 저주가 하나 붙는다"는 구조만 확인한다.
// 이 경로는 세션 RNG를 쓰지 않고, 주간 변이는 날짜로 정해진다.

func ngPlusContext(t *testing.T) *Context {
	t.Helper()
	cfg, _ := config.Load("")
	return &Context{Lobby: game.NewLobby(cfg), Fingerprint: "fp-hero", PlayerName: "Hero"}
}

func pressN(s *ResultScreen, ctx *Context) Screen {
	next, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, ctx)
	return next
}

func assertNGPlusSession(t *testing.T, ctx *Context, wantLevel int, wantRoomName string) {
	t.Helper()
	if ctx.RoomCode == "" || ctx.Session == nil {
		t.Fatalf("NG+ should join a new room with a session: code=%q session=%v", ctx.RoomCode, ctx.Session)
	}
	if ctx.Session.NGPlusLevel != wantLevel {
		t.Fatalf("NGPlusLevel = %d, want %d", ctx.Session.NGPlusLevel, wantLevel)
	}
	curse := ctx.Session.ActiveCurse
	if curse == nil {
		t.Fatal("NG+ session must carry a curse")
	}
	known := false
	for _, c := range game.AvailableCurses {
		if c == *curse {
			known = true
		}
	}
	if !known {
		t.Fatalf("curse %+v is not one of AvailableCurses", *curse)
	}
	weekly := game.GetWeeklyMutation()
	if ctx.Session.ActiveMutation == nil || *ctx.Session.ActiveMutation != weekly {
		t.Fatalf("ActiveMutation = %v, want this week's %v", ctx.Session.ActiveMutation, weekly)
	}
	room, ok := ctx.Lobby.RoomSnapshot(ctx.RoomCode)
	if !ok {
		t.Fatalf("room %s not found", ctx.RoomCode)
	}
	if room.Name != wantRoomName || room.Session != ctx.Session {
		t.Fatalf("room name=%q session match=%v, want name %q", room.Name, room.Session == ctx.Session, wantRoomName)
	}
	if len(room.Players) != 1 || room.Players[0].Fingerprint != "fp-hero" {
		t.Fatalf("room players = %+v, want only the hero", room.Players)
	}
}

func TestPinNGPlusFromVictory(t *testing.T) {
	ctx := ngPlusContext(t)
	previous := game.NewGameSession(ctx.Lobby.Cfg())
	previous.NGPlusLevel = 2
	ctx.Session = previous

	next := pressN(NewResultScreen(game.GameState{Victory: true}, true), ctx)
	if _, ok := next.(*ClassSelectScreen); !ok {
		t.Fatalf("next screen = %T, want *ClassSelectScreen", next)
	}
	if ctx.Session == previous {
		t.Fatal("NG+ must start a fresh session")
	}
	assertNGPlusSession(t, ctx, 3, "Hero NG+3")
}

func TestPinNGPlusWithoutPreviousSession(t *testing.T) {
	ctx := ngPlusContext(t)
	next := pressN(NewResultScreen(game.GameState{Victory: true}, true), ctx)
	if _, ok := next.(*ClassSelectScreen); !ok {
		t.Fatalf("next screen = %T, want *ClassSelectScreen", next)
	}
	assertNGPlusSession(t, ctx, 1, "Hero NG+1")
}

func TestPinNGPlusNeedsVictory(t *testing.T) {
	ctx := ngPlusContext(t)
	screen := NewResultScreen(game.GameState{Victory: false}, true)
	if next := pressN(screen, ctx); next != Screen(screen) {
		t.Fatalf("without victory [n] must keep the result screen, got %T", next)
	}
	if ctx.RoomCode != "" || ctx.Session != nil {
		t.Fatalf("no room should be created: code=%q", ctx.RoomCode)
	}
}

func TestPinNGPlusRoomQuotaError(t *testing.T) {
	ctx := ngPlusContext(t)
	if _, err := ctx.Lobby.CreateRoom("fp-hero", "already waiting"); err != nil {
		t.Fatal(err)
	}
	next := pressN(NewResultScreen(game.GameState{Victory: true}, true), ctx)
	lobby, ok := next.(*LobbyScreen)
	if !ok {
		t.Fatalf("next screen = %T, want *LobbyScreen", next)
	}
	if lobby.errMsg != game.ErrOwnerHasWaitingRoom.Error() {
		t.Fatalf("errMsg = %q, want %q", lobby.errMsg, game.ErrOwnerHasWaitingRoom.Error())
	}
	if ctx.RoomCode != "" || ctx.Session != nil {
		t.Fatalf("failed NG+ must leave no room or session: code=%q", ctx.RoomCode)
	}
}
