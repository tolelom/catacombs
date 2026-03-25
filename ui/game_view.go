package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

// GameScreen handles the main gameplay: exploration, combat, and chat.
type GameScreen struct {
	gameState     game.GameState
	targetCursor  int
	moveCursor    int
	chatting      bool
	chatInput     string
	rankingSaved  bool
	codexRecorded map[string]bool
	prevPhase     game.GamePhase
}

func NewGameScreen() *GameScreen {
	return &GameScreen{
		codexRecorded: make(map[string]bool),
	}
}

func (s *GameScreen) leaveGame(ctx *Context) (Screen, tea.Cmd) {
	if ctx.Lobby != nil && ctx.Fingerprint != "" {
		ctx.Lobby.UnregisterSession(ctx.Fingerprint)
	}
	if ctx.Session != nil {
		ctx.Session.Stop()
		ctx.Session = nil
	}
	if ctx.Lobby != nil && ctx.RoomCode != "" {
		ctx.Lobby.RemoveRoom(ctx.RoomCode)
	}
	ctx.RoomCode = ""
	ls := NewLobbyScreen()
	ls.refreshLobby(ctx)
	return ls, ls.pollLobby()
}

func (s *GameScreen) pollState() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (s *GameScreen) getNeighbors() []int {
	if s.gameState.Floor == nil {
		return nil
	}
	cur := s.gameState.Floor.CurrentRoom
	if cur < 0 || cur >= len(s.gameState.Floor.Rooms) {
		return nil
	}
	return s.gameState.Floor.Rooms[cur].Neighbors
}

