package mobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Weights are RELATIVE: 3*0.25 + 3*0.15 = 1.2, which is not a distribution.
// Normalisation is what lets an author write the ratio they mean.
func TestArchetypeStatShares_Normalise(t *testing.T) {
	primary, secondary := archetypeStatShares(0.25, 0.15)

	assert.InDelta(t, 0.2083, primary, 0.0001)
	assert.InDelta(t, 0.1250, secondary, 0.0001)
	assert.InDelta(t, 1.0, 3*primary+3*secondary, 1e-9,
		"the six shares must sum to exactly 1")
}

// The old shipped behaviour must be reproducible, so the change can be switched
// off by configuration alone.
func TestArchetypeStatShares_ReproduceOldSplit(t *testing.T) {
	primary, secondary := archetypeStatShares(0.80/3, 0.20/3)

	assert.InDelta(t, 0.80/3, primary, 1e-9)
	assert.InDelta(t, 0.20/3, secondary, 1e-9)
}

// Equal weights must reproduce the uniform ("") archetype exactly, so the knob
// spans the full range from specialisation to no archetype at all.
func TestArchetypeStatShares_EqualIsUniform(t *testing.T) {
	primary, secondary := archetypeStatShares(0.2, 0.2)

	assert.InDelta(t, 1.0/6, primary, 1e-9)
	assert.InDelta(t, 1.0/6, secondary, 1e-9)
}

func TestArchetypeStatShares_ZeroIsUniform(t *testing.T) {
	primary, secondary := archetypeStatShares(0, 0)
	assert.InDelta(t, 1.0/6, primary, 1e-9)
	assert.InDelta(t, 1.0/6, secondary, 1e-9)
}

// The tests above prove the ARITHMETIC. This proves the LOOP: that a spawned
// mob's stats actually land in the intended proportions.
func TestArchetypeDistribution_CastingMatchesIntendedShares(t *testing.T) {
	const pool = 60000 // large enough that sampling noise is well under a percent

	primaryShare, secondaryShare := archetypeStatShares(0.25, 0.15)

	m := &Mob{}
	m.Archetype = "casting"
	distributeStatPool(m, pool, primaryShare)

	s := m.Character.Stats
	physical := s.Strength.Base + s.Dexterity.Base + s.Vitality.Base
	mental := s.Perception.Base + s.Willpower.Base + s.Charisma.Base
	require.Equal(t, pool, physical+mental, "every point must land somewhere")

	// A casting mob's PHYSICAL stats are the NON-primary group.
	assert.InDelta(t, 3*secondaryShare, float64(physical)/float64(pool), 0.02,
		"casting mob physical share")
	assert.InDelta(t, 3*primaryShare, float64(mental)/float64(pool), 0.02,
		"casting mob mental share")
}

// The mirror: a fighting mob's physical stats are the PRIMARY group.
func TestArchetypeDistribution_FightingMirrorsCasting(t *testing.T) {
	const pool = 60000
	primaryShare, _ := archetypeStatShares(0.25, 0.15)

	m := &Mob{}
	m.Archetype = "fighting"
	distributeStatPool(m, pool, primaryShare)

	s := m.Character.Stats
	physical := s.Strength.Base + s.Dexterity.Base + s.Vitality.Base
	assert.InDelta(t, 3*primaryShare, float64(physical)/float64(pool), 0.02)
}

// The tank archetype keeps its own bespoke six-way split and must NOT respond
// to the primary/secondary weights.
func TestArchetypeDistribution_TankIgnoresWeights(t *testing.T) {
	const pool = 60000

	shares := func(primaryShare float64) float64 {
		m := &Mob{}
		m.Archetype = "tank"
		distributeStatPool(m, pool, primaryShare)
		s := m.Character.Stats
		return float64(s.Charisma.Base) / float64(pool)
	}

	// Charisma is 25% of a tank's pool by its own hardcoded table, whatever the
	// primary/secondary weights say.
	assert.InDelta(t, 0.25, shares(0.2083), 0.02)
	assert.InDelta(t, 0.25, shares(0.80/3), 0.02)
}
