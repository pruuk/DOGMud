package hooks

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An area drain sweeps every player in the room, hidden or not, but it does
// not reveal them: a single-target special move reveals its target as a
// combat round does (#382), an area move that never picked anyone out does
// not. A hidden player it clips takes the damage and stays hidden, and a
// room line about that player never names them.

// hideCharacter puts c into the Hidden awareness state the way sneak does.
func hideCharacter(t *testing.T, c *characters.Character) {
	t.Helper()
	reason := state.TransitionReason{Trigger: "hidden_player_area_drain_test"}
	require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden())
}

func coreRechargeSpellForTest() *spells.SpellData {
	return &spells.SpellData{SpellId: "test-core-recharge", Name: "Core Recharge", EffectType: "drain_area",
		AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetArea}
}

// A defended drain that still clips a hidden player (a partial pull) leaves
// them hidden. A clean hit pulls the player into the fight
// (targeting.Commit), and entering combat ends stealth through the awareness
// cascade, as it did before special moves revealed their targets; this test
// pins the sweep itself, which reveals no one.
func TestMobDrainArea_HiddenPlayerStaysHidden(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	// A narrow defensive win: defended, not a crit, partial damage.
	t.Cleanup(combat.SetChannelAttackContestRunnerForTest(deterministicContestRunner(t, -0.3, 0.2, 0.3)))
	f.room.Lamp = rooms.LampPtr(90)
	hideCharacter(t, f.targetUser.Character)
	healthBefore := f.targetUser.Character.Health

	resolveMobDrainArea(f.casterMob, f.room, coreRechargeSpellForTest())

	assert.Less(t, f.targetUser.Character.Health, healthBefore, "the drain still clips the hidden player")
	assert.True(t, f.targetUser.Character.IsHidden(), "an area drain does not reveal a hidden player")
}

func TestMobDrainArea_DefenceLineDoesNotNameAHiddenPlayer(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	// A decisive defensive win: zero damage, so every swept player's
	// defence triad is spoken, observer line included.
	t.Cleanup(combat.SetChannelAttackContestRunnerForTest(alwaysDefensiveCritContest(t)))
	f.room.Lamp = rooms.LampPtr(90)
	hideCharacter(t, f.targetUser.Character)
	require.False(t, f.watcher.Character.Perceives(f.targetUser.Character))
	hiddenName := f.targetUser.Character.Name
	visibleName := f.casterUser.Character.Name

	resolveMobDrainArea(f.casterMob, f.room, coreRechargeSpellForTest())

	got := drainPlain(f.watcher.UserId)
	t.Logf("watcher read: %v", got)
	assert.Zero(t, countContaining(got, hiddenName), "the watcher must not read the hidden player's name: %v", got)
	// Null probe: the visible player's own defence line still names them,
	// so the watcher does read the triad this test checks.
	assert.NotZero(t, countContaining(got, visibleName), "the visible player's defence is still named: %v", got)
}

// A kick refused before it resolves (here, on the special-move cooldown)
// never reaches combat.ExecuteSkillMove, so it reveals nothing.
func TestHiddenMob_RefusedKickLeavesItHidden(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	require.NoError(t, movenarration.LoadFrom(filepath.Join(filepath.Dir(thisFile),
		"..", "..", "_datafiles", "world", "dogmud", "narration", "special-moves")))
	actor, m, room := seeHiddenCasterOnHiddenSkeleton(t)
	const leggedSpeciesId = 7201
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		leggedSpeciesId: {SpeciesId: leggedSpeciesId, Name: "Human", UnarmedName: "fist",
			BodyParts: []string{"head", "hands", "arms", "legs"}},
	}))
	savedSpecies := actor.Character.SpeciesId
	t.Cleanup(func() { actor.Character.SpeciesId = savedSpecies })
	actor.Character.SpeciesId = leggedSpeciesId
	t.Cleanup(func() { actions.ReleaseSpecialMove(actor.Character) })
	require.True(t, actions.ClaimSpecialMove(actor.Character), "the cooldown is now running")

	handled, err := usercommands.Kick("skeleton", actor, room, events.EventFlag(0))
	require.NoError(t, err)
	require.True(t, handled)

	mine := drainPlain(1)
	assert.NotZero(t, countContaining(mine, "recover before attempting"), "the kick was refused: %v", mine)
	assert.True(t, m.Character.IsHidden(), "a refused kick does not reveal its target")
}
