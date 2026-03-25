# Phase 2: Combat & Dungeon Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add strategic depth to combat (skill trees, combo skills, elite monsters, new status effects) and diversify dungeon exploration (random events, secret rooms, floor themes, mini-bosses, seed-based generation).

**Architecture:** Extend existing entity/combat/game packages with new data types and mechanics. Skill trees are run-scoped (reset each run). Combo detection hooks into the existing turn resolution pipeline after action collection. Elite monsters are prefix-modified regular monsters. Floor themes apply environmental effects via a new dungeon/theme package. Seed-based generation threads `*rand.Rand` through the dungeon generator for Phase 3 daily challenge support.

**Tech Stack:** Go 1.25.1, existing codebase patterns, `math/rand` (v1) for seeded RNG

**Module path:** `github.com/tolelom/catacombs`

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `entity/skill_tree.go` | SkillBranch, SkillNode, PlayerSkills structs; branch definitions per class |
| `entity/skill_tree_test.go` | Skill tree allocation and effect tests |
| `entity/elite.go` | ElitePrefix struct, prefix definitions, ApplyPrefix function |
| `entity/elite_test.go` | Elite stat modification tests |
| `combat/combo.go` | ComboDefinition, combo detection in a turn's actions |
| `combat/combo_test.go` | Combo detection tests |
| `dungeon/theme.go` | FloorTheme struct, theme-by-floor lookup, environmental effects |
| `dungeon/theme_test.go` | Theme lookup and effect tests |
| `game/random_event.go` | RandomEvent, EventChoice structs; event pool and resolution |
| `game/random_event_test.go` | Event choice outcome tests |

### Modified Files
| File | Changes |
|------|---------|
| `entity/player.go` | Add StatusBleed, StatusCurse; add PlayerSkills field; add skill effect methods |
| `entity/player_test.go` | Tests for new status effects and skill bonuses |
| `entity/monster.go` | Add IsMiniBoss, IsElite, ElitePrefix fields; mini-boss definitions |
| `entity/monster_test.go` | Tests for mini-boss and elite creation |
| `combat/combat.go` | Update MonsterAI for elite/mini-boss behaviors |
| `combat/combat_test.go` | Tests for updated AI |
| `game/event.go` | Add spawnElite, spawnMiniBoss, RoomSecret handling; use themes; update triggerEvent to use random_event |
| `game/turn.go` | Integrate combo detection; apply theme damage modifiers; apply skill tree bonuses; handle bleed/curse in TickEffects |
| `game/session.go` | Add AllocateSkillPoint method; store floor theme |
| `dungeon/generator.go` | Accept `*rand.Rand` parameter; thread RNG through all random calls |
| `dungeon/generator_test.go` | Update tests for seeded generation |
| `dungeon/room.go` | Add RoomSecret type; update RandomRoomType for secret room chance |
| `config/config.go` | Add Phase2 config fields (elite spawn chance, theme damage multiplier, etc.) |
| `ui/game_view.go` | Add skill tree allocation UI on floor transition; show elite/theme info; render combo messages |

---

## Task 1: New Status Effects (Bleed, Curse)

**Files:**
- Modify: `entity/player.go`
- Modify: `entity/player_test.go`

- [ ] **Step 1: Write failing tests for Bleed and Curse**

```go
// Add to entity/player_test.go
func TestBleedEffect(t *testing.T) {
	p := NewPlayer("Test", ClassWarrior)
	p.AddEffect(ActiveEffect{Type: StatusBleed, Duration: 3, Value: 2})

	// Bleed should stack: value increases each tick
	msgs := p.TickEffects()
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "bleed") {
			found = true
		}
	}
	if !found {
		t.Error("expected bleed damage message")
	}
	// After first tick, bleed value should have increased by 1
	for _, e := range p.Effects {
		if e.Type == StatusBleed && e.Value != 3 {
			t.Errorf("expected bleed value 3 after tick, got %d", e.Value)
		}
	}
}

func TestCurseEffect(t *testing.T) {
	p := NewPlayer("Test", ClassHealer)
	p.AddEffect(ActiveEffect{Type: StatusCurse, Duration: 3, Value: 50})

	startHP := p.HP
	p.Heal(100)
	healed := p.HP - startHP
	// Curse reduces healing by Value% (50%), so 100 heal → 50 actual
	if healed != 50 {
		t.Errorf("expected cursed heal of 50, got %d", healed)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./entity/ -run "TestBleed|TestCurse" -v`
Expected: FAIL — StatusBleed/StatusCurse undefined

- [ ] **Step 3: Implement Bleed and Curse**

In `entity/player.go`:

Add to StatusEffect enum:
```go
const (
	StatusPoison StatusEffect = iota
	StatusBurn
	StatusFreeze
	StatusBleed
	StatusCurse
)
```

Rewrite `TickEffects()` to use index-based loop (`for i := 0; i < len(p.Effects); i++`) instead of range loop, so we can mutate `p.Effects[i].Value` for Bleed stacking. Add all missing status cases:

```go
case StatusBleed:
	p.HP -= p.Effects[i].Value
	msgs = append(msgs, fmt.Sprintf("%s takes %d bleed damage", p.Name, p.Effects[i].Value))
	p.Effects[i].Value++ // Bleed intensifies each turn
case StatusFreeze:
	msgs = append(msgs, fmt.Sprintf("%s is frozen and cannot act!", p.Name))
	// Freeze effect: the turn resolution checks HasEffect(StatusFreeze) to skip player action
case StatusCurse:
	msgs = append(msgs, fmt.Sprintf("%s is cursed! Healing reduced", p.Name))
	// Curse effect: handled in Heal() method
```

