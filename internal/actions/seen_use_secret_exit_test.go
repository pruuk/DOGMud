package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 15: disarming the trap on a secret exit's lock is a use
// of it, so a sighted player and NPC who see the line learn the exit. The one
// disarming is not changed by the rule; in the dark nobody learns.
func TestDefuse_OnlookersLearnTheSecretExitTheySeeDisarmed(t *testing.T) {
	for _, lit := range []bool{true, false} {
		t.Run(map[bool]string{true: "lit", false: "dark"}[lit], func(t *testing.T) {
			t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
			const roomId, watcherId, mobId = 99561, 99571, 98571
			lamp := rooms.LampPtr(60)
			if !lit {
				lamp = rooms.LampPtr(0)
			}
			t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
				roomId: {RoomId: roomId, Zone: "test", SkyLight: rooms.SkyLightPtr(0), Lamp: lamp, Exits: map[string]exit.RoomExit{
					"crack": {RoomId: 99562, Secret: true, Lock: gamelock.Lock{Difficulty: 3, TrapConditionIds: []int{42}}},
				}},
			}, map[string]*rooms.ZoneConfig{}))
			room := rooms.LoadRoom(roomId)
			watcher := users.NewTestUser(watcherId, "wenna", "Wenna", 0)
			watcher.Character.RoomId = roomId
			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{watcherId: watcher}))
			room.AddPlayer(watcherId)
			npc := &mobs.Mob{InstanceId: mobId, Character: *characters.New()}
			npc.Character.RoomId = roomId
			mobs.SetInstanceForTest(mobId, npc)
			t.Cleanup(func() { mobs.SetInstanceForTest(mobId, nil) })
			room.AddMob(mobId)

			actor := newDefuseActor(500, 5)
			actor.room = room
			events.DrainQueuedMessagesForTest(watcherId)

			clearDefuseTrap(actor, room, defuseLockTarget{kind: lockTargetExit, exitName: "crack"})

			got := events.DrainQueuedMessagesForTest(watcherId)
			if lit {
				require.NotEmpty(t, got, "premise: the watcher read the disarm line")
			}
			require.Equal(t, lit, watcher.Character.KnowsSecretExit(roomId, "crack"), "the player onlooker")
			require.Equal(t, lit, npc.Character.KnowsSecretExit(roomId, "crack"), "the NPC onlooker")
			require.False(t, actor.GetCharacter().KnowsSecretExit(roomId, "crack"), "the actor's own knowledge is not this rule's")
		})
	}
}
