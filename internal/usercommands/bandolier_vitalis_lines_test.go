package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #794: the player is told plainly when the Vitalis Bandolier refuses a
// potion they sort into it, and when a potion rots on leaving it.

const (
	vlVitalisId = 39331
	vlSalveId   = 39332
	vlFlaskId   = 39333
	vlPlainId   = 39334
	vlTonicId   = 39335
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

	// Salvaging a held potion takes it out: it sours, and the player is told.
	user.Character.Items = nil
	user.Character.PotionItems = nil
	user.Character.Activity = activity.NewMachine()
	require.True(t, user.Character.StoreItem(items.New(vlSalveId)))
	aimTold(1)
	_, err = userCommands["salvage"].Func("test salve", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, aimTold(1), "Test Salve sours the moment it leaves", "salvage names the rot")
}

// A topic hand-in from the Vitalis tells the player the potion soured.
func TestVitalisHandInTellsThePotionSoured(t *testing.T) {
	user, _, done := fixtureWith(t, potionHandInYAML)
	defer done()
	// The shared seed's 40182 is a plain bandolier; make it preserve here.
	spec := items.GetItemSpec(40182)
	require.NotNil(t, spec)
	preserves, ambient := spec.PreservesContents, spec.AmbientPotions
	spec.PreservesContents, spec.AmbientPotions = true, true
	t.Cleanup(func() { spec.PreservesContents, spec.AmbientPotions = preserves, ambient })
	wearBandolier(t, user)
	// The shared seed's potion has no held effect, so the Vitalis would not
	// take it by StoreItem; slot it as held directly.
	held := items.New(30001)
	held.Preserved = true
	user.Character.PotionItems = []items.Item{held}

	_, err := Give("healing potion skeleton", user, mustRoom(t, user), 0)
	require.NoError(t, err)
	require.Equal(t, 0, countItemsById(user.Character.PotionItems, 30001), "the hand-in took it")
	assert.Contains(t, drained(1), "sours the moment it leaves")
}

// Swapping onto the Vitalis from a roomier bandolier lets go of the
// overflow, and the player reads that it is still sound.
func TestVitalisEquipNamesWhatItLetsGo(t *testing.T) {
	user, room := aimScene(t, aimFull)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		vlVitalisId: {ItemId: vlVitalisId, Name: "Vitalis Bandolier", Type: items.Belt, Subtype: items.Wearable,
			IsBandolier: true, BandolierCapacity: 1, AmbientPotions: true, PreservesContents: true},
		vlPlainId: {ItemId: vlPlainId, Name: "plain bandolier", Type: items.Belt, Subtype: items.Wearable,
			IsBandolier: true, BandolierCapacity: 12},
		vlSalveId: {ItemId: vlSalveId, Name: "test salve", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{54}, Uses: 1},
		vlTonicId: {ItemId: vlTonicId, Name: "test tonic", Type: items.Potion, Subtype: items.Drinkable,
			ConditionIds: []int{55}, Uses: 1},
	}))
	c := user.Character
	c.Equipment.Belt = items.New(vlPlainId)
	c.Items = []items.Item{items.New(vlVitalisId)}
	c.PotionItems = []items.Item{items.New(vlSalveId), items.New(vlTonicId)}
	aimTold(1)

	_, err := userCommands["equip"].Func("vitalis bandolier", user, room, 0)
	require.NoError(t, err)
	told := aimTold(1)
	assert.Contains(t, told, "lets go of the Test Tonic: it has no room left")
	assert.Contains(t, told, "still sound")
	require.Len(t, c.PotionItems, 1)
}
