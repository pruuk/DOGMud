package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/stretchr/testify/require"
)

// The equip quest event names the item put on (2026-10-17 newbie-duo
// playtest): `wield cudgel` finished First Shot's "ready the sling" step,
// because the event carried no item and every equip counted. A trigger with
// an `item:` filter now hears only its own item.
func TestEquip_QuestCommandEventNamesTheItem(t *testing.T) {
	user, room := armUser(t)
	const sling, cudgel = 96101, 96102
	require.True(t, user.Character.StoreItem(armItem(cudgel, "cudgel", items.Weapon, 1, false)))
	require.True(t, user.Character.StoreItem(armItem(sling, "sling", items.Weapon, 1, false)))

	restore := questengine.ResetEngineForTest()
	t.Cleanup(restore)
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999301,
		Name:    "equip item probe",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:   "command",
			Command: "equip",
			Item:    sling,
			Actions: []questengine.ActionDef{{Grant: "999301-start"}},
		}},
	})
	t.Cleanup(func() { user.Character.ClearQuestToken("999301-start") })

	armOut(t, user, room, "cudgel")
	require.Equal(t, cudgel, user.Character.Equipment.Weapon.ItemId, "the cudgel must be wielded for the probe to mean anything")
	require.False(t, user.Character.HasQuest("999301-start"), "wielding the cudgel must not satisfy a sling trigger")

	user.Character.Equipment.Weapon = items.Item{} // free the hand
	armOut(t, user, room, "sling")
	require.Equal(t, sling, user.Character.Equipment.Weapon.ItemId)
	require.True(t, user.Character.HasQuest("999301-start"), "wielding the sling must satisfy the sling trigger")
}
