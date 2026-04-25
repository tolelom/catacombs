package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/config"
)

// HelpScreen shows controls and tips.
type HelpScreen struct{}

func NewHelpScreen() *HelpScreen {
	return &HelpScreen{}
}

func (s *HelpScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if isKey(key, "h") || isEnter(key) || isQuit(key) {
			return NewTitleScreen(), nil
		}
	}
	return s, nil
}

func (s *HelpScreen) View(ctx *Context) string {
	var cfg *config.Config
	if ctx.Lobby != nil {
		cfg = ctx.Lobby.Cfg()
	}
	return renderHelp(ctx.Width, ctx.Height, cfg)
}

func renderHelp(width, height int, cfg *config.Config) string {
	title := styleHeader.Render("── 조작법 ──")

	turnTimeout := 10
	maxPlayers := 4
	coopPct := 10
	skillUses := 3
	inventoryLimit := 10
	maxFloors := 20
	if cfg != nil {
		if cfg.Game.TurnTimeoutSec > 0 {
			turnTimeout = cfg.Game.TurnTimeoutSec
		}
		if cfg.Game.MaxPlayers > 0 {
			maxPlayers = cfg.Game.MaxPlayers
		}
		if cfg.Game.CoopBonus > 0 {
			coopPct = int(cfg.Game.CoopBonus * 100)
		}
		if cfg.Game.SkillUses > 0 {
			skillUses = cfg.Game.SkillUses
		}
		if cfg.Game.InventoryLimit > 0 {
			inventoryLimit = cfg.Game.InventoryLimit
		}
		if cfg.Game.MaxFloors > 0 {
			maxFloors = cfg.Game.MaxFloors
		}
	}

	sections := []struct{ header, body string }{
		{"로비", `  [C]  방 만들기     [J]  코드로 참가
  [Enter] 선택한 방 참가
  [D]  일일 도전      [H]  하드 모드 전환
  [Q]  타이틀로 돌아가기`},
		{"탐험", `  [Up/Down]  방 선택
  [Enter]    방으로 이동
  [[]  / []]  스킬 포인트 배분 (분기 1/2)
  [/]        채팅
  [Q]        종료`},
		{fmt.Sprintf("전투 (턴당 %d초)", turnTimeout), `  [1]  공격        [2]  스킬
  [3]  아이템 사용  [4]  도주
  [5]  방어        [Tab] 적 대상 변경
  [Shift+Tab]  아군 대상 변경 (Healer)
  [/]  채팅`},
		{"상점", `  [1-3]  아이템 구매
  [Q]    상점 나가기 (멀티플레이는 모두 동의 시 종료)`},
		{"직업", `  Warrior  120HP 12ATK 8DEF  Taunt (2턴간 적 공격 유도)
  Mage      70HP 20ATK 3DEF  Fireball (광역 0.8배)
  Healer    90HP  8ATK 5DEF  Heal (HP 30 회복)
  Rogue     85HP 15ATK 4DEF  Scout (주변 방 공개)`},
		{"멀티플레이", fmt.Sprintf(`  • 방당 최대 %d명
  • 협동 보너스: 2명 이상이 같은 적 공격 시 피해 +%d%%
  • 직업 콤보로 추가 효과 발동
  • 모든 플레이어 준비 완료 시 게임 시작`, maxPlayers, coopPct)},
		{"팁", fmt.Sprintf(`  • 스킬은 전투당 %d회 사용 가능
  • 아이템은 플레이어당 %d개 제한
  • 사망한 플레이어는 다음 층에서 HP 30%%로 부활
  • 보스는 5, 10, 15, 20층에 등장 (총 %d층)
  • 스킬 포인트: 층 클리어당 1포인트 (최대 3)
  • 주간 변이가 게임플레이를 변경`, skillUses, inventoryLimit, maxFloors)},
	}

	var content string
	headerStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(colorWhite)

	for _, s := range sections {
		content += headerStyle.Render(s.header) + "\n"
		content += bodyStyle.Render(s.body) + "\n\n"
	}

	footer := styleSystem.Render("[H] 뒤로")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, footer))
}
