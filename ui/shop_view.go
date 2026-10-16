package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

type ShopScreen struct {
	gameState game.GameState
	shopMsg   string
}

func NewShopScreen(state game.GameState) *ShopScreen {
	return &ShopScreen{gameState: state}
}

func (s *ShopScreen) pollState(ctx *Context) tea.Cmd {
	return scheduleTick(ctx, time.Millisecond*300)
}

func (s *ShopScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if _, ok := msg.(tickMsg); ok {

		if ctx.Session != nil {
			st := ctx.Session.GetState()
			s.gameState = st
			if st.Phase != game.PhaseShop {
				gs := NewGameScreen()
				gs.gameState = st
				return gs, nil
			}
		}
		return s, s.pollState(ctx)
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "1", "2", "3", "4":
			if ctx.Session != nil {
				idx := int(key.String()[0] - '1')
				switch ctx.Session.BuyItem(ctx.Fingerprint, idx) {
				case game.BuyOK:
					s.shopMsg = "구매 완료!"
				case game.BuyNoGold:
					s.shopMsg = "골드가 부족합니다!"
				case game.BuyInventoryFull:
					s.shopMsg = "인벤토리가 가득 찼습니다!"
				default:
					s.shopMsg = "구매할 수 없습니다!"
				}
				s.gameState = ctx.Session.GetState()
			}
		case "q":
			if ctx.Session != nil {
				if ctx.Session.LeaveShop(ctx.Fingerprint) {
					gs := NewGameScreen()
					gs.gameState = ctx.Session.GetState()
					return gs, nil
				}
				s.gameState = ctx.Session.GetState()
				ready, total := 0, 0
				for _, p := range s.gameState.Players {
					if p.IsOut() {
						continue
					}
					total++
					if s.gameState.ShopLeavers[p.Fingerprint] {
						ready++
					}
				}
				s.shopMsg = fmt.Sprintf("상점 떠나는 중 %d/%d — 모두 [Q] 누를 때까지 대기", ready, total)
			}
		}
	}
	return s, nil
}

func (s *ShopScreen) View(ctx *Context) string {
	return renderShop(s.gameState, ctx.Width, ctx.Height, s.shopMsg)
}

func itemTypeLabel(item entity.Item) string {
	switch item.Type {
	case entity.ItemWeapon:
		return fmt.Sprintf("[ATK+%d]", item.Bonus)
	case entity.ItemArmor:
		return fmt.Sprintf("[DEF+%d]", item.Bonus)
	case entity.ItemConsumable:
		return fmt.Sprintf("[HP+%d]", item.Bonus)
	case entity.ItemCharm:
		return "[도박 손해 1회 무효]"
	default:
		return fmt.Sprintf("[+%d]", item.Bonus)
	}
}

func renderShop(state game.GameState, width, height int, shopMsg string) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
	goldStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("220"))
	msgStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	header := headerStyle.Render("── 상점 ──")

	goldLine := ""
	for _, p := range state.Players {
		inventoryCount := len(p.Inventory)
		goldLine += goldStyle.Render(fmt.Sprintf("  %s — 골드: %d  아이템: %d/10", p.Name, p.Gold, inventoryCount))
		goldLine += "\n"
	}

	items := ""
	for i, item := range state.ShopItems {
		label := itemTypeLabel(item)
		soldOut := i < len(state.ShopSold) && state.ShopSold[i]
		if soldOut {
			items += fmt.Sprintf("  [%d] %s %s — (품절)\n", i+1, item.Name, label)
		} else {
			items += fmt.Sprintf("  [%d] %s %s — %d 골드\n", i+1, item.Name, label, item.Price)
		}
	}

	charmHint := ""
	for _, item := range state.ShopItems {
		if item.Type == entity.ItemCharm {

			charmHint = styleSystem.Render(
				"  행운 부적: 🎲 도박 실패 손해(HP/골드/저주)를 1회 무효화\n" +
					"  (고정 비용·성공 보상에는 미적용)")
			break
		}
	}

	menu := "[1-4] 구매  [Q] 상점 나가기"

	parts := []string{header, "", goldLine, items}
	if charmHint != "" {
		parts = append(parts, charmHint)
	}
	parts = append(parts, "", menu)
	if shopMsg != "" {
		parts = append(parts, "", msgStyle.Render(shopMsg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
