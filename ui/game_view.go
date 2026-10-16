package ui

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

// GameScreen handles the main gameplay: exploration, combat, and chat.
type GameScreen struct {
	gameState     game.GameState
	targetCursor  int
	allyCursor    int // for Healer skill targeting allies
	moveCursor    int
	relicCursor   int // index into local player's active relics; cycled with [r]
	chatting      bool
	chatInput     string
	codexRecorded map[string]bool
	prevPhase     game.GamePhase
	skillMsg      string
}

func NewGameScreen() *GameScreen {
	return &GameScreen{
		codexRecorded: make(map[string]bool),
	}
}

func (s *GameScreen) leaveGame(ctx *Context) (Screen, tea.Cmd) {
	if ctx.Lobby != nil && ctx.RoomCode != "" {
		ctx.Lobby.LeaveSession(ctx.RoomCode, ctx.Fingerprint)
	}
	ctx.Session = nil
	ctx.RoomCode = ""
	ls := NewLobbyScreen()
	ls.refreshLobby(ctx)
	return ls, ls.pollLobby(ctx)
}

func (s *GameScreen) pollState(ctx *Context) tea.Cmd {
	return scheduleTick(ctx, time.Millisecond*200)
}

const maxChatRunes = 40

func dropLastRune(s string) string {
	r := []rune(s)
	return string(r[:len(r)-1])
}

