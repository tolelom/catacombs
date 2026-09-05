package game

import (
	"math/rand"
	"testing"

	"github.com/tolelom/catacombs/dungeon"
	"github.com/tolelom/catacombs/entity"
)

// Solo balance simulation: drives the REAL spawn + turn pipeline
// (spawnMonsters/spawnBoss → resolvePlayerActions → resolveMonsterActions)
// with a base-stat solo player of each class and measures survival.
//
// This exists to catch class-viability regressions: every class must be able
// to win a fresh regular encounter on the floors it is expected to fight at
// base stats (no gear, no skill tree), and the floor-5 boss must be beatable.

const (
	simRuns     = 300
	simMaxTurns = 80
)

// soloSimPolicy picks a reasonable-but-simple action for the solo player:
//   - Healer: smite opener at full HP, heal below 50%, otherwise attack
//   - Mage: AoE skill while 2+ monsters stand
//   - Warrior/Rogue: plain attacks (taunt/scout are useless solo)
func soloSimPolicy(s *GameSession, p *entity.Player) PlayerAction {
	target := s.firstAliveMonsterIdx()
	alive := 0
	for _, m := range s.state.Monsters {
		if !m.IsDead() {
			alive++
		}
	}
	switch p.Class {
	case entity.ClassHealer:
		if p.SkillUses > 0 && (p.HP == p.MaxHP || p.HP*2 < p.MaxHP) {
			return PlayerAction{Type: ActionSkill, TargetIdx: -1}
		}
	case entity.ClassMage:
		if p.SkillUses > 0 && alive >= 2 {
			return PlayerAction{Type: ActionSkill}
		}
	}
	return PlayerAction{Type: ActionAttack, TargetIdx: target}
}

// simulateSoloCombat runs one fresh encounter to completion. Returns whether
// the player won and how many turns it took.
func simulateSoloCombat(t *testing.T, class entity.Class, floor int, seed int64, boss bool) (bool, int) {
	t.Helper()
	s := NewGameSession(testCfg(t))
	s.rng = rand.New(rand.NewSource(seed))
	p := entity.NewPlayer("Sim", class)
	p.Fingerprint = "fp-sim"
	s.AddPlayer(p)
	s.state.SoloMode = true
	s.state.Phase = PhaseCombat
	s.state.FloorNum = floor
	s.state.Floor = dungeon.GenerateFloor(floor, s.rng) // combat-over path touches the floor map
	if boss {
		s.spawnBoss()
	} else {
		s.spawnMonsters()
	}

	for turn := 1; turn <= simMaxTurns; turn++ {
		s.actions = map[string]PlayerAction{p.Fingerprint: soloSimPolicy(s, p)}
		s.state.SubmittedActions = make(map[string]string)
		s.state.TurnResolving = true
		s.resolvePlayerActions()
		s.resolveMonsterActions()
		s.state.TurnResolving = false
		if p.IsDead() {
			return false, turn
		}
		if s.firstAliveMonsterIdx() < 0 {
			return true, turn
		}
	}
	return false, simMaxTurns // stalled out = loss
}

// simWinRate runs simRuns seeded combats and returns win-rate plus the mean
// turn count of winning runs.
func simWinRate(t *testing.T, class entity.Class, floor int, boss bool) (float64, float64) {
	t.Helper()
	wins, winTurns := 0, 0
	for seed := int64(0); seed < simRuns; seed++ {
		won, turns := simulateSoloCombat(t, class, floor, seed, boss)
		if won {
			wins++
			winTurns += turns
		}
	}
	rate := float64(wins) / float64(simRuns)
	avg := 0.0
	if wins > 0 {
		avg = float64(winTurns) / float64(wins)
	}
	return rate, avg
}

// TestSoloClassBalanceSim asserts every class clears base-stat viability bars
// on early floors, and logs the full survival table for balance review.
func TestSoloClassBalanceSim(t *testing.T) {
	if testing.Short() {
		t.Skip("simulation is slow; skipped in -short")
	}
	classes := []entity.Class{entity.ClassWarrior, entity.ClassMage, entity.ClassHealer, entity.ClassRogue}
	floors := []int{1, 2, 3, 4, 5, 6, 8}

	type bar struct {
		floor   int
		minRate float64
	}
	// Viability bars: fresh base-stat solo runs should reliably clear the
	// onboarding floors and still mostly survive the early-threat peak.
	bars := []bar{{1, 0.95}, {3, 0.80}, {5, 0.60}}

	for _, class := range classes {
		rates := make(map[int]float64)
		for _, f := range floors {
			rate, avgTurns := simWinRate(t, class, f, false)
			rates[f] = rate
			t.Logf("%-7s floor %2d: win %5.1f%%  avg %4.1f turns", class, f, rate*100, avgTurns)
		}
		for _, b := range bars {
			if rates[b.floor] < b.minRate {
				t.Errorf("%s floor %d win rate %.1f%% below viability bar %.0f%%",
					class, b.floor, rates[b.floor]*100, b.minRate*100)
			}
		}
	}
}

// TestSoloBossFloor5Sim asserts the first boss is beatable by every base-stat
// class — by floor 5 real players have gear, so base-stat viability here is a
// conservative floor, not a balance target.
func TestSoloBossFloor5Sim(t *testing.T) {
	if testing.Short() {
		t.Skip("simulation is slow; skipped in -short")
	}
	classes := []entity.Class{entity.ClassWarrior, entity.ClassMage, entity.ClassHealer, entity.ClassRogue}
	for _, class := range classes {
		rate, avgTurns := simWinRate(t, class, 5, true)
		t.Logf("%-7s boss5: win %5.1f%%  avg %4.1f turns", class, rate*100, avgTurns)
		if rate < 0.30 {
			t.Errorf("%s boss5 win rate %.1f%% below 30%% conservative floor", class, rate*100)
		}
	}
}
