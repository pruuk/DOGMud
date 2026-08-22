package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/stats"
)

// withSpeciesData makes the real species roster available to a test, and only
// to that test.
//
// species.LoadDataFiles() is only called from main.go, so a test binary starts
// with an empty roster. Filling it permanently is NOT inert: this package's
// Wear tests build bare fixtures whose behaviour changes once a species record
// exists to hydrate from, and they fail if the roster is left loaded.
// species.LoadForTest snapshots and restores it, the same contract as
// configs.SetConfigForTest.
//
// The load reads configs.GetFilePathsConfig().DataFiles relative to the working
// directory, so withRepoRoot (poolmax_test.go) must do its chdir first.
func withSpeciesData(t *testing.T) {
	t.Helper()
	withRepoRoot(t)
	species.LoadForTest(t)
}

// GetStatTraining reads .Training only. Once Phase A folds authored mob
// training into base and Phase C moves spawn pools there too, Training means
// gains-since-spawn for every character type — so the progression curve keyed to
// it is gear-free and baseline-free by construction.
func TestGetStatTraining_ReadsTrainingOnly(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 115
	c.Stats.Strength.Training = 21
	c.Stats.Strength.SetMod(40)
	c.Stats.Strength.Recalculate()

	if got := c.GetStatTraining("strength"); got != 21 {
		t.Errorf("GetStatTraining(strength) = %d, want 21 (Base and Mods must not leak)", got)
	}
	if got := c.GetStatValue("strength"); got != 176 {
		t.Errorf("sanity: GetStatValue(strength) = %d, want 176", got)
	}
}

func TestGetStatTraining_AllSixStats(t *testing.T) {
	c := New()
	c.Stats.Strength.Training = 1
	c.Stats.Dexterity.Training = 2
	c.Stats.Perception.Training = 3
	c.Stats.Vitality.Training = 4
	c.Stats.Willpower.Training = 5
	c.Stats.Charisma.Training = 6

	for name, want := range map[string]int{
		"strength": 1, "dexterity": 2, "perception": 3,
		"vitality": 4, "willpower": 5, "charisma": 6,
	} {
		if got := c.GetStatTraining(name); got != want {
			t.Errorf("GetStatTraining(%q) = %d, want %d", name, got, want)
		}
	}
	if got := c.GetStatTraining("nonsense"); got != 0 {
		t.Errorf("GetStatTraining(nonsense) = %d, want 0", got)
	}
}

// ── StatPoolTotal ───────────────────────────────────────────────────────────

// poolFixture builds a bare character with the six Base/Training pairs set.
//
// Deliberately NOT characters.New(): New() rolls a gaussian Base for all six
// stats, so two New() characters never agree and an "unchanged by
// representation" comparison between them would be comparing two different
// creatures. StatPoolTotal reads Base, Training and SpeciesId only, so none of
// New()'s machinery is needed.
func poolFixture(speciesId int, pairs [6][2]int) *Character {
	c := &Character{SpeciesId: speciesId}
	set := []*stats.StatInfo{
		&c.Stats.Strength, &c.Stats.Dexterity, &c.Stats.Perception,
		&c.Stats.Vitality, &c.Stats.Willpower, &c.Stats.Charisma,
	}
	for i, p := range pairs {
		set[i].Base, set[i].Training = p[0], p[1]
		set[i].Recalculate()
	}
	return c
}

// StatPoolTotal is "how much creature is there": the authored pool plus gains,
// excluding the species baseline and excluding equipment.
//
// This invariance is what lets the accessor land before the data moves. The same
// creature expressed either way must report the same pool.
func TestStatPoolTotal_UnchangedByWhereThePoolLives(t *testing.T) {
	// "Before": species baseline in Base, authored pool in Training.
	// "After" (post-fold): both in Base. Same creature, same answer.
	before := poolFixture(1, [6][2]int{{100, 30}, {100, 20}, {100, 0}, {100, 0}, {100, 0}, {100, 0}})
	after := poolFixture(1, [6][2]int{{130, 0}, {120, 0}, {100, 0}, {100, 0}, {100, 0}, {100, 0}})

	if before.StatPoolTotal() != after.StatPoolTotal() {
		t.Errorf("pool total moved with the representation: %d vs %d",
			before.StatPoolTotal(), after.StatPoolTotal())
	}
}

// The invariance test above passes even for an implementation that never
// subtracts the species baseline, because the baseline is identical on both
// sides. This one does not: it pins the actual number the four consumers'
// thresholds are calibrated against. A species-1 creature carrying nothing but
// its baseline is a pool of zero, not a pool of 600.
func TestStatPoolTotal_SubtractsTheSpeciesBaseline(t *testing.T) {
	withSpeciesData(t)
	if species.GetSpecies(1) == nil {
		t.Fatal("species 1 did not load; the subtraction is untested without it")
	}

	baselineOnly := poolFixture(1, [6][2]int{{100, 0}, {100, 0}, {100, 0}, {100, 0}, {100, 0}, {100, 0}})
	if got := baselineOnly.StatPoolTotal(); got != 0 {
		t.Errorf("a creature that is nothing but its species baseline has pool %d, want 0", got)
	}

	trained := poolFixture(1, [6][2]int{{100, 30}, {100, 20}, {100, 0}, {100, 0}, {100, 0}, {100, 0}})
	if got := trained.StatPoolTotal(); got != 50 {
		t.Errorf("StatPoolTotal = %d, want 50 (the authored pool, baseline excluded)", got)
	}
}

func TestStatPoolTotal_CountsGains(t *testing.T) {
	c := poolFixture(1, [6][2]int{{130, 0}, {100, 0}, {100, 0}, {100, 0}, {100, 0}, {100, 0}})
	baseline := c.StatPoolTotal()

	c.Stats.Strength.Training = 7
	c.Stats.Strength.Recalculate()
	if got := c.StatPoolTotal(); got != baseline+7 {
		t.Errorf("StatPoolTotal = %d after 7 gains, want %d", got, baseline+7)
	}
}

func TestStatPoolTotal_IgnoresEquipment(t *testing.T) {
	c := poolFixture(1, [6][2]int{{130, 0}, {100, 0}, {100, 0}, {100, 0}, {100, 0}, {100, 0}})
	baseline := c.StatPoolTotal()

	c.Stats.Strength.SetMod(50)
	c.Stats.Strength.Recalculate()
	if got := c.StatPoolTotal(); got != baseline {
		t.Errorf("equipment leaked into StatPoolTotal: %d, want %d", got, baseline)
	}
}

// Species baselines are NOT uniform — they range from 0 (20-orb has no stats
// block at all) to 6000 (99-ascended); only species 1 sums to 600. An unknown
// species contributes no baseline, which is the right fallback because such a
// character's Base was never hydrated either.
func TestStatPoolTotal_UnknownSpeciesContributesNoBaseline(t *testing.T) {
	c := poolFixture(999999, [6][2]int{{42, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}})
	if got := c.StatPoolTotal(); got != 42 {
		t.Errorf("StatPoolTotal = %d for an unknown species, want 42", got)
	}
}