func clampIndex(i, n int) int {
	switch {
	case i < 0:
		return 0
	case i >= n:
		return n - 1
	default:
		return i
	}
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
				name := m.BaseName()
				key := "monster:" + name
				if !s.codexRecorded[key] {
					recordCodex(ctx, "monster", name)
					s.codexRecorded[key] = true
				}
			}
		}

		// Record codex entries for shop items when entering shop
		if ctx.Store != nil && s.gameState.Phase == game.PhaseShop && s.prevPhase != game.PhaseShop {
			for _, item := range s.gameState.ShopItems {
				key := "item:" + item.Name
				if !s.codexRecorded[key] {
					recordCodex(ctx, "item", item.Name)
					s.codexRecorded[key] = true
				}
			}
		}

		// Record codex entries for inventory items and relics. Treasure rooms
		// and events drop weapons/armor straight into Inventory, so the
		// shop-only loop above never sees them. Relics (boss drops) live in
		// p.Relics and likewise bypass the shop list.
		if ctx.Store != nil {
			for _, p := range s.gameState.Players {
				if p.Fingerprint != ctx.Fingerprint {
					continue
				}
				for _, it := range p.Inventory {
					key := "item:" + it.Name
					if !s.codexRecorded[key] {
						recordCodex(ctx, "item", it.Name)
						s.codexRecorded[key] = true
					}
				}
				for _, r := range p.Relics {
					key := "item:" + r.Name
					if !s.codexRecorded[key] {
						recordCodex(ctx, "item", r.Name)
						s.codexRecorded[key] = true
					}
				}
			}
		}

		// Record codex entries for events
		if ctx.Store != nil && s.gameState.LastEventName != "" {
			key := "event:" + s.gameState.LastEventName
			if !s.codexRecorded[key] {
				recordCodex(ctx, "event", s.gameState.LastEventName)
				s.codexRecorded[key] = true
			}
		}

		s.prevPhase = s.gameState.Phase
	}

	if s.gameState.GameOver {
		return NewResultScreen(s.gameState, ctx.Store != nil), nil
	}
	if s.gameState.Phase == game.PhaseEvent {
		es := NewEventScreen(s.gameState)
		return es, es.pollState(ctx)
	}
	if s.gameState.Phase == game.PhaseShop {
		ss := NewShopScreen(s.gameState)
		return ss, ss.pollState(ctx)
	}

	switch msg.(type) {
	case tickMsg:
		if ctx.Session != nil {
			ctx.Session.RevealNextLog()
		}
		if s.gameState.Phase == game.PhaseCombat {
			return s, s.pollState(ctx)
		}
		if len(s.gameState.PendingLogs) > 0 {
			return s, s.pollState(ctx)
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
				s.chatInput = dropLastRune(s.chatInput)
			} else if len(key.Runes) == 1 && utf8.RuneCountInString(s.chatInput) < maxChatRunes {
				s.chatInput += string(key.Runes)
			}
			if s.gameState.Phase == game.PhaseCombat {
				return s, s.pollState(ctx)
			}
			return s, nil
		}

		if isKey(key, "/") {
			s.chatting = true
			s.chatInput = ""
			if s.gameState.Phase == game.PhaseCombat {
				return s, s.pollState(ctx)
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
			// Skill point allocation. The meaning of [ and ] depends on the next
			// allocation step:
			//   - Branch-select stage (BranchIndex == -1): keys pick which branch.
			//   - At a fork (T2 or T4): keys mean A/B choice; branchIdx stays locked.
			//   - At an auto tier (T1 already done, T3 step): either key works;
			//     branchIdx stays locked so the entity layer doesn't reject it.
			if isKey(key, "[") || isKey(key, "]") {
				if ctx.Session != nil {
					var localSkills *entity.PlayerSkills
					for _, p := range s.gameState.Players {
						if p.Fingerprint == ctx.Fingerprint && p.Skills != nil {
							localSkills = p.Skills
							break
						}
					}
					branchIdx := 0
					var choice byte
					if localSkills != nil && localSkills.BranchIndex != -1 {
						// Branch already locked — always pass the locked index.
						branchIdx = localSkills.BranchIndex
						if localSkills.NextIsBranch() {
							if isKey(key, "[") {
								choice = 'A'
							} else {
								choice = 'B'
							}
						}
						// At an auto tier (T3 step), choice stays 0 — entity layer ignores it.
					} else {
						// Branch-select stage: keys pick branch.
						if isKey(key, "]") {
							branchIdx = 1
						}
					}
					if err := ctx.Session.AllocateSkillPoint(ctx.Fingerprint, branchIdx, choice); err != nil {
						s.skillMsg = err.Error()
					} else {
						s.skillMsg = ""
					}
					s.gameState = ctx.Session.GetState()
				}
				return s, nil
			}
			neighbors := s.getNeighbors()
			// Block input if this player already voted in multiplayer
			alreadyVoted := false
			if !s.gameState.SoloMode && s.gameState.MoveVotes != nil {
				_, alreadyVoted = s.gameState.MoveVotes[ctx.Fingerprint]
			}
			if isUp(key) {
				if !alreadyVoted && s.moveCursor > 0 {
					s.moveCursor--
				}
			} else if isDown(key) {
				if !alreadyVoted && s.moveCursor < len(neighbors)-1 {
					s.moveCursor++
				}
			} else if isEnter(key) {
				if ctx.Session != nil && len(neighbors) > 0 && !alreadyVoted {
					s.moveCursor = clampIndex(s.moveCursor, len(neighbors))
					roomIdx := neighbors[s.moveCursor]
					if s.gameState.SoloMode {
						ctx.Session.EnterRoom(roomIdx)
					} else {
						ctx.Session.SubmitMoveVote(ctx.Fingerprint, roomIdx)
					}
					s.gameState = ctx.Session.GetState()
					s.moveCursor = 0
					if s.gameState.Phase == game.PhaseCombat {
						return s, s.pollState(ctx)
					}
				}
			} else if isForceQuit(key) {
				return s, tea.Quit
			} else if key.String() == "6" {
				if ctx.Session != nil && len(neighbors) > 0 {
					s.moveCursor = clampIndex(s.moveCursor, len(neighbors))
					target := neighbors[s.moveCursor]
					if err := ctx.Session.SubmitForesight(ctx.Fingerprint, target); err != nil {
						s.skillMsg = err.Error()
					} else {
						s.skillMsg = "정찰 완료!"
					}
					s.gameState = ctx.Session.GetState()
				}
			} else if isKey(key, "q") {
				return s.leaveGame(ctx)
			}
		case game.PhaseCombat:
			isPlayerOut := false
			for _, p := range s.gameState.Players {
				if p.Fingerprint == ctx.Fingerprint && p.IsOut() {
					isPlayerOut = true
					break
				}
			}
			if isPlayerOut {
				return s, s.pollState(ctx)
			}
			if isKey(key, "tab") || key.Type == tea.KeyTab {
				if key.Type == tea.KeyShiftTab {
					// Shift+Tab: cycle ally target (for Healer)
					if len(s.gameState.Players) > 0 {
						s.allyCursor = (s.allyCursor + 1) % len(s.gameState.Players)
					}
				} else {
					// Tab: cycle enemy target
					if len(s.gameState.Monsters) > 0 {
						s.targetCursor = (s.targetCursor + 1) % len(s.gameState.Monsters)
					}
				}
				return s, s.pollState(ctx)
			}
			if ctx.Session != nil {
				// Determine current player's class for skill targeting
				myClass := entity.ClassWarrior
				for _, p := range s.gameState.Players {
					if p.Fingerprint == ctx.Fingerprint {
						myClass = p.Class
						break
					}
				}
				switch key.String() {
				case "1":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionAttack, TargetIdx: s.targetCursor})
				case "2":
					skillTarget := s.targetCursor
					if myClass == entity.ClassHealer {
						skillTarget = s.allyCursor
					}
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionSkill, TargetIdx: skillTarget})
				case "3":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionItem})
				case "4":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionFlee})
				case "5":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionWait})
				case "6":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionCoop, TargetIdx: s.targetCursor})
				case "7":
					ctx.Session.SubmitAction(ctx.Fingerprint, game.PlayerAction{Type: game.ActionRelic, TargetIdx: s.relicCursor})
				case "r":
					if n := countActiveRelics(s.gameState, ctx.Fingerprint); n > 0 {
						s.relicCursor = (s.relicCursor + 1) % n
					}
				}
				return s, s.pollState(ctx)
			}
		}
	}
	return s, nil
}

