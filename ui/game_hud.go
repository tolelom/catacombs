package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

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

	writePlayerSummaries(&sb, state.Players)
	switch state.Phase {
	case game.PhaseCombat:
		writeCombatHUD(&sb, state, targetCursor, allyCursor, relicCursor, fingerprint)
	case game.PhaseExploring:
		writeExplorationHUD(&sb, state, moveCursor, fingerprint)
	}

	if state.Phase == game.PhaseCombat {
		return sb.String()
	}
	return border.Render(sb.String())
}

func writePlayerSummaries(sb *strings.Builder, players []*entity.Player) {
	for _, p := range players {
		hpBar := renderHPBar(p.HP, p.MaxHP, 20)
		status := ""
		if p.IsDead() {
			status = " [사망]"
		} else if p.Fled {
			status = " [도주]"
		}
		fmt.Fprintf(sb, "%s (%s) %s %d/%d%s  골드: %d",
			p.Name, p.Class, hpBar, p.HP, p.MaxHP, status, p.Gold)

		itemCount := len(p.Inventory)
		relicCount := len(p.Relics)
		if itemCount > 0 || relicCount > 0 {
			fmt.Fprintf(sb, "  아이템:%d 유물:%d", itemCount, relicCount)
		}
		sb.WriteString("\n")
	}
}

func writeCombatHUD(sb *strings.Builder, state game.GameState, targetCursor, allyCursor, relicCursor int, fingerprint string) {
	isHealer := viewerIsHealer(state.Players, fingerprint)
	sb.WriteString(renderCombatPanels(state, targetCursor, allyCursor, isHealer))
	sb.WriteString("\n")
	sb.WriteString(combatActionBar(state, relicCursor, fingerprint, isHealer))
	sb.WriteString("\n")
	writeSoloChainHint(sb, state.SoloChain)
	writeTurnTimer(sb, state.TurnDeadline)
	writeFirstActiveSkillInfo(sb, state)
}

func viewerIsHealer(players []*entity.Player, fingerprint string) bool {
	for _, p := range players {
		if p.Fingerprint == fingerprint && p.Class == entity.ClassHealer {
			return true
		}
	}
	return false
}

func renderCombatPanels(state game.GameState, targetCursor, allyCursor int, isHealer bool) string {
	partyPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorGray).
		Width(35).
		Padding(0, 1).
		Render(renderPartyPanel(state.Players, state.SubmittedActions, isHealer, allyCursor))

	enemyPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorGray).
		Width(38).
		Padding(0, 1).
		Render(renderEnemyPanel(state.Monsters, targetCursor))

	return lipgloss.JoinHorizontal(lipgloss.Top, partyPanel, enemyPanel)
}

func combatActionBar(state game.GameState, relicCursor int, fingerprint string, isHealer bool) string {
	coopHint := "[6]협동기"
	if state.CoopAvailable != nil && !state.CoopAvailable[fingerprint] {
		coopHint = styleSystem.Render("[6]협동기(사용됨)")
	}
	relicHint := activeRelicHint(state, relicCursor, fingerprint)
	targetKeys := " [Tab]대상 [/]채팅"
	if isHealer {
		targetKeys = " [Tab]적 [Shift+Tab]아군 [/]채팅"
	}
	return styleAction.Render("[1]공격 [2]스킬 [3]아이템 [4]도주 [5]대기 ") + coopHint + styleAction.Render(relicHint+targetKeys)
}

func activeRelicHint(state game.GameState, relicCursor int, fingerprint string) string {
	actives := playerActiveRelics(state, fingerprint)
	if len(actives) == 0 {
		return " [7]유물"
	}
	idx := relicCursor
	if idx < 0 || idx >= len(actives) {
		idx = 0
	}
	if len(actives) > 1 {
		return fmt.Sprintf(" [7]유물(%s) [r]전환", actives[idx].Name)
	}
	return fmt.Sprintf(" [7]유물(%s)", actives[idx].Name)
}

