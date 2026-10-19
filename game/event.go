package game

import (
	"fmt"

	"github.com/tolelom/catacombs/entity"
)

func (s *GameSession) applyEventOutcome(o EventOutcome) {
	alive := s.activePlayersLocked()
	if len(alive) == 0 {
		return
	}
	charmProtected := s.consumeCharmsAgainstLossLocked(o, alive)
	s.applyEventHPChangeLocked(o.HPChange, alive, charmProtected)
	s.applyEventGoldChangeLocked(o.GoldChange, alive, charmProtected)
	s.applyEventItemsLocked(o, alive)
}

func (s *GameSession) activePlayersLocked() []*entity.Player {
	alive := make([]*entity.Player, 0, len(s.state.Players))
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive = append(alive, p)
		}
	}
	return alive
}

func (s *GameSession) consumeCharmsAgainstLossLocked(o EventOutcome, alive []*entity.Player) map[string]bool {
	charmProtected := make(map[string]bool)
	if !o.Gambled || (o.HPChange >= 0 && o.GoldChange >= 0) {
		return charmProtected
	}
	for _, p := range alive {
		if p.ConsumeCharm() {
			charmProtected[p.Fingerprint] = true
			s.addLog(fmt.Sprintf("  🍀 %s의 행운 부적이 불운을 막아냈다!", p.Name))
		}
	}
	return charmProtected
}

func (s *GameSession) applyEventHPChangeLocked(change int, alive []*entity.Player, charmProtected map[string]bool) {
	if change > 0 {
		for _, p := range alive {
			before := p.HP
			p.Heal(change)
			s.addLog(fmt.Sprintf("  %s HP %d 회복", p.Name, p.HP-before))
		}
		return
	}
	if change == 0 {
		return
	}
	dmg := -change
	for _, p := range alive {
		if charmProtected[p.Fingerprint] {
			continue
		}
		p.TakeDamage(dmg)
		s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, dmg))
		if p.IsDead() {
			s.handlePlayerDeathLocked(p)
		}
	}
}

func (s *GameSession) applyEventGoldChangeLocked(total int, alive []*entity.Player, charmProtected map[string]bool) {
	if total == 0 {
		return
	}
	share := total / len(alive)
	remainder := total - share*len(alive)
	for i, p := range alive {
		delta := share
		if i == 0 {
			delta += remainder
		}
		if delta < 0 && charmProtected[p.Fingerprint] {
			continue
		}
		p.Gold = max(p.Gold+delta, 0)
		if delta > 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 획득", p.Name, delta))
		} else if delta < 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", p.Name, -delta))
		}
	}
}

func (s *GameSession) applyEventItemsLocked(o EventOutcome, alive []*entity.Player) {
	if o.Item != nil {
		for _, p := range alive {
			s.grantEventItemLocked(p, *o.Item)
		}
		return
	}
	if o.ItemDrop {
		for _, p := range alive {
			s.rollEventItemDropLocked(p)
		}
	}
}

func (s *GameSession) grantEventItemLocked(p *entity.Player, item entity.Item) {
	if len(p.Inventory) < s.cfg.Game.InventoryLimit {
		p.Inventory = append(p.Inventory, item)
		s.addLog(fmt.Sprintf("  %s %s 획득", p.Name, item.Name))
		return
	}
	comp := item.Bonus * 5
	p.Gold += comp
	s.addLog(fmt.Sprintf("  %s 인벤토리 가득 → +%dG로 전환", p.Name, comp))
}

func (s *GameSession) rollEventItemDropLocked(p *entity.Player) {
	if len(p.Inventory) >= s.cfg.Game.InventoryLimit {
		s.addLog(fmt.Sprintf("  %s의 인벤토리가 가득 찼습니다!", p.Name))
		return
	}
	floor := s.state.FloorNum
	if s.rng.Float64() < 0.5 {
		bonus := 3 + s.rng.Intn(6) + floor/3
		item := entity.Item{Name: weaponName(floor), Type: entity.ItemWeapon, Bonus: bonus}
		p.Inventory = append(p.Inventory, item)
		s.addLog(fmt.Sprintf("  %s %s 발견 (ATK+%d)", p.Name, item.Name, item.Bonus))
		return
	}
	bonus := 2 + s.rng.Intn(4) + floor/4
	item := entity.Item{Name: armorName(floor), Type: entity.ItemArmor, Bonus: bonus}
	p.Inventory = append(p.Inventory, item)
	s.addLog(fmt.Sprintf("  %s %s 발견 (DEF+%d)", p.Name, item.Name, item.Bonus))
}

