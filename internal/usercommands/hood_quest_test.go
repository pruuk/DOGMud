package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// #375 phase 7 (quest 98, First Light): closing a lantern's hood is a quest
// step, so hood raises a `command` event naming the lantern as its item.
// A probe quest in an isolated engine holds the only trigger.
const hoodProbeQuest = 99702

func registerHoodProbe(t *testing.T) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: hoodProbeQuest,
		Name:    "Hood Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: "hood",
			Item:    hoodTestLanternItem,
			Actions: []quests.ActionDef{{Grant: "99702-start"}},
		}},
	})
}

func TestHood_RaisesTheHoodQuestEvent(t *testing.T) {
	user, room := hoodFixture(t)
	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	registerHoodProbe(t)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)

	require.True(t, user.Character.HasQuest("99702-start"),
		"closing the hood must raise command hood with the lantern as its item")
}

func TestHood_RefusalRaisesNoQuestEvent(t *testing.T) {
	user, room := hoodFixture(t)
	registerHoodProbe(t)

	_, err := Hood("", user, room, 0) // no light worn: refused
	require.NoError(t, err)

	require.False(t, user.Character.HasQuest("99702-start"),
		"a refused hood must raise nothing")
}

// The empty-slot refusal above carries no item, so the probe's item filter
// would hide a stray event there. A second hood is refused with the lantern
// still worn, so this one can see it, and the event stays once per closing.
func TestHood_AlreadyHoodedRaisesNoSecondEvent(t *testing.T) {
	user, room := hoodFixture(t)
	_, ok, why := user.Character.Wear(items.New(hoodTestLanternItem))
	require.True(t, ok, why)
	registerHoodProbe(t)

	_, err := Hood("", user, room, 0)
	require.NoError(t, err)
	require.True(t, user.Character.HasQuest("99702-start"), "the first hood closes it")
	user.Character.ClearQuestToken("99702-start")

	_, err = Hood("", user, room, 0)
	require.NoError(t, err)

	require.False(t, user.Character.HasQuest("99702-start"),
		"a hood already closed must raise nothing")
}
