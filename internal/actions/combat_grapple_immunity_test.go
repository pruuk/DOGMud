package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ⚠️ One message used to serve all four grapple refusals:
// "You reach for X but your hands pass right through!"
//
// That is correct for something INCORPOREAL and actively false for everything
// else. The Arena Champion (324-arena_champion.yaml) ships
// spawnmutations [colossus-form, dense-muscles], and colossus-form grants the
// control-immune flag -- so it is refused for being IMMOVABLE, the opposite of
// intangible, and the player was told their hands passed through a creature too
// solid to shift.
//
// Worse, two of the four refusals are about the ACTOR and carry no Target at
// all, so the shared message rendered an EMPTY name. Nobody hit that because
// both need an unusual body, which is exactly why it needs a test rather than
// a playtest.
func grappleImmunityFixture(t *testing.T, selfSpecies, targetSpecies int, targetMutations map[string]int, selfParts, targetParts []string) GrappleResult {
	t.Helper()

	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		selfSpecies:   {SpeciesId: selfSpecies, Name: "self", BodyParts: selfParts},
		targetSpecies: {SpeciesId: targetSpecies, Name: "target", BodyParts: targetParts},
	})
	t.Cleanup(cleanup)

	// ⚠️ IsControlImmune resolves the flag through the loaded mutation
	// SPECS, not the owned map, so a test binary sees no flag at all unless the
	// registry is seeded. Mirrors the real colossus-form.yaml, which carries
	// `type: flag, target: control-immune` under pros.
	mutCleanup := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"colossus-form": {
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "control-immune"}},
		},
	})
	t.Cleanup(mutCleanup)

	targetMob := &mobs.Mob{InstanceId: 9701}
	targetMob.Character.Name = "Arena Champion"
	targetMob.Character.SpeciesId = targetSpecies
	targetMob.Character.Mutations = targetMutations
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	t.Cleanup(func() { mobs.SetInstanceForTest(targetMob.InstanceId, nil) })

	m := newTestMob(t, func(m *mobs.Mob) { m.Character.SpeciesId = selfSpecies })
	m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)

	return ExecuteGrapple(&MobActor{Mob: m, Room: nil})
}

// The reported case: colossus-form makes the target immovable, not intangible.
func TestExecuteGrapple_ControlImmuneTargetIsImmovableNotIncorporeal(t *testing.T) {
	res := grappleImmunityFixture(t, 1, 2,
		map[string]int{"colossus-form": 1}, []string{"arms"}, []string{"arms"})

	require.True(t, res.GrappleImmune)
	assert.Equal(t, GrappleImmuneTargetImmovable, res.ImmuneReason,
		"a control-immune target must NOT be reported as incorporeal")
	assert.Equal(t, "Arena Champion", res.Target.Name,
		"a target-side refusal must carry the Target so the message can name it")
}

// ⚠️ THE SILENT BUG. Both actor-side refusals carry no Target, so any caller
// printing res.Target.Name unconditionally renders an empty name.
func TestExecuteGrapple_ActorSideRefusalsCarryNoTarget(t *testing.T) {
	t.Run("no arms", func(t *testing.T) {
		res := grappleImmunityFixture(t, 1, 2, nil, []string{"legs"}, []string{"arms"})
		require.True(t, res.GrappleImmune)
		assert.Equal(t, GrappleImmuneSelfNoArms, res.ImmuneReason)
		assert.Empty(t, res.Target.Name,
			"actor-side refusal has no target; the message must not try to name one")
	})

	t.Run("own species cannot grapple", func(t *testing.T) {
		cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
			1: {SpeciesId: 1, Name: "ethereal", BodyParts: []string{"arms"}, GrappleImmune: true},
			2: {SpeciesId: 2, Name: "target", BodyParts: []string{"arms"}},
		})
		defer cleanup()

		targetMob := &mobs.Mob{InstanceId: 9702}
		targetMob.Character.Name = "Arena Champion"
		targetMob.Character.SpeciesId = 2
		mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
		defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

		m := newTestMob(t, func(m *mobs.Mob) { m.Character.SpeciesId = 1 })
		m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)

		res := ExecuteGrapple(&MobActor{Mob: m, Room: nil})
		require.True(t, res.GrappleImmune)
		assert.Equal(t, GrappleImmuneSelfSpecies, res.ImmuneReason)
		assert.Empty(t, res.Target.Name,
			"actor-side refusal has no target; the message must not try to name one")
	})
}

// The case the original message was actually written for.
func TestExecuteGrapple_IncorporealTargetKeepsItsReason(t *testing.T) {
	cleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "self", BodyParts: []string{"arms"}},
		3: {SpeciesId: 3, Name: "wraith", BodyParts: []string{"arms"}, GrappleImmune: true},
	})
	defer cleanup()

	targetMob := &mobs.Mob{InstanceId: 9703}
	targetMob.Character.Name = "Wraith"
	targetMob.Character.SpeciesId = 3
	mobs.SetInstanceForTest(targetMob.InstanceId, targetMob)
	defer mobs.SetInstanceForTest(targetMob.InstanceId, nil)

	m := newTestMob(t, func(m *mobs.Mob) { m.Character.SpeciesId = 1 })
	m.Character.SetAggro(0, targetMob.InstanceId, characters.DefaultAttack)

	res := ExecuteGrapple(&MobActor{Mob: m, Room: nil})
	require.True(t, res.GrappleImmune)
	assert.Equal(t, GrappleImmuneTargetIncorporeal, res.ImmuneReason)
	assert.Equal(t, "Wraith", res.Target.Name)
}
