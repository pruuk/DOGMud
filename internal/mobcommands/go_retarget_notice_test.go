package mobcommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goRetargetNoticeTagPattern matches one ANSI tag, mirroring
// internal/hooks/narration_testhelpers_test.go's plainText helper.
var goRetargetNoticeTagPattern = regexp.MustCompile(`<[^>]*>`)

func goRetargetNoticePlainText(line string) string {
	return strings.TrimSpace(goRetargetNoticeTagPattern.ReplaceAllString(line, ""))
}

// clearRoomAggroOnDeparture named the mob a player was retargeted onto
// regardless of the player's sight ("You turn your attention to Windscour
// Wyrm!" in an unlit cave with no night vision). The room is unlit, the
// player has no night vision, so the notice must say "something" and must
// not name mob B (the mob the player is retargeted onto) anywhere.
func TestClearRoomAggroOnDeparture_DarkRoomHidesTheRetargetedName(t *testing.T) {
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{}))

	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", DarkArea: true, MovementCost: 1},
	}))

	room := &rooms.Room{RoomId: 9300, Biome: "cave"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9300: room}, map[string]*rooms.ZoneConfig{}))
	require.Zero(t, room.GetVisibility(), "room must actually be unlit")

	mobA := &mobs.Mob{
		MobId: 9300, InstanceId: 9301, HomeRoomId: 9300,
		Character: characters.Character{
			Name: "Departing Wolf", RoomId: 9300, Health: 100,
			Buffs: buffs.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	mobA.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(9301, mobA)
	t.Cleanup(func() { mobs.SetInstanceForTest(9301, nil) })

	mobB := &mobs.Mob{
		MobId: 9302, InstanceId: 9303, HomeRoomId: 9300,
		Character: characters.Character{
			Name: "Windscour Wyrm", RoomId: 9300, Health: 100,
			Buffs: buffs.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	mobB.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(9303, mobB)
	t.Cleanup(func() { mobs.SetInstanceForTest(9303, nil) })

	u := users.NewTestUser(9310, "kesh", "Kesh", 98310)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9310: u}))
	u.Character.RoomId = 9300
	room.AddPlayer(9310)
	room.AddMob(9301)
	room.AddMob(9303)

	// Player is fighting the departing mob A; mob B is fighting the player,
	// so RetargetOrEnd-equivalent logic in clearRoomAggroOnDeparture puts the
	// player onto mob B when mob A leaves.
	require.True(t, targeting.Commit(u.Character, state.ActorRef{MobInstanceId: mobA.InstanceId}, targeting.ReasonAttack))
	require.True(t, targeting.Commit(&mobB.Character, state.ActorRef{UserId: u.UserId}, targeting.ReasonAttack))

	events.DrainQueuedMessagesForTest(u.UserId)

	clearRoomAggroOnDeparture(room, mobA.InstanceId)

	lines := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1, "the player should get exactly one retarget notice")
	assert.Equal(t, "You turn your attention to something!", goRetargetNoticePlainText(lines[0]))
	assert.NotContains(t, lines[0], mobB.Character.Name, "dark routing leaked the retargeted mob's identity")
}

// The two sends in clearRoomAggroOnDeparture are textually identical: one
// fires when a hostile mob is attacking the player directly, the other when
// it is attacking one of the player's companions. Only the direct branch is
// covered above; this drives the companion branch so it cannot regress
// silently while the direct branch's test stays green.
func TestClearRoomAggroOnDeparture_DarkRoomHidesTheRetargetedName_CompanionBranch(t *testing.T) {
	t.Cleanup(buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{}))

	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", DarkArea: true, MovementCost: 1},
	}))

	room := &rooms.Room{RoomId: 9400, Biome: "cave"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9400: room}, map[string]*rooms.ZoneConfig{}))
	require.Zero(t, room.GetVisibility(), "room must actually be unlit")

	mobA := &mobs.Mob{
		MobId: 9400, InstanceId: 9401, HomeRoomId: 9400,
		Character: characters.Character{
			Name: "Departing Wolf", RoomId: 9400, Health: 100,
			Buffs: buffs.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	mobA.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(9401, mobA)
	t.Cleanup(func() { mobs.SetInstanceForTest(9401, nil) })

	mobB := &mobs.Mob{
		MobId: 9402, InstanceId: 9403, HomeRoomId: 9400,
		Character: characters.Character{
			Name: "Windscour Wyrm", RoomId: 9400, Health: 100,
			Buffs: buffs.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	mobB.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(9403, mobB)
	t.Cleanup(func() { mobs.SetInstanceForTest(9403, nil) })

	companion := &mobs.Mob{
		MobId: 9404, InstanceId: 9405, HomeRoomId: 9400,
		Character: characters.Character{
			Name: "Spirit Wolf", RoomId: 9400, Health: 100,
			Buffs: buffs.New(), Cooldowns: map[string]int{}, SpeciesId: 1,
		},
	}
	companion.Character.HealthMax.Value = 100
	mobs.SetInstanceForTest(9405, companion)
	t.Cleanup(func() { mobs.SetInstanceForTest(9405, nil) })

	u := users.NewTestUser(9410, "kesh", "Kesh", 98410)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9410: u}))
	u.Character.RoomId = 9400
	u.Character.Companions = []characters.CompanionInfo{{MobId: 9404, InstanceId: 9405, Name: "Spirit Wolf"}}
	room.AddPlayer(9410)
	room.AddMob(9401)
	room.AddMob(9403)

	// Player is fighting the departing mob A; mob B is fighting the player's
	// COMPANION, not the player directly, so the companion branch of
	// clearRoomAggroOnDeparture is the one that must retarget the player.
	require.True(t, targeting.Commit(u.Character, state.ActorRef{MobInstanceId: mobA.InstanceId}, targeting.ReasonAttack))
	require.True(t, targeting.Commit(&mobB.Character, state.ActorRef{MobInstanceId: companion.InstanceId}, targeting.ReasonAttack))

	events.DrainQueuedMessagesForTest(u.UserId)

	clearRoomAggroOnDeparture(room, mobA.InstanceId)

	lines := events.DrainQueuedMessagesForTest(u.UserId)
	require.Len(t, lines, 1, "the player should get exactly one retarget notice")
	assert.Equal(t, "You turn your attention to something!", goRetargetNoticePlainText(lines[0]))
	assert.NotContains(t, lines[0], mobB.Character.Name, "dark routing leaked the retargeted mob's identity")
}
