package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: look_corpse_sight_test.go
//
// `look corpse` follows the looker's own sight as the ground listing does
// (#428 review). room.FindCorpse matches the word "corpse" by substring, so a
// shapes-only looker reaches any corpse without knowing whose it is; their own
// line and the description must not tell them.

// lookCorpseFixture puts one corpse of a dead mob in room 1 and returns a
// restore func.
func lookCorpseFixture(t *testing.T) func() {
	t.Helper()
	room := rooms.LoadRoom(1)
	require.NotNil(t, room)
	orig := room.Corpses
	beggar := rooms.Corpse{MobId: 12}
	beggar.Character.Name = "City Beggar"
	beggar.Character.Description = "A ragged beggar of the city streets."
	room.Corpses = []rooms.Corpse{beggar}
	return func() { room.Corpses = orig }
}

func TestLookCorpse_ShapesLookerReadsTheCorpseAsAFigure(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	useDogmudTemplates(t)
	darkenCraftRoom(t, 1)
	defer lookCorpseFixture(t)()

	looker := users.GetByUserId(1)
	require.True(t, looker.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))

	craftPlainLines(1)
	craftPlainLines(2)
	handled, err := Look("corpse", looker, rooms.LoadRoom(1), 0)
	require.True(t, handled)
	require.NoError(t, err)

	lines := craftPlainLines(1)
	require.Equal(t, 1, craftCountContaining(lines, "You look at the corpse of a figure."),
		"the looker's own line must take the hidden form: got %v", lines)
	require.Equal(t, 0, craftCountContaining(lines, "Beggar"), "the dead mob was named: %v", lines)
	require.Equal(t, 0, craftCountContaining(lines, "ragged"), "the description was shown: %v", lines)
}

// The lit case is unchanged for the looker, and a clear-sighted bystander
// still reads the dead mob's name in the observer line.
func TestLookCorpse_ClearLookerAndWatcherReadTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	defer lookCorpseFixture(t)()

	looker := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	require.Greater(t, room.LightLevel(), 0, "precondition: room 1 is lit")

	craftPlainLines(1)
	craftPlainLines(2)
	handled, err := Look("corpse", looker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lookerLines, watcherLines := craftPlainLines(1), craftPlainLines(2)
	require.Equal(t, 1, craftCountContaining(lookerLines, "You look at the City Beggar corpse."),
		"a clear-sighted looker reads the corpse by name: got %v", lookerLines)
	require.Equal(t, 1, craftCountContaining(lookerLines, "ragged"),
		"a clear-sighted looker reads the description: got %v", lookerLines)
	require.Equal(t, 1, craftCountContaining(watcherLines, "City Beggar"),
		"a clear-sighted watcher reads the dead mob's name: got %v", watcherLines)
}
