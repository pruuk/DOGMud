package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/require"
)

// #480: Core Drain carries min_cast_rounds: 2 like Core Discharge. The
// Guardian's Core Charge is fed only when the drain RESOLVES
// (resolveMobDrainArea), so across the longer cast the charge must stay put
// on the held round, rise by one when the drain lands, and stay put when a
// disruptor cancels the drain mid-windup.

func coreDrainWindupBoss(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedDrainAreaRegistries(t, []int{7101, 7102}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-core-drain": {
		SpellId: "test-core-drain", Name: "Test Core Drain", MobOnly: true,
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetArea,
		EffectType: "drain_area", BaseFolds: 2, MinCastRounds: 2,
		WaitRoomText: "{actor}'s filaments pull tight.",
	}}))
	boss := mobs.GetInstance(9562)
	require.NotNil(t, boss)
	boss.Character.ConvictionMax.Value, boss.Character.Conviction = 1000, 1000
	boss.Character.Activity = activity.NewMachine()
	require.NoError(t, boss.Character.Activity.TransitionToCasting(
		activity.CastingData{SpellId: "test-core-drain", FoldsNeeded: 2, FoldsPerRound: 57, TotalConvictionCost: 20},
		state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	return boss, rooms.LoadRoom(1)
}

func coreCharge(m *mobs.Mob) int {
	return behaviortree.EnsureBTreeState(m).GetInt("core_charge")
}

func TestCoreDrainWindup_ChargeRisesOnlyWhenTheDrainLands(t *testing.T) {
	boss, room := coreDrainWindupBoss(t)
	require.Zero(t, coreCharge(boss))

	handleMobFoldCasting(boss, room)
	require.True(t, boss.Character.IsCasting(), "round 1 of the drain's windup is held")
	require.Zero(t, coreCharge(boss), "a held round feeds no charge")

	handleMobFoldCasting(boss, room)
	require.False(t, boss.Character.IsCasting(), "round 2 resolves the drain")
	require.Equal(t, 1, coreCharge(boss), "the landed drain feeds one charge")
}

func TestCoreDrainWindup_InterruptedDrainFeedsNoCharge(t *testing.T) {
	boss, room := coreDrainWindupBoss(t)

	handleMobFoldCasting(boss, room)
	require.True(t, boss.Character.IsCasting())
	require.True(t, maybeInterruptSpellOnTarget(&boss.Character, "neural-stun", state.ActorRef{UserId: 7101}),
		"a disruptor in round 2 cancels the drain")

	require.False(t, handleMobFoldCasting(boss, room))
	require.Zero(t, coreCharge(boss), "an interrupted drain feeds no charge")
}
