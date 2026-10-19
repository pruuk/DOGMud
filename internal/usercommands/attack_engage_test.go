package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// engageMarkerStore seeds a prepare pool whose lines are easy to find. Every
// subtype falls back to Generic, so the marker fires for any attacker.
func engageMarkerStore(t *testing.T) {
	t.Helper()
	one := func(s string) items.SkillTieredMessages {
		return items.SkillTieredMessages{Beginner: items.MessageOptions{items.ItemMessage(s)}}
	}
	marker := items.AttackOptions{Together: items.TogetherMessages{
		ToAttacker: one(`ENGAGE-ACTOR you square up to {actee}.`),
		ToDefender: one(`ENGAGE-ACTEE {actor} squares up to you.`),
		ToRoom:     one(`ENGAGE-ROOM {actor} squares up to {actee}.`),
	}}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: {OptionId: items.Generic, Options: items.AttackTypes{items.Prepare: marker}},
	}))
}

// Ruling A: in a lit room the attacker and the room read the prepare line,
// and the hand-written line is gone.
func TestAttack_Lit_EngageLineComesFromThePreparePool(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	engageMarkerStore(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	t.Cleanup(func() { user.Character.EndAggro() })
	witness := users.GetByUserId(2)
	require.NotNil(t, witness)
	events.DrainQueuedMessagesForTest(user.UserId)
	events.DrainQueuedMessagesForTest(witness.UserId)

	handled, err := Attack("skeleton", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n")
	require.Contains(t, out, "ENGAGE-ACTOR", "the attacker reads the prepare line: %q", out)
	require.NotContains(t, out, "mortal combat", "the hand-written line is retired in the light: %q", out)

	seen := strings.Join(events.DrainQueuedMessagesForTest(witness.UserId), "\n")
	require.Equal(t, user.Character.RoomId, witness.Character.RoomId, "witness must share the room")
	require.Contains(t, seen, "ENGAGE-ROOM", "the witness reads the prepare room line: %q", seen)
	require.NotContains(t, seen, "prepares to fight", "the hand-written room line is retired: %q", seen)
}

// PvP at full sight: the defender reads the prepare pool's defender line and
// a third player in the room its observer line; neither reads the
// hand-written "prepares to fight". The room line excludes both duelists, so
// the fixture's two players are joined by a witness.
func TestAttack_PvP_Lit_DefenderAndRoomReadThePreparePool(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	engageMarkerStore(t)
	user, room := getTestUserAndRoom(t)
	user.Character.EndAggro()
	t.Cleanup(func() { user.Character.EndAggro() })
	defender := users.GetByUserId(2)
	require.NotNil(t, defender)

	witness := users.NewTestUser(3, "cara", "Carabel", 1003)
	witness.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: user, 2: defender, 3: witness}))
	room.AddPlayer(witness.UserId)
	t.Cleanup(func() { room.RemovePlayer(witness.UserId) })

	require.Equal(t, user.Character.RoomId, defender.Character.RoomId, "defender must share the room")
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(defender.Character, room),
		"fixture: the defender must be at full sight")
	for _, id := range []int{user.UserId, defender.UserId, witness.UserId} {
		events.DrainQueuedMessagesForTest(id)
	}

	handled, err := Attack(defender.Character.Name, user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(events.DrainQueuedMessagesForTest(defender.UserId), "\n")
	require.Contains(t, out, "ENGAGE-ACTEE", "the defender reads the prepare line: %q", out)
	require.NotContains(t, out, "prepares to fight", "the hand-written line is retired at full sight: %q", out)

	seen := strings.Join(events.DrainQueuedMessagesForTest(witness.UserId), "\n")
	require.Contains(t, seen, "ENGAGE-ROOM", "the witness reads the prepare room line: %q", seen)
	require.NotContains(t, seen, "prepares to fight", "the hand-written room line is retired: %q", seen)
}
