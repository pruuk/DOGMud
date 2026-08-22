package mobs

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/species"
)

// StatNames is the canonical stat order used by the legacy-training migration.
var StatNames = [6]string{"strength", "dexterity", "perception", "vitality", "willpower", "charisma"}

// InstanceSchemaVersion is the current save-schema version. A save without one
// (zero) predates U10b-0 Phase C and needs LegacyTrainingToGains applied on
// load; a save at this version is already gains-only and is restored verbatim.
//
// Without this marker the migration would run again on the next load and
// subtract the pool a second time.
const InstanceSchemaVersion = 1

// LegacyTrainingToGains converts a pre-U10b-0 saved per-stat Training map into
// gains-since-spawn, for the given mob template.
//
// Before Phase C, a saved Training value was three things fused together:
//
//	saved = authored + spawnPool + gains
//
// The template's authored stats were copied into Training at spawn, the runtime
// stat pool was added on top of them, and progression then accumulated in the
// same field. After Phase C the first two live in Base — the template supplies
// authored, and a fresh spawn re-rolls the pool — so reusing a saved value
// as-is would count both a second time. An untouched pet would roughly double.
//
// Both subtracted terms are recoverable:
//
//   - authored = template.Base − species.Base, exactly. Phase A folded each
//     mob's authored `training:` into `base:` as species_base + authored, so
//     differencing against the species record recovers it. Verified against all
//     1,764 folded rows with zero mismatches.
//   - spawnPool = template.StatPool, exact in total.
//
// The per-stat split of the pool is NOT recoverable — the roll was random and
// was never recorded. Rather than guess it with an even split (which would be
// badly wrong for archetype-weighted mobs, leaving phantom gains on the favoured
// stats and eating real ones on the rest), the subtraction is distributed
// proportionally to each stat's own saved value. That hits the total exactly and
// preserves the shape of the saved distribution, which reflects the same
// archetype weighting the pool followed.
//
// Returns a map with the same keys as saved. If the template or species record
// is missing, saved is returned unchanged: refusing to migrate is better than
// corrupting a value with a guessed baseline.
func LegacyTrainingToGains(mobId MobId, saved map[string]int) map[string]int {
	out := make(map[string]int, len(saved))

	savedTotal := 0
	for _, s := range StatNames {
		savedTotal += saved[s]
	}
	if savedTotal <= 0 {
		for k := range saved {
			out[k] = 0
		}
		return out
	}

	tmpl := GetMobSpec(mobId)
	if tmpl == nil {
		return copyStatMap(saved)
	}
	sp := species.GetSpecies(tmpl.Character.SpeciesId)
	if sp == nil {
		return copyStatMap(saved)
	}

	ts, ss := &tmpl.Character.Stats, &sp.Stats
	authored := (ts.Strength.Base - ss.Strength.Base) +
		(ts.Dexterity.Base - ss.Dexterity.Base) +
		(ts.Perception.Base - ss.Perception.Base) +
		(ts.Vitality.Base - ss.Vitality.Base) +
		(ts.Willpower.Base - ss.Willpower.Base) +
		(ts.Charisma.Base - ss.Charisma.Base)
	if authored < 0 {
		authored = 0
	}

	gainsTotal := savedTotal - authored - tmpl.StatPool
	if gainsTotal <= 0 {
		// Never progressed beyond what it spawned with. This is the common case
		// and it must land on exactly zero, not on a small positive remainder.
		for k := range saved {
			out[k] = 0
		}
		return out
	}
	if gainsTotal > savedTotal {
		gainsTotal = savedTotal
	}

	scale := float64(gainsTotal) / float64(savedTotal)
	assigned := 0
	for _, s := range StatNames {
		v := int(math.Round(float64(saved[s]) * scale))
		if v < 0 {
			v = 0
		}
		out[s] = v
		assigned += v
	}
	// Rounding drift: hand the remainder to (or take it from) the largest stat,
	// so the totals agree exactly.
	if diff := gainsTotal - assigned; diff != 0 {
		largest := StatNames[0]
		for _, s := range StatNames {
			if out[s] > out[largest] {
				largest = s
			}
		}
		if out[largest]+diff >= 0 {
			out[largest] += diff
		}
	}
	// Preserve any keys the caller had that are not canonical stats.
	for k, v := range saved {
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out
}

func copyStatMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
