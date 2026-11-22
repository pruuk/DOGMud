package usercommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #940 (owner ruling 2026-11-22): a copy a quest handed out fades whenever it
// leaves its holder's hands; a bought copy of the same item moves exactly as
// before. Each way out below runs twice, once per copy. The Iron Sword (10001)
// stands in for the starter gear: one template, two kinds of copy. Give is in
// quest_given_give_test.go and look in quest_given_look_test.go.

const questGearId = 10001 // Iron Sword, seeded by seedAllRegistries

// questGearCopy is a copy as acquire.AcquireItemForChar marks it for a quest
// hand-out; boughtCopy is an ordinary copy of the same item. Built without
// items.New, which needs the spec seeded and these are often called first.
func questGearCopy() items.Item {
	itm := boughtCopy()
	itm.QuestGiven = true
	return itm
}

func boughtCopy() items.Item {
	return items.Item{ItemId: questGearId, UUID: items.NewItemUUID()}
}

// fadeLine is the holder's line for the seeded sword.
var fadeLine = fmt.Sprintf(actions.QuestGearFadesLine, "Iron Sword")

// questGearHolder is user 1 in the seeded square carrying exactly itm.
func questGearHolder(t *testing.T, itm items.Item) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 100
	user.Character.Items = []items.Item{itm}
	for _, id := range []int{1, 2} {
		events.DrainQueuedMessagesForTest(id)
	}
	return user, room
}

func carriesSword(c []items.Item) int { return countItemsById(c, questGearId) }

func TestQuestGear_Drop(t *testing.T) {
	t.Run("quest gear fades", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		_, err := Drop("iron sword", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, fadeLine)
		assert.NotContains(t, out, "You drop")
		assert.Equal(t, 0, carriesSword(user.Character.Items))
		assert.Equal(t, 0, carriesSword(room.Items), "nothing reaches the floor")
		assert.Contains(t, sentTo(users.GetByUserId(2)), "Iron Sword</ansi> fades away.", "the room sees it fade")
	})
	t.Run("bought copy drops", func(t *testing.T) {
		user, room := questGearHolder(t, boughtCopy())
		_, err := Drop("iron sword", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), "You drop the")
		assert.Equal(t, 1, carriesSword(room.Items))
	})
	t.Run("drop all.sword: one line per faded copy, the bought one drops", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		user.Character.Items = append(user.Character.Items, questGearCopy(), boughtCopy())
		_, err := Drop("all.sword", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Equal(t, 2, strings.Count(out, fadeLine))
		assert.Contains(t, out, "You drop")
		assert.Equal(t, 1, carriesSword(room.Items))
		assert.Equal(t, 0, carriesSword(user.Character.Items))
	})
	t.Run("drop all.sword with only quest gear says nothing more", func(t *testing.T) {
		user, room := questGearHolder(t, questGearCopy())
		_, err := Drop("all.sword", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, fadeLine)
		assert.NotContains(t, out, "You drop")
		assert.NotContains(t, out, "You don't have")
	})
}

// A way out named by a word that fits both copies takes the bought one: the
// let-go lookup (characters.FindInBackpackToLetGo), shared by every way out.
func TestQuestGear_DropTakesTheBoughtCopyFirst(t *testing.T) {
	user, room := questGearHolder(t, questGearCopy())
	user.Character.Items = append(user.Character.Items, boughtCopy())
	_, err := Drop("iron sword", user, room, 0)
	require.NoError(t, err)
	assert.Contains(t, sentTo(user), "You drop the")
	require.Equal(t, 1, carriesSword(room.Items))
	assert.False(t, room.Items[0].IsQuestGiven())
	require.Equal(t, 1, carriesSword(user.Character.Items))
	assert.True(t, user.Character.Items[0].IsQuestGiven(), "the quest copy stays")
}

// The inventory marks a quest copy's row (items.SameStack keeps it off a
// bought copy's row; internal/items/quest_given_stack_test.go).
func TestQuestGear_InventoryRowIsMarked(t *testing.T) {
	name, formatted := questGearListName(questGearCopy(), "Firebomb", `<ansi fg="itemname">Firebomb</ansi>`)
	assert.Equal(t, "Firebomb (quest gear)", name)
	assert.Equal(t, `<ansi fg="itemname">Firebomb</ansi> <ansi fg="uses-left">(quest gear)</ansi>`, formatted)
	name, _ = questGearListName(boughtCopy(), "Firebomb", `<ansi fg="itemname">Firebomb</ansi>`)
	assert.Equal(t, "Firebomb", name)
}

