package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Chunk 5.11f — reflect damage answers to the attacker's matching mitigation.
// Before this it bypassed mitigation entirely.
// ---------------------------------------------------------------------------

func TestReflectDamage_UnmitigatedMatchesLegacy(t *testing.T) {
	// 100 damage dealt, 25% reflect, no mitigation -> 25, the old behaviour.
	assert.Equal(t, 25, ReflectDamage(100, 25, 0.0, 0.75))
}

func TestReflectDamage_MitigationReduces(t *testing.T) {
	// Same, but the attacker carries 40% of the matching channel.
	assert.Equal(t, 15, ReflectDamage(100, 25, 0.40, 0.75),
		"25 reflected, 40%% mitigated -> 15")
}

func TestReflectDamage_RespectsCap(t *testing.T) {
	// 90% mitigation is clamped to the 75% cap, so 25 -> 6 (6.25 truncated).
	assert.Equal(t, 6, ReflectDamage(100, 25, 0.90, 0.75),
		"mitigation above the cap must clamp to the cap")
}

func TestReflectDamage_ZeroPercentIsZero(t *testing.T) {
	assert.Equal(t, 0, ReflectDamage(100, 0, 0.0, 0.75))
	assert.Equal(t, 0, ReflectDamage(100, -5, 0.0, 0.75))
}

func TestReflectDamage_ZeroDamageIsZero(t *testing.T) {
	assert.Equal(t, 0, ReflectDamage(0, 25, 0.0, 0.75))
}

// The Elemental King case that motivated the chunk: 25% reflect, and a player
// who has invested in the RIGHT channel should feel it.
func TestReflectDamage_ChannelInvestmentPaysOff(t *testing.T) {
	const dealt, pct = 200.0, 25

	bare := ReflectDamage(dealt, pct, 0.0, 0.75)
	warded := ReflectDamage(dealt, pct, 0.50, 0.75)

	assert.Equal(t, 50, bare)
	assert.Equal(t, 25, warded)
	assert.Less(t, warded, bare, "magical mitigation must blunt elemental backlash")
}

func TestParseReflectChannel(t *testing.T) {
	assert.Equal(t, ReflectMagical, ParseReflectChannel("magical"))
	assert.Equal(t, ReflectPhysical, ParseReflectChannel("physical"))

	// Default-to-physical keeps existing content working with no data change,
	// and a typo degrades rather than panicking a boot.
	assert.Equal(t, ReflectPhysical, ParseReflectChannel(""))
	assert.Equal(t, ReflectPhysical, ParseReflectChannel("MAGICAL"))
	assert.Equal(t, ReflectPhysical, ParseReflectChannel("nonsense"))
}

// ── SplitReflectPercents ────────────────────────────────────────────────────

// The Elemental King: 25% species reflect on a magical-channel species, no
// thorns gear. All of it must route to magical.
func TestSplitReflectPercents_ElementalGoesMagical(t *testing.T) {
	phys, mag := SplitReflectPercents(0, 0, 25, ReflectMagical)
	assert.Equal(t, 0, phys)
	assert.Equal(t, 25, mag)
}

// A thornguard-enchanted defender of an ordinary species: all physical.
func TestSplitReflectPercents_ThornsGoPhysical(t *testing.T) {
	phys, mag := SplitReflectPercents(15, 0, 0, ReflectPhysical)
	assert.Equal(t, 15, phys)
	assert.Equal(t, 0, mag)
}

// Ironhide Reflect Skin is a mutation, and mutations are physical regardless of
// what the species declares — an elemental Ironhide splits across BOTH channels.
func TestSplitReflectPercents_MutationStaysPhysicalEvenOnAnElemental(t *testing.T) {
	phys, mag := SplitReflectPercents(0, 10, 25, ReflectMagical)
	assert.Equal(t, 10, phys, "mutation reflect is thorns, not elemental")
	assert.Equal(t, 25, mag, "species reflect follows the species channel")
}

// A species with no declared channel defaults to physical, so existing content
// behaves exactly as before this chunk.
func TestSplitReflectPercents_DefaultChannelIsPhysical(t *testing.T) {
	phys, mag := SplitReflectPercents(5, 10, 20, ParseReflectChannel(""))
	assert.Equal(t, 35, phys, "everything lands on physical by default")
	assert.Equal(t, 0, mag)
}
