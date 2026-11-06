package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #794: the player is told plainly when the Vitalis Bandolier refuses a
// potion they sort into it, and when a potion rots on leaving it.

const (
	vlVitalisId = 39331
	vlSalveId   = 39332
	vlFlaskId   = 39333
)

func TestVitalisTellsThePlayerWhatItRefusesAndWhatRots(t *testing.T) {
	user, room := aimScene(t, aimFull)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		vlVitalisId: {ItemId: vlVitalisId, Name: "Vitalis Bandolier", Type: items.Belt, Subtype: items.Wearable,
			IsBandolier: true, BandolierCapacity: 4, AmbientPotions: true, PreservesContents: true},
		vlSalveId: {ItemId: vlSalveId, Name: "test salve", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{54}, Uses: 1},
		vlFlaskId: {ItemId: vlFlaskId, Name: "test flask", Type: items.Object, Subtype: items.Throwable,
			ConditionIds: []int{78}, Uses: 1},
	}))
	c := user.Character
	c.Equipment.Belt = items.New(vlVitalisId)
	c.PotionItems = nil
	c.Items = []items.Item{items.New(vlFlaskId), items.New(vlSalveId)}
	events.DrainQueuedMessagesForTest(1)

	_, err := userCommands["sort"].Func("", user, room, 0)
	require.NoError(t, err)
	told := aimTold(1)
	assert.Contains(t, told, "will not take the Test Flask", "sort names the refused throwable")
	require.Len(t, c.PotionItems, 1, "the salve slots")

	_, err = userCommands["get"].Func("test salve from bandolier", user, room, 0)
	require.NoError(t, err)
	told = aimTold(1)
	assert.Contains(t, told, "Test Salve sours the moment it leaves", "taking it out names the rot")
	assert.NotRegexp(t, `[0-9]`, bandolierRotLine("Vitalis Bandolier", items.New(vlSalveId)), "no raw numbers")
}
