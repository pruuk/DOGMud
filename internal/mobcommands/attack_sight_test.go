package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A mob must not attack by name a creature it cannot perceive (slice F parity
// with players). Perceives is about CONCEALMENT, not darkness.
//
// This drives the real Attack command rather than FindAttackTarget directly:
// the defect is that attack.go passed a nil viewer, so calling the helper with
// a viewer would pass even with the bug in place.
func TestMobAttackCannotNameAHiddenPlayer(t *testing.T) {
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{}))

	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"city": {BiomeId: "city", LitArea: true},
	}))

	room := &rooms.Room{RoomId: 8200, Biome: "city"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{8200: room}, map[string]*rooms.ZoneConfig{}))

	m := &mobs.Mob{
		MobId: 8200, InstanceId: 8201, HomeRoomId: 8200,
		Character: characters.Character{
			Name: "Lurker", RoomId: 8200, Health: 100,
			Buffs: buffs.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	m.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(8201, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(8201, nil) })

	u := users.NewTestUser(8210, "kesh", "Kesh", 98210)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{8210: u}))
	room.AddPlayer(8210)

	reason := state.TransitionReason{Trigger: "slice_f_test"}
	require.NoError(t, u.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	u.Character.Awareness.ResolveConcealment(true, reason)
	require.True(t, u.Character.IsHidden())

	_, err := Attack("kesh", m, room)
	require.NoError(t, err)
	require.False(t, m.Character.IsInCombat(),
		"a mob with no see-hidden must not start a fight with a hidden player it named")
}
