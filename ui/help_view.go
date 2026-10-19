package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/config"
)

const helpPageCount = 3

type HelpScreen struct {
	page int
}

func NewHelpScreen() *HelpScreen {
	return &HelpScreen{}
}

func (s *HelpScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Type == tea.KeyRight || key.Type == tea.KeyTab:
			if s.page < helpPageCount-1 {
				s.page++
			}
		case key.Type == tea.KeyLeft:
			if s.page > 0 {
				s.page--
			}
		case isKey(key, "h") || isEnter(key) || isQuit(key):
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
	return renderHelp(ctx.Width, ctx.Height, cfg, s.page)
}

type helpSection struct{ header, body string }

func helpPageControls(turnTimeout int) []helpSection {
	return []helpSection{
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
	}
}

func helpPageCombat(skillUses, maxPlayers, coopPct int) []helpSection {
	return []helpSection{
		{"솔로 연계 (혼자 플레이 시)", `  직전 행동이 다음 공격을 강화합니다:
  스킬 → 공격   = 잔향 일격 (피해 1.4배)
  협동 → 공격   = 반격 태세 (확정 치명타)
  공격 3연속    = 연속 공세 (피해 1.25배 +3)
  아이템 사용/행동 실패 시 연계가 끊깁니다`},
		{"스킬 분기", `  층 클리어마다 스킬 포인트 1 획득
  1포인트: 브랜치 선택 (T1 자동 습득)
  2포인트: T2 분기 — [[/]]로 좌/우 선택
  3포인트: T3 자동 습득
  4포인트: T4 캡스톤 분기 — 강력한 최종 선택
  분기 옆 괄호로 효과를 비교하고 고르세요`},
		{"멀티플레이", fmt.Sprintf(`  • 방당 최대 %d명
  • 협동 보너스: 2명 이상이 같은 적 공격 시 피해 +%d%%
  • 직업 콤보로 추가 효과 발동 (솔로에선 연계로 대체)
  • 모든 플레이어 준비 완료 시 게임 시작`, maxPlayers, coopPct)},
		{"전투 팁", fmt.Sprintf(`  • 스킬은 전투당 %d회 사용 가능
  • Healer는 부상자가 없으면 힐이 신성 일격(공격)으로 전환
  • 보스는 5, 10, 15, 20층 / 미니보스는 4, 9, 14, 19층`, skillUses)},
	}
}

func helpPageExploration(inventoryLimit, maxFloors int) []helpSection {
	return []helpSection{
		{"이벤트 방", `  선택지는 파티 다수결 투표로 결정됩니다 (동률은 무작위)
  선택지 옆 태그가 결과 종류를 귀띔합니다:
  + 회복   - 피해   $ 골드   ✨ 아이템   🎲 도박   ⤳ 통과`},
		{"보물 방", `  플레이어마다 전리품 3개 중 1개를 직접 고릅니다
  수치가 공개되니 내 빌드에 맞는 것을 고르세요`},
		{"행운 부적", `  상점에서 구매하는 1회용 보험입니다
  🎲 도박성 선택의 실패 손해(HP/골드/저주)를 1회 무효화
  성공 보상과 고정 비용(통행료 등)에는 쓰이지 않습니다`},
		{"팁", fmt.Sprintf(`  • 아이템은 플레이어당 %d개 제한
  • 사망한 플레이어는 다음 층에서 HP 30%%로 부활
  • 보스 처치 시 유물 획득 (총 %d층)
  • 주간 변이가 게임플레이를 변경`, inventoryLimit, maxFloors)},
	}
}

type helpSettings struct {
	turnTimeout    int
	maxPlayers     int
	coopPct        int
	skillUses      int
	inventoryLimit int
	maxFloors      int
}

func renderHelp(width, height int, cfg *config.Config, page int) string {
	page = clampHelpPage(page)
	pageTitle, sections := helpPage(page, resolveHelpSettings(cfg))

	title := styleHeader.Render(fmt.Sprintf("── 도움말: %s (%d/%d) ──", pageTitle, page+1, helpPageCount))
	content := renderHelpSections(sections)
	footer := styleSystem.Render("[←/→] 페이지  [H] 뒤로")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", content, footer))
}

func resolveHelpSettings(cfg *config.Config) helpSettings {
	settings := helpSettings{
		turnTimeout:    10,
		maxPlayers:     4,
		coopPct:        10,
		skillUses:      3,
		inventoryLimit: 10,
		maxFloors:      20,
	}
	if cfg == nil {
		return settings
	}
	if cfg.Game.TurnTimeoutSec > 0 {
		settings.turnTimeout = cfg.Game.TurnTimeoutSec
	}
	if cfg.Game.MaxPlayers > 0 {
		settings.maxPlayers = cfg.Game.MaxPlayers
	}
	if cfg.Game.CoopBonus > 0 {
		settings.coopPct = int(cfg.Game.CoopBonus * 100)
	}
	if cfg.Game.SkillUses > 0 {
		settings.skillUses = cfg.Game.SkillUses
	}
	if cfg.Game.InventoryLimit > 0 {
		settings.inventoryLimit = cfg.Game.InventoryLimit
	}
	if cfg.Game.MaxFloors > 0 {
		settings.maxFloors = cfg.Game.MaxFloors
	}
	return settings
}

func clampHelpPage(page int) int {
	if page < 0 {
		page = 0
	}
	if page >= helpPageCount {
		page = helpPageCount - 1
	}
	return page
}

func helpPage(page int, settings helpSettings) (string, []helpSection) {
	switch page {
	case 0:
		return "조작법·직업", helpPageControls(settings.turnTimeout)
	case 1:
		return "전투 시스템", helpPageCombat(settings.skillUses, settings.maxPlayers, settings.coopPct)
	case 2:
		return "탐험·경제", helpPageExploration(settings.inventoryLimit, settings.maxFloors)
	}
	return "", nil
}

func renderHelpSections(sections []helpSection) string {
	var content string
	headerStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(colorWhite)

	for _, s := range sections {
		content += headerStyle.Render(s.header) + "\n"
		content += bodyStyle.Render(s.body) + "\n\n"
	}
	return content
}