Update `Heal()` to check for Curse:
```go
func (p *Player) Heal(amount int) {
	for _, e := range p.Effects {
		if e.Type == StatusCurse {
			amount = amount * (100 - e.Value) / 100
			break
		}
	}
	p.HP += amount
	if p.HP > p.MaxHP {
		p.HP = p.MaxHP
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./entity/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add entity/player.go entity/player_test.go
git commit -m "feat: add Bleed and Curse status effects"
```

---

## Task 2: Skill Tree Data Model

**Files:**
- Create: `entity/skill_tree.go`
- Create: `entity/skill_tree_test.go`
- Modify: `entity/player.go` (add Skills field)

- [ ] **Step 1: Write failing tests**

```go
// entity/skill_tree_test.go
package entity

import "testing"

func TestGetBranches(t *testing.T) {
	branches := GetBranches(ClassWarrior)
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches for Warrior, got %d", len(branches))
	}
	if branches[0].Name != "Tank" {
		t.Errorf("expected first branch 'Tank', got %q", branches[0].Name)
	}
	if len(branches[0].Nodes) != 3 {
		t.Errorf("expected 3 nodes per branch, got %d", len(branches[0].Nodes))
	}
}

func TestAllocateSkillPoint(t *testing.T) {
	skills := &PlayerSkills{}
	err := skills.Allocate(0, ClassWarrior) // branch 0, first point
	if err != nil {
		t.Fatal(err)
	}
	if skills.Allocated != 1 {
		t.Errorf("expected 1 allocated, got %d", skills.Allocated)
	}
	if skills.BranchIndex != 0 {
		t.Errorf("expected branch 0, got %d", skills.BranchIndex)
	}
}

func TestCannotSwitchBranch(t *testing.T) {
	skills := &PlayerSkills{BranchIndex: 0, Allocated: 1, Points: 2}
	err := skills.Allocate(1, ClassWarrior) // try other branch
	if err == nil {
		t.Error("expected error switching branch after allocation")
	}
}

func TestSkillBonuses(t *testing.T) {
	p := NewPlayer("Test", ClassWarrior)
	p.Skills = &PlayerSkills{BranchIndex: 0, Allocated: 1, Points: 1}
	// Tank branch node 0 should give DEF boost
	bonus := p.Skills.GetATKBonus(ClassWarrior)
	defBonus := p.Skills.GetDEFBonus(ClassWarrior)
	// At least one of these should be non-zero for Warrior Tank
	if bonus == 0 && defBonus == 0 {
		t.Error("expected at least one bonus from skill allocation")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./entity/ -run "TestGetBranch|TestAllocate|TestCannot|TestSkillBonus" -v`
Expected: FAIL

- [ ] **Step 3: Implement skill tree**

