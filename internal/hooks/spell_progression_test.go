package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// OnSkillUseScaled already rolls the skill's primary stat, and manifestation's
// primary stat IS charisma. The explicit OnStatUse beside it made every cast
// take two charisma rolls instead of one, on both the player and mob branches.
func TestSpellCast_TracksItsStatOnce(t *testing.T) {
	caster := newCasterForTest(t)
	before := caster.GetStatUseCount("charisma")

	castManifestationSpellForTest(t, caster)

	if got := caster.GetStatUseCount("charisma") - before; got != 1 {
		t.Errorf("charisma tracked %d times for one cast, want 1", got)
	}
}

// ────────────────────────────────────────────────────────────────────────
// Test-only fixtures and drivers specific to this file.
// ────────────────────────────────────────────────────────────────────────

// newCasterForTest builds user 1 (from seedAllRegistries) into a ready caster
// and returns its Character. Reuses the shared PvM-quadrant fixture registry
// rather than a bespoke one -- user 1 already has a valid RoomId (room 1
// exists, per hooks_test.go), which handlePlayerFoldCasting needs to resolve
// the room on cast completion.
func newCasterForTest(t *testing.T) *characters.Character {
	t.Helper()

	cleanupRegistries := seedAllRegistries()
	t.Cleanup(cleanupRegistries)

	u := users.GetByUserId(1)
	require.NotNil(t, u)
	return u.Character
}

// castManifestationSpellForTest drives caster through exactly one complete
// cast of a manifestation-school spell via handlePlayerFoldCasting -- the
// same entry point NewRound_DoCombat.go calls in production.
//
// The seeded spell uses SpellType "" (Neutral) rather than HelpSingle,
// HarmArea, or HarmMulti, so neither the self-cast penalty nor the
// no-targets-hit AoE guard in handlePlayerFoldCasting zeroes spellBonus --
// this test wants the ordinary spellBonus>0 path, not either guard's edge
// case. FoldsNeeded=1 with FoldsPerRound=1 means simulateFoldRound's first
// (and only) iteration reaches FoldsNeeded immediately, so the cast
// completes in a single round -- no need to drive multiple rounds or wire up
// a target.
func castManifestationSpellForTest(t *testing.T, caster *characters.Character) {
	t.Helper()

	cleanupSpells := spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"test-manifest-bolt": {
			SpellId:   "test-manifest-bolt",
			Name:      "Test Manifest Bolt",
			Schools:   []string{spells.SchoolManifestation},
			BaseFolds: 1,
		},
	})
	t.Cleanup(cleanupSpells)

	caster.Activity = activity.NewMachine()
	require.NoError(t, caster.Activity.TransitionToCasting(
		activity.CastingData{
			SpellId:       "test-manifest-bolt",
			FoldsNeeded:   1,
			FoldsPerRound: 1,
		},
		state.TransitionReason{Trigger: activity.TriggerCastBegin},
	))

	u := users.GetByUserId(1)
	require.NotNil(t, u)

	result := handlePlayerFoldCasting(u, u.UserId)
	require.True(t, result, "a casting player must be handled by handlePlayerFoldCasting")
	require.True(t, u.Character.Activity == nil || u.Character.Activity.IsFree(),
		"the one-fold cast must complete within a single round")
}