func writeSoloChainHint(sb *strings.Builder, chain *game.SoloChainView) {
	if hint := soloChainHint(chain); hint != "" {
		sb.WriteString(styleSystem.Render(hint))
		sb.WriteString("\n")
	}
}

func writeTurnTimer(sb *strings.Builder, deadline time.Time) {
	if deadline.IsZero() {
		return
	}
	remaining := max(time.Until(deadline), 0)
	sb.WriteString(styleTimer.Render(fmt.Sprintf("  타이머: %.1f초", remaining.Seconds())))
	sb.WriteString("\n")
}

func writeFirstActiveSkillInfo(sb *strings.Builder, state game.GameState) {
	for _, p := range state.Players {
		if p.IsOut() {
			continue
		}
		sb.WriteString(styleSystem.Render(skillInfo(p, state.CoopAvailable)))
		sb.WriteString("\n")
		return
	}
}

func skillInfo(p *entity.Player, coopAvailable map[string]bool) string {
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
	coopStatus := "사용가능"
	if coopAvailable == nil || !coopAvailable[p.Fingerprint] {
		coopStatus = "사용됨"
	}
	skillDesc += fmt.Sprintf("\n협동기: %s [%s]", entity.GetCoopAbility(p.Class).Name, coopStatus)
	return skillDesc
}

func writeExplorationHUD(sb *strings.Builder, state game.GameState, moveCursor int, fingerprint string) {
	myVoted := viewerVotedToMove(state, fingerprint)
	writeRoomChoices(sb, state, moveCursor, myVoted)
	writeSkillPointPrompt(sb, state.Players, fingerprint)
	writeRoomPreview(sb, state.NextRoomPreview)
	sb.WriteString(explorationKeyHints(state, fingerprint, myVoted))
}

func viewerVotedToMove(state game.GameState, fingerprint string) bool {
	if state.SoloMode || state.MoveVotes == nil {
		return false
	}
	_, voted := state.MoveVotes[fingerprint]
	return voted
}

func writeRoomChoices(sb *strings.Builder, state game.GameState, moveCursor int, myVoted bool) {
	if state.Floor == nil || state.Floor.CurrentRoom < 0 || state.Floor.CurrentRoom >= len(state.Floor.Rooms) {
		return
	}
	current := state.Floor.Rooms[state.Floor.CurrentRoom]
	if len(current.Neighbors) == 0 {
		return
	}
	sb.WriteString("\n")
	if !state.SoloMode {
		writeMoveVoteStatus(sb, state, myVoted)
	}
	votesPerRoom := countMoveVotes(state.MoveVotes)
	for i, n := range current.Neighbors {
		if n < 0 || n >= len(state.Floor.Rooms) {
			continue
		}
		sb.WriteString(roomChoiceLine(state, i, n, moveCursor, votesPerRoom))
		sb.WriteString("\n")
	}
}

