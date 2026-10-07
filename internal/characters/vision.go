package characters

import (
	"math"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/lightscale"
	"github.com/GoMudEngine/GoMud/internal/mutations"
)

// bareFlagDefaults names the bestVisionNumber argument at its one call site
// (NightVisionStrength), so it does not read as a bare true. Only nightvision
// defaults on a bare flag; see bestVisionNumber's doc comment.
const bareFlagDefaults = true

// NightVisionStrength reports how far DOWN the light scale this character's
// usable band shifts, in scale points.
//
// Zero means no night sight at all. A character holding a vision flag that
// declares no strength of its own falls back to the configured default, so a
// bare flag still means something; see LightDefaultVisionStrength.
//
// The strongest source wins across conditions and mutations alike. It is
// never summed: the window MOVES rather than widening, so two abilities
// cannot combine into a window wider than the better one grants.
func (c *Character) NightVisionStrength() int {
	return c.bestVisionNumber(conditions.EffectNightVisionStrength, conditions.NightVision, bareFlagDefaults)
}

// InfraReach reports how far into the dark this character still reads shapes
// by sensing heat: shapes at any light down to minus this number. Zero means
// not at all.
//
// Independent of NightVisionStrength on purpose: a creature can sense heat
// deeply while being no better than anyone else at using faint light.
//
// Unlike nightvision it COMBINES its sources (lighting plan 5c, owner ruling):
// every held condition's reach and every mutation's rank-scaled reach are
// combined through lightscale.Combine at the light scale's doubling step,
// the rule room light already uses (since lighting plan 6 a sum of linear
// brightness), so a single source reads its own value, two equal sources
// read a little under one step above one, and a much weaker source adds
// almost nothing. The result is capped at LightInfraReachCap and rounded
// once. A bare infrared flag with no number still reads zero.
func (c *Character) InfraReach() int {
	var vals []float64
	for _, v := range c.Conditions.EffectValues(conditions.EffectInfraReach) {
		if v > 0 {
			vals = append(vals, v)
		}
	}
	for _, v := range mutations.FlagValues(c.Mutations, string(conditions.InfraredVision)) {
		if v > 0 {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return 0
	}
	cfg := configs.GetLightingConfig()
	reach := lightscale.Combine(cfg.DoublingStep, vals...)
	if limit := float64(cfg.InfraReachCap); reach > limit {
		reach = limit
	}
	return int(math.Round(reach))
}

// bestVisionNumber serves NightVisionStrength only; InfraReach combines its
// sources instead of taking the strongest (lighting plan 5c). effectKind is
// the numeric channel (conditions.Conditions.Effect, which aggregates every
// held, unexpired condition by MAX for a max kind; see effects.go's isMax).
// flag is the boolean channel, checked both on conditions and on mutations
// via mutations.FlagValue, which independently scales each candidate mutation
// by its own rank and also returns the largest scaled value, never a sum.
//
// Both channels are compared here in float64, and the winner is rounded to
// the nearest int exactly once, at the return. Rounding, like truncation, is
// a monotonic non-decreasing function of its input, so for a monotonic
// function f, max(f(a), f(b)) == f(max(a, b)) always: casting each source to
// int before comparing, versus comparing the two float64 sources and casting
// only the winner, can NEVER pick a different winner. What comparing first
// buys is not a different WINNER, it is one rounding instead of two, and it
// matches the codebase's existing convention of a single math.Round at the
// point where a float becomes an int (see internal/characters/companions.go
// and internal/characters/cast_helpers.go), rather than int()'s silent
// truncation.
//
// defaultOnBareFlag says whether a flag carrying no number falls back to the
// configured default. Nightvision's does: "sees in the dark" plainly means at
// least a little. (A bare infrared flag, "senses heat" with no stated range,
// has no sensible fallback; InfraReach reads it as 0.)
func (c *Character) bestVisionNumber(effectKind conditions.EffectKind, flag conditions.Flag, defaultOnBareFlag bool) int {
	best := c.Conditions.Effect(effectKind)

	if m := mutations.FlagValue(c.Mutations, string(flag)); m > best {
		best = m
	}

	if best == 0 && defaultOnBareFlag && c.HasFlagFromAnySource(flag) {
		best = float64(configs.GetBalanceConfig().LightDefaultVisionStrength)
	}

	return int(math.Round(best))
}
