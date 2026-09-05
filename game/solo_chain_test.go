package game

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

// runSoloTurn submits one action for the solo player and resolves the player
// phase (monster phase skipped to keep HP deterministic).
func runSoloTurn(t *testing.T, s *GameSession, action PlayerAction) {
	t.Helper()
	p := s.state.Players[0]
	s.actions = map[string]PlayerAction{p.Fingerprint: action}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false
}

// monsterHP returns current HP of the first monster.
func monsterHP(s *GameSession) int {
	return s.state.Monsters[0].HP
}

// TestSoloChainEchoStrikeBoostsAttack — skill on turn N makes the turn N+1
// attack hit ~1.4x harder than a cold attack.
func TestSoloChainEchoStrikeBoostsAttack(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000
	s.state.Monsters[0].DEF = 0
	s.state.Monsters[0].IsBoss = true // suppress the 5% dodge/glance rolls (deterministic damage)
	p := s.state.Players[0]
	atk := p.EffectiveATK()

	// Cold attack first (no chain) to measure the baseline.
	before := monsterHP(s)
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	coldDmg := before - monsterHP(s)
	if coldDmg <= 0 {
		t.Fatal("baseline attack should deal damage")
	}

	// Skill (warrior taunt), then attack — Echo Strike should fire.
	p.SkillUses = 3 // setupCombatCoop bypasses the combat-start reset
	runSoloTurn(t, s, PlayerAction{Type: ActionSkill})
	if s.soloLastAction != "skill" {
		t.Fatalf("lastAction should be skill, got %q", s.soloLastAction)
	}
	before = monsterHP(s)
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	echoDmg := before - monsterHP(s)

	// Echo = 1.4x. Allow for the dodge/glance variance bands by asserting the
	// echo hit exceeds a plain max-roll hit (ATK = max plain damage with DEF 0).
	if echoDmg <= atk {
		t.Errorf("Echo Strike hit (%d) should exceed plain ATK (%d); cold was %d", echoDmg, atk, coldDmg)
	}
	if !s.state.CombosFiredThisRun["Echo Strike"] {
		t.Error("Echo Strike should register in CombosFiredThisRun")
	}
}

// TestSoloChainRelentlessFromThirdAttack — the 3rd consecutive attack fires
// Relentless and registers it.
func TestSoloChainRelentlessFromThirdAttack(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000

	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	if s.state.CombosFiredThisRun["Relentless Assault"] {
		t.Fatal("Relentless must not fire before the 3rd attack")
	}
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	if !s.state.CombosFiredThisRun["Relentless Assault"] {
		t.Error("3rd consecutive attack should fire Relentless")
	}
	if s.soloAttackStreak != 3 {
		t.Errorf("attack streak should be 3, got %d", s.soloAttackStreak)
	}
}

// TestSoloChainCounterStanceGuaranteesCrit — coop then attack must crit (2x).
func TestSoloChainCounterStanceGuaranteesCrit(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000
	s.state.Monsters[0].DEF = 0
	s.state.Monsters[0].IsBoss = true // suppress the 5% dodge/glance rolls (deterministic damage)
	p := s.state.Players[0]
	atk := p.EffectiveATK()

	runSoloTurn(t, s, PlayerAction{Type: ActionCoop})
	if s.soloLastAction != "coop" {
		t.Fatalf("lastAction should be coop, got %q", s.soloLastAction)
	}
	before := monsterHP(s)
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	dmg := before - monsterHP(s)
	// Crit doubles the roll; even the minimum crit (0.9*2 = 1.8x ATK band)
	// beats any non-crit max roll (1.1x ATK... conservative: > atk).
	if dmg <= atk {
		t.Errorf("Counter Stance crit (%d) should exceed plain ATK (%d)", dmg, atk)
	}
	if !s.state.CombosFiredThisRun["Counter Stance"] {
		t.Error("Counter Stance should register in CombosFiredThisRun")
	}
}