func (s *GameScreen) View(ctx *Context) string {
	return renderGame(s.gameState, ctx.Width, ctx.Height, s.targetCursor, s.allyCursor, s.moveCursor, s.relicCursor, s.chatting, s.chatInput, ctx.Fingerprint, s.skillMsg)
}

// countCoopCombos returns how many of the 6 coop pair combos appear in the
// run-scoped combo set.

// playerActiveRelics returns the local player's active (triggerable) relics in
// inventory order. Used by the [r] cycle key and the HUD armed-relic readout.
func playerActiveRelics(state game.GameState, fingerprint string) []entity.Relic {
	for _, p := range state.Players {
		if p.Fingerprint != fingerprint {
			continue
		}
		out := make([]entity.Relic, 0, len(p.Relics))
		for _, r := range p.Relics {
			if r.Effect.IsActive() {
				out = append(out, r)
			}
		}
		return out
	}
	return nil
}

func countActiveRelics(state game.GameState, fingerprint string) int {
	return len(playerActiveRelics(state, fingerprint))
}

func renderGame(state game.GameState, width, height int, targetCursor int, allyCursor int, moveCursor int, relicCursor int, chatting bool, chatInput string, fingerprint string, skillMsg string) string {
	mapView := renderMap(state.Floor)
	hudView := renderHUD(state, targetCursor, allyCursor, moveCursor, relicCursor, fingerprint)
	logView := renderCombatLog(state.CombatLog)

	if chatting {
		chatStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
		chatView := chatStyle.Render(fmt.Sprintf("> %s_", chatInput))
		hint := styleSystem.Render("emote: " + strings.Join(game.EmoteList(), " "))
		return lipgloss.JoinVertical(lipgloss.Left, mapView, hudView, logView, chatView, hint)
	}

	parts := []string{mapView, hudView, logView}
	if skillMsg != "" {
		parts = append(parts, styleDamage.Render("  ! "+skillMsg))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
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

// soloChainHint renders the one-line solo-chain status for the combat HUD,
// or "" when nothing is armed. Nil chain (party mode / out of combat) shows
// nothing.
func soloChainHint(chain *game.SoloChainView) string {
	if chain == nil {
		return ""
	}
	switch {
	case chain.LastAction == "skill":
		return "⛓ 연계 준비: 공격 시 잔향 일격(1.4×)!"
	case chain.LastAction == "coop":
		return "⛓ 연계 준비: 공격 시 반격 태세(크리 확정)!"
	case chain.AttackStreak >= 3:
		return fmt.Sprintf("⛓ 연속 공세 지속 중 (%d연타, 1.25×)", chain.AttackStreak)
	case chain.AttackStreak == 2:
		return "⛓ 연계 준비: 공격 시 연속 공세(1.25×)!"
	case chain.AttackStreak == 1:
		return "⛓ 공격 1연타 (3연타부터 연속 공세)"
	}
	return ""
}

func renderHUD(state game.GameState, targetCursor int, allyCursor int, moveCursor int, relicCursor int, fingerprint string) string {
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
		} else if p.Fled {
			status = " [도주]"
		}
		fmt.Fprintf(&sb, "%s (%s) %s %d/%d%s  골드: %d",
			p.Name, p.Class, hpBar, p.HP, p.MaxHP, status, p.Gold)

		// Show inventory count
		itemCount := len(p.Inventory)
		relicCount := len(p.Relics)
		if itemCount > 0 || relicCount > 0 {
			fmt.Fprintf(&sb, "  아이템:%d 유물:%d", itemCount, relicCount)
		}
		sb.WriteString("\n")
	}

	if state.Phase == game.PhaseCombat {
		// Two-panel layout: PARTY | ENEMIES
		// Determine if current player is Healer for ally targeting display
		isHealer := false
		for _, p := range state.Players {
			if p.Fingerprint == fingerprint && p.Class == entity.ClassHealer {
				isHealer = true
				break
			}
		}
		partyContent := renderPartyPanel(state.Players, state.SubmittedActions, isHealer, allyCursor)
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
		coopHint := "[6]협동기"
		if state.CoopAvailable != nil && !state.CoopAvailable[fingerprint] {
			coopHint = styleSystem.Render("[6]협동기(사용됨)")
		}
		// Relic hint: show armed relic name when player owns at least one
		// active relic, plus a [r] cycle hint when more than one is owned.
		relicHint := " [7]유물"
		if actives := playerActiveRelics(state, fingerprint); len(actives) > 0 {
			idx := relicCursor
			if idx < 0 || idx >= len(actives) {
				idx = 0
			}
			if len(actives) > 1 {
				relicHint = fmt.Sprintf(" [7]유물(%s) [r]전환", actives[idx].Name)
			} else {
				relicHint = fmt.Sprintf(" [7]유물(%s)", actives[idx].Name)
			}
		}
		if isHealer {
			sb.WriteString(styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 ") + coopHint + styleAction.Render(relicHint+" [Tab]적 [Shift+Tab]아군 [/]채팅"))
		} else {
			sb.WriteString(styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 ") + coopHint + styleAction.Render(relicHint+" [Tab]대상 [/]채팅"))
		}
		sb.WriteString("\n")

		// Solo chain hint — shows the chain the next attack would trigger.
		if hint := soloChainHint(state.SoloChain); hint != "" {
			sb.WriteString(styleSystem.Render(hint))
			sb.WriteString("\n")
		}

		// Timer
		if !state.TurnDeadline.IsZero() {
			remaining := time.Until(state.TurnDeadline)
			if remaining < 0 {
				remaining = 0
			}
			sb.WriteString(styleTimer.Render(fmt.Sprintf("  타이머: %.1f초", remaining.Seconds())))
			sb.WriteString("\n")
		}

		// Skill description for first available player (skips dead and fled —
		// fled players can't use skills this combat, so their SkillUses count
		// would mislead).
		for _, p := range state.Players {
			if !p.IsOut() {
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
				coopAbility := entity.GetCoopAbility(p.Class)
				coopStatus := "사용가능"
				if state.CoopAvailable == nil || !state.CoopAvailable[p.Fingerprint] {
					coopStatus = "사용됨"
				}
				skillDesc += fmt.Sprintf("\n협동기: %s [%s]", coopAbility.Name, coopStatus)
				sb.WriteString(styleSystem.Render(skillDesc))
				sb.WriteString("\n")
				break
			}
		}
	} else if state.Phase == game.PhaseExploring {
		// Count votes per room for display
		votesPerRoom := make(map[int]int)
		if state.MoveVotes != nil {
			for _, room := range state.MoveVotes {
				votesPerRoom[room]++
			}
		}
		myVoted := false
		if !state.SoloMode && state.MoveVotes != nil {
			_, myVoted = state.MoveVotes[fingerprint]
		}

		if state.Floor != nil && state.Floor.CurrentRoom >= 0 && state.Floor.CurrentRoom < len(state.Floor.Rooms) {
			current := state.Floor.Rooms[state.Floor.CurrentRoom]
			if len(current.Neighbors) > 0 {
				sb.WriteString("\n")
				// Show vote status in multiplayer
				if !state.SoloMode {
					aliveCount := 0
					votedCount := 0
					for _, p := range state.Players {
						// Match SubmitMoveVote's consensus rule (!IsOut) so the
						// counter doesn't show "2/3" while the move actually
						// fires at 2/2 because a fled player isn't required.
						if !p.IsOut() {
							aliveCount++
							if state.MoveVotes != nil {
								if _, ok := state.MoveVotes[p.Fingerprint]; ok {
									votedCount++
								}
							}
						}
					}
					voteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
					if myVoted {
						sb.WriteString(voteStyle.Render(fmt.Sprintf("투표 완료! 대기 중... (%d/%d)", votedCount, aliveCount)))
					} else {
						sb.WriteString(voteStyle.Render(fmt.Sprintf("이동할 방을 선택하세요 (%d/%d 투표)", votedCount, aliveCount)))
					}
					sb.WriteString("\n")
				}
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
						voteInfo := ""
						if !state.SoloMode {
							if count, ok := votesPerRoom[n]; ok {
								voteInfo = fmt.Sprintf(" [%d표]", count)
							}
						}
						sb.WriteString(style.Render(fmt.Sprintf("%s방 %d: %s%s", marker, n, status, voteInfo)))
						sb.WriteString("\n")
					}
				}
			}
		}
		// Show skill tree allocation UI if player has unspent points.
		// Display adapts to which tier is next: branch-select / T2 fork / T3 auto / T4 capstone.
		for _, p := range state.Players {
			if p.Fingerprint == fingerprint && p.Skills != nil && p.Skills.Points > p.Skills.Allocated && p.Skills.Allocated < 4 {
				branches := entity.GetBranches(p.Class)
				sb.WriteString("\n")
				skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
				sb.WriteString(skillStyle.Render(fmt.Sprintf("  스킬 포인트 사용 가능! (미사용: %d)", p.Skills.Points-p.Skills.Allocated)))
				sb.WriteString("\n")
				if p.Skills.BranchIndex == -1 {
					// Branch-select stage: each key picks a branch (and auto-takes T1).
					for i, tree := range branches {
						key := "["
						if i == 1 {
							key = "]"
						}
						fmt.Fprintf(&sb, "    [%s] %s -> %s (%s)\n", key, tree.Name, tree.T1.Name, tree.T1.EffectLabel())
					}
				} else {
					tree := branches[p.Skills.BranchIndex]
					otherTree := branches[1-p.Skills.BranchIndex]
					otherKey := "["
					if p.Skills.BranchIndex == 0 {
						otherKey = "]"
					}
					// Show the locked branch line.
					fmt.Fprintf(&sb, "    [%s] %s (잠김)\n", otherKey, otherTree.Name)
					// Determine fork or auto next.
					switch p.Skills.Allocated {
					case 1:
						// T2 fork
						fmt.Fprintf(&sb, "    [[] %s -> %s (%s)\n", tree.Name, tree.T2[0].Name, tree.T2[0].EffectLabel())
						fmt.Fprintf(&sb, "    []] %s -> %s (%s)\n", tree.Name, tree.T2[1].Name, tree.T2[1].EffectLabel())
					case 2:
						// T3 auto — both keys do the same.
						fmt.Fprintf(&sb, "    [[/]] %s -> %s (%s, 자동)\n", tree.Name, tree.T3.Name, tree.T3.EffectLabel())
					case 3:
						// T4 capstone fork
						fmt.Fprintf(&sb, "    [[] %s -> %s (%s) ★ 캡스톤\n", tree.Name, tree.T4[0].Name, tree.T4[0].EffectLabel())
						fmt.Fprintf(&sb, "    []] %s -> %s (%s) ★ 캡스톤\n", tree.Name, tree.T4[1].Name, tree.T4[1].EffectLabel())
					}
				}
				break
			}
		}
		if state.NextRoomPreview != nil {
			previewStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
			line := fmt.Sprintf("\n[정찰] 다음 방: %s", state.NextRoomPreview.RoomTypeLabel)
			if !state.NextRoomPreview.HideMonsters && len(state.NextRoomPreview.MonsterNames) > 0 {
				line += " — " + strings.Join(state.NextRoomPreview.MonsterNames, ", ")
			}
			sb.WriteString(previewStyle.Render(line))
			sb.WriteString("\n")
		}
		myCanScout := false
		for _, p := range state.Players {
			if p.Fingerprint == fingerprint && p.Class == entity.ClassRogue {
				if state.CoopAvailable != nil && state.CoopAvailable[fingerprint] {
					myCanScout = true
				}
				break
			}
		}
		if !state.SoloMode && myVoted {
			sb.WriteString("[Q] 종료  — 다른 파티원의 투표를 기다리는 중...")
		} else if myCanScout {
			sb.WriteString("[Up/Down] 선택  [Enter] 이동  [6] 정찰  [Q] 종료")
		} else {
			sb.WriteString("[Up/Down] 선택  [Enter] 이동  [Q] 종료")
		}
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
	case strings.Contains(msg, "AEGIS") || strings.Contains(msg, "CRUSADER") || strings.Contains(msg, "EXPOSE") || strings.Contains(msg, "SANCTUARY") || strings.Contains(msg, "MIRAGE") || strings.Contains(msg, "VENOMOUS") || strings.Contains(msg, "강철결계") || strings.Contains(msg, "성전사") || strings.Contains(msg, "급소") || strings.Contains(msg, "성역") || strings.Contains(msg, "환영진") || strings.Contains(msg, "독칼"):
		return styleCombo.Render(msg)
	case strings.Contains(msg, "견고함") || strings.Contains(msg, "결계") || strings.Contains(msg, "신성 가호") || strings.Contains(msg, "정찰"):
		return styleCoop.Render(msg)
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

func renderPartyPanel(players []*entity.Player, submittedActions map[string]string, showAllyCursor bool, allyCursor int) string {
	var sb strings.Builder
	sb.WriteString(styleHeader.Render("  아군") + "\n\n")

	for i, p := range players {
		marker := "  ♦"
		if showAllyCursor && i == allyCursor {
			marker = " >♦"
		}
		nameStr := stylePlayer.Render(fmt.Sprintf("%s %s", marker, p.Name))
		classStr := styleSystem.Render(fmt.Sprintf(" (%s)", p.Class))
		status := ""
		if p.IsDead() {
			status = styleDamage.Render(" [사망]")
		} else if p.Fled {
			status = styleFlee.Render(" [도주]")
		}
		sb.WriteString(nameStr + classStr + status + "\n")

		hpBar := renderHPBar(p.HP, p.MaxHP, 16)
		fmt.Fprintf(&sb, "    %s %d/%d\n", hpBar, p.HP, p.MaxHP)

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

		fmt.Fprintf(&sb, "    ATK:%-3d DEF:%-3d  ", p.EffectiveATK(), p.EffectiveDEF())
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
		fmt.Fprintf(&sb, "  %s[%d] %s %s %d/%d%s\n\n",
			marker, i, styleEnemy.Render(m.Name), hpBar, m.HP, m.MaxHP, taunt)
	}
	return sb.String()
}

// --- best-effort persistence helpers ---------------------------------------
// These store writes fire from the UI tick and have no user-facing error
// channel; silently dropping them meant lost codex entries, souls, and
// achievements with zero trace. Logging keeps failures visible to the
// operator without interrupting play.

func recordCodex(ctx *Context, category, name string) {
	if ctx.Store == nil {
		return
	}
	if err := ctx.Store.RecordCodexEntry(ctx.Fingerprint, category, name); err != nil {
		slog.Error("codex record failed", "error", err, "category", category, "name", name)
	}
}