func writeMoveVoteStatus(sb *strings.Builder, state game.GameState, myVoted bool) {
	aliveCount := 0
	votedCount := 0
	for _, p := range state.Players {
		if p.IsOut() {
			continue
		}
		aliveCount++
		if _, ok := state.MoveVotes[p.Fingerprint]; ok {
			votedCount++
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

func countMoveVotes(moveVotes map[string]int) map[int]int {
	votesPerRoom := make(map[int]int)
	for _, room := range moveVotes {
		votesPerRoom[room]++
	}
	return votesPerRoom
}

func roomChoiceLine(state game.GameState, choiceIdx, roomIdx, moveCursor int, votesPerRoom map[int]int) string {
	r := state.Floor.Rooms[roomIdx]
	status := r.Type.String()
	if r.Cleared {
		status = "클리어"
	}
	marker := "  "
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	if choiceIdx == moveCursor {
		marker = "> "
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	}
	voteInfo := ""
	if !state.SoloMode {
		if count, ok := votesPerRoom[roomIdx]; ok {
			voteInfo = fmt.Sprintf(" [%d표]", count)
		}
	}
	return style.Render(fmt.Sprintf("%s방 %d: %s%s", marker, roomIdx, status, voteInfo))
}

func writeSkillPointPrompt(sb *strings.Builder, players []*entity.Player, fingerprint string) {
	for _, p := range players {
		if p.Fingerprint != fingerprint || p.Skills == nil || p.Skills.Points <= p.Skills.Allocated || p.Skills.Allocated >= 4 {
			continue
		}
		sb.WriteString("\n")
		skillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
		sb.WriteString(skillStyle.Render(fmt.Sprintf("  스킬 포인트 사용 가능! (미사용: %d)", p.Skills.Points-p.Skills.Allocated)))
		sb.WriteString("\n")
		writeSkillBranchChoices(sb, p.Class, p.Skills)
		return
	}
}

func writeSkillBranchChoices(sb *strings.Builder, class entity.Class, skills *entity.PlayerSkills) {
	branches := entity.GetBranches(class)
	if skills.BranchIndex == -1 {
		for i, tree := range branches {
			key := "["
			if i == 1 {
				key = "]"
			}
			fmt.Fprintf(sb, "    [%s] %s -> %s (%s)\n", key, tree.Name, tree.T1.Name, tree.T1.EffectLabel())
		}
		return
	}
	tree := branches[skills.BranchIndex]
	otherTree := branches[1-skills.BranchIndex]
	otherKey := "["
	if skills.BranchIndex == 0 {
		otherKey = "]"
	}
	fmt.Fprintf(sb, "    [%s] %s (잠김)\n", otherKey, otherTree.Name)
	switch skills.Allocated {
	case 1:
		fmt.Fprintf(sb, "    [[] %s -> %s (%s)\n", tree.Name, tree.T2[0].Name, tree.T2[0].EffectLabel())
		fmt.Fprintf(sb, "    []] %s -> %s (%s)\n", tree.Name, tree.T2[1].Name, tree.T2[1].EffectLabel())
	case 2:
		fmt.Fprintf(sb, "    [[/]] %s -> %s (%s, 자동)\n", tree.Name, tree.T3.Name, tree.T3.EffectLabel())
	case 3:
		fmt.Fprintf(sb, "    [[] %s -> %s (%s) ★ 캡스톤\n", tree.Name, tree.T4[0].Name, tree.T4[0].EffectLabel())
		fmt.Fprintf(sb, "    []] %s -> %s (%s) ★ 캡스톤\n", tree.Name, tree.T4[1].Name, tree.T4[1].EffectLabel())
	}
}

func writeRoomPreview(sb *strings.Builder, preview *game.RoomPreview) {
	if preview == nil {
		return
	}
	previewStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	line := fmt.Sprintf("\n[정찰] 다음 방: %s", preview.RoomTypeLabel)
	if !preview.HideMonsters && len(preview.MonsterNames) > 0 {
		line += " — " + strings.Join(preview.MonsterNames, ", ")
	}
	sb.WriteString(previewStyle.Render(line))
	sb.WriteString("\n")
}

func explorationKeyHints(state game.GameState, fingerprint string, myVoted bool) string {
	if !state.SoloMode && myVoted {
		return "[Q] 종료  — 다른 파티원의 투표를 기다리는 중..."
	}
	if viewerCanScout(state, fingerprint) {
		return "[Up/Down] 선택  [Enter] 이동  [6] 정찰  [Q] 종료"
	}
	return "[Up/Down] 선택  [Enter] 이동  [Q] 종료"
}

func viewerCanScout(state game.GameState, fingerprint string) bool {
	for _, p := range state.Players {
		if p.Fingerprint == fingerprint && p.Class == entity.ClassRogue {
			return state.CoopAvailable != nil && state.CoopAvailable[fingerprint]
		}
	}
	return false
}
