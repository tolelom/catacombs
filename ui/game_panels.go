package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolelom/catacombs/entity"
)

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
		writePartyMember(&sb, p, showAllyCursor && i == allyCursor, submittedActions)
	}
	return sb.String()
}

func writePartyMember(sb *strings.Builder, p *entity.Player, selected bool, submittedActions map[string]string) {
	sb.WriteString(partyMemberTitle(p, selected) + "\n")
	fmt.Fprintf(sb, "    %s %d/%d\n", renderHPBar(p.HP, p.MaxHP, 16), p.HP, p.MaxHP)
	if len(p.Effects) > 0 {
		sb.WriteString("    " + strings.Join(effectBadges(p.Effects), " ") + "\n")
	}
	fmt.Fprintf(sb, "    ATK:%-3d DEF:%-3d  ", p.EffectiveATK(), p.EffectiveDEF())
	sb.WriteString(styleGold.Render(fmt.Sprintf("G:%d", p.Gold)))
	sb.WriteString("\n")
	writeActionStatus(sb, p, submittedActions)
	sb.WriteString("\n")
}

func partyMemberTitle(p *entity.Player, selected bool) string {
	marker := "  ♦"
	if selected {
		marker = " >♦"
	}
	status := ""
	if p.IsDead() {
		status = styleDamage.Render(" [사망]")
	} else if p.Fled {
		status = styleFlee.Render(" [도주]")
	}
	return stylePlayer.Render(fmt.Sprintf("%s %s", marker, p.Name)) + styleSystem.Render(fmt.Sprintf(" (%s)", p.Class)) + status
}

func effectBadges(effects []entity.ActiveEffect) []string {
	var badges []string
	for _, e := range effects {
		switch e.Type {
		case entity.StatusPoison:
			badges = append(badges, styleHeal.Render(fmt.Sprintf("☠Poison(%dt)", e.Duration)))
		case entity.StatusBurn:
			badges = append(badges, styleDamage.Render(fmt.Sprintf("🔥Burn(%dt)", e.Duration)))
		case entity.StatusFreeze:
			badges = append(badges, styleFlee.Render(fmt.Sprintf("❄Freeze(%dt)", e.Duration)))
		}
	}
	return badges
}

func writeActionStatus(sb *strings.Builder, p *entity.Player, submittedActions map[string]string) {
	if action, ok := submittedActions[p.Fingerprint]; ok {
		sb.WriteString(styleHeal.Render(fmt.Sprintf("    ✓ %s", action)))
		sb.WriteString("\n")
		return
	}
	if !p.IsOut() {
		sb.WriteString(styleSystem.Render("    ... 대기중"))
		sb.WriteString("\n")
	}
}

func renderEnemyPanel(monsters []*entity.Monster, targetCursor int) string {
	var sb strings.Builder
	sb.WriteString(styleHeader.Render("  적") + "\n\n")

	for i, m := range monsters {
		if m.IsDead() {
			continue
		}

		art := MonsterArt(m.Type)
		for _, line := range art {
			sb.WriteString(styleEnemy.Render("  "+line) + "\n")
		}

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