// TestSoloChainItemBreaksStreak — using an item resets the attack streak.
func TestSoloChainItemBreaksStreak(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000
	p := s.state.Players[0]
	p.Inventory = append(p.Inventory, entity.NewHPPotion())
	p.HP = 1 // make the potion meaningful

	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	runSoloTurn(t, s, PlayerAction{Type: ActionItem})
	if s.soloAttackStreak != 0 {
		t.Errorf("item should reset streak, got %d", s.soloAttackStreak)
	}
	if s.soloLastAction != "item" {
		t.Errorf("lastAction should be item, got %q", s.soloLastAction)
	}
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	if s.state.CombosFiredThisRun["Relentless Assault"] {
		t.Error("Relentless must not fire after an item broke the streak")
	}
}

// TestSoloChainFailedSkillDoesNotArm — a skill attempt with 0 uses must not
// set up Echo Strike (only successful actions count).
func TestSoloChainFailedSkillDoesNotArm(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000
	s.state.Players[0].SkillUses = 0

	runSoloTurn(t, s, PlayerAction{Type: ActionSkill})
	if s.soloLastAction != "" {
		t.Errorf("failed skill should record empty action, got %q", s.soloLastAction)
	}
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	if s.state.CombosFiredThisRun["Echo Strike"] {
		t.Error("Echo Strike must not fire after a failed skill")
	}
}

// TestSoloChainFailedCoopDoesNotArm — second coop use (already used) must not
// arm Counter Stance.
func TestSoloChainFailedCoopDoesNotArm(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000

	runSoloTurn(t, s, PlayerAction{Type: ActionCoop})
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0}) // consumes Counter Stance
	runSoloTurn(t, s, PlayerAction{Type: ActionCoop})                 // fails: already used
	if s.soloLastAction != "" {
		t.Errorf("failed coop should record empty action, got %q", s.soloLastAction)
	}
}

// TestSoloChainsDoNotFireInParty — with 2+ players the chain layer stays off
// (party combos own that space).
func TestSoloChainsDoNotFireInParty(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior, entity.ClassMage)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000
	w := s.state.Players[0]
	m := s.state.Players[1]

	// Warrior: skill then attack — would be Echo Strike in solo.
	s.actions = map[string]PlayerAction{
		w.Fingerprint: {Type: ActionSkill},
		m.Fingerprint: {Type: ActionWait},
	}
	s.state.SubmittedActions = make(map[string]string)
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false

	s.actions = map[string]PlayerAction{
		w.Fingerprint: {Type: ActionAttack, TargetIdx: 0},
		m.Fingerprint: {Type: ActionWait},
	}
	s.state.TurnResolving = true
	s.resolvePlayerActions()
	s.state.TurnResolving = false

	if s.state.CombosFiredThisRun["Echo Strike"] {
		t.Error("solo chains must not fire in party mode")
	}
}

// TestSoloChainResetOnCombatStart — chain state must clear when a new combat
// begins.
func TestSoloChainResetOnCombatStart(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})
	if s.soloAttackStreak != 2 {
		t.Fatalf("streak should be 2, got %d", s.soloAttackStreak)
	}

	s.mu.Lock()
	s.resetPerCombatStateLocked()
	s.mu.Unlock()

	if s.soloAttackStreak != 0 || s.soloLastAction != "" {
		t.Errorf("combat start should reset chain state, got streak=%d last=%q",
			s.soloAttackStreak, s.soloLastAction)
	}
}

// TestGetStateExposesSoloChainView — solo combat snapshots carry the chain
// state for the HUD; party snapshots don't.
func TestGetStateExposesSoloChainView(t *testing.T) {
	s := setupCombatCoop(t, entity.ClassWarrior)
	s.state.Monsters[0].HP = 10000
	s.state.Monsters[0].MaxHP = 10000
	runSoloTurn(t, s, PlayerAction{Type: ActionAttack, TargetIdx: 0})

	st := s.GetState()
	if st.SoloChain == nil {
		t.Fatal("solo combat snapshot should expose SoloChain")
	}
	if st.SoloChain.AttackStreak != 1 || st.SoloChain.LastAction != "attack" {
		t.Errorf("unexpected chain view: %+v", st.SoloChain)
	}

	party := setupCombatCoop(t, entity.ClassWarrior, entity.ClassMage)
	if party.GetState().SoloChain != nil {
		t.Error("party snapshot must not expose SoloChain")
	}
}
