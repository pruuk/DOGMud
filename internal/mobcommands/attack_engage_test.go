package mobcommands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mob-versus-mob room line comes from the prepare pool, once per
// reader, and the double-announce guard still holds on a repeat command.
func TestAttackMob_EngageRoomLineComesFromThePreparePool(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	one := func(s string) items.SkillTieredMessages {
		return items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(s)}}
	}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: {OptionId: items.Generic, Options: items.AttackTypes{items.Prepare: {Together: items.TogetherMessages{
			ToAttacker: one(`ENGAGE-ACTOR`), ToDefender: one(`ENGAGE-ACTEE`), ToRoom: one(`ENGAGE-ROOM {actor} faces {actee}.`),
		}}}},
	}))

	attacker, room := getTestMobAndRoom(t)
	var targetInstanceId int
	for _, id := range room.GetMobs() {
		if id != attacker.InstanceId {
			targetInstanceId = id
			break
		}
	}
	require.NotZero(t, targetInstanceId, "need a second mob in the room to attack")
	captured, mu, done := captureAnnounces(t)
	defer done()
	target := fmt.Sprintf("#%d", targetInstanceId)

	_, err := Attack(target, attacker, room)
	require.NoError(t, err)
	require.Equal(t, 1, countPerRecipient(t, captured, mu, "ENGAGE-ROOM"))
	assert.Zero(t, countPerRecipient(t, captured, mu, "prepares to fight"))

	_, err = Attack(target, attacker, room)
	require.NoError(t, err)
	assert.Equal(t, 1, countPerRecipient(t, captured, mu, "ENGAGE-ROOM"),
		"re-attacking a mob already being fought must NOT announce a second time")
}

// A mob engaging a player: the victim, at full sight, reads the prepare
// pool's defender line and the room its observer line; neither reads the
// hand-written "prepares to fight".
func TestAttackMob_VictimAtFullSightReadsThePreparePool(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	one := func(s string) items.SkillTieredMessages {
		return items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(s)}}
	}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: {OptionId: items.Generic, Options: items.AttackTypes{items.Prepare: {Together: items.TogetherMessages{
			ToAttacker: one(`ENGAGE-ACTOR`),
			ToDefender: one(`ENGAGE-ACTEE {actor} squares up to you.`),
			ToRoom:     one(`ENGAGE-ROOM {actor} squares up to {actee}.`),
		}}}},
	}))

	attacker, room := getTestMobAndRoom(t)
	t.Cleanup(func() { attacker.Character.EndAggro() })
	// The fixture room has no lamp; pin one so the victim sees clearly.
	origLamp := room.Lamp
	room.Lamp = rooms.LampPtr(60)
	t.Cleanup(func() { room.Lamp = origLamp })

	victim := users.GetByUserId(1)
	witness := users.GetByUserId(2)
	require.NotNil(t, victim)
	require.NotNil(t, witness)
	require.Equal(t, room.RoomId, victim.Character.RoomId, "victim must share the room")
	require.Equal(t, room.RoomId, witness.Character.RoomId, "witness must share the room")
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(victim.Character, room),
		"fixture: the victim must be at full sight")
	events.DrainQueuedMessagesForTest(victim.UserId)
	events.DrainQueuedMessagesForTest(witness.UserId)

	_, err := Attack(fmt.Sprintf("@%d", victim.UserId), attacker, room)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(victim.UserId), "\n")
	require.Contains(t, out, "ENGAGE-ACTEE", "the victim reads the prepare line: %q", out)
	require.NotContains(t, out, "prepares to fight", "the hand-written line is retired at full sight: %q", out)

	seen := strings.Join(events.DrainQueuedMessagesForTest(witness.UserId), "\n")
	require.Contains(t, seen, "ENGAGE-ROOM", "the witness reads the prepare room line: %q", seen)
	require.NotContains(t, seen, "ENGAGE-ACTEE", "the witness must not read the victim's line: %q", seen)
}