func (s *GameScreen) Update(msg tea.Msg, ctx *Context) (Screen, tea.Cmd) {
	if ctx.Session != nil && ctx.Fingerprint != "" {
		ctx.Session.TouchActivity(ctx.Fingerprint)
	}
	// Refresh state on every update
	if ctx.Session != nil {
		s.gameState = ctx.Session.GetState()
		// Clamp target cursor to valid range after monsters die
		if len(s.gameState.Monsters) > 0 {
			if s.targetCursor >= len(s.gameState.Monsters) {
				s.targetCursor = len(s.gameState.Monsters) - 1
			}
		} else {
			s.targetCursor = 0
		}

		// Record codex entries for monsters when entering combat
		if ctx.Store != nil && s.gameState.Phase == game.PhaseCombat {
			for _, m := range s.gameState.Monsters {
				key := "monster:" + m.Name
				if !s.codexRecorded[key] {
					ctx.Store.RecordCodexEntry(ctx.Fingerprint, "monster", m.Name)
					s.codexRecorded[key] = true
				}
			}
		}

		// Record codex entries for shop items when entering shop
		if ctx.Store != nil && s.gameState.Phase == game.PhaseShop && s.prevPhase != game.PhaseShop {
			for _, item := range s.gameState.ShopItems {
				key := "item:" + item.Name
				if !s.codexRecorded[key] {
					ctx.Store.RecordCodexEntry(ctx.Fingerprint, "item", item.Name)
					s.codexRecorded[key] = true
				}
			}
		}

		// Record codex entries for events
		if ctx.Store != nil && s.gameState.LastEventName != "" {
			key := "event:" + s.gameState.LastEventName
			if !s.codexRecorded[key] {
				ctx.Store.RecordCodexEntry(ctx.Fingerprint, "event", s.gameState.LastEventName)
				s.codexRecorded[key] = true
			}
		}

		s.prevPhase = s.gameState.Phase
	}

	if s.gameState.GameOver {
		if ctx.Store != nil && !s.rankingSaved {
			score := 0
			for _, p := range s.gameState.Players {
				score += p.Gold
			}
			playerClass := ""
			var members []string
			for _, p := range s.gameState.Players {
				if p.Fingerprint == ctx.Fingerprint {
					playerClass = p.Class.String()
				}
				members = append(members, p.Name)
			}
			ctx.Store.SaveRun(ctx.PlayerName, s.gameState.FloorNum, score, playerClass, members)
			// Check achievements
			if s.gameState.FloorNum >= 5 {
				ctx.Store.UnlockAchievement(ctx.PlayerName, "first_clear")
			}
			if s.gameState.FloorNum >= 10 {
				ctx.Store.UnlockAchievement(ctx.PlayerName, "floor10")
			}
			if s.gameState.Victory {
				ctx.Store.UnlockAchievement(ctx.PlayerName, "floor20")
			}
			if s.gameState.SoloMode && s.gameState.FloorNum >= 5 {
				ctx.Store.UnlockAchievement(ctx.PlayerName, "solo_clear")
			}
			if s.gameState.BossKilled {
				ctx.Store.UnlockAchievement(ctx.PlayerName, "boss_slayer")
			}
			if s.gameState.FleeSucceeded {
				ctx.Store.UnlockAchievement(ctx.PlayerName, "flee_master")
			}
			for _, p := range s.gameState.Players {
				if p.Gold >= 200 {
					ctx.Store.UnlockAchievement(p.Name, "gold_hoarder")
				}
				if len(p.Relics) >= 3 {
					ctx.Store.UnlockAchievement(p.Name, "relic_collector")
				}
			}
			if len(s.gameState.Players) >= 4 {
				ctx.Store.UnlockAchievement(ctx.PlayerName, "full_party")
			}

			// Unlock triggers
			if s.gameState.FloorNum >= 10 {
				ctx.Store.UnlockContent(ctx.Fingerprint, "fifth_class")
			}
			if len(s.gameState.Players) >= 3 && s.gameState.FloorNum >= 5 {
				ctx.Store.UnlockContent(ctx.Fingerprint, "hard_mode")
			}
			if s.gameState.Victory {
				ctx.Store.UnlockContent(ctx.Fingerprint, "mutations")
			}

			// Title triggers
			ctx.Store.EarnTitle(ctx.Fingerprint, "novice")
			if s.gameState.FloorNum >= 5 {
				ctx.Store.EarnTitle(ctx.Fingerprint, "explorer")
			}
			if s.gameState.FloorNum >= 10 {
				ctx.Store.EarnTitle(ctx.Fingerprint, "veteran")
			}
			if s.gameState.Victory {
				ctx.Store.EarnTitle(ctx.Fingerprint, "champion")
			}
			// Check player gold for gold_king title
			for _, p := range s.gameState.Players {
				if p.Fingerprint == ctx.Fingerprint && p.Gold >= 500 {
					ctx.Store.EarnTitle(ctx.Fingerprint, "gold_king")
				}
			}

			// Save daily record if in daily mode
			if ctx.Session != nil && ctx.Session.DailyMode {
				playerGold := 0
				for _, p := range s.gameState.Players {
					if p.Fingerprint == ctx.Fingerprint {
						playerGold = p.Gold
						break
					}
				}
				ctx.Store.SaveDaily(store.DailyRecord{
					Date:         ctx.Session.DailyDate,
					Player:       ctx.Fingerprint,
					PlayerName:   ctx.PlayerName,
					FloorReached: s.gameState.FloorNum,
					GoldEarned:   playerGold,
				})
			}

			s.rankingSaved = true
		}
		return NewResultScreen(s.gameState, s.rankingSaved), nil
	}
	if s.gameState.Phase == game.PhaseShop {
		return NewShopScreen(s.gameState), nil
	}

	switch msg.(type) {
	case tickMsg:
		if ctx.Session != nil {
			ctx.Session.RevealNextLog()
		}
		if s.gameState.Phase == game.PhaseCombat {
			return s, s.pollState()
		}
		if len(s.gameState.PendingLogs) > 0 {
			return s, s.pollState()
		}
		return s, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		// Chat mode
		if s.chatting {
			if isEnter(key) && len(s.chatInput) > 0 {
				if ctx.Session != nil {
					ctx.Session.SendChat(ctx.PlayerName, s.chatInput)
					s.gameState = ctx.Session.GetState()
				}
				s.chatting = false
				s.chatInput = ""
			} else if isKey(key, "esc") || key.Type == tea.KeyEsc {
				s.chatting = false
				s.chatInput = ""
			} else if key.Type == tea.KeyBackspace && len(s.chatInput) > 0 {
				s.chatInput = s.chatInput[:len(s.chatInput)-1]
			} else if len(key.Runes) == 1 && len(s.chatInput) < 40 {
				s.chatInput += string(key.Runes)
			}
			if s.gameState.Phase == game.PhaseCombat {
				return s, s.pollState()
			}
			return s, nil
		}

		if isKey(key, "/") {
			s.chatting = true
			s.chatInput = ""
			if s.gameState.Phase == game.PhaseCombat {
				return s, s.pollState()
			}
			return s, nil
		}

		switch s.gameState.Phase {
		case game.PhaseExploring:
			if isForceQuit(key) {
				return s, tea.Quit
			}
			for _, p := range s.gameState.Players {
				if p.Fingerprint == ctx.Fingerprint && p.IsDead() {
					if isKey(key, "q") {
						return s.leaveGame(ctx)
					}
					return s, nil
				}
			}
			// Skill point allocation
			if isKey(key, "[") || isKey(key, "]") {
				if ctx.Session != nil {
					branchIdx := 0
					if isKey(key, "]") {
						branchIdx = 1
					}
					ctx.Session.AllocateSkillPoint(ctx.Fingerprint, branchIdx)
					s.gameState = ctx.Session.GetState()
				}
				return s, nil
			}
			neighbors := s.getNeighbors()
			if isUp(key) {
				if s.moveCursor > 0 {
					s.moveCursor--
				}
			} else if isDown(key) {
				if s.moveCursor < len(neighbors)-1 {
					s.moveCursor++
				}
			} else if isEnter(key) {
				if ctx.Session != nil && len(neighbors) > 0 {
					roomIdx := neighbors[s.moveCursor]
					ctx.Session.EnterRoom(roomIdx)
					s.gameState = ctx.Session.GetState()
					s.moveCursor = 0
					if s.gameState.Phase == game.PhaseCombat {
						return s, s.pollState()
					}
				}
			} else if isForceQuit(key) {
				return s, tea.Quit
			} else if isKey(key, "q") {
				return s.leaveGame(ctx)
			}
		case game.PhaseCombat:
			isPlayerDead := false
			for _, p := range s.gameState.Players {
				if p.Fingerprint == ctx.Fingerprint && p.IsDead() {
					isPlayerDead = true
					break
				}
			}
			if isPlayerDead {
				return s, s.pollState()
			}
			if isKey(key, "tab") || key.Type == tea.KeyTab {
				if len(s.gameState.Monsters) > 0 {
					s.targetCursor = (s.targetCursor + 1) % len(s.gameState.Monsters)
				}
				return s, s.pollState()
			}
			if ctx.Session != nil {
				switch key.String() {
				case "1":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionAttack, TargetIdx: s.targetCursor})
				case "2":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionSkill, TargetIdx: s.targetCursor})
				case "3":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionItem})
				case "4":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionFlee})
				case "5":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionWait})
				}
				return s, s.pollState()
			}
		}
	}
	return s, nil
}

