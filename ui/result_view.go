package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// ResultScreen shows the end-of-run summary and rankings.
type ResultScreen struct {
	gameState    game.GameState
	rankingSaved bool
}

func NewResultScreen(state game.GameState, rankingSaved bool) *ResultScreen {
	return &ResultScreen{gameState: state, rankingSaved: rankingSaved}
}

func (s *ResultScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isEnter(key) {
			if ctx.Lobby != nil && ctx.RoomCode != "" {
				ctx.Lobby.LeaveSession(ctx.RoomCode, ctx.Fingerprint)
			}
			ctx.Session = nil
			ctx.RoomCode = ""
			ls := NewLobbyScreen()
			ls.refreshLobby(ctx)
			return ls, ls.pollLobby()
		} else if isKey(key, "n") && s.gameState.Victory && ctx.Lobby != nil {
			// NG+ branch: leave current session, create new one with NGPlusLevel++ and a random curse
			oldNG := 0
			if ctx.Session != nil {
				oldNG = ctx.Session.NGPlusLevel
			}
			if ctx.RoomCode != "" {
				ctx.Lobby.LeaveSession(ctx.RoomCode, ctx.Fingerprint)
			}
			ctx.Session = nil
			ctx.RoomCode = ""
			code := ctx.Lobby.CreateRoom(ctx.PlayerName + fmt.Sprintf(" NG+%d", oldNG+1))
			if err := ctx.Lobby.JoinRoom(code, ctx.PlayerName, ctx.Fingerprint); err == nil {
				ctx.RoomCode = code
				session := ctx.Lobby.EnsureSession(code, func() *game.GameSession {
					gs := game.NewGameSession(ctx.Lobby.Cfg())
					gs.NGPlusLevel = oldNG + 1
					curse := game.PickCurse(nil)
					gs.ActiveCurse = &curse
					gs.ApplyWeeklyMutation()
					return gs
				})
				if session != nil {
					ctx.Session = session
				}
				return NewClassSelectScreen(), nil
			}
		} else if isForceQuit(key) {
			return s, tea.Quit
		}
	}
	return s, nil
}

func (s *ResultScreen) View(ctx *Context) string {
	var rankings []store.RunRecord
	if ctx.Store != nil {
		rankings, _ = ctx.Store.TopRuns(10)
	}
	return renderResult(s.gameState, rankings, ctx.PlayerName)
}

// findPlayerRank returns the 1-indexed position of the player's best run in
// the supplied ranking slice, or 0 if they don't appear in the top N.
func findPlayerRank(rankings []store.RunRecord, playerName string) int {
	for i, r := range rankings {
		if r.Player == playerName {
			return i + 1
		}
	}
	return 0
}

func renderResult(state game.GameState, rankings []store.RunRecord, playerName string) string {
	var sb strings.Builder

	// Title
	if state.Victory {
		sb.WriteString(styleHeal.Render("  ✦ 승리 ✦  ") + "\n\n")
		sb.WriteString(styleSystem.Render("  카타콤을 정복했습니다!") + "\n\n")
	} else {
		sb.WriteString(styleDamage.Render("  ✦ 패배 ✦  ") + "\n\n")
		sb.WriteString(styleSystem.Render(fmt.Sprintf("  B%d층에서 쓰러졌습니다", state.FloorNum)) + "\n\n")
	}

	// Player summary
	sb.WriteString(styleHeader.Render("── 파티 요약 ──") + "\n\n")
	totalGold := 0
	for _, p := range state.Players {
		status := styleHeal.Render("생존")
		if p.IsDead() {
			status = styleDamage.Render("사망")
		}
		fmt.Fprintf(&sb, "  %s (%s)  %s  골드: %d  아이템: %d  유물: %d\n",
			stylePlayer.Render(p.Name), p.Class, status, p.Gold, len(p.Inventory), len(p.Relics))
		totalGold += p.Gold
	}
	fmt.Fprintf(&sb, "\n  총 골드: %s\n", styleGold.Render(fmt.Sprintf("%d", totalGold)))

	// Rankings
	if len(rankings) > 0 {
		sb.WriteString("\n" + styleHeader.Render("── 최고 기록 ──") + "\n\n")
		myRank := findPlayerRank(rankings, playerName)
		for i, r := range rankings {
			medal := "  "
			switch i {
			case 0:
				medal = styleGold.Render("🥇")
			case 1:
				medal = styleSystem.Render("🥈")
			case 2:
				medal = styleGold.Render("🥉")
			}
			party := ""
			if len(r.Members) > 1 {
				party = fmt.Sprintf(" (%s)", strings.Join(r.Members, ", "))
			}
			line := fmt.Sprintf("  %s %s  B%d층  점수: %d%s", medal, r.Player, r.Floor, r.Score, party)
			// Highlight the current player's row so they can find themselves.
			if i+1 == myRank {
				line = stylePlayer.Render(line + "  ←")
			}
			sb.WriteString(line + "\n")
		}
		if myRank > 0 {
			fmt.Fprintf(&sb, "\n  당신의 순위: %s\n",
				styleGold.Render(fmt.Sprintf("%d위", myRank)))
		} else {
			sb.WriteString(styleSystem.Render("\n  이번 기록은 상위 10위 밖입니다.\n"))
		}
	}

	if state.Victory {
		sb.WriteString("\n" + styleAction.Render("  [Enter] 로비로 돌아가기  [N] New Game+ (저주 1개 + 적 +25%)") + "\n")
	} else {
		sb.WriteString("\n" + styleAction.Render("  [Enter] 로비로 돌아가기") + "\n")
	}

	return sb.String()
}