func (s *GameSession) resolveEventVoteLocked() {
	if s.pendingEvent == nil {
		s.clearPendingEventLocked(true)
		return
	}
	if s.eventPerPlayer {
		s.resolveLootPicksLocked()
		return
	}

	tally := make(map[int]int, len(s.pendingEvent.Choices))
	for _, idx := range s.eventVotes {
		tally[idx]++
	}
	winnerIdx := -1
	bestCount := -1
	for idx, count := range tally {
		if count > bestCount || (count == bestCount && idx < winnerIdx) {
			bestCount = count
			winnerIdx = idx
		}
	}
	if winnerIdx < 0 || winnerIdx >= len(s.pendingEvent.Choices) {
		s.clearPendingEventLocked(true)
		return
	}
	choice := s.pendingEvent.Choices[winnerIdx]
	outcome := choice.Resolve(s.state.FloorNum, s.rng)
	outcome.Gambled = tagsContain(choice.Tags, TagGamble)
	s.addLog(fmt.Sprintf("  → %s: %s", choice.Label, outcome.Description))

	s.applyEventOutcome(outcome)
	s.clearPendingEventLocked(true)
	s.endRunIfPartyWipedLocked()
}

func (s *GameSession) triggerEvent() {
	event := PickRandomEvent(s.rng)
	s.state.LastEventName = event.Name
	s.addLog(fmt.Sprintf("이벤트: %s — %s", event.Name, event.Description))

	alive := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive++
		}
	}
	if alive == 0 {
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
		return
	}

	s.startEventPhaseLocked(event, false)
}

func (s *GameSession) triggerLoot() {
	alive := 0
	for _, p := range s.state.Players {
		if !p.IsOut() {
			alive++
		}
	}
	if alive == 0 {
		if s.state.Floor != nil && s.state.Floor.CurrentRoom >= 0 && s.state.Floor.CurrentRoom < len(s.state.Floor.Rooms) {
			s.state.Floor.Rooms[s.state.Floor.CurrentRoom].Cleared = true
		}
		return
	}

	event := BuildLootEvent(s.state.FloorNum, s.rng)
	s.addLog("보물 발견! 각자 하나를 선택하세요.")
	s.startEventPhaseLocked(event, true)
}

func (s *GameSession) startEventPhaseLocked(event RandomEvent, perPlayer bool) {
	ev := event
	s.pendingEvent = &ev
	s.eventPerPlayer = perPlayer

	view := &PendingEventView{
		Name:        event.Name,
		Description: event.Description,
		PerPlayer:   perPlayer,
		Choices:     make([]EventChoiceView, 0, len(event.Choices)),
	}
	for _, c := range event.Choices {
		tagsCopy := make([]ChoiceTag, len(c.Tags))
		copy(tagsCopy, c.Tags)
		view.Choices = append(view.Choices, EventChoiceView{Label: c.Label, Tags: tagsCopy})
	}
	s.state.PendingEvent = view
	s.state.EventVotes = make(map[string]int)
	s.eventVotes = make(map[string]int)
	s.state.Phase = PhaseEvent
}

func (s *GameSession) resolveLootPicksLocked() {
	floor := s.state.FloorNum
	for _, p := range s.state.Players {
		if p.IsOut() {
			continue
		}
		idx, ok := s.eventVotes[p.Fingerprint]
		if !ok || idx < 0 || idx >= len(s.pendingEvent.Choices) {
			continue
		}
		choice := s.pendingEvent.Choices[idx]
		outcome := choice.Resolve(floor, s.rng)
		outcome.Gambled = tagsContain(choice.Tags, TagGamble)
		s.addLog(fmt.Sprintf("  %s → %s", p.Name, outcome.Description))
		s.applyOutcomeToPlayer(p, outcome)
	}
	s.clearPendingEventLocked(true)
	s.endRunIfPartyWipedLocked()
}

func (s *GameSession) applyOutcomeToPlayer(p *entity.Player, o EventOutcome) {
	hpChange, goldChange := o.HPChange, o.GoldChange

	if o.Gambled && (hpChange < 0 || goldChange < 0) && p.ConsumeCharm() {
		s.addLog(fmt.Sprintf("  🍀 %s의 행운 부적이 불운을 막아냈다!", p.Name))
		if hpChange < 0 {
			hpChange = 0
		}
		if goldChange < 0 {
			goldChange = 0
		}
	}

	if hpChange > 0 {
		before := p.HP
		p.Heal(hpChange)
		s.addLog(fmt.Sprintf("  %s HP %d 회복", p.Name, p.HP-before))
	} else if hpChange < 0 {
		p.TakeDamage(-hpChange)
		if p.Dead {
			s.state.AnyPlayerDied = true
		}
		s.addLog(fmt.Sprintf("  %s %d 피해를 받음", p.Name, -hpChange))
	}

	if goldChange != 0 {
		p.Gold += goldChange
		if p.Gold < 0 {
			p.Gold = 0
		}
		if goldChange > 0 {
			s.addLog(fmt.Sprintf("  %s 골드 %d 획득", p.Name, goldChange))
		} else {
			s.addLog(fmt.Sprintf("  %s 골드 %d 잃음", p.Name, -goldChange))
		}
	}

	if o.Item != nil {
		if len(p.Inventory) < s.cfg.Game.InventoryLimit {
			p.Inventory = append(p.Inventory, *o.Item)
			s.addLog(fmt.Sprintf("  %s %s 획득", p.Name, o.Item.Name))
		} else {
			comp := o.Item.Bonus * 5
			p.Gold += comp
			s.addLog(fmt.Sprintf("  %s 인벤토리 가득 → +%dG로 전환", p.Name, comp))
		}
	}
}
