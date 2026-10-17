package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-10-17 (the Malia party playtest): party auto-follow keys
// off the LEADER's light. A member who carries no light follows a leader
// who carries one; nobody follows a leader lost in true darkness.

const partyFollowLanternId = 9617

// partyFollowScene is the seeded square (room 1) and cave (room 2), both
// pitch dark, with Aliceia (user 1) leading a party Bobrick (user 2) has
// joined. leaderLit gives the leader a plain lantern.
func partyFollowScene(t *testing.T, leaderLit bool) (leader, member *users.UserRecord, room *rooms.Room) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		partyFollowLanternId: {ConditionId: partyFollowLanternId, Name: "Test Lantern", TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 54}}},
	}))
	leader, room = getTestUserAndRoom(t)
	member = users.GetByUserId(2)
	require.NotNil(t, member)
	room2 := rooms.LoadRoom(2)
	require.NotNil(t, room2)
	room.Lamp, room.SkyLight = rooms.LampPtr(0), rooms.SkyLightPtr(0)
	room2.Lamp, room2.SkyLight = rooms.LampPtr(0), rooms.SkyLightPtr(0)
	if leaderLit {
		require.True(t, leader.Character.Conditions.AddCondition(partyFollowLanternId, true))
	}

	p := parties.New(1)
	require.NotNil(t, p)
	require.True(t, p.InvitePlayer(2))
	require.True(t, p.AcceptInvite(2))

	leader.Character.ActionPoints = 100
	member.Character.ActionPoints = 100
	events.DrainQueuedUserInputsForTest(2)
	events.DrainQueuedMessagesForTest(2)
	return leader, member, room
}

// runQueuedInputs runs every input queued for userId, as the world's input
// loop would, and returns what the user was told.
func runQueuedInputs(t *testing.T, userId int) string {
	t.Helper()
	for _, in := range events.DrainQueuedUserInputsForTest(userId) {
		cmd, rest, _ := strings.Cut(in, " ")
		_, err := TryCommand(cmd, rest, userId, 0)
		require.NoError(t, err)
	}
	return strings.Join(events.DrainQueuedMessagesForTest(userId), "")
}

func TestPartyFollow_UnlitMemberFollowsALitLeader(t *testing.T) {
	leader, member, room := partyFollowScene(t, true)
	require.Empty(t, member.Character.Conditions.LightAndDarknessSources(), "fixture premise: the member carries no light")
	require.Equal(t, 0, rooms.LoadRoom(2).LightLevel(), "fixture premise: the cave is pitch dark")

	handled, err := Go("north", leader, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, leader.Character.RoomId)

	told := strings.Join(events.DrainQueuedMessagesForTest(2), "")
	require.Contains(t, told, "You follow the party leader.")
	runQueuedInputs(t, 2)
	require.Equal(t, 2, member.Character.RoomId, "an unlit member follows a leader who carries a light")
}

func TestPartyFollow_NobodyFollowsAnUnlitLeaderInTrueDarkness(t *testing.T) {
	leader, member, room := partyFollowScene(t, false)
	require.Equal(t, 0, room.LightLevel(), "fixture premise: pitch dark")

	handled, err := Go("north", leader, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 2, leader.Character.RoomId)

	told := strings.Join(events.DrainQueuedMessagesForTest(2), "")
	require.NotContains(t, told, "You follow the party leader.")
	require.Contains(t, told, PartyFollowLostLine)
	require.Empty(t, events.DrainQueuedUserInputsForTest(2), "no step is queued for the member")
	require.Equal(t, 1, member.Character.RoomId, "the member stays where they are")
}