func (s *GameScreen) View(ctx *Context) string {
	return renderGame(s.gameState, ctx.Width, ctx.Height, s.targetCursor, s.moveCursor, s.chatting, s.chatInput, ctx.Fingerprint)
}

func renderGame(state game.GameState, width, height int, targetCursor int, moveCursor int, chatting bool, chatInput string, fingerprint string) string {
	mapView := renderMap(state.Floor)
	hudView := renderHUD(state, targetCursor, moveCursor, fingerprint)
	logView := renderCombatLog(state.CombatLog)

	if chatting {
		chatStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
		chatView := chatStyle.Render(fmt.Sprintf("> %s_", chatInput))
		return lipgloss.JoinVertical(lipgloss.Left, mapView, hudView, logView, chatView)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		mapView,
		hudView,
		logView,
	)
}

func renderMap(floor *dungeon.Floor) string {
	if floor == nil {
		return ""
	}
	theme := dungeon.GetFloorTheme(floor.Number)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	// Count explored rooms
	explored := 0
	for _, r := range floor.Rooms {
		if r.Visited || r.Cleared {
			explored++
		}
	}
	total := len(floor.Rooms)

	header := headerStyle.Render(fmt.Sprintf("── 카타콤 B%d: %s ── %d/%d 방 ──", floor.Number, theme.Name, explored, total))
	return header + "\n" + dungeon.RenderFloor(floor, floor.CurrentRoom, true)
}

