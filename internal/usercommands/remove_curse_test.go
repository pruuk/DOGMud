package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hexed(id int, name string, t items.ItemType) items.Item {
	return items.Item{ItemId: id, Spec: &items.ItemSpec{ItemId: id, Name: name, Type: t, Subtype: items.Wearable, Cursed: true}}
}

func removeOut(t *testing.T, rest string) (string, func() *items.Item) {
	t.Helper()
	user, room := getTestUserAndRoom(t)
	events.DrainQueuedMessagesForTest(user.UserId)
	handled, err := Remove(rest, user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"), func() *items.Item { return &user.Character.Equipment.Ring }
}

// A cursed item refuses through Character.CursedRefusal, the line wear uses
// (M7 slice 3, #371).
func TestRemove_CursedLine(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, _ := getTestUserAndRoom(t)
	user.Character.Equipment.Ring = hexed(96301, "hexed ring", items.Ring)
	out, ring := removeOut(t, "hexed ring")
	assert.Contains(t, out, "is cursed and prevents you from removing it.")
	assert.NotContains(t, out, "CURSED!")
	assert.Equal(t, 96301, ring().ItemId)

	user.Character.SetSkill("spellcasting", 4)
	out, ring = removeOut(t, "hexed ring")
	assert.Contains(t, out, "luckily your")
	assert.Equal(t, 0, ring().ItemId)
}

// `remove all` leaves cursed gear on, with the cursed line, and takes the
// rest off (it stripped cursed gear before).
func TestRemoveAll_SkipsCursedGear(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, _ := getTestUserAndRoom(t)
	user.Character.Equipment.Ring = hexed(96302, "hexed ring", items.Ring)
	user.Character.Equipment.Head = items.Item{ItemId: 96303, Spec: &items.ItemSpec{ItemId: 96303, Name: "cap", Type: items.Head, Subtype: items.Wearable}}
	out, ring := removeOut(t, "all")
	assert.Contains(t, out, "is cursed and prevents you from removing it.")
	assert.Equal(t, 96302, ring().ItemId)
	assert.Equal(t, 0, user.Character.Equipment.Head.ItemId)
}
