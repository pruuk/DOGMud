package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #435: `loot pass` named the party member the share goes to, whatever the
// looter could see. The name now follows the looter's sight, as the corpse's
// own name already did (corpseNameFor). Aliceia (user 1) passes her
// round-robin share to Bobrick (user 2) in room 1.
func lootPassScene(t *testing.T) *rooms.Room {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	restoreCond := seedCraftHidingCondition()
	t.Cleanup(restoreCond)

	p := parties.New(1)
	require.NotNil(t, p)
	t.Cleanup(p.Disband)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))

	room := rooms.LoadRoom(1)
	origCorpses := room.Corpses
	t.Cleanup(func() { room.Corpses = origCorpses })
	sword := items.New(10001)
	corpse := rooms.Corpse{
		MobId:           1,
		OwnerUserIds:    []int{1, 2},
		LootMode:        "roundrobin",
		RoundOwnedUntil: 1 << 62,
		RRAssignee:      map[string]int{sword.UUID.String(): 1},
	}
	corpse.Character.Name = "Skeleton"
	corpse.Loot.AddItem(sword)
	room.Corpses = []rooms.Corpse{corpse}
	craftPlainLines(1)
	return room
}

func TestLootPass_ShapesOnlyLooterDoesNotReadTheMembersName(t *testing.T) {
	room := lootPassScene(t)
	darkenCraftRoom(t, 1)
	looter := users.GetByUserId(1)
	require.True(t, looter.Character.Conditions.AddCondition(craftHidingInfraredConditionId, true))

	handled, err := Loot("pass corpse", looter, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	lines := craftPlainLines(1)
	require.Equal(t, []string{"You pass your share of the corpse of a figure to a figure."}, lines,
		"a shapes-only looter must not read whom the share goes to")
}

func TestLootPass_ClearSightNamesTheMember(t *testing.T) {
	room := lootPassScene(t)
	room.Lamp = rooms.LampPtr(90)

	handled, err := Loot("pass corpse", users.GetByUserId(1), room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	require.Equal(t, []string{"You pass your share of the Skeleton corpse to Bobrick."}, craftPlainLines(1))
}