func renderHUD(state game.GameState, targetCursor int, moveCursor int, fingerprint string) string {
	var sb strings.Builder
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1)

	// Player info
	for _, p := range state.Players {
		hpBar := renderHPBar(p.HP, p.MaxHP, 20)
		status := ""
		if p.IsDead() {
			status = " [사망]"
		}
		sb.WriteString(fmt.Sprintf("%s (%s) %s %d/%d%s  골드: %d",
			p.Name, p.Class, hpBar, p.HP, p.MaxHP, status, p.Gold))

		// Show inventory count
		itemCount := len(p.Inventory)
		relicCount := len(p.Relics)
		if itemCount > 0 || relicCount > 0 {
			sb.WriteString(fmt.Sprintf("  아이템:%d 유물:%d", itemCount, relicCount))
		}
		sb.WriteString("\n")
	}

	if state.Phase == game.PhaseCombat {
		// Two-panel layout: PARTY | ENEMIES
		partyContent := renderPartyPanel(state.Players, state.SubmittedActions)
		enemyContent := renderEnemyPanel(state.Monsters, targetCursor)

		partyPanel := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGray).
			Width(35).
			Padding(0, 1).
			Render(partyContent)

		enemyPanel := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGray).
			Width(38).
			Padding(0, 1).
			Render(enemyContent)

		panels := lipgloss.JoinHorizontal(lipgloss.Top, partyPanel, enemyPanel)
		sb.WriteString(panels)
		sb.WriteString("\n")

		// Action bar
		sb.WriteString(styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 [Tab]대상 [/]채팅"))
		sb.WriteString("\n")

		// Timer
		if !state.TurnDeadline.IsZero() {
			remaining := time.Until(state.TurnDeadline)
			if remaining < 0 {
				remaining = 0
			}
			sb.WriteString(styleTimer.Render(fmt.Sprintf("  타이머: %.1f초", remaining.Seconds())))
			sb.WriteString("\n")
		}

		// Skill description for first alive player only
		for _, p := range state.Players {
			if !p.IsDead() {
				var skillDesc string
				switch p.Class {
				case entity.ClassWarrior:
					skillDesc = "스킬: Taunt — 2턴간 적의 공격을 끌어옴"
				case entity.ClassMage:
					skillDesc = "스킬: Fireball — 전체 적에게 0.8배 피해"
				case entity.ClassHealer:
					skillDesc = "스킬: Heal — 아군 HP 30 회복"
				case entity.ClassRogue:
					skillDesc = "스킬: Scout — 주변 방 공개"
				}
				skillDesc += fmt.Sprintf(" (남은 횟수: %d)", p.SkillUses)
				sb.WriteString(styleSystem.Render(skillDesc))
				sb.WriteString("\n")
				break
			}
		}
	} else if state.Phase == game.PhaseExploring {
		if state.Floor != nil && state.Floor.CurrentRoom >= 0 && state.Floor.CurrentRoom < len(state.Floor.Rooms) {
			current := state.Floor.Rooms[state.Floor.CurrentRoom]
			if len(current.Neighbors) > 0 {
				sb.WriteString("\n")
				selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
				normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
				for i, n := range current.Neighbors {
					if n >= 0 && n < len(state.Floor.Rooms) {
						r := state.Floor.Rooms[n]
						status := r.Type.String()
						if r.Cleared {
							status = "클리어"
						}
						marker := "  "
						style := normalStyle
						if i == moveCursor {
							marker = "> "
							style = selectedStyle
						}
						sb.WriteString(style.Render(fmt.Sprintf("%s방 %d: %s", marker, n, status)))
						sb.WriteString("\n")
					}
				}
			}
		}
		// Show skill tree allocation UI if player has unspent points
		for _, p := range state.Players {
			if p.Fingerprint == fingerprint && p.Skills != nil && p.Skills.Points > p.Skills.Allocated && p.Skills.Allocated < 3 {
				branches := entity.GetBranches(p.Class)
				sb.WriteString("\n")
				skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
				sb.WriteString(skillStyle.Render(fmt.Sprintf("  스킬 포인트 사용 가능! (미사용: %d)", p.Skills.Points-p.Skills.Allocated)))
				sb.WriteString("\n")
				for i, branch := range branches {
					key := "["
					if i == 1 {
						key = "]"
					}
					nextNode := p.Skills.Allocated
					if p.Skills.BranchIndex >= 0 && p.Skills.BranchIndex != i {
						sb.WriteString(fmt.Sprintf("    [%s] %s (잠김)\n", key, branch.Name))
					} else if nextNode < 3 {
						node := branch.Nodes[nextNode]
						sb.WriteString(fmt.Sprintf("    [%s] %s -> %s\n", key, branch.Name, node.Name))
					}
				}
				break
			}
		}
		sb.WriteString("[Up/Down] 선택  [Enter] 이동  [Q] 종료")
	}

	if state.Phase == game.PhaseCombat {
		return sb.String()
	}
	return border.Render(sb.String())
}

func renderCombatLog(log []string) string {
	if len(log) == 0 {
		return ""
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorGray).
		Padding(0, 1)

	var sb strings.Builder
	for _, msg := range log {
		colored := colorizeLog(msg)
		sb.WriteString("  > " + colored + "\n")
	}
	return border.Render(sb.String())
}

