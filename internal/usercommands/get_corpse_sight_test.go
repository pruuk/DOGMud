package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: get_corpse_sight_test.go
//
// #428 review: `get <item|gold> from <corpse>` echoed the corpse's name to a
// shapes-only taker and, through a plain SendTextVisual, to every shapes-only
// bystander. Same treatment as loot (loot_corpse_sight_test.go).

func TestGetFromCorpse_ShapesTakerAndWatcherDoNotReadTheCorpseName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	defer lootCorpseFixture(t, 5, 10001)()
	taker, _ := darkShapesPair(t)
	taker.Character.Items = nil
	room := rooms.LoadRoom(1)

	for _, cmd := range []string{"gold from corpse", "iron sword from corpse"} {
		craftPlainLines(1)
		craftPlainLines(2)
		handled, err := Get(cmd, taker, room, 0)
		require.True(t, handled)
		require.NoError(t, err)

		takerLines, watcherLines := craftPlainLines(1), craftPlainLines(2)
		require.Equal(t, 1, craftCountContaining(takerLines, "from the corpse of a figure."),
			"%q: the taker's line must take the hidden form: got %v", cmd, takerLines)
		require.Equal(t, 0, craftCountContaining(takerLines, "Beggar"), "%q: the taker read the name: %v", cmd, takerLines)
		require.Equal(t, 1, craftCountContaining(watcherLines, "A figure loots"),
			"%q: the watcher must get the room line: got %v", cmd, watcherLines)
		require.Equal(t, 0, craftCountContaining(watcherLines, "Beggar"), "%q: the watcher read the name: %v", cmd, watcherLines)
	}

	craftPlainLines(1)
	handled, err := Get("dagger from corpse", taker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, []string{"You don't see a dagger in the corpse of a figure."}, craftPlainLines(1))

	craftPlainLines(1)
	handled, err = Get("all corpse", taker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, []string{"There's nothing left on the corpse of a figure."}, craftPlainLines(1))
}

// #435 review: `get all corpse` found the corpse by "corpse", then re-entered
// Get with the corpse's full name, which the shapes-sight lookup refuses. It
// took nothing and named the dead in the refusal. It now takes from the
// corpse it already found.
func TestGetAllCorpse_ShapesTakerTakesEverythingWithoutReadingTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	defer lootCorpseFixture(t, 5, 10001)()
	taker, _ := darkShapesPair(t)
	taker.Character.Items = nil
	room := rooms.LoadRoom(1)
	goldBefore := taker.Character.Gold

	craftPlainLines(1)
	craftPlainLines(2)
	handled, err := Get("all corpse", taker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	takerLines, watcherLines := craftPlainLines(1), craftPlainLines(2)
	require.Equal(t, goldBefore+5, taker.Character.Gold, "the gold must be taken: %v", takerLines)
	require.Len(t, taker.Character.Items, 1, "the sword must be taken: %v", takerLines)
	require.Equal(t, 0, room.Corpses[0].Loot.Gold)
	require.Empty(t, room.Corpses[0].Loot.Items)
	require.Equal(t, 2, craftCountContaining(takerLines, "from the corpse of a figure."),
		"the gold and item lines must take the hidden form: got %v", takerLines)
	require.Equal(t, 0, craftCountContaining(takerLines, "Beggar"), "the taker read the name: %v", takerLines)
	require.Equal(t, 0, craftCountContaining(watcherLines, "Beggar"), "the watcher read the name: %v", watcherLines)
}

// Clear sight: `get all corpse` still takes everything and names the corpse.
func TestGetAllCorpse_ClearTakerTakesEverythingAndReadsTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer lootCorpseFixture(t, 5, 10001)()
	taker := users.GetByUserId(1)
	taker.Character.Items = nil
	room := rooms.LoadRoom(1)
	require.Greater(t, room.LightLevel(), 0, "precondition: room 1 is lit")
	goldBefore := taker.Character.Gold

	craftPlainLines(1)
	handled, err := Get("all corpse", taker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	takerLines := craftPlainLines(1)
	require.Equal(t, goldBefore+5, taker.Character.Gold, "the gold must be taken: %v", takerLines)
	require.Len(t, taker.Character.Items, 1, "the sword must be taken: %v", takerLines)
	require.Equal(t, 2, craftCountContaining(takerLines, "from the City Beggar corpse."),
		"a clear-sighted taker reads the corpse by name: got %v", takerLines)
}

// Lit: the taker reads the corpse by name and a clear-sighted watcher reads
// both names.
func TestGetFromCorpse_ClearTakerAndWatcherReadTheNames(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer lootCorpseFixture(t, 5)()
	taker := users.GetByUserId(1)
	require.Greater(t, rooms.LoadRoom(1).LightLevel(), 0, "precondition: room 1 is lit")

	craftPlainLines(1)
	craftPlainLines(2)
	handled, err := Get("gold from corpse", taker, rooms.LoadRoom(1), 0)
	require.True(t, handled)
	require.NoError(t, err)

	require.Equal(t, 1, craftCountContaining(craftPlainLines(1), "from the City Beggar corpse."))
	require.Equal(t, []string{"Aliceia loots some gold from the corpse of City Beggar."}, craftPlainLines(2))
}
