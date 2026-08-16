package spells

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

// shippedSummonCalibration is the table the design settled on: every reservation
// figure in the companion model derives from these multipliers, and every cast
// price is the flat entry gate the same design set. Pinning them here means a
// typo in a YAML file fails a test rather than surfacing in play as a
// mysteriously cheap or mysteriously feeble companion.
var shippedSummonCalibration = map[string]struct {
	mult float64
	cost int
}{
	"conjure-magma": {1.25, 50},
	"conjure-earth": {1.05, 45},
	"conjure-fire":  {1.00, 45},
	"conjure-air":   {0.90, 40},
	"conjure-water": {0.75, 30},

	"raise-golem":    {1.00, 50},
	"raise-vampire":  {0.83, 45},
	"raise-spectre":  {0.75, 40},
	"raise-zombie":   {0.67, 35},
	"raise-wraith":   {0.58, 35},
	"raise-skeleton": {0.50, 30},

	"summon-steppe-spirit": {0.75, 35},
	"summon-hive-swarm":    {0.30, 30},
}

// shippedSpellDir is the real data directory, relative to this package. A Go
// test binary runs with its working directory set to the package directory, so
// the shipped files are two levels up.
const shippedSpellDir = `../../_datafiles/world/dogmud/spells`

// TestShippedSummonPetMultipliers reads the files the server actually loads.
//
// It decodes them directly rather than going through LoadSpellFiles, which
// would need the config plumbing and would replace the package globals that
// SeedSpellsForTest hands to other tests in this binary. Decoding into the same
// SpellData the loader uses still proves the struct tag maps the key, which is
// the half of this that could silently regress.
func TestShippedSummonPetMultipliers(t *testing.T) {
	for id, want := range shippedSummonCalibration {
		path := filepath.Join(shippedSpellDir, id+".yaml")

		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}

		var sd SpellData
		if err := yaml.Unmarshal(raw, &sd); err != nil {
			t.Errorf("%s: unmarshal: %v", id, err)
			continue
		}

		if sd.SpellId != id {
			t.Errorf("%s: spellid = %q, want %q (filename and spellid must agree)", id, sd.SpellId, id)
		}
		if sd.SummonMobId <= 0 {
			t.Errorf("%s: summon_mob_id = %d, want a real mob", id, sd.SummonMobId)
		}
		if sd.SummonPetMultiplier != want.mult {
			t.Errorf("%s: summon_pet_multiplier = %v, want %v", id, sd.SummonPetMultiplier, want.mult)
		}
		if sd.Cost != want.cost {
			t.Errorf("%s: cost = %d, want %d", id, sd.Cost, want.cost)
		}
	}
}

// TestShippedSummonsAllCarryAMultiplier is the completeness half: the table
// above pins the thirteen spells it knows about, but a fourteenth summon added
// later with no multiplier would field a companion floored to a pool of one and
// pass every assertion above by simply not being listed.
func TestShippedSummonsAllCarryAMultiplier(t *testing.T) {
	entries, err := os.ReadDir(shippedSpellDir)
	if err != nil {
		t.Fatalf("read %s: %v", shippedSpellDir, err)
	}

	seen := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(shippedSpellDir, e.Name()))
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}

		var sd SpellData
		if err := yaml.Unmarshal(raw, &sd); err != nil {
			t.Errorf("%s: unmarshal: %v", e.Name(), err)
			continue
		}
		if sd.SummonMobId <= 0 {
			continue
		}
		seen++

		if sd.SummonPetMultiplier <= 0 {
			t.Errorf("%s: summons mob %d but has no summon_pet_multiplier; its companion "+
				"pool would floor to 1", e.Name(), sd.SummonMobId)
		}
		if _, pinned := shippedSummonCalibration[sd.SpellId]; !pinned {
			t.Errorf("%s: new summon spell %q is not in shippedSummonCalibration; add it "+
				"with the multiplier and cast cost the design assigned", e.Name(), sd.SpellId)
		}
	}

	if seen != len(shippedSummonCalibration) {
		t.Errorf("found %d summon spells on disk, expected %d", seen, len(shippedSummonCalibration))
	}
}
