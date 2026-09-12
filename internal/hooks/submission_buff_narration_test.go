package hooks

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Buffs 83 Broken Limb and 84 Stunned are applied by the submission outcome
// inside internal/combat, which sends no player text at all, so their authored
// start_user_text reached nobody: a player whose arm was just snapped read the
// submission's outcome line and nothing about the break. Both buffs are now
// flagged silent-start and this hook owes the victim the start line, right
// after the outcome. These lanes are that debt.
//
// The expected text is READ FROM THE SHIPPED YAML, not retyped here, so the
// assertion is about the authored line arriving rather than about a literal
// this test invented.

const (
	brokenLimbBuffFile = "../../_datafiles/world/dogmud/buffs/83-broken_limb.yaml"
	stunnedBuffFile    = "../../_datafiles/world/dogmud/buffs/84-stunned.yaml"
)

// loadAuthoredBuffSpec reads one shipped buff file into a spec.
func loadAuthoredBuffSpec(t *testing.T, path string, wantId int) *buffs.BuffSpec {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "the shipped buff file must be readable from internal/hooks")
	var spec buffs.BuffSpec
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	require.Equal(t, wantId, spec.BuffId, "%s must be the buff this narration covers", path)
	require.NotEmpty(t, spec.StartUserText, "%s must carry start_user_text", path)
	require.Contains(t, spec.Flags, buffs.SilentStart,
		"%s must be silent-start, or the event path would narrate it twice", path)
	return &spec
}

// seedAuthoredSubmissionBuffs installs the two shipped specs and returns both
// the restore func and their authored lines, tag-stripped for comparison.
func seedAuthoredSubmissionBuffs(t *testing.T) (restore func(), brokenLine, stunnedLine string) {
	t.Helper()
	broken := loadAuthoredBuffSpec(t, brokenLimbBuffFile, combat.BrokenLimbBuffId)
	stunned := loadAuthoredBuffSpec(t, stunnedBuffFile, combat.StunnedBuffId)
	restore = buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		broken.BuffId:  broken,
		stunned.BuffId: stunned,
	})
	return restore, plainText(broken.StartUserText), plainText(stunned.StartUserText)
}

func TestSubmissionStunIsNarratedToAPlayerVictim(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore, _, stunnedLine := seedAuthoredSubmissionBuffs(t)
	defer restore()
	drainPlain(1)

	victim := users.GetByUserId(1).Character
	require.NotNil(t, victim)

	narrateSubmissionEffects(combat.SubmissionOutcomeEffects{StunnedVictim: victim})

	require.Contains(t, stunnedLine,
		"The submission's pressure leaves you stunned, unable to react.",
		"the authored stun line changed; update this lane with the copy")
	assert.Equal(t, 1, countContaining(drainPlain(1), stunnedLine),
		"a stunned player must be told the submission stunned them, exactly once")
}

func TestSubmissionBrokenLimbIsNarratedToAPlayerVictim(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore, brokenLine, _ := seedAuthoredSubmissionBuffs(t)
	defer restore()
	drainPlain(1)

	victim := users.GetByUserId(1).Character
	require.NotNil(t, victim)

	narrateSubmissionEffects(combat.SubmissionOutcomeEffects{
		BrokenLimbVictim: victim,
		BrokenBodyPart:   "arm",
	})

	require.Contains(t, brokenLine, "You feel the wrench of a broken limb.",
		"the authored broken-limb line changed; update this lane with the copy")
	assert.Equal(t, 1, countContaining(drainPlain(1), brokenLine),
		"a player whose limb was broken must be told, exactly once")
}

// Both effects can land in the same round only in theory (crit + mercy stuns,
// cripple breaks, and they are different policies), but the narrator must not
// assume that: each non-nil victim gets its own line and neither leaks to the
// other user in the room.
func TestSubmissionEffectsNarrateOnlyToTheVictim(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore, brokenLine, stunnedLine := seedAuthoredSubmissionBuffs(t)
	defer restore()
	drainPlain(1)
	drainPlain(2)

	victim := users.GetByUserId(1).Character
	narrateSubmissionEffects(combat.SubmissionOutcomeEffects{
		StunnedVictim:    victim,
		BrokenLimbVictim: victim,
		BrokenBodyPart:   "arm",
	})

	got := drainPlain(1)
	assert.Equal(t, 1, countContaining(got, stunnedLine))
	assert.Equal(t, 1, countContaining(got, brokenLine))

	bystander := drainPlain(2)
	assert.Equal(t, 0, countContaining(bystander, stunnedLine),
		"a buff's start_user_text is the holder's line, not the room's")
	assert.Equal(t, 0, countContaining(bystander, brokenLine))
}

func TestSubmissionEffectsSayNothingToAMobVictim(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore, brokenLine, stunnedLine := seedAuthoredSubmissionBuffs(t)
	defer restore()
	drainPlain(1)
	drainPlain(2)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)

	narrateSubmissionEffects(combat.SubmissionOutcomeEffects{
		StunnedVictim:    &mob.Character,
		BrokenLimbVictim: &mob.Character,
		BrokenBodyPart:   "arm",
	})

	for _, userId := range []int{1, 2} {
		got := drainPlain(userId)
		assert.Equal(t, 0, countContaining(got, stunnedLine),
			"a mob victim has no client; nobody reads its start line")
		assert.Equal(t, 0, countContaining(got, brokenLine))
	}
}

func TestSubmissionEffectsWithNothingAppliedSendNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore, _, _ := seedAuthoredSubmissionBuffs(t)
	defer restore()
	drainPlain(1)

	narrateSubmissionEffects(combat.SubmissionOutcomeEffects{})

	assert.Empty(t, drainPlain(1), "an empty report narrates nothing")
}