// Taking an item from your pet while overloaded puts it back on the pet, or,
// when the pet cannot hold it either, drops it at your feet. Quest gear fades
// instead of dropping (#940).
func TestQuestGear_TakeFromPetWhileOverloaded(t *testing.T) {
	for _, quest := range []bool{true, false} {
		user, room := questGearHolder(t, items.Item{})
		t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
			94601: {ItemId: 94601, Name: "Anvil", NameSimple: "anvil", Type: items.Object, Weight: 500},
		}))
		user.Character.Items = nil
		user.Character.Stats.Strength.ValueAdj = 1
		anvil := items.New(94601)
		anvil.QuestGiven = quest
		user.Character.Pet = pets.Pet{Name: "Rex", Type: "dog", Capacity: 0, Items: []items.Item{anvil}}
		room.Items = nil

		_, err := Get("anvil from rex", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Empty(t, user.Character.Items, "quest=%v: too heavy to carry", quest)
		assert.Empty(t, user.Character.Pet.Items, "quest=%v: a pet with no room cannot hold it", quest)
		if quest {
			assert.Contains(t, out, fmt.Sprintf(actions.QuestGearFadesLine, "Anvil"))
			assert.Empty(t, room.Items, "quest gear never reaches the floor")
		} else {
			assert.Len(t, room.Items, 1, "a bought anvil falls at the player's feet")
		}
	}
}

func TestQuestGear_Put(t *testing.T) {
	for _, tc := range []struct {
		name  string
		itm   items.Item
		fades bool
	}{
		{"quest gear fades", questGearCopy(), true},
		{"bought copy goes in", boughtCopy(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, room := questGearHolder(t, tc.itm)
			room.Containers = map[string]rooms.Container{"chest": {}}
			_, err := Put("iron sword in chest", user, room, 0)
			require.NoError(t, err)
			out := sentTo(user)
			assert.Equal(t, 0, carriesSword(user.Character.Items))
			if tc.fades {
				assert.Contains(t, out, fadeLine)
				assert.Equal(t, 0, carriesSword(room.Containers["chest"].Items))
			} else {
				assert.Contains(t, out, "You place your")
				assert.Equal(t, 1, carriesSword(room.Containers["chest"].Items))
			}
		})
	}
}

func TestQuestGear_Stash(t *testing.T) {
	for _, tc := range []struct {
		name  string
		itm   items.Item
		fades bool
	}{
		{"quest gear fades", questGearCopy(), true},
		{"bought copy is stashed", boughtCopy(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, room := questGearHolder(t, tc.itm)
			_, err := Stash("iron sword", user, room, 0)
			require.NoError(t, err)
			out := sentTo(user)
			assert.Equal(t, 0, carriesSword(user.Character.Items))
			if tc.fades {
				assert.Contains(t, out, fadeLine)
				assert.Equal(t, 0, carriesSword(room.Stash))
				assert.NotContains(t, sentTo(users.GetByUserId(2)), "fades away", "a stash is quiet")
			} else {
				assert.Contains(t, out, "You stash the")
				assert.Equal(t, 1, carriesSword(room.Stash))
			}
		})
	}
}

