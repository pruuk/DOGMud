package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// Rank is trained points, not uses. A character with an enormous use counter and
// no trained points is a BEGINNER and must roll the same chance as a fresh one.
//
// The old model measured how often you had swung, which punished frequency: a
// character who used a stat constantly exhausted its curve while one who used it
// rarely kept a cheap rank forever.
func TestStatRank_IsTrainingNotUseCount(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "ManyUsesNoGains"
	c.StatUseCount["dexterity"] = 39772 // Meirok's real counter
	c.Stats.Dexterity.Training = 0
	c.Stats.Dexterity.Recalculate()

	fresh := New()
	fresh.Name = "Fresh"

	got := c.statProgressionChance("dexterity", 1.0)
	want := fresh.statProgressionChance("dexterity", 1.0)
	if got != want {
		t.Errorf("use count still moves the chance: %v vs a fresh character's %v", got, want)
	}
}

// Equipment must never make a stat harder to train. This is the gear leak the
// deleted value floor caused: it keyed on GetStatValue, which includes Mods.
func TestStatRank_EquipmentDoesNotRaiseDifficulty(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "Geared"
	c.Stats.Strength.Base = 120
	c.Stats.Strength.Training = 10
	c.Stats.Strength.Recalculate()
	bare := c.statProgressionChance("strength", 1.0)

	c.Stats.Strength.SetMod(80) // a big stat item
	c.Stats.Strength.Recalculate()
	if got := c.statProgressionChance("strength", 1.0); got != bare {
		t.Errorf("equipment changed training difficulty: %v geared vs %v bare", got, bare)
	}
}

// A high Base must not either. A mob with a large authored stat pool is not
// harder to train than a small one; that asymmetry is what the re-key removes.
func TestStatRank_BaseDoesNotRaiseDifficulty(t *testing.T) {
	withRepoRoot(t)

	small := New()
	small.Stats.Vitality.Base = 60
	small.Stats.Vitality.Recalculate()

	huge := New()
	huge.Stats.Vitality.Base = 600
	huge.Stats.Vitality.Recalculate()

	a := small.statProgressionChance("vitality", 1.0)
	b := huge.statProgressionChance("vitality", 1.0)
	if a != b {
		t.Errorf("Base moved the chance: %v at base 60 vs %v at base 600", a, b)
	}
}

// Trained points DO raise difficulty. The complement of the three tests above:
// the re-key must not flatten the curve, only re-key it.
func TestStatRank_TrainingRaisesDifficulty(t *testing.T) {
	withRepoRoot(t)

	fresh := New()
	trained := New()
	trained.Stats.Perception.Training = 40
	trained.Stats.Perception.Recalculate()

	a := fresh.statProgressionChance("perception", 1.0)
	b := trained.statProgressionChance("perception", 1.0)
	if !(b < a) {
		t.Errorf("40 trained points did not make the stat harder: fresh %v, trained %v", a, b)
	}
}

// The two documented anchors from spec section 5, which are what pin soft cap 50.
// perception's per-stat multiplier is 1.0, so it shows the raw curve.
func TestStatChance_ReproducesTheDocumentedAnchors(t *testing.T) {
	withRepoRoot(t)

	b := configs.GetBalanceConfig()
	if int(b.StatProgressionSoftCap) != 50 {
		t.Fatalf("StatProgressionSoftCap is %d, want 50", int(b.StatProgressionSoftCap))
	}

	fresh := New()
	if got := fresh.statProgressionChance("perception", 1.0); got < 0.26 || got > 0.28 {
		t.Errorf("fresh stat chance %v, want ~0.27 (0.12 x 2.25)", got)
	}

	trained := New()
	trained.Stats.Perception.Training = 50 // the old "stat at 150"
	trained.Stats.Perception.Recalculate()
	if got := trained.statProgressionChance("perception", 1.0); got < 0.012 || got > 0.015 {
		t.Errorf("chance at Training 50 is %v, want ~0.0134", got)
	}
}

// Skills key on the skill level. Above the soft cap they effectively already
// did; below it, the use counter must stop mattering.
func TestSkillRank_IsLevelNotUseCount(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "SkillUses"
	c.Skills["weapon-combat"] = 5
	c.SkillUseCount["weapon-combat"] = 40000

	fresh := New()
	fresh.Skills["weapon-combat"] = 5

	got := c.skillProgressionChance("weapon-combat", 1.0)
	want := fresh.skillProgressionChance("weapon-combat", 1.0)
	if got != want {
		t.Errorf("skill use count still moves the chance: %v vs %v", got, want)
	}
}

// And the skill level itself still decays the curve.
func TestSkillRank_LevelRaisesDifficulty(t *testing.T) {
	withRepoRoot(t)

	low := New()
	low.Skills["weapon-combat"] = 1
	high := New()
	high.Skills["weapon-combat"] = 30

	a := low.skillProgressionChance("weapon-combat", 1.0)
	b := high.skillProgressionChance("weapon-combat", 1.0)
	if !(b < a) {
		t.Errorf("skill level did not decay the curve: level 1 %v, level 30 %v", a, b)
	}
}

// regenDamperFactor is the fourth rank site and is easy to forget: it damps the
// regen faucet by rank, and if it kept reading the use counter the faucet would
// stay keyed to the retired model after everything else moved.
func TestRegenDamper_KeysOnTrainingNotUseCount(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.StatUseCount["vitality"] = 50000
	c.Stats.Vitality.Training = 0
	c.Stats.Vitality.Recalculate()

	fresh := New()

	if got, want := c.regenDamperFactor("vitality"), fresh.regenDamperFactor("vitality"); got != want {
		t.Errorf("regen damper still reads the use counter: %v vs %v", got, want)
	}

	trained := New()
	trained.Stats.Vitality.Training = 40
	trained.Stats.Vitality.Recalculate()
	if got := trained.regenDamperFactor("vitality"); !(got < fresh.regenDamperFactor("vitality")) {
		t.Errorf("regen damper did not bite at Training 40: %v vs fresh %v",
			got, fresh.regenDamperFactor("vitality"))
	}
}
