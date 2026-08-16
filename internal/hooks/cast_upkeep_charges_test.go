package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This is the fact that makes "a refused summon must never enter the Casting
// state" the correct proof that a refused summon costs nothing.
//
// Conviction for a spell is not taken at the moment the player types `cast`.
// It is taken here, one slice per round, for as long as the cast channels. The
// U7b reservation gates in companion_summon.go and charm_spell.go sit at
// RESOLUTION, so before the initiation gate in usercommands/skill.cast.go a
// doomed summon channelled to the end, paid in full round by round, and only
// then refused. An adversarial playtest measured a whole pool's worth of
// conviction gone on refused elementals, three runs in a row.
//
// Pair this with TestCast_RefusedSummon_ChargesNoConviction in
// internal/usercommands: that test asserts the Casting state is never entered,
// and this one asserts that entering it is what spends the pool.
func TestProcessFoldRound_ChargesConvictionEveryRound(t *testing.T) {
	cleanup := spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"upkeep-probe": {SpellId: "upkeep-probe", BaseFolds: 4, Cost: 40},
	})
	defer cleanup()

	ch := &characters.Character{}
	ch.Activity = activity.NewMachine()
	require.NoError(t, ch.Activity.TransitionToCasting(
		activity.CastingData{
			SpellId:             "upkeep-probe",
			FoldsNeeded:         4,
			FoldsPerRound:       1,
			TotalConvictionCost: 40,
		},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	))
	ch.ConvictionMax.Value = 100
	ch.Conviction = 100

	result := processFoldRound(ch)

	assert.Greater(t, result.ConvictionCost, 0,
		"a channelling round must charge conviction, or the initiation gate is guarding nothing")
	assert.Less(t, ch.Conviction, 100,
		"the charge must actually leave the caster's pool")
}

// The mirror image: a character NOT in the Casting state is charged nothing,
// whatever else is true of it. This is exactly the state the initiation gate
// leaves a refused summoner in.
func TestProcessFoldRound_NotCasting_ChargesNothing(t *testing.T) {
	ch := &characters.Character{}
	ch.Activity = activity.NewMachine()
	ch.ConvictionMax.Value = 100
	ch.Conviction = 100

	result := processFoldRound(ch)

	assert.Equal(t, 0, result.ConvictionCost)
	assert.Equal(t, 100, ch.Conviction,
		"a caster that never began channelling must keep every point")
}
