package game

import (
	"fmt"
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func setupCombatCoop(t *testing.T, classes ...entity.Class) *GameSession {
	t.Helper()
	s := NewGameSession(testCfg(t))
	for i, c := range classes {
		p := entity.NewPlayer(fmt.Sprintf("P%d", i), c)
		p.Fingerprint = fmt.Sprintf("fp-%d", i)
		s.AddPlayer(p)
	}
	s.state.SoloMode = len(classes) == 1
	s.state.Phase = PhaseCombat
	s.state.Monsters = []*entity.Monster{
		{Name: "Slime", HP: 100, MaxHP: 100, ATK: 20, DEF: 0},
	}
	s.coopUsedThisCombat = make(map[string]bool)
	return s
}

func TestWarriorCoopReducesIncomingDamageInSession(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior, entity.ClassMage)
	warrior := s.state.Players[0]
	hpBefore := warrior.HP
	s.actions = map[string]PlayerAction{
		warrior.Fingerprint:            {Type: ActionCoop},
		s.state.Players[1].Fingerprint: {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.resolveMonsterActions()
	s.state.TurnResolving = false
	dmg := hpBefore - warrior.HP
	if dmg > 6 {
		t.Errorf("Bulwark expected to reduce damage substantially; took %d", dmg)
	}
}

func TestHealerCoopHealsParty(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	w := s.state.Players[1]
	w.HP = w.MaxHP / 2
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionCoop},
		w.Fingerprint:      {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if w.HP <= w.MaxHP/2 {
		t.Errorf("warrior should have been healed; HP=%d MaxHP=%d", w.HP, w.MaxHP)
	}
}

func TestHealerCoopRevivesDeadAlly(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	w := s.state.Players[1]
	w.TakeDamage(w.HP)
	if !w.IsDead() {
		t.Fatal("warrior should be dead before coop")
	}
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionCoop},
		w.Fingerprint:      {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if w.IsDead() {
		t.Error("warrior should have been revived by Divine Grace")
	}
	if !s.healerReviveUsed[healer.Fingerprint] {
		t.Error("healerReviveUsed should be set after revive")
	}
}

func TestCoopUsedOncePerCombat(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	warrior := s.state.Players[0]
	s.actions = map[string]PlayerAction{warrior.Fingerprint: {Type: ActionCoop}}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if !s.coopUsedThisCombat[warrior.Fingerprint] {
		t.Fatal("coopUsedThisCombat should be set after first use")
	}
	logsBefore := len(s.state.PendingLogs) + len(s.state.CombatLog)
	s.actions = map[string]PlayerAction{warrior.Fingerprint: {Type: ActionCoop}}
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	logsAfter := len(s.state.PendingLogs) + len(s.state.CombatLog)
	if logsAfter <= logsBefore {
		t.Error("expected a refusal log on second coop use")
	}
}

func TestCoopComboCrusaderBoostsHeal(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassHealer, entity.ClassWarrior)
	healer := s.state.Players[0]
	w := s.state.Players[1]
	w.HP = 1
	hpBefore := w.HP
	s.actions = map[string]PlayerAction{
		healer.Fingerprint: {Type: ActionCoop},
		w.Fingerprint:      {Type: ActionCoop},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	healed := w.HP - hpBefore
	if healed <= w.MaxHP*30/100 {
		t.Errorf("Crusader should boost heal beyond base 30%%; healed=%d MaxHP=%d", healed, w.MaxHP)
	}
}

func TestCoopComboExposeWeaknessSetsCritFlag(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior, entity.ClassRogue)
	w := s.state.Players[0]
	r := s.state.Players[1]
	s.actions = map[string]PlayerAction{
		w.Fingerprint: {Type: ActionCoop},
		r.Fingerprint: {Type: ActionCoop},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
	if !s.critGuaranteedNext {
		t.Error("Expose Weakness combo should set critGuaranteedNext")
	}
}
