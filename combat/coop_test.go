package combat

import (
	"testing"

	"github.com/tolelom/catacombs/entity"
)

func TestWarriorCoopReducesSelfDamageOnly(t *testing.T) {
	w := entity.NewPlayer("Tank", entity.ClassWarrior)
	w.Fingerprint = "fp-w"
	res := ApplyWarriorCoop(w)
	if got := res.DamageReduction[w.Fingerprint]; got != 0.75 {
		t.Errorf("warrior reduction = %v, want 0.75", got)
	}
	if len(res.DamageReduction) != 1 {
		t.Errorf("Bulwark should affect only the warrior, got %d entries", len(res.DamageReduction))
	}
}

func TestMageCoopMultiTargetsParty(t *testing.T) {
	m := entity.NewPlayer("Mage", entity.ClassMage)
	m.Fingerprint = "fp-m"
	w := entity.NewPlayer("Tank", entity.ClassWarrior)
	w.Fingerprint = "fp-w"
	dead := entity.NewPlayer("Ghost", entity.ClassRogue)
	dead.Fingerprint = "fp-d"
	dead.Dead = true
	party := []*entity.Player{m, w, dead}

	res := ApplyMageCoop(m, party, false)
	if res.DamageReduction[m.Fingerprint] != 0.50 || res.DamageReduction[w.Fingerprint] != 0.50 {
		t.Errorf("Ward should give 0.50 to all alive party, got %+v", res.DamageReduction)
	}
	if _, has := res.DamageReduction[dead.Fingerprint]; has {
		t.Error("Ward should skip dead members")
	}
}

func TestMageCoopSoloOnlySelf(t *testing.T) {
	m := entity.NewPlayer("Mage", entity.ClassMage)
	m.Fingerprint = "fp-m"
	other := entity.NewPlayer("Other", entity.ClassWarrior)
	other.Fingerprint = "fp-o"
	res := ApplyMageCoop(m, []*entity.Player{m, other}, true)
	if len(res.DamageReduction) != 1 || res.DamageReduction[m.Fingerprint] != 0.50 {
		t.Errorf("Solo Ward should target self only, got %+v", res.DamageReduction)
	}
}

func TestHealerCoopRevivesWhenAllowed(t *testing.T) {
	h := entity.NewPlayer("Heal", entity.ClassHealer)
	h.Fingerprint = "fp-h"
	corpse := entity.NewPlayer("Dead", entity.ClassWarrior)
	corpse.Fingerprint = "fp-c"
	corpse.Dead = true
	res := ApplyHealerCoop(h, []*entity.Player{h, corpse}, true)
	if res.ReviveFingerprint != "fp-c" {
		t.Errorf("expected revive on dead corpse, got %q", res.ReviveFingerprint)
	}
	if res.ReviveHPPercent != 0.50 {
		t.Errorf("revive HP percent = %v, want 0.50", res.ReviveHPPercent)
	}
}

func TestHealerCoopHealsWhenNoCorpse(t *testing.T) {
	h := entity.NewPlayer("Heal", entity.ClassHealer)
	h.Fingerprint = "fp-h"
	w := entity.NewPlayer("W", entity.ClassWarrior)
	w.Fingerprint = "fp-w"
	res := ApplyHealerCoop(h, []*entity.Player{h, w}, true)
	if res.ReviveFingerprint != "" {
		t.Error("no corpse → no revive expected")
	}
	if len(res.Logs) == 0 {
		t.Error("expected heal log line")
	}
}

func TestHealerCoopReviveBlockedWhenUsed(t *testing.T) {
	h := entity.NewPlayer("Heal", entity.ClassHealer)
	h.Fingerprint = "fp-h"
	corpse := entity.NewPlayer("Dead", entity.ClassWarrior)
	corpse.Fingerprint = "fp-c"
	corpse.Dead = true
	res := ApplyHealerCoop(h, []*entity.Player{h, corpse}, false)
	if res.ReviveFingerprint != "" {
		t.Error("reviveAllowed=false should suppress revive")
	}
}
