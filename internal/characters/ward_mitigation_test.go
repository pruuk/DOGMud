package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	wardTestPhysical = 9331 // mitigation_flat only
	wardTestSpell    = 9332 // mitigation_flat and mitigation_magical
	wardTestAll      = 9333 // all three kinds
	wardTestPotion   = 9334 // a magical_mitigation statmod, no family
)

func seedWardMitigationSpecs(t *testing.T) {
	t.Helper()
	mag := conditions.EffectValue{UsesMagnitude: true}
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		wardTestPhysical: {ConditionId: wardTestPhysical, Name: "Physical Ward", Family: conditions.FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectMitigationFlat: mag}},
		wardTestSpell: {ConditionId: wardTestSpell, Name: "Spell Ward", Family: conditions.FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectMitigationFlat: mag, conditions.EffectMitigationMagical: mag}},
		wardTestAll: {ConditionId: wardTestAll, Name: "All Ward", Family: conditions.FamilyWard, TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectMitigationFlat: mag, conditions.EffectMitigationMagical: mag, conditions.EffectMitigationConviction: mag}},
		wardTestPotion: {ConditionId: wardTestPotion, Name: "Mind Potion", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 10,
			StatMods: statmods.StatMods{"magical_mitigation": 15}},
	})
	t.Cleanup(restore)
}

type mitigations struct{ physical, magical, conviction float64 }

func mitigationOf(c *Character) mitigations {
	return mitigations{c.GetPhysicalMitigation(), c.GetMagicalMitigation(), c.GetConvictionMitigation()}
}

// Each ward adds its magnitude, in points, to exactly the channels it lists
// and to no other (spec section 4; damage types physical, mental and social
// meet these three channels, combat.MitigationChannelFor).
func TestWardEffects_FeedOnlyTheirOwnChannels(t *testing.T) {
	seedWardMitigationSpecs(t)
	for _, tc := range []struct {
		id   int
		want mitigations
	}{
		{wardTestPhysical, mitigations{0.20, 0, 0}},
		{wardTestSpell, mitigations{0.20, 0.20, 0}},
		{wardTestAll, mitigations{0.20, 0.20, 0.20}},
	} {
		c := pinCharacter()
		require.NoError(t, c.AddConditionMagnitude(tc.id, 10, 20, "test"))
		got := mitigationOf(c)
		assert.InDelta(t, tc.want.physical, got.physical, 1e-9, "physical, ward %d", tc.id)
		assert.InDelta(t, tc.want.magical, got.magical, 1e-9, "magical, ward %d", tc.id)
		assert.InDelta(t, tc.want.conviction, got.conviction, 1e-9, "conviction, ward %d", tc.id)
	}
}

// R4: a potion's mitigation still sums on top of a ward of the same kind.
func TestWardEffects_APotionStillSumsWithAWard(t *testing.T) {
	seedWardMitigationSpecs(t)
	c := pinCharacter()
	require.NoError(t, c.AddConditionMagnitude(wardTestSpell, 10, 20, "test"))
	require.NoError(t, c.AddCondition(wardTestPotion, false))
	assert.InDelta(t, 0.35, c.GetMagicalMitigation(), 1e-9)
}
