package ui

import (
	"log/slog"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
)

type GameScreen struct {
	gameState     game.GameState
	targetCursor  int
	allyCursor    int
	moveCursor    int
	relicCursor   int
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
	s.refresh(ctx)
	if next, cmd, routed := s.routeByPhase(ctx); routed {
		return next, cmd
	}
	switch msg := msg.(type) {
	case tickMsg:
		return s.onTick(ctx)
	case tea.KeyMsg:
		return s.handleKey(msg, ctx)
	}
	return s, nil
}

func (s *GameScreen) refresh(ctx *Context) {
	if ctx.Session == nil {
		return
	}
	s.gameState = ctx.Session.GetState()
	s.clampTargetCursor()
	s.recordMonsterCodex(ctx)
	s.recordShopCodex(ctx)
	s.recordOwnedItemCodex(ctx)
	s.recordEventCodex(ctx)
	s.prevPhase = s.gameState.Phase
}

func (s *GameScreen) clampTargetCursor() {
	if len(s.gameState.Monsters) > 0 {
		if s.targetCursor >= len(s.gameState.Monsters) {
			s.targetCursor = len(s.gameState.Monsters) - 1
		}
	} else {
		s.targetCursor = 0
	}
}

func (s *GameScreen) recordMonsterCodex(ctx *Context) {
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
}

func (s *GameScreen) recordShopCodex(ctx *Context) {
	if ctx.Store != nil && s.gameState.Phase == game.PhaseShop && s.prevPhase != game.PhaseShop {
		for _, item := range s.gameState.ShopItems {
			key := "item:" + item.Name
			if !s.codexRecorded[key] {
				recordCodex(ctx, "item", item.Name)
				s.codexRecorded[key] = true
			}
		}
	}
}

func (s *GameScreen) recordOwnedItemCodex(ctx *Context) {
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
}

func (s *GameScreen) recordEventCodex(ctx *Context) {
	if ctx.Store != nil && s.gameState.LastEventName != "" {
		key := "event:" + s.gameState.LastEventName
		if !s.codexRecorded[key] {
			recordCodex(ctx, "event", s.gameState.LastEventName)
			s.codexRecorded[key] = true
		}
	}
}

func (s *GameScreen) routeByPhase(ctx *Context) (Screen, tea.Cmd, bool) {
	if s.gameState.GameOver {
		return NewResultScreen(s.gameState, ctx.Store != nil), nil, true
	}
	if s.gameState.Phase == game.PhaseEvent {
		es := NewEventScreen(s.gameState)
		return es, es.pollState(ctx), true
	}
	if s.gameState.Phase == game.PhaseShop {
		ss := NewShopScreen(s.gameState)
		return ss, ss.pollState(ctx), true
	}
	return nil, nil, false
}

func (s *GameScreen) onTick(ctx *Context) (Screen, tea.Cmd) {
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

func (s *GameScreen) handleKey(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
	if s.chatting {
		return s.handleChatKey(key, ctx)
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
		return s.handleExploringKey(key, ctx)
	case game.PhaseCombat:
		return s.handleCombatKey(key, ctx)
	}
	return s, nil
}

func (s *GameScreen) handleChatKey(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
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

func (s *GameScreen) handleExploringKey(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
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
	if isKey(key, "[") || isKey(key, "]") {
		return s.allocateSkillPoint(key, ctx)
	}
	neighbors := s.getNeighbors()
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
	return s, nil
}

func (s *GameScreen) allocateSkillPoint(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
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
			branchIdx = localSkills.BranchIndex
			if localSkills.NextIsBranch() {
				if isKey(key, "[") {
					choice = 'A'
				} else {
					choice = 'B'
				}
			}
		} else {
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

func (s *GameScreen) handleCombatKey(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
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
			if len(s.gameState.Players) > 0 {
				s.allyCursor = (s.allyCursor + 1) % len(s.gameState.Players)
			}
		} else {
			if len(s.gameState.Monsters) > 0 {
				s.targetCursor = (s.targetCursor + 1) % len(s.gameState.Monsters)
			}
		}
		return s, s.pollState(ctx)
	}
	if ctx.Session != nil {
		return s.submitCombatKey(key, ctx)
	}
	return s, nil
}

func (s *GameScreen) submitCombatKey(key tea.KeyMsg, ctx *Context) (Screen, tea.Cmd) {
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

func (s *GameScreen) View(ctx *Context) string {
	return renderGame(s.gameState, ctx.Width, ctx.Height, s.targetCursor, s.allyCursor, s.moveCursor, s.relicCursor, s.chatting, s.chatInput, ctx.Fingerprint, s.skillMsg)
}

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

func recordCodex(ctx *Context, category, name string) {
	if ctx.Store == nil {
		return
	}
	if err := ctx.Store.RecordCodexEntry(ctx.Fingerprint, category, name); err != nil {
		slog.Error("codex record failed", "error", err, "category", category, "name", name)
	}
}
