package combat

import "testing"

func TestDetectSoloChainEchoStrikeAfterSkill(t *testing.T) {
	c := DetectSoloChain("skill", 0)
	if c == nil {
		t.Fatal("skill → attack should trigger Echo Strike")
	}
	if c.DamageMultiplier != 1.4 {
		t.Errorf("Echo Strike multiplier should be 1.4, got %v", c.DamageMultiplier)
	}
	if c.GuaranteeCrit {
		t.Error("Echo Strike must not guarantee crit")
	}
}

func TestDetectSoloChainCounterStanceAfterCoop(t *testing.T) {
	c := DetectSoloChain("coop", 0)
	if c == nil {
		t.Fatal("coop → attack should trigger Counter Stance")
	}
	if !c.GuaranteeCrit {
		t.Error("Counter Stance should guarantee crit")
	}
	if c.DamageMultiplier != 0 {
		t.Errorf("Counter Stance should not add a damage multiplier, got %v", c.DamageMultiplier)
	}
}

func TestDetectSoloChainRelentlessFromThirdAttack(t *testing.T) {
	// streak counts attacks BEFORE this one: 2 prior attacks → this is the 3rd.
	c := DetectSoloChain("attack", 2)
	if c == nil {
		t.Fatal("3rd consecutive attack should trigger Relentless")
	}
	if c.DamageMultiplier != 1.25 || c.BonusDamage != 3 {
		t.Errorf("Relentless should be 1.25x +3, got %vx +%d", c.DamageMultiplier, c.BonusDamage)
	}
	if DetectSoloChain("attack", 5) == nil {
		t.Error("Relentless should keep firing past the 3rd attack")
	}
}

func TestDetectSoloChainNilCases(t *testing.T) {
	cases := []struct {
		last   string
		streak int
	}{
		{"", 0},        // fresh combat
		{"attack", 0},  // 1st attack
		{"attack", 1},  // 2nd attack
		{"item", 0},    // item breaks chains
		{"", 5},        // broken streak with stale count (defensive)
	}
	for _, tc := range cases {
		if c := DetectSoloChain(tc.last, tc.streak); c != nil {
			t.Errorf("DetectSoloChain(%q, %d) should be nil, got %s", tc.last, tc.streak, c.Name)
		}
	}
}

func TestDetectSoloChainReturnsCopies(t *testing.T) {
	a := DetectSoloChain("skill", 0)
	b := DetectSoloChain("skill", 0)
	a.DamageMultiplier = 99
	if b.DamageMultiplier == 99 {
		t.Error("DetectSoloChain must return independent copies")
	}
}

func TestSoloChainsHaveNamesAndMessages(t *testing.T) {
	for _, tc := range []struct {
		last   string
		streak int
	}{{"skill", 0}, {"coop", 0}, {"attack", 2}} {
		c := DetectSoloChain(tc.last, tc.streak)
		if c == nil {
			t.Fatalf("expected chain for (%q, %d)", tc.last, tc.streak)
		}
		if c.Name == "" || c.Message == "" {
			t.Errorf("chain for (%q, %d) missing Name/Message: %+v", tc.last, tc.streak, c)
		}
	}
}
