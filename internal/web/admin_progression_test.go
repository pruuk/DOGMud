package web

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/stats"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// testBalance pins every knob these tests depend on. A test binary never loads
// config.yaml, and auth_test.go's setup chdirs to the repo root and calls
// ReloadConfig() on a binary shared with these tests, so nothing ambient is
// trustworthy here.
func testBalance(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.BaseProgressionChance = 0.12
	cfg.Balance.ProgressionDecayBelowCap = 3.0
	cfg.Balance.ProgressionDecayAboveCap = 2.0
	cfg.Balance.StatProgressionSoftCap = 50
	cfg.Balance.SkillSoftCap = 50
	cfg.Balance.StatProgressionRate = 2.25
	cfg.Balance.ProgressionChanceFloor = 1e-5
	cfg.Balance.StatProgressionMultipliers = map[string]float64{
		"strength": 1.0, "dexterity": 1.0, "perception": 1.0,
		"vitality": 1.0, "willpower": 1.0, "charisma": 1.0,
	}
	configs.SetConfigForTest(t, cfg)
}

func TestUsesToReach_IsIncreasingAndConvex(t *testing.T) {
	testBalance(t)
	c := characters.New()
	chanceAt := func(rank int) float64 {
		c.Stats.Perception.Training = rank
		return c.ProgressionChanceForStat("perception", 1.0)
	}

	prev, prevStep := 0.0, 0.0
	for r := 1; r <= 40; r++ {
		got := usesToReach(chanceAt, r)
		if got <= prev {
			t.Fatalf("usesToReach(%d) = %v, not greater than %v", r, got, prev)
		}
		step := got - prev
		if r > 1 && step <= prevStep {
			t.Errorf("rank %d costs %v, not more than rank %d's %v; curve not decaying", r, step, r-1, prevStep)
		}
		prev, prevStep = got, step
	}
}

// NOTE the Ceil. usesToReach returns a float and expectedRankForUses answers
// "the highest rank affordable within this many uses", so truncating would ask
// for one use less than rank r costs and the inverse would return r-1 for every
// non-integer case.
func TestExpectedRankForUses_InvertsUsesToReach(t *testing.T) {
	testBalance(t)
	c := characters.New()
	chanceAt := func(rank int) float64 {
		c.Skills = map[string]int{"weapon-combat": rank}
		return c.ProgressionChanceForSkill("weapon-combat", 1.0)
	}

	for _, wantRank := range []int{0, 1, 5, 15, 30} {
		uses := usesToReach(chanceAt, wantRank)
		if math.IsInf(uses, 1) {
			t.Fatalf("rank %d unreachable", wantRank)
		}
		if got := expectedRankForUses(chanceAt, int(math.Ceil(uses)), 50); got != wantRank {
			t.Errorf("expectedRankForUses(%d) = %d, want %d", int(math.Ceil(uses)), got, wantRank)
		}
	}
}

// The dashboard must display the chance production rolls. Bare
// CalculateProgressionChance omitted StatProgressionRate and every per-stat,
// per-skill, mutation and buff multiplier -- the reason this page could never
// have surfaced the sealed stats Phase B fixed.
func TestPlayerOverview_ChanceMatchesProduction(t *testing.T) {
	testBalance(t)

	c := characters.New()
	c.Name = "Fixture"
	c.Stats.Strength.Base = 115
	c.Stats.Strength.Training = 21
	c.Stats.Strength.Recalculate()
	c.Skills = map[string]int{"weapon-combat": 12}

	out := buildPlayerOverview([]*users.UserRecord{{UserId: 1, Character: c}})
	if len(out) != 1 {
		t.Fatalf("expected 1 player, got %d", len(out))
	}
	if want, got := c.ProgressionChanceForStat("strength", 1.0), out[0].Stats["strength"].ProgressionChance; math.Abs(got-want) > 1e-12 {
		t.Errorf("dashboard stat chance %.12f, production %.12f", got, want)
	}
	if want, got := c.ProgressionChanceForSkill("weapon-combat", 1.0), out[0].Skills["weapon-combat"].ProgressionChance; math.Abs(got-want) > 1e-12 {
		t.Errorf("dashboard skill chance %.12f, production %.12f", got, want)
	}
}

