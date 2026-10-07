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
