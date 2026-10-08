package hooks

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A special move reveals its hidden mob target before any line is built, as
// handleCombatRound does for a melee swing (NewRound_DoCombat_unified.go).
// A see-hidden player kicked a hidden mob, the mob stayed hidden until the
// next round, and the kick's room line named it to every observer who reads
// faces (epic #382). Kick stands for the combat.ExecuteSkillMove seam (bash,
// drain, gore, hamstring, maul, pounce, rake, throttle, trip, fire); grapple
// for combat.ExecuteGrappleMove; taunt reveals in ExecuteTaunt.
func TestHiddenMob_SpecialMoveRevealsItsTarget(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	require.NoError(t, movenarration.LoadFrom(filepath.Join(filepath.Dir(thisFile),
		"..", "..", "_datafiles", "world", "dogmud", "narration", "special-moves")))

	cases := []struct {
		verb string
		run  func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error)
	}{
		{"kick", usercommands.Kick},
		{"grapple", usercommands.Grapple},
		{"taunt", usercommands.Taunt},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			actor, m, room := seeHiddenCasterOnHiddenSkeleton(t)
			// A kick needs legs and a grapple hands (the anatomy gates).
			const leggedSpeciesId = 7201
			t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
				leggedSpeciesId: {SpeciesId: leggedSpeciesId, Name: "Human", UnarmedName: "fist",
					BodyParts: []string{"head", "hands", "arms", "legs"}},
			}))
			savedSpecies := actor.Character.SpeciesId
			t.Cleanup(func() { actor.Character.SpeciesId = savedSpecies })
			actor.Character.SpeciesId = leggedSpeciesId
			// A clean, non-crit, non-fumble hit: the 2.3% fumble rate cannot
			// abort the move before the line this test reads.
			t.Cleanup(combat.SetChannelAttackContestRunnerForTest(deterministicContestRunner(t, 0.5, 0.5, -0.5)))

			handled, err := tc.run("skeleton", actor, room, events.EventFlag(0))
			require.NoError(t, err)
			require.True(t, handled)

			mine := drainPlain(1)
			assert.NotZero(t, countContaining(mine, "Skeleton"),
				"the see-hidden actor reads the mob's name in its own line: %v", mine)
			got := drainPlain(2)
			t.Logf("observer read: %v", got)
			assert.Zero(t, countContaining(got, "hidden"), "no hidden adjective: %v", got)
			// The mob is revealed before the line is built, so an observer who
			// names it is naming a mob it can see, never a hidden one.
			assert.False(t, m.Character.IsHidden(), "the %s target is revealed", tc.verb)
			assert.True(t, users.GetByUserId(2).Character.Perceives(&m.Character),
				"the observer perceives the mob the line names")
		})
	}
}
