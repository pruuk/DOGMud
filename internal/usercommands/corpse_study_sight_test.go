package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// File: corpse_study_sight_test.go
//
// #428 review sweep: `assess <corpse>` and `salvage <corpse>` echoed the dead
// one's name to a player who makes out shapes only and reached the corpse by
// the word "corpse". Both now read it by the player's sight.

const corpseStudyMobId = 12

func corpseStudyFixture(t *testing.T) func() {
	t.Helper()
	restoreMobs := mobs.SeedMobsForTest(map[int]*mobs.Mob{
		corpseStudyMobId: {MobId: corpseStudyMobId, Groups: []string{"animal"}},
	}, map[int]*mobs.Mob{})
	restoreCorpses := lootCorpseFixture(t, 0)
	users.GetByUserId(1).Character.Activity = activity.NewMachine()
	return func() { restoreCorpses(); restoreMobs() }
}

func TestAssessAndSalvageCorpse_ShapesPlayerDoesNotReadTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreCond := seedCraftHidingCondition()
	defer restoreCond()
	defer corpseStudyFixture(t)()
	player, _ := darkShapesPair(t)
	room := rooms.LoadRoom(1)

	craftPlainLines(1)
	handled, err := Assess("corpse", player, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	lines := craftPlainLines(1)
	require.Equal(t, 1, craftCountContaining(lines, "You study the remains of a figure."), "got %v", lines)
	require.Equal(t, 0, craftCountContaining(lines, "Beggar"), "assess named the corpse: %v", lines)

	handled, err = Salvage("corpse", player, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	lines = craftPlainLines(1)
	require.Equal(t, 1, craftCountContaining(lines, "You begin carefully working over the corpse of a figure..."), "got %v", lines)
	require.Equal(t, 0, craftCountContaining(lines, "Beggar"), "salvage named the corpse: %v", lines)
}

func TestAssessAndSalvageCorpse_ClearPlayerReadsTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer corpseStudyFixture(t)()
	player := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	require.Greater(t, room.LightLevel(), 0, "precondition: room 1 is lit")

	craftPlainLines(1)
	handled, err := Assess("corpse", player, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, craftCountContaining(craftPlainLines(1), "You study the remains of City Beggar."))

	handled, err = Salvage("corpse", player, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	lines := craftPlainLines(1)
	require.Equal(t, 1, craftCountContaining(lines, "You begin carefully working over the City Beggar corpse..."), "got %v", lines)
}
