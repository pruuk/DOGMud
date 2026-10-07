package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: loot_corpse_sight_test.go
//
// #428 review: `loot` named the corpse to a looter who makes out shapes only
// and, through a plain SendTextVisual, to every shapes-only bystander (a
// mob-corpse tag is not an identity tag, so Anonymize leaves it). The looter's
// lines now read Corpse.NameAt at the looter's sight and the room line hides
// the corpse and looter names per bystander.

// lootCorpseFixture seeds room 1 with one unowned City Beggar corpse holding
// gold and the given items, and returns a restore func.
func lootCorpseFixture(t *testing.T, gold int, itemIds ...int) func() {
	t.Helper()
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	orig := room.Corpses
	loot := rooms.Container{Gold: gold}
	for _, id := range itemIds {
		loot.Items = append(loot.Items, items.New(id))
	}
	beggar := rooms.Corpse{MobId: 12, Loot: loot}
	beggar.Character.Name = "City Beggar"
	room.Corpses = []rooms.Corpse{beggar}
	return func() { room.Corpses = orig }
}

// darkShapesPair darkens room 1 and gives both test users heat sight, so
// each makes out shapes only.
func darkShapesPair(t *testing.T) (*users.UserRecord, *users.UserRecord) {
	t.Helper()
	darkenCraftRoom(t, 1)
	a, b := users.GetByUserId(1), users.GetByUserId(2)
	require.True(t, a.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))
	require.True(t, b.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))
	return a, b
}

func TestLoot_ShapesLooterAndWatcherDoNotReadTheCorpseName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	defer lootCorpseFixture(t, 5, 10001)()
	looter, _ := darkShapesPair(t)
	looter.Character.Items = nil

	craftPlainLines(1)
	craftPlainLines(2)
	handled, err := Loot("corpse", looter, rooms.LoadRoom(1), 0)
	require.True(t, handled)
	require.NoError(t, err)

	looterLines, watcherLines := craftPlainLines(1), craftPlainLines(2)
	require.Equal(t, 2, craftCountContaining(looterLines, "from the corpse of a figure."),
		"the item and gold lines must take the hidden form: got %v", looterLines)
	require.Equal(t, 0, craftCountContaining(looterLines, "Beggar"), "the looter read the name: %v", looterLines)
	require.Equal(t, []string{"A figure loots the corpse of a figure."}, watcherLines,
		"a shapes-only watcher must read neither name")
}

func TestLoot_ShapesLooterOfAnEmptyCorpseDoesNotReadItsName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	defer lootCorpseFixture(t, 0)()
	looter, _ := darkShapesPair(t)

	craftPlainLines(1)
	handled, err := Loot("corpse", looter, rooms.LoadRoom(1), 0)
	require.True(t, handled)
	require.NoError(t, err)

	require.Equal(t, []string{"There's nothing to loot from the corpse of a figure."}, craftPlainLines(1))
}

// Lit: the looter reads the corpse by name and a clear-sighted watcher reads
// both names.
func TestLoot_ClearLooterAndWatcherReadTheNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer lootCorpseFixture(t, 5)()
	looter := users.GetByUserId(1)
	require.Greater(t, rooms.LoadRoom(1).LightLevel(), 0, "precondition: room 1 is lit")

	craftPlainLines(1)
	craftPlainLines(2)
	handled, err := Loot("corpse", looter, rooms.LoadRoom(1), 0)
	require.True(t, handled)
	require.NoError(t, err)

	looterLines, watcherLines := craftPlainLines(1), craftPlainLines(2)
	require.Equal(t, 1, craftCountContaining(looterLines, "from the City Beggar corpse."),
		"a clear-sighted looter reads the corpse by name: got %v", looterLines)
	require.Equal(t, []string{"Aliceia loots the corpse of City Beggar."}, watcherLines)
}
