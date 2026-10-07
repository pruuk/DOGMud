package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: look_corpse_observer_hiding_test.go
//
// A player corpse's display name is "<name> corpse" inside a user-corpse
// tag, which messaging.Anonymize does not strip (it is not an identity tag),
// so the observer line of `look <corpse>` must pass the dead player's name
// to SendTextVisualHidingNames or a shapes-only bystander reads it.
//
// Aliceia (user 1) looks at the corpse; Bobrick (user 2) watches. Both carry
// infrared in an unlit cave, so both are at shapes: the looker may still
// look (only SightNone is refused), and the watcher cannot tell who is who.
func TestLookCorpse_ShapesOnlyObserverDoesNotReadTheDeadPlayersName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	darkenCraftRoom(t, 1)

	looker, watcher := users.GetByUserId(1), users.GetByUserId(2)
	require.True(t, looker.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))
	require.True(t, watcher.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))

	room := rooms.LoadRoom(1)
	origCorpses := room.Corpses
	defer func() { room.Corpses = origCorpses }()
	corpse := rooms.Corpse{UserId: 777}
	corpse.Character.Name = "Deadric"
	room.Corpses = append([]rooms.Corpse{}, corpse)

	craftPlainLines(1)
	craftPlainLines(2)

	handled, err := Look("deadric corpse", looker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lookerLines, watcherLines := craftPlainLines(1), craftPlainLines(2)
	require.Equal(t, 1, craftCountContaining(lookerLines, "You look at the Deadric corpse."),
		"precondition: the looker must reach the corpse branch: got %v", lookerLines)
	require.Equal(t, 1, craftCountContaining(watcherLines, "is looking at the"),
		"the watcher must get the observer line: got %v", watcherLines)
	require.Equal(t, 0, craftCountContaining(watcherLines, "Deadric"),
		"a shapes-only observer must not read the dead player's name: got %v", watcherLines)
	require.Equal(t, []string{"A figure is looking at the corpse of a figure."}, watcherLines,
		"the hidden line must still read as English")
}

// #428: the room look's "On the Ground" line named a dead mob (and a dead
// player) to a viewer who makes out shapes only. It now reads one "corpse
// of a figure" per corpse, the same hidden form as the observer line above.
func TestLookRoom_ShapesViewerReadsCorpsesOnTheGroundAsFigures(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	useDogmudTemplates(t)
	darkenCraftRoom(t, 1)

	looker := users.GetByUserId(1)
	require.True(t, looker.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))

	room := rooms.LoadRoom(1)
	origCorpses := room.Corpses
	defer func() { room.Corpses = origCorpses }()
	beggar := rooms.Corpse{MobId: 12}
	beggar.Character.Name = "City Beggar"
	deadric := rooms.Corpse{UserId: 777}
	deadric.Character.Name = "Deadric"
	room.Corpses = []rooms.Corpse{beggar, deadric}

	craftPlainLines(1)
	handled, err := Look(``, looker, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lines := craftPlainLines(1)
	require.Equal(t, 1, craftCountContaining(lines, "On the Ground: corpse of a figure and corpse of a figure"),
		"the ground line must list both corpses as figures: got %v", lines)
	require.Equal(t, 0, craftCountContaining(lines, "Beggar"), "a dead mob was named: %v", lines)
	require.Equal(t, 0, craftCountContaining(lines, "Deadric"), "a dead player was named: %v", lines)
}
