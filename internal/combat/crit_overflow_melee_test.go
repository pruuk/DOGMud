package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #465 on the melee path: buildDamageParams carries the same crit reduction
// the shared resolver applies (crit_overflow_test.go), and a melee crit keeps
// the same share of its unmitigated mean as a spell-path crit.

const critOverflowConditionId = 9465

// critOverflowDefender is a parity combatant carrying rawPoints of physical
// mitigation as a condition statmod (melee_parity_stat_test.go's injection).
func critOverflowDefender(t *testing.T, rawPoints int) *characters.Character {
	t.Helper()
	d := parityCombatant(t, "overflow defender")
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		critOverflowConditionId: {
			ConditionId:  critOverflowConditionId,
			Name:         "overflow mitigation",
			TriggerCount: 1,
			StatMods:     statmods.StatMods{"physical_mitigation": rawPoints},
		},
	}))
	d.Conditions.List = append(d.Conditions.List, &conditions.Condition{
		ConditionId: critOverflowConditionId, TriggersLeft: 1000000000,
	})
	d.Conditions.Validate(true)
	require.InDelta(t, float64(rawPoints)/100.0, d.GetPhysicalMitigation(), 1e-9)
	return d
}

// The melee path carries the same reduction the shared resolver applies:
// none below the knee, the overflow above it.
func TestBuildDamageParams_CritReductionIsTheOverflow(t *testing.T) {
	pinShippedCurve(t)
	attacker := parityCombatant(t, "overflow attacker")

	for _, tc := range []struct {
		points int
		want   float64
	}{
		{0, 0},
		{40, 0},
		{50, 0},
		{100, 0.25},
	} {
		defender := critOverflowDefender(t, tc.points)
		plan := buildAttackPlan(attacker, defender)
		require.NotEmpty(t, plan.weapons)
		sdp := buildDamageParams(attacker, defender, plan.weapons[0], 0, User)
		assert.InDelta(t, tc.want, sdp.critReduction, 1e-9, "raw %d points", tc.points)
		assert.InDelta(t, MitigationCurveFor(ChannelPhysical).CritReduction(float64(tc.points)/100.0),
			sdp.critReduction, 1e-12, "melee and the curve agree at raw %d points", tc.points)
	}
}

// Melee and the shared resolver agree on the crit itself: against the same
// raw mitigation, a melee crit and a spell-path crit keep the same share of
// their unmitigated crit mean.
func TestMeleeCritAndSpellCritAgreeAboveTheKnee(t *testing.T) {
	pinShippedCurve(t)
	const samples = 20000
	attacker := parityCombatant(t, "overflow attacker")
	defender := critOverflowDefender(t, 100)

	plan := buildAttackPlan(attacker, defender)
	require.NotEmpty(t, plan.weapons)
	sdp := buildDamageParams(attacker, defender, plan.weapons[0], 0, User)
	fullCrit := sdp.rawDmgForCrit * sdp.critDmgMult

	meleeTotal := 0.0
	for i := 0; i < samples; i++ {
		dmg, _ := calcHitDamage(&AttackResult{}, true, false, sdp)
		meleeTotal += float64(dmg)
	}
	meleeShare := meleeTotal / samples / fullCrit

	rank := attacker.GetCombatSkillLevel()
	spellBare := 0.0
	spellHeavy := 0.0
	curve := MitigationCurveFor(ChannelPhysical)
	for i := 0; i < samples; i++ {
		spellBare += float64(CritOrMitigatedDamage(sdp.rawDmgForCrit, rank, true, 0, curve))
		spellHeavy += float64(CritOrMitigatedDamage(sdp.rawDmgForCrit, rank, true, defender.GetPhysicalMitigation(), curve))
	}
	spellShare := spellHeavy / spellBare

	assert.InDelta(t, 0.75, meleeShare, 0.03, "melee crit keeps 1 - overflow")
	assert.InDelta(t, 0.75, spellShare, 0.03, "spell-path crit keeps 1 - overflow")
	assert.InDelta(t, meleeShare, spellShare, 0.03, "melee and spell agree")
}
