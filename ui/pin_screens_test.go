package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"
	"github.com/tolelom/catacombs/config"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// Pins the codex, lobby, leaderboard, help and result screens before they are
// split into steps. Ascii pins layout and text, TrueColor pins color codes.
// The daily leaderboard header prints today's date, so it is replaced with a
// placeholder. Regenerate: go test ./ui/ -run Pin -update

func renderScreensForPin() string {
	var b strings.Builder
	section := func(label, body string) {
		fmt.Fprintf(&b, "==== %s\n%s\n", label, body)
	}

	codex := &CodexScreen{
		codex: store.Codex{
			Monsters: map[string]bool{allMonsters[0]: true, allMonsters[2]: true, "Unlisted Beast": true},
			Items:    map[string]bool{allItems[1]: true},
			Events:   map[string]bool{},
		},
		bossKills: map[string]int64{bossNames[0]: 3, bossNames[2]: 1},
	}
	for tab := 0; tab < 4; tab++ {
		codex.tab = tab
		section(fmt.Sprintf("codex tab %d", tab), codex.View(&Context{Width: 100, Height: 60}))
	}

	rooms := []roomInfo{
		{Code: "ABCD", Name: "Alice의 방", Status: "대기중", Players: []playerInfo{{Name: "Alice", Class: "Warrior", Ready: true}, {Name: "Bob"}}},
		{Code: "EFGH", Name: "Carol의 방", Status: "진행중", Players: []playerInfo{{Name: "Carol", Class: "Mage"}}},
	}
	dailyMod := game.DailyModifier{Name: "Glass Cannon", Desc: "적 공격력 증가"}
	section("lobby empty", renderLobby(lobbyState{online: 1}, 100, 40))
	section("lobby rooms", renderLobby(lobbyState{rooms: rooms, cursor: 1, online: 5, dailyMod: dailyMod, hardUnlocked: true, hardMode: true}, 100, 40))
	section("lobby joining", renderLobby(lobbyState{rooms: rooms, joining: true, codeInput: "AB", hardUnlocked: true, errMsg: "room not found"}, 100, 40))

	var runs []store.RunRecord
	for i := 0; i < 12; i++ {
		run := store.RunRecord{Player: fmt.Sprintf("P%d", i), Floor: 20 - i, Score: 1000 - i*50}
		if i%2 == 0 {
			run.Class = "Rogue"
		}
		if i%3 == 0 {
			run.Members = []string{run.Player, "Friend"}
		}
		runs = append(runs, run)
	}
	var daily []store.DailyRecord
	for i := 0; i < 22; i++ {
		daily = append(daily, store.DailyRecord{Player: fmt.Sprintf("fp-%d", i), PlayerName: fmt.Sprintf("D%d", i), FloorReached: 10 - i%5, GoldEarned: 300 - i})
	}
	today := time.Now().Format("2006-01-02")
	for tab := 0; tab < 3; tab++ {
		out := renderLeaderboard(runs, runs[2:], daily, tab, 120, 50, "P3", "fp-4")
		section(fmt.Sprintf("leaderboard tab %d", tab), strings.ReplaceAll(out, today, "YYYY-MM-DD"))
	}
	section("leaderboard empty daily", strings.ReplaceAll(renderLeaderboard(nil, nil, nil, 2, 120, 50, "P3", "fp-4"), today, "YYYY-MM-DD"))

	cfg, _ := config.Load("")
	cfg.Game.TurnTimeoutSec = 7
	cfg.Game.MaxPlayers = 3
	cfg.Game.CoopBonus = 0.2
	cfg.Game.SkillUses = 2
	cfg.Game.InventoryLimit = 8
	cfg.Game.MaxFloors = 15
	for page := -1; page <= helpPageCount; page++ {
		section(fmt.Sprintf("help default page %d", page), renderHelp(120, 70, nil, page))
	}
	section("help custom page 1", renderHelp(120, 70, cfg, 1))
	section("help custom page 2", renderHelp(120, 70, cfg, 2))

	alive := entity.NewPlayer("Alice", entity.ClassWarrior)
	alive.Gold = 150
	alive.Inventory = []entity.Item{{Name: "Sword", Type: entity.ItemWeapon, Bonus: 5}}
	dead := entity.NewPlayer("Bob", entity.ClassMage)
	dead.Gold = 40
	dead.HP = 0
	players := []*entity.Player{alive, dead}
	section("result victory ranked", renderResult(game.GameState{Victory: true, FloorNum: 20, Players: players}, runs[:10], "P1"))
	section("result defeat unranked", renderResult(game.GameState{FloorNum: 7, Players: players}, runs[:10], "Nobody"))
	section("result no rankings", renderResult(game.GameState{FloorNum: 3, Players: players}, nil, "Alice"))
	return b.String()
}

// TestPinScreensAscii pins the text and layout of these screens. If it breaks,
// players see different text or a different layout.
func TestPinScreensAscii(t *testing.T) {
	withColorProfile(t, termenv.Ascii)
	checkGolden(t, "screens_ascii", renderScreensForPin())
}

// TestPinScreensTrueColor pins the colors of these screens.
func TestPinScreensTrueColor(t *testing.T) {
	withColorProfile(t, termenv.TrueColor)
	checkGolden(t, "screens_truecolor", renderScreensForPin())
}
