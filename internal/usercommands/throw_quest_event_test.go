package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A throw raises one `command` quest event per mob its blast reached, under
// one evaluation guard (#375 phase 3b slice 2). These drive the real Throw
// path through the seam fixture of throw_seam_test.go with a probe quest.

const (
	throwEventProbeToken = "97-start" // a fresh character may only start a quest
	throwEventProbeQuest = 97
	throwEventMobId      = 1 // seedThrowSeamFixture's mob template id
)

// throwEventProbe registers an engine whose only trigger is the given one,
// made a `command: throw` trigger that grants the probe token.
func throwEventProbe(t *testing.T, trig quests.TriggerDef) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	trig.Event = "command"
	trig.Command = "throw"
	trig.Actions = []quests.ActionDef{{Grant: throwEventProbeToken}}
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId:  throwEventProbeQuest,
		Name:     "Throw Event Probe",
		Triggers: []quests.TriggerDef{trig},
	})
}

func TestThrowEvent_MobTriggerFiresForAMobTheBlastReached(t *testing.T) {
	user, room, _ := seedThrowSeamFixture(t, 3, 0, 0)
	throwEventProbe(t, quests.TriggerDef{Target: quests.TargetMob, Mob: throwEventMobId})
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		return throwSeamResult(atkScore, entries, 0.5, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.True(t, user.Character.HasQuest(throwEventProbeToken),
		"a throw that reached a mob of template 1 must raise a mob-target command event")
}

func TestThrowEvent_OtherMobTemplateDoesNotFire(t *testing.T) {
	user, room, _ := seedThrowSeamFixture(t, 2, 0, 0)
	throwEventProbe(t, quests.TriggerDef{Target: quests.TargetMob, Mob: throwEventMobId + 500})
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		return throwSeamResult(atkScore, entries, 0.5, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.False(t, user.Character.HasQuest(throwEventProbeToken),
		"a trigger keyed to another mob template must not fire")
}

func TestThrowEvent_UnfilteredTriggerFiresOnceForSeveralMobs(t *testing.T) {
	user, room, _ := seedThrowSeamFixture(t, 3, 0, 0)
	throwEventProbe(t, quests.TriggerDef{})
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		return throwSeamResult(atkScore, entries, 0.5, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.True(t, user.Character.HasQuest(throwEventProbeToken),
		"a command: throw trigger with no target filter still fires")
}

func TestThrowEvent_NoMobInTheRoomRaisesOneEventAimedAtNothing(t *testing.T) {
	user, room, _ := seedThrowSeamFixture(t, 0, 0, 0)
	throwEventProbe(t, quests.TriggerDef{Target: quests.TargetNone})
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		return throwSeamResult(atkScore, entries, 0.5, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.True(t, user.Character.HasQuest(throwEventProbeToken),
		"a throw that reached no one is one command event aimed at nothing")
}