```go
// entity/skill_tree.go
package entity

import "fmt"

type SkillEffect int

const (
	EffectATKBoost SkillEffect = iota
	EffectDEFBoost
	EffectMaxHPBoost
	EffectSkillPower  // multiplier for class skill
	EffectCritChance  // chance for 1.5x damage
	EffectHealBoost   // bonus to healing given/received
)

type SkillNode struct {
	Name     string
	Effect   SkillEffect
	Value    int // flat bonus or percentage
	Required int // points spent in branch to unlock (0, 1, 2)
}

type SkillBranch struct {
	Name  string
	Nodes [3]SkillNode
}

type PlayerSkills struct {
	BranchIndex int  // -1 = not chosen, 0 or 1
	Points      int  // total points earned (1 per floor clear)
	Allocated   int  // points spent in chosen branch
}

// Allocate spends 1 point in the given branch. Errors if switching after first allocation.
func (s *PlayerSkills) Allocate(branchIdx int, class Class) error {
	if s.Allocated == 0 {
		s.BranchIndex = branchIdx
	} else if s.BranchIndex != branchIdx {
		return fmt.Errorf("cannot switch branch after allocation")
	}
	branches := GetBranches(class)
	if s.Allocated >= 3 {
		return fmt.Errorf("branch fully allocated")
	}
	node := branches[branchIdx].Nodes[s.Allocated]
	if s.Allocated < node.Required {
		return fmt.Errorf("prerequisite not met")
	}
	s.Allocated++
	return nil
}

func (s *PlayerSkills) GetATKBonus(class Class) int {
	if s == nil || s.Allocated == 0 {
		return 0
	}
	bonus := 0
	branches := GetBranches(class)
	branch := branches[s.BranchIndex]
	for i := 0; i < s.Allocated; i++ {
		if branch.Nodes[i].Effect == EffectATKBoost {
			bonus += branch.Nodes[i].Value
		}
	}
	return bonus
}

func (s *PlayerSkills) GetDEFBonus(class Class) int {
	if s == nil || s.Allocated == 0 {
		return 0
	}
	bonus := 0
	branches := GetBranches(class)
	branch := branches[s.BranchIndex]
	for i := 0; i < s.Allocated; i++ {
		if branch.Nodes[i].Effect == EffectDEFBoost {
			bonus += branch.Nodes[i].Value
		}
	}
	return bonus
}

func (s *PlayerSkills) GetMaxHPBonus(class Class) int {
	if s == nil || s.Allocated == 0 {
		return 0
	}
	bonus := 0
	branches := GetBranches(class)
	branch := branches[s.BranchIndex]
	for i := 0; i < s.Allocated; i++ {
		if branch.Nodes[i].Effect == EffectMaxHPBoost {
			bonus += branch.Nodes[i].Value
		}
	}
	return bonus
}

func (s *PlayerSkills) GetSkillPower(class Class) int {
	if s == nil || s.Allocated == 0 {
		return 0
	}
	total := 0
	branches := GetBranches(class)
	branch := branches[s.BranchIndex]
	for i := 0; i < s.Allocated; i++ {
		if branch.Nodes[i].Effect == EffectSkillPower {
			total += branch.Nodes[i].Value
		}
	}
	return total
}

// GetBranches returns the 2 skill branches for a class.
func GetBranches(class Class) [2]SkillBranch {
	switch class {
	case ClassWarrior:
		return [2]SkillBranch{
			{"Tank", [3]SkillNode{
				{"Iron Skin", EffectDEFBoost, 3, 0},
				{"Fortify", EffectMaxHPBoost, 20, 1},
				{"Bulwark", EffectDEFBoost, 5, 2},
			}},
			{"Berserker", [3]SkillNode{
				{"Fury", EffectATKBoost, 4, 0},
				{"Reckless Strike", EffectSkillPower, 20, 1},
				{"Bloodlust", EffectATKBoost, 6, 2},
			}},
		}
	case ClassMage:
		return [2]SkillBranch{
			{"Elementalist", [3]SkillNode{
				{"Amplify", EffectSkillPower, 15, 0},
				{"Surge", EffectATKBoost, 5, 1},
				{"Overload", EffectSkillPower, 25, 2},
			}},
			{"Chronomancer", [3]SkillNode{
				{"Time Warp", EffectDEFBoost, 3, 0},
				{"Haste", EffectATKBoost, 3, 1},
				{"Temporal Shield", EffectMaxHPBoost, 15, 2},
			}},
		}
	case ClassHealer:
		return [2]SkillBranch{
			{"Guardian", [3]SkillNode{
				{"Blessing", EffectHealBoost, 20, 0},
				{"Aegis", EffectDEFBoost, 4, 1},
				{"Divine Grace", EffectHealBoost, 30, 2},
			}},
			{"Priest", [3]SkillNode{
				{"Smite", EffectATKBoost, 5, 0},
				{"Holy Fire", EffectSkillPower, 20, 1},
				{"Exorcism", EffectATKBoost, 7, 2},
			}},
		}
	case ClassRogue:
		return [2]SkillBranch{
			{"Assassin", [3]SkillNode{
				{"Backstab", EffectATKBoost, 5, 0},
				{"Lethality", EffectCritChance, 15, 1},
				{"Execute", EffectATKBoost, 8, 2},
			}},
			{"Alchemist", [3]SkillNode{
				{"Brew", EffectHealBoost, 15, 0},
				{"Toxicology", EffectSkillPower, 20, 1},
				{"Elixir", EffectMaxHPBoost, 25, 2},
			}},
		}
	default:
		return [2]SkillBranch{}
	}
}
```

Add to `entity/player.go` Player struct:
```go
Skills *PlayerSkills
```

Update `EffectiveATK()` to include skill bonus:
```go
func (p *Player) EffectiveATK() int {
	atk := p.ATK
	// ... existing weapon/relic bonuses ...
	atk += p.Skills.GetATKBonus(p.Class)
	return atk
}
```

Similarly update `EffectiveDEF()` for skill DEF bonus.

- [ ] **Step 4: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./entity/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add entity/skill_tree.go entity/skill_tree_test.go entity/player.go entity/player_test.go
git commit -m "feat: add skill tree system with 2 branches per class"
```

---

## Task 3: Elite Monsters

**Files:**
- Create: `entity/elite.go`
- Create: `entity/elite_test.go`
- Modify: `entity/monster.go` (add IsElite, ElitePrefix fields)
- Modify: `game/event.go` (spawn elites with ~20% chance)

- [ ] **Step 1: Write failing tests**

```go
// entity/elite_test.go
package entity

import "testing"

func TestApplyPrefix(t *testing.T) {
	m := NewMonster(MonsterOrc, 5, 1.15)
	origHP := m.HP
	ApplyPrefix(m, PrefixVenomous)
	if !m.IsElite {
		t.Error("expected IsElite to be true")
	}
	if m.HP <= origHP {
		t.Error("expected HP to increase for elite")
	}
	if m.ElitePrefix != PrefixVenomous {
		t.Errorf("expected PrefixVenomous, got %d", m.ElitePrefix)
	}
}

func TestRandomPrefix(t *testing.T) {
	p := RandomPrefix()
	if p < PrefixVenomous || p > PrefixVampiric {
		t.Errorf("unexpected prefix: %d", p)
	}
}
```

- [ ] **Step 2: Implement elite system**

```go
// entity/elite.go
package entity

import "math/rand"

type ElitePrefixType int

const (
	PrefixVenomous  ElitePrefixType = iota // applies poison on hit
	PrefixBurning                           // applies burn on hit
	PrefixFreezing                          // applies freeze on hit
	PrefixBleeding                          // applies bleed on hit
	PrefixVampiric                          // heals on hit
)