func TestQuestGear_Storage(t *testing.T) {
	storageRoom := func(t *testing.T, itm items.Item) (*users.UserRecord, *rooms.Room) {
		user, room := questGearHolder(t, itm)
		room.IsStorage = true
		user.ItemStorage = users.Storage{}
		return user, room
	}
	t.Run("storage add: quest gear fades", func(t *testing.T) {
		user, room := storageRoom(t, questGearCopy())
		_, err := Storage("add sword", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, fadeLine)
		assert.NotContains(t, out, "into storage")
		assert.Equal(t, 0, user.ItemStorage.SlotCount())
		assert.Equal(t, 0, carriesSword(user.Character.Items))
	})
	t.Run("storage add: bought copy is stored", func(t *testing.T) {
		user, room := storageRoom(t, boughtCopy())
		_, err := Storage("add sword", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), "into storage")
		assert.Equal(t, 1, user.ItemStorage.SlotCount())
	})
	t.Run("storage add all sword: quest gear fades, the bought copy is stored", func(t *testing.T) {
		user, room := storageRoom(t, questGearCopy())
		user.Character.Items = append(user.Character.Items, boughtCopy())
		_, err := Storage("add all sword", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, fadeLine)
		assert.Contains(t, out, "into storage")
		assert.Equal(t, 1, user.ItemStorage.SlotCount())
		assert.Equal(t, 0, carriesSword(user.Character.Items))
	})
	t.Run("storage add 2 sword with one quest copy: no not-carrying line", func(t *testing.T) {
		user, room := storageRoom(t, questGearCopy())
		_, err := Storage("add 2 sword", user, room, 0)
		require.NoError(t, err)
		out := sentTo(user)
		assert.Contains(t, out, fadeLine)
		assert.NotContains(t, out, "You don't have")
	})
	t.Run("storage add all: quest gear fades, the rest is stored", func(t *testing.T) {
		user, room := storageRoom(t, questGearCopy())
		user.Character.Items = append(user.Character.Items, items.New(20001))
		_, err := Storage("add all", user, room, 0)
		require.NoError(t, err)
		assert.Contains(t, sentTo(user), fadeLine)
		assert.Equal(t, 1, user.ItemStorage.SlotCount())
		assert.Empty(t, user.Character.Items)
	})
}

func TestQuestGear_Stow(t *testing.T) {
	user, room := questGearHolder(t, items.Item{})
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		94201: {ItemId: 94201, Name: "Iron Ore", NameSimple: "ore", Type: items.Object, IsComponent: true, Value: 5},
	}))
	ore := items.New(94201)
	ore.QuestGiven = true
	user.Character.Items = []items.Item{ore, items.New(94201)}
	room.IsStorage = true
	user.ItemStorage = users.Storage{}
	_, err := Stow("", user, room, 0)
	require.NoError(t, err)
	out := sentTo(user)
	assert.Contains(t, out, fmt.Sprintf(actions.QuestGearFadesLine, "Iron Ore"))
	assert.Contains(t, out, "You stow 1 crafting component")
	assert.Equal(t, 1, user.ItemStorage.SlotCount())
	assert.Empty(t, user.Character.Items)
}

func TestQuestGear_Mail(t *testing.T) {
	for _, tc := range []struct {
		name  string
		itm   items.Item
		fades bool
	}{
		{"quest gear fades and the mail goes without it", questGearCopy(), true},
		{"bought copy travels", boughtCopy(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, room := questGearHolder(t, tc.itm)
			bob := users.GetByUserId(2)
			bob.Inbox = nil
			t.Cleanup(user.ClearPrompt)
			run := func() { _, _ = Mail("Bobrick", user, room, 0) }
			drive(t, user, run, "hello there", "0", "sword")
			out := drive(t, user, run, "1")
			assert.Contains(t, out, "is on its way")
			require.Len(t, bob.Inbox, 1)
			assert.Equal(t, 0, carriesSword(user.Character.Items))
			if tc.fades {
				assert.Contains(t, out, fadeLine)
				assert.Nil(t, bob.Inbox[0].Item, "no quest gear in the mail")
			} else {
				assert.NotContains(t, out, "fades away")
				require.NotNil(t, bob.Inbox[0].Item)
				assert.Equal(t, questGearId, bob.Inbox[0].Item.ItemId)
			}
		})
	}
}

func TestQuestGear_GuildDonate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		itm   items.Item
		fades bool
	}{
		{"quest gear fades", questGearCopy(), true},
		{"bought copy is donated", boughtCopy(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, _ := questGearHolder(t, tc.itm)
			t.Cleanup(guilds.SetDataDirForTest(t.TempDir()))
			_, err := guilds.Create("QG", "Quest Gear", user.UserId, user.Character.Name)
			require.NoError(t, err)
			t.Cleanup(func() { guilds.Delete("QG") })

			guildDonate(user, "sword")
			out := sentTo(user)
			g, _ := guilds.Get("QG")
			assert.Equal(t, 0, carriesSword(user.Character.Items))
			if tc.fades {
				assert.Contains(t, out, fadeLine)
				assert.Empty(t, g.Vault)
			} else {
				assert.Contains(t, out, "You donate")
				assert.Len(t, g.Vault, 1)
			}
		})
	}
}