func colorizeLog(msg string) string {
	switch {
	case strings.Contains(msg, "도주"):
		return styleFlee.Render(msg)
	case strings.Contains(msg, "협동"):
		return styleCoop.Render(msg)
	case strings.Contains(msg, "회복") || strings.Contains(msg, "Heal") || strings.Contains(msg, "치유") || strings.Contains(msg, "부활"):
		return styleHeal.Render(msg)
	case strings.Contains(msg, "피해") || strings.Contains(msg, "공격") || strings.Contains(msg, "Trap") || strings.Contains(msg, "함정"):
		return styleDamage.Render(msg)
	case strings.Contains(msg, "Taunt") || strings.Contains(msg, "정찰"):
		return styleStatus.Render(msg)
	case strings.Contains(msg, "골드") || strings.Contains(msg, "Gold") || strings.Contains(msg, "발견"):
		return styleGold.Render(msg)
	case strings.Contains(msg, "처치") || strings.Contains(msg, "클리어") || strings.Contains(msg, "내려갑니다") || strings.Contains(msg, "정복"):
		return styleSystem.Render(msg)
	default:
		return msg
	}
}

func renderHPBar(current, max, width int) string {
	if max == 0 {
		return ""
	}
	filled := current * width / max
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	pct := float64(current) / float64(max)
	var barStyle lipgloss.Style
	switch {
	case pct > 0.5:
		barStyle = lipgloss.NewStyle().Foreground(colorGreen)
	case pct > 0.25:
		barStyle = lipgloss.NewStyle().Foreground(colorYellow)
	default:
		barStyle = lipgloss.NewStyle().Foreground(colorRed)
	}
	emptyStyle := lipgloss.NewStyle().Foreground(colorGray)

	return barStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))
}

func renderPartyPanel(players []*entity.Player, submittedActions map[string]string) string {
	var sb strings.Builder
	sb.WriteString(styleHeader.Render("  아군") + "\n\n")

	for _, p := range players {
		nameStr := stylePlayer.Render(fmt.Sprintf("  ♦ %s", p.Name))
		classStr := styleSystem.Render(fmt.Sprintf(" (%s)", p.Class))
		status := ""
		if p.IsDead() {
			status = styleDamage.Render(" [사망]")
		}
		sb.WriteString(nameStr + classStr + status + "\n")

		hpBar := renderHPBar(p.HP, p.MaxHP, 16)
		sb.WriteString(fmt.Sprintf("    %s %d/%d\n", hpBar, p.HP, p.MaxHP))

		if len(p.Effects) > 0 {
			var effects []string
			for _, e := range p.Effects {
				switch e.Type {
				case entity.StatusPoison:
					effects = append(effects, styleHeal.Render(fmt.Sprintf("☠Poison(%dt)", e.Duration)))
				case entity.StatusBurn:
					effects = append(effects, styleDamage.Render(fmt.Sprintf("🔥Burn(%dt)", e.Duration)))
				case entity.StatusFreeze:
					effects = append(effects, styleFlee.Render(fmt.Sprintf("❄Freeze(%dt)", e.Duration)))
				}
			}
			sb.WriteString("    " + strings.Join(effects, " ") + "\n")
		}

		sb.WriteString(fmt.Sprintf("    ATK:%-3d DEF:%-3d  ", p.EffectiveATK(), p.EffectiveDEF()))
		sb.WriteString(styleGold.Render(fmt.Sprintf("G:%d", p.Gold)))
		sb.WriteString("\n")

		if action, ok := submittedActions[p.Fingerprint]; ok {
			sb.WriteString(styleHeal.Render(fmt.Sprintf("    ✓ %s", action)))
			sb.WriteString("\n")
		} else if !p.IsOut() {
			sb.WriteString(styleSystem.Render("    ... 대기중"))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderEnemyPanel(monsters []*entity.Monster, targetCursor int) string {
	var sb strings.Builder
	sb.WriteString(styleHeader.Render("  적") + "\n\n")

	for i, m := range monsters {
		if m.IsDead() {
			continue
		}
		// ASCII art
		art := MonsterArt(m.Type)
		for _, line := range art {
			sb.WriteString(styleEnemy.Render("  "+line) + "\n")
		}

		// Name + HP
		marker := "  "
		if i == targetCursor {
			marker = "> "
		}
		hpBar := renderHPBar(m.HP, m.MaxHP, 12)
		taunt := ""
		if m.TauntTarget {
			taunt = styleStatus.Render(fmt.Sprintf(" [도발됨 %d턴]", m.TauntTurns))
		}
		sb.WriteString(fmt.Sprintf("  %s[%d] %s %s %d/%d%s\n\n",
			marker, i, styleEnemy.Render(m.Name), hpBar, m.HP, m.MaxHP, taunt))
	}
	return sb.String()
}