// Phase B's chance floor makes the sealed state unreachable in production, so
// this alarm is a regression detector. Force the condition and confirm it fires.
func TestPlayerOverview_DeadStatAlarm(t *testing.T) {
	testBalance(t)

	c := characters.New()
	c.Name = "Meirok"
	c.Stats.Dexterity.Base = 98
	c.Stats.Dexterity.Training = 12
	c.Stats.Dexterity.Recalculate()

	out := buildPlayerOverview([]*users.UserRecord{{UserId: 3, Character: c}})
	if dex := out[0].Stats["dexterity"]; dex.Dead || dex.ProgressionChance <= 0 {
		t.Errorf("dexterity reported dead at training 12; chance %.9f", dex.ProgressionChance)
	}

	cfg := configs.GetConfig()
	cfg.Balance.ProgressionChanceFloor = 0 // disable the floor to reach the condition
	cfg.Balance.StatProgressionMultipliers = map[string]float64{"dexterity": 1e-12}
	configs.SetConfigForTest(t, cfg)

	out = buildPlayerOverview([]*users.UserRecord{{UserId: 3, Character: c}})
	if !out[0].Stats["dexterity"].Dead {
		t.Error("dead-stat alarm did not fire when the threshold truncates to 0")
	}
}

// Fragile is the warning tier the monolith dropped: a threshold under 10 parts
// per million is not sealed, but it is one config nudge away. Pick a multiplier
// that lands the threshold inside (0, 10).
func TestPlayerOverview_FragileWarningTier(t *testing.T) {
	testBalance(t)

	c := characters.New()
	c.Name = "Fragile"
	c.Stats.Charisma.Base = 93
	c.Stats.Charisma.Training = 30
	c.Stats.Charisma.Recalculate()

	// Unfloored chance at training 30 is ~0.12*exp(-1.8)*2.25 = 0.0447.
	// Scaling by 1e-4 puts it near 4.5e-6, i.e. a threshold of 4.
	cfg := configs.GetConfig()
	cfg.Balance.ProgressionChanceFloor = 0
	cfg.Balance.StatProgressionMultipliers = map[string]float64{"charisma": 1e-4}
	configs.SetConfigForTest(t, cfg)

	cha := buildPlayerOverview([]*users.UserRecord{{UserId: 4, Character: c}})[0].Stats["charisma"]
	if threshold := characters.ProgressionRollThreshold(cha.ProgressionChance); threshold <= 0 || threshold >= 10 {
		t.Fatalf("fixture precondition failed: threshold = %d, want 1..9", threshold)
	}
	if cha.Dead {
		t.Error("charisma reported dead when its threshold is still positive")
	}
	if !cha.Fragile {
		t.Error("fragile warning did not fire for a threshold under 10")
	}
}

// The live pre-existing bug. StatInfo.Value is yaml:"-" and loadRecentUserFiles
// unmarshals raw YAML without Recalculate(), so an offline player reports 0.
//
// The fixture must NOT use characters.New(): New() calls Validate(), which
// populates Value and hydrates Base, so a New()-based fixture cannot reproduce
// the defect and asserts vacuously.
func TestStatHealth_OfflinePlayerIsNotBucketedAsZero(t *testing.T) {
	testBalance(t)

	c := &characters.Character{
		Name: "Offline",
		Stats: stats.Statistics{
			Strength: stats.StatInfo{Base: 115, Training: 21}, // Value deliberately 0
		},
	}
	if c.Stats.Strength.Value != 0 {
		t.Fatalf("fixture precondition failed: Value = %d, want 0", c.Stats.Strength.Value)
	}

	got := buildStatHealth([]*users.UserRecord{{UserId: 9, Character: c}})
	if got["strength"].Distribution["0-50"] != 0 {
		t.Errorf("offline player bucketed as 0-50: %v", got["strength"].Distribution)
	}
	if got["strength"].Distribution["101-150"] != 1 {
		t.Errorf("expected the 136-point player in 101-150: %v", got["strength"].Distribution)
	}
}

// Spec section 10.4 item 4: the curve reads Training, so a Training histogram
// is the population view that describes progression difficulty. The value
// histogram stays as a second series.
func TestStatHealth_TrainingDistribution(t *testing.T) {
	testBalance(t)

	c := &characters.Character{
		Name: "Offline",
		Stats: stats.Statistics{
			Perception: stats.StatInfo{Base: 101, Training: 51},
			Vitality:   stats.StatInfo{Base: 104, Training: 14},
		},
	}
	got := buildStatHealth([]*users.UserRecord{{UserId: 9, Character: c}})
	if got["perception"].TrainingDistribution["51+"] != 1 {
		t.Errorf("perception training 51 not in 51+: %v", got["perception"].TrainingDistribution)
	}
	if got["vitality"].TrainingDistribution["11-25"] != 1 {
		t.Errorf("vitality training 14 not in 11-25: %v", got["vitality"].TrainingDistribution)
	}
	// Strength was never trained on this fixture and must land in the 0 bucket,
	// not vanish.
	if got["strength"].TrainingDistribution["0"] != 1 {
		t.Errorf("untrained strength not in the 0 bucket: %v", got["strength"].TrainingDistribution)
	}
}