type ElitePrefixDef struct {
	Name    string
	HPMult  float64 // HP multiplier
	ATKMult float64 // ATK multiplier
	OnHit   StatusEffect // applied when monster hits player
}

// (moved to exported ElitePrefixDefs above)
// PrefixVenomous: {"Venomous", 1.3, 1.0, StatusPoison},
// PrefixBurning:  {"Burning", 1.2, 1.1, StatusBurn},
// PrefixFreezing: {"Freezing", 1.2, 1.0, StatusFreeze},
// PrefixBleeding: {"Bleeding", 1.1, 1.2, StatusBleed},
// PrefixVampiric: {"Vampiric", 1.4, 1.1, StatusEffect(-1)}, // special: heals self
// }

func (p ElitePrefixType) String() string {
	return ElitePrefixDefs[p].Name
}

// NOTE: Map is exported as ElitePrefixDefs so game/turn.go can access it for on-hit effects.
var ElitePrefixDefs = map[ElitePrefixType]ElitePrefixDef{

func RandomPrefix() ElitePrefixType {
	return ElitePrefixType(rand.Intn(5))
}

func ApplyPrefix(m *Monster, prefix ElitePrefixType) {
	def := elitePrefixes[prefix]
	m.IsElite = true
	m.ElitePrefix = prefix
	m.Name = def.Name + " " + m.Name
	m.HP = int(float64(m.HP) * def.HPMult)
	m.MaxHP = m.HP
	m.ATK = int(float64(m.ATK) * def.ATKMult)
}
```

Add to `entity/monster.go` Monster struct:
```go
IsElite     bool
ElitePrefix ElitePrefixType
```

In `game/event.go` `spawnMonsters()`, after creating each monster, add ~20% elite chance:
```go
if rand.Float64() < 0.20 && !m.IsBoss {
	ApplyPrefix(m, RandomPrefix())
}
```

In `game/turn.go` `resolveMonsterActions()`, after a monster hits a player, check elite prefix (use exported `entity.ElitePrefixDefs`):
```go
if m.IsElite {
	def := entity.ElitePrefixDefs[m.ElitePrefix]
	if def.OnHit >= 0 {
		p.AddEffect(entity.ActiveEffect{Type: def.OnHit, Duration: 2, Value: 3})
	} else if m.ElitePrefix == entity.PrefixVampiric {
		m.HP = min(m.HP + dmg/4, m.MaxHP)
	}
}
```

- [ ] **Step 3: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./entity/ ./game/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add entity/elite.go entity/elite_test.go entity/monster.go game/event.go game/turn.go
git commit -m "feat: add elite monsters with 5 prefix types"
```

---

## Task 4: Combo Skills

**Files:**
- Create: `combat/combo.go`
- Create: `combat/combo_test.go`
- Modify: `game/turn.go` (integrate combo detection after action collection)

- [ ] **Step 1: Write failing tests**

```go
// combat/combo_test.go
package combat

import (
	"testing"
	"github.com/tolelom/catacombs/entity"
)

func TestDetectCombo_IceShatter(t *testing.T) {
	actions := map[string]ComboAction{
		"mage":    {Class: entity.ClassMage, ActionType: "skill"},
		"warrior": {Class: entity.ClassWarrior, ActionType: "attack"},
	}
	combos := DetectCombos(actions)
	if len(combos) == 0 {
		t.Fatal("expected Ice Shatter combo")
	}
	if combos[0].Name != "Ice Shatter" {
		t.Errorf("expected 'Ice Shatter', got %q", combos[0].Name)
	}
}

func TestDetectCombo_NoMatch(t *testing.T) {
	actions := map[string]ComboAction{
		"warrior1": {Class: entity.ClassWarrior, ActionType: "attack"},
		"warrior2": {Class: entity.ClassWarrior, ActionType: "attack"},
	}
	combos := DetectCombos(actions)
	if len(combos) != 0 {
		t.Errorf("expected no combos, got %d", len(combos))
	}
}
```

- [ ] **Step 2: Implement combo system**

```go
// combat/combo.go
package combat

import "github.com/tolelom/catacombs/entity"

type ComboAction struct {
	Class      entity.Class
	ActionType string // "attack", "skill", "item"
}

type ComboEffect struct {
	DamageMultiplier float64 // applied to all attacks this turn
	BonusDamage      int     // flat bonus damage to all monsters
	HealAll          int     // heal all players
	Message          string  // displayed in combat log
}

type ComboDef struct {
	Name     string
	Required []ComboAction // required class+action pairs (order doesn't matter)
	Effect   ComboEffect
}

var comboDefs = []ComboDef{
	{
		Name: "Ice Shatter",
		Required: []ComboAction{
			{Class: entity.ClassMage, ActionType: "skill"},
			{Class: entity.ClassWarrior, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.5, Message: "💥 ICE SHATTER! Frozen enemies shatter!"},
	},
	{
		Name: "Holy Assault",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "skill"},
			{Class: entity.ClassWarrior, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.3, HealAll: 10, Message: "✨ HOLY ASSAULT! Blessed strikes heal the party!"},
	},
	{
		Name: "Shadow Strike",
		Required: []ComboAction{
			{Class: entity.ClassRogue, ActionType: "skill"},
			{Class: entity.ClassMage, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.4, Message: "🗡️ SHADOW STRIKE! Magical shadows amplify the attack!"},
	},
	{
		Name: "Full Assault",
		Required: []ComboAction{
			{Class: entity.ClassWarrior, ActionType: "attack"},
			{Class: entity.ClassMage, ActionType: "attack"},
			{Class: entity.ClassRogue, ActionType: "attack"},
		},
		Effect: ComboEffect{DamageMultiplier: 1.3, BonusDamage: 5, Message: "⚔️ FULL ASSAULT! Combined attack overwhelms!"},
	},
	{
		Name: "Restoration",
		Required: []ComboAction{
			{Class: entity.ClassHealer, ActionType: "skill"},
			{Class: entity.ClassRogue, ActionType: "item"},
		},
		Effect: ComboEffect{HealAll: 20, Message: "💚 RESTORATION! Combined healing surges!"},
	},
}

// DetectCombos checks if the current turn's actions trigger any combos.
func DetectCombos(actions map[string]ComboAction) []ComboDef {
	var triggered []ComboDef
	for _, combo := range comboDefs {
		if matchesCombo(combo.Required, actions) {
			triggered = append(triggered, combo)
		}
	}
	return triggered
}

func matchesCombo(required []ComboAction, actions map[string]ComboAction) bool {
	for _, req := range required {
		found := false
		for _, act := range actions {
			if act.Class == req.Class && act.ActionType == req.ActionType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
```

In `game/turn.go` `resolvePlayerActions()`, after collecting all actions and before `ResolveAttacks()`:
1. Build `comboActions` map from player actions (key=playerID, value=ComboAction{Class, ActionType})
2. Call `combat.DetectCombos(comboActions)`
3. Apply combo effects to existing attack intents:
   - **DamageMultiplier**: multiply each intent's `Multiplier` field: `intent.Multiplier *= combo.Effect.DamageMultiplier`. This stacks multiplicatively with the co-op bonus (which is applied separately inside `ResolveAttacks`).
   - **BonusDamage**: add flat bonus to each intent's `PlayerATK`: `intent.PlayerATK += combo.Effect.BonusDamage`
   - **HealAll**: heal all alive players after attack resolution
4. Log combo messages to PendingLogs before attack results

- [ ] **Step 3: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./combat/ ./game/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add combat/combo.go combat/combo_test.go game/turn.go
git commit -m "feat: add combo skill system with 5 combos"
```

---

## Task 5: Floor Themes

**Files:**
- Create: `dungeon/theme.go`
- Create: `dungeon/theme_test.go`
- Modify: `game/event.go` (align boss patterns with themes)
- Modify: `game/turn.go` (apply theme damage modifiers to status effects)

- [ ] **Step 1: Write failing tests**

```go
// dungeon/theme_test.go
package dungeon

import "testing"

func TestGetTheme(t *testing.T) {
	tests := []struct {
		floor    int
		expected string
	}{
		{1, "Swamp"}, {5, "Swamp"},
		{6, "Volcano"}, {10, "Volcano"},
		{11, "Glacier"}, {15, "Glacier"},
		{16, "Inferno"}, {20, "Inferno"},
	}
	for _, tt := range tests {
		theme := GetTheme(tt.floor)
		if theme.Name != tt.expected {
			t.Errorf("floor %d: expected %q, got %q", tt.floor, tt.expected, theme.Name)
		}
	}
}
```

- [ ] **Step 2: Implement floor themes**

```go
// dungeon/theme.go
package dungeon

import "github.com/tolelom/catacombs/entity"

type FloorTheme struct {
	Name         string
	StatusBoost  entity.StatusEffect // which status is empowered
	DamageMult   float64             // multiplier for boosted status damage
	Description  string
}

var themes = []FloorTheme{
	{"Swamp", entity.StatusPoison, 1.5, "Toxic marshes amplify poison"},
	{"Volcano", entity.StatusBurn, 1.5, "Volcanic heat intensifies burns"},
	{"Glacier", entity.StatusFreeze, 1.5, "Glacial cold strengthens frost"},
	{"Inferno", entity.StatusEffect(-1), 1.3, "Hellfire empowers all afflictions"},
}

// GetTheme returns the floor theme for the given floor number.
func GetTheme(floor int) FloorTheme {
	switch {
	case floor <= 5:
		return themes[0] // Swamp
	case floor <= 10:
		return themes[1] // Volcano
	case floor <= 15:
		return themes[2] // Glacier
	default:
		return themes[3] // Inferno
	}
}
```

In `game/event.go` `spawnBoss()`, realign boss patterns:
```go
// Floor 5: Guardian → PatternPoison (Swamp theme)
// Floor 10: Warden → PatternBurn (Volcano theme)
// Floor 15: Overlord → PatternAoE + freeze (Glacier theme — add PatternFreeze)
// Floor 20: Archlich → PatternHeal + PatternAoE (Inferno)
```

Add `PatternFreeze` to `entity/monster.go` BossPattern enum.

In `game/turn.go` `resolveMonsterActions()`, add `PatternFreeze` handler alongside existing pattern cases:
```go
case entity.PatternFreeze:
	for _, p := range alivePlayers {
		p.AddEffect(entity.ActiveEffect{Type: entity.StatusFreeze, Duration: 1, Value: 0})
		s.addLog(fmt.Sprintf("%s freezes %s!", m.Name, p.Name))
	}
```

In `game/turn.go` status effect damage processing, apply theme multiplier:
```go
theme := dungeon.GetTheme(s.state.FloorNum)
// When applying status damage, if theme.StatusBoost matches the effect type
// OR theme is Inferno (StatusEffect(-1)), multiply damage by theme.DamageMult
```

- [ ] **Step 3: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./dungeon/ ./game/ ./entity/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add dungeon/theme.go dungeon/theme_test.go game/event.go game/turn.go entity/monster.go
git commit -m "feat: add floor themes with status effect modifiers"
```

---

## Task 6: Random Event Rooms

**Files:**
- Create: `game/random_event.go`
- Create: `game/random_event_test.go`
- Modify: `game/event.go` (replace old triggerEvent with new system)

- [ ] **Step 1: Write failing tests**

```go
// game/random_event_test.go
package game

import "testing"

func TestGetRandomEvents(t *testing.T) {
	events := GetRandomEvents()
	if len(events) < 8 {
		t.Errorf("expected at least 8 events, got %d", len(events))
	}
	for _, e := range events {
		if len(e.Choices) < 2 {
			t.Errorf("event %q has fewer than 2 choices", e.Description)
		}
	}
}

func TestResolveChoice(t *testing.T) {
	events := GetRandomEvents()
	result := events[0].Choices[0].Resolve(5) // floor 5
	// Should return a non-empty description
	if result.Description == "" {
		t.Error("expected non-empty result description")
	}
}
```

- [ ] **Step 2: Implement random events**

```go
// game/random_event.go
package game

import "math/rand"

type EventOutcome struct {
	Description string
	HPChange    int // positive = heal, negative = damage
	GoldChange  int
	ItemDrop    bool // grants random treasure
}

type EventChoice struct {
	Text    string
	Resolve func(floor int) EventOutcome
}

type RandomEvent struct {
	ID          string
	Description string
	Choices     []EventChoice
}

func GetRandomEvents() []RandomEvent {
	return []RandomEvent{
		{
			ID: "altar", Description: "You discover a mysterious altar glowing with dark energy.",
			Choices: []EventChoice{
				{"Offer gold (50g)", func(f int) EventOutcome {
					return EventOutcome{"The altar grants you power!", 20 + f, -50, false}
				}},
				{"Smash it", func(f int) EventOutcome {
					if rand.Float64() < 0.5 {
						return EventOutcome{"Shards cut you!", -(10 + f), 0, false}
					}
					return EventOutcome{"You find hidden treasure!", 0, 30 + f*2, true}
				}},
				{"Walk away", func(f int) EventOutcome {
					return EventOutcome{"You leave the altar undisturbed.", 0, 0, false}
				}},
			},
		},
		{
			ID: "fountain", Description: "A crystal fountain bubbles with shimmering liquid.",
			Choices: []EventChoice{
				{"Drink", func(f int) EventOutcome {
					if rand.Float64() < 0.7 {
						return EventOutcome{"Refreshing! You feel restored.", 25 + f, 0, false}
					}
					return EventOutcome{"The water burns! It was acid!", -(15 + f), 0, false}
				}},
				{"Fill a bottle", func(f int) EventOutcome {
					return EventOutcome{"You bottle the liquid for later.", 0, 0, true}
				}},
			},
		},
		{
			ID: "merchant", Description: "A hooded figure offers you a trade in the shadows.",
			Choices: []EventChoice{
				{"Trade HP for gold", func(f int) EventOutcome {
					return EventOutcome{"A fair deal.", -(20), 40 + f*3, false}
				}},
				{"Trade gold for HP", func(f int) EventOutcome {
					return EventOutcome{"You feel better.", 30 + f, -30, false}
				}},
				{"Decline", func(f int) EventOutcome {
					return EventOutcome{"The figure vanishes.", 0, 0, false}
				}},
			},
		},
		{
			ID: "trap_room", Description: "The floor is covered with suspicious tiles.",
			Choices: []EventChoice{
				{"Walk carefully", func(f int) EventOutcome {
					if rand.Float64() < 0.6 {
						return EventOutcome{"You navigate safely.", 0, 0, false}
					}
					return EventOutcome{"CLICK! A trap springs!", -(10 + f/2), 0, false}
				}},
				{"Rush through", func(f int) EventOutcome {
					return EventOutcome{"Traps trigger but you're fast!", -(5 + f/3), 10, false}
				}},
			},
		},
		{
			ID: "shrine", Description: "An ancient shrine hums with residual magic.",
			Choices: []EventChoice{
				{"Pray", func(f int) EventOutcome {
					return EventOutcome{"A warm light envelops you.", 15 + f/2, 0, false}
				}},
				{"Desecrate", func(f int) EventOutcome {
					if rand.Float64() < 0.4 {
						return EventOutcome{"The shrine curses you!", -(20 + f), 0, false}
					}
					return EventOutcome{"You plunder sacred relics!", 0, 50 + f*2, true}
				}},
			},
		},
		{
			ID: "chest", Description: "A locked chest sits in the corner, slightly rusted.",
			Choices: []EventChoice{
				{"Force it open", func(f int) EventOutcome {
					if rand.Float64() < 0.5 {
						return EventOutcome{"It was trapped!", -(8 + f/2), 0, false}
					}
					return EventOutcome{"Treasure inside!", 0, 20 + f*2, true}
				}},
				{"Pick the lock", func(f int) EventOutcome {
					return EventOutcome{"Click! It opens smoothly.", 0, 15 + f, false}
				}},
			},
		},
		{
			ID: "ghost", Description: "A spectral figure appears and speaks in riddles.",
			Choices: []EventChoice{
				{"Answer the riddle", func(f int) EventOutcome {
					if rand.Float64() < 0.5 {
						return EventOutcome{"Correct! The ghost grants wisdom.", 10, 20, false}
					}
					return EventOutcome{"Wrong! The ghost wails and hurts you.", -(10 + f/2), 0, false}
				}},
				{"Ignore it", func(f int) EventOutcome {
					return EventOutcome{"The ghost fades away.", 0, 0, false}
				}},
			},
		},
		{
			ID: "mushroom", Description: "Strange glowing mushrooms grow from the walls.",
			Choices: []EventChoice{
				{"Eat one", func(f int) EventOutcome {
					r := rand.Float64()
					if r < 0.33 {
						return EventOutcome{"Delicious! You feel stronger.", 20 + f, 0, false}
					} else if r < 0.66 {
						return EventOutcome{"Disgusting! You feel sick.", -(10 + f/2), 0, false}
					}
					return EventOutcome{"It tastes like nothing.", 0, 0, false}
				}},
				{"Harvest for later", func(f int) EventOutcome {
					return EventOutcome{"You carefully collect the mushrooms.", 0, 0, true}
				}},
			},
		},
	}
}

func PickRandomEvent() *RandomEvent {
	events := GetRandomEvents()
	e := events[rand.Intn(len(events))]
	return &e
}
```

Update `game/event.go` `triggerEvent()` to use the new system:
- Pick a random event
- Store it in GameState (add `CurrentEvent *RandomEvent` field to GameState if needed for UI)
- For now, auto-resolve with random choice (full choice UI can be added in the UI task)

- [ ] **Step 3: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./game/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add game/random_event.go game/random_event_test.go game/event.go game/session.go
git commit -m "feat: add 8 random event rooms with choice-based outcomes"
```

---

## Task 7: Secret Rooms and Mini-Bosses

**Files:**
- Modify: `dungeon/room.go` (add RoomSecret, update RandomRoomType)
- Modify: `entity/monster.go` (add mini-boss definitions)
- Modify: `game/event.go` (handle RoomSecret, spawnMiniBoss for floors 4/9/14/19)
- Modify: `dungeon/generator.go` (assign mini-boss rooms on correct floors)

- [ ] **Step 1: Add RoomSecret to room types**

In `dungeon/room.go`, add `RoomSecret` to the enum and update `RandomRoomType()`:
```go
const (
	RoomCombat RoomType = iota
	RoomTreasure
	RoomShop
	RoomEvent
	RoomEmpty
	RoomBoss
	RoomSecret   // NEW
	RoomMiniBoss // NEW
)
```

Update `RoomType.String()` to include new types: `"Secret"` and `"MiniBoss"` (prevents index-out-of-range panic).

Update `RandomRoomType()` — insert 5% secret room chance, reduce Empty to 10%:
```go
case r < 45:  return RoomCombat    // 45%
case r < 60:  return RoomTreasure  // 15%
case r < 70:  return RoomShop      // 10%
case r < 85:  return RoomEvent     // 15%
case r < 90:  return RoomSecret    // 5%
default:      return RoomEmpty     // 10%
```

- [ ] **Step 2: Add mini-boss monster definitions**

In `entity/monster.go`, add 4 mini-boss types. Use existing Monster struct with `IsMiniBoss bool` field:

```go
// Add to MonsterType enum after MonsterBoss20:
MonsterMiniBoss5  // Guardian's Herald
MonsterMiniBoss10 // Warden's Shadow
MonsterMiniBoss15 // Overlord's Lieutenant
MonsterMiniBoss20 // Archlich's Harbinger

// Add to monsterDefs:
// Stats are exactly 60% of corresponding boss stats (HP/ATK/DEF)
// Guardian: 150/15/8 → 90/9/5
// Warden: 250/22/12 → 150/13/7
// Overlord: 400/30/16 → 240/18/10
// Archlich: 600/40/20 → 360/24/12
MonsterMiniBoss5:  {"Guardian's Herald", 90, 9, 5, 5, false},
MonsterMiniBoss10: {"Warden's Shadow", 150, 13, 7, 10, false},
MonsterMiniBoss15: {"Overlord's Lieutenant", 240, 18, 10, 15, false},
MonsterMiniBoss20: {"Archlich's Harbinger", 360, 24, 12, 20, false},
```

Add `IsMiniBoss bool` to Monster struct. In `NewMonster()`, set `IsMiniBoss = true` for mini-boss types and assign a BossPattern (same as the subsequent boss but weaker effect).

- [ ] **Step 3: Handle in game/event.go**

Add `spawnMiniBoss(floor int)` method that spawns the appropriate mini-boss for the floor.

In `dungeon/generator.go`, for floors 4, 9, 14, 19, assign one room as `RoomMiniBoss` instead of random type.

Handle `RoomSecret` in `EnterRoom()` — grant rare/high-value treasure (double normal treasure bonus).

- [ ] **Step 4: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dungeon/room.go dungeon/generator.go entity/monster.go game/event.go
git commit -m "feat: add secret rooms and mini-bosses on floors 4/9/14/19"
```

---

## Task 8: Seed-Based Dungeon Generation

**Files:**
- Modify: `dungeon/generator.go` (accept `*rand.Rand`, thread through all random calls)
- Modify: `dungeon/room.go` (RandomRoomType accepts `*rand.Rand`)
- Modify: `dungeon/generator_test.go` (verify determinism)
- Modify: `game/session.go` (create `*rand.Rand` and pass to GenerateFloor)
- Modify: `game/turn.go` (pass rng to advanceFloor's GenerateFloor call)

- [ ] **Step 1: Write determinism test**

```go
// Add to dungeon/generator_test.go
func TestDeterministicGeneration(t *testing.T) {
	rng1 := rand.New(rand.NewSource(42))
	rng2 := rand.New(rand.NewSource(42))

	f1 := GenerateFloor(5, rng1)
	f2 := GenerateFloor(5, rng2)

	if len(f1.Rooms) != len(f2.Rooms) {
		t.Fatalf("room counts differ: %d vs %d", len(f1.Rooms), len(f2.Rooms))
	}
	for i, r := range f1.Rooms {
		if r.Type != f2.Rooms[i].Type || r.X != f2.Rooms[i].X || r.Y != f2.Rooms[i].Y {
			t.Errorf("room %d differs between same-seed generations", i)
		}
	}
}
```

- [ ] **Step 2: Refactor GenerateFloor signature**

Change `GenerateFloor(floorNum int) *Floor` to `GenerateFloor(floorNum int, rng *rand.Rand) *Floor`.

Replace ALL `rand.Xxx()` calls in generator.go with `rng.Xxx()`:
- `rand.Shuffle` → `rng.Shuffle`
- `rand.Intn` → `rng.Intn`
- `rand.Float64` → `rng.Float64`

**Important:** Also thread `rng` into `splitBSP()` — it currently uses `rand.Float64()` and `rand.Intn()` internally. Change its signature to accept `rng *rand.Rand` and replace all internal random calls.

Similarly in `room.go`, change `RandomRoomType()` to `RandomRoomType(rng *rand.Rand)`.

- [ ] **Step 3: Update all callers**

In `game/session.go` `StartFloor()` and `game/turn.go` `advanceFloor()`, pass a `rand.New(rand.NewSource(time.Now().UnixNano()))` for normal gameplay. (Phase 3 will pass a date-based seed for daily challenges.)

- [ ] **Step 4: Run determinism test and all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./dungeon/ -run TestDeterministic -v`
Expected: PASS

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add dungeon/ game/session.go game/turn.go
git commit -m "feat: seed-based dungeon generation for deterministic floors"
```

---

## Task 9: Skill Tree UI and Integration

**Files:**
- Modify: `ui/game_view.go` (add skill allocation UI on floor transition)
- Modify: `game/session.go` (add AllocateSkillPoint method, grant points on floor clear)
- Modify: `game/turn.go` (grant skill point in advanceFloor, apply skill bonuses to combat)

- [ ] **Step 1: Grant skill points on floor clear**

In `game/turn.go` `advanceFloor()`, after incrementing floor:
```go
for _, p := range s.state.Players {
	if p.Skills == nil {
		p.Skills = &entity.PlayerSkills{BranchIndex: -1}
	}
	p.Skills.Points++
}
```

- [ ] **Step 2: Add AllocateSkillPoint to session**

```go
// game/session.go
func (s *GameSession) AllocateSkillPoint(fingerprint string, branchIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Players {
		if p.Fingerprint == fingerprint {
			if p.Skills == nil || p.Skills.Points <= p.Skills.Allocated {
				return fmt.Errorf("no skill points available")
			}
			return p.Skills.Allocate(branchIdx, p.Class)
		}
	}
	return fmt.Errorf("player not found")
}
```

- [ ] **Step 3: Add skill tree display in game_view.go**

In `GameScreen.Update()`, when transitioning between floors (PhaseExploring and new floor detected), show skill allocation options if player has unspent points. Add keybindings `[` and `]` to select branch, `Enter` to allocate.

In `GameScreen.View()`, render skill tree info in the HUD when points are available.

- [ ] **Step 4: Apply skill bonuses to combat**

The `EffectiveATK()` and `EffectiveDEF()` already include skill bonuses from Task 2. Verify the skill power bonus affects class skills in `resolvePlayerActions()`:
- Mage Fireball: base 0.8x → 0.8x + skillPower/100
- Healer Heal: base 30 HP → 30 + skillPower/2
- etc.

- [ ] **Step 5: Run all tests**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add game/session.go game/turn.go ui/game_view.go entity/
git commit -m "feat: integrate skill tree UI and combat bonuses"
```

---

## Task 10: Final Verification and UI Polish

- [ ] **Step 1: Run full test suite**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Run go vet**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go vet ./...`
Expected: No issues

- [ ] **Step 3: Build**

Run: `cd C:/Users/SSAFY/Desktop/projects/Catacombs && go build -o catacombs.exe .`
Expected: Success

- [ ] **Step 4: Verify elite monsters show prefix in combat UI**

Read `ui/game_view.go` rendering functions — confirm elite monster names (e.g., "Venomous Orc") are displayed. The existing `renderEnemyPanel` should already pick up `m.Name` which includes the prefix from `ApplyPrefix()`.

- [ ] **Step 5: Verify floor theme is displayed**

Confirm the floor theme name appears in the map rendering area. If not, add it to `renderMap()` or the HUD.

- [ ] **Step 6: Commit any UI polish**

```bash
git add -A
git commit -m "chore: phase 2 complete — combat and dungeon enhancement verified"
```
