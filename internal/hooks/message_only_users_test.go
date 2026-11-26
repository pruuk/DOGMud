package hooks

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #781 D43: events.Message.OnlyUserIds is the allow-list of a RoomId
// message, honoured by Message_SendMessage's RoomId branch.

const (
	onlyRoom, onlyNext                 = 4810, 4811
	onlyKnower, onlyWalkIn, onlyAbsent = 4821, 4822, 4823
	onlyHearCond                       = 4830
)

// onlyFixture seeds room 4810 (north to 4811, whose way back, "crack", is
// secret) and three players with captured screens: Kesh (4821) in 4811, who
// found the crack; Wren (4822) and Abe (4823) in 4810.
func onlyFixture(t *testing.T) map[int]*screenCapture {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	events.DrainAllQueuedEventsForTest()
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		onlyHearCond: {ConditionId: onlyHearCond, Name: "Test Keen Ears", TriggerCount: 1000000,
			Flags: []conditions.Flag{conditions.SuperHearing}},
	}))
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		onlyRoom: {RoomId: onlyRoom, Exits: map[string]exit.RoomExit{"north": {RoomId: onlyNext}}},
		onlyNext: {RoomId: onlyNext, Exits: map[string]exit.RoomExit{"crack": {RoomId: onlyRoom, Secret: true}}},
	}, map[string]*rooms.ZoneConfig{}))
	screens := map[int]*screenCapture{}
	seeded := map[int]*users.UserRecord{}
	for uid, name := range map[int]string{onlyKnower: "Kesh", onlyWalkIn: "Wren", onlyAbsent: "Abe"} {
		connId, sc := captureConnection(t)
		u := users.NewTestUser(uid, strings.ToLower(name), name, connId)
		seeded[uid], screens[uid] = u, sc
	}
	t.Cleanup(users.SeedUsersForTest(seeded))
	place := func(uid, roomId int) {
		users.GetByUserId(uid).Character.RoomId = roomId
		rooms.LoadRoom(roomId).AddPlayer(uid)
	}
	place(onlyKnower, onlyNext)
	place(onlyWalkIn, onlyRoom)
	place(onlyAbsent, onlyRoom)
	users.GetByUserId(onlyKnower).Character.LearnSecretExit(onlyNext, "crack")
	return screens
}

// markRound numbers the markers, so a second delivery waits for its own.
var markRound int

// deliver runs every queued RoomId message for roomId through the hook, then
// a marker to each of uids, so a negative check can wait for the marker.
func deliver(roomId int, uids ...int) {
	markRound++
	for _, m := range events.DrainQueuedRoomMessagesForTest(roomId) {
		Message_SendMessage(m)
	}
	for _, uid := range uids {
		Message_SendMessage(events.Message{UserId: uid, Text: fmt.Sprintf("MARK-%d-%d\n", markRound, uid)})
	}
}

// screenAfterMark waits for uid's marker from the last delivery and returns
// the screen up to it.
func screenAfterMark(t *testing.T, sc *screenCapture, uid int) string {
	t.Helper()
	mark := fmt.Sprintf("MARK-%d-%d", markRound, uid)
	require.Eventually(t, func() bool { return strings.Contains(sc.bytes(), mark) },
		time.Second, 5*time.Millisecond, "screen %q", sc.bytes())
	return sc.bytes()
}

// A stranger who walks in between queueing and delivery is in no allow-list:
// they read "(From nearby)" only, never the secret exit's name.
func TestMessageOnlyUserIds_AWalkInReadsOnlyNearby(t *testing.T) {
	screens := onlyFixture(t)
	next := rooms.LoadRoom(onlyNext)
	// A stranger already there when the sound is queued, so both halves go.
	stranger := onlyAbsent
	rooms.LoadRoom(onlyRoom).RemovePlayer(stranger)
	users.GetByUserId(stranger).Character.RoomId = onlyNext
	next.AddPlayer(stranger)

	rooms.LoadRoom(onlyRoom).SendTextToExits("You hear someone talking.", false)

	// Wren walks in before delivery.
	rooms.LoadRoom(onlyRoom).RemovePlayer(onlyWalkIn)
	users.GetByUserId(onlyWalkIn).Character.RoomId = onlyNext
	next.AddPlayer(onlyWalkIn)
	deliver(onlyNext, onlyKnower, onlyWalkIn, stranger)

	walkIn := screenAfterMark(t, screens[onlyWalkIn], onlyWalkIn)
	require.Contains(t, walkIn, "(From nearby) You hear someone talking.")
	require.NotContains(t, walkIn, "crack", "a walk-in never reads the secret exit's name")
	require.Contains(t, screenAfterMark(t, screens[onlyKnower], onlyKnower), "crack", "the knower reads the name")
	require.NotContains(t, screenAfterMark(t, screens[stranger], stranger), "crack")
}

// The allow-list narrows; it never widens. A listed player who is not in the
// room reads nothing, and IsQuiet still needs keen hearing.
func TestMessageOnlyUserIds_KeepsTheRoomAndHearingChecks(t *testing.T) {
	screens := onlyFixture(t)
	only := []int{onlyWalkIn, onlyKnower} // Kesh is listed but in 4811

	Message_SendMessage(events.Message{RoomId: onlyRoom, Text: "LOUD\n", OnlyUserIds: only})
	Message_SendMessage(events.Message{RoomId: onlyRoom, Text: "QUIET\n", OnlyUserIds: only, IsQuiet: true})
	deliver(0, onlyWalkIn, onlyKnower, onlyAbsent)
	wren := screenAfterMark(t, screens[onlyWalkIn], onlyWalkIn)
	require.Contains(t, wren, "LOUD", "listed and in the room")
	require.NotContains(t, wren, "QUIET", "listed, but IsQuiet needs keen hearing")
	require.NotContains(t, screenAfterMark(t, screens[onlyKnower], onlyKnower), "LOUD", "listed, but not in the room")
	require.NotContains(t, screenAfterMark(t, screens[onlyAbsent], onlyAbsent), "LOUD", "in the room, but not listed")

	require.NoError(t, users.GetByUserId(onlyWalkIn).Character.AddCondition(onlyHearCond, true))
	require.True(t, users.GetByUserId(onlyWalkIn).Character.HasConditionFlag(conditions.SuperHearing), "fixture: keen ears")
	Message_SendMessage(events.Message{RoomId: onlyRoom, Text: "WHISPERED\n", OnlyUserIds: only, IsQuiet: true})
	deliver(0, onlyWalkIn)
	require.Contains(t, screenAfterMark(t, screens[onlyWalkIn], onlyWalkIn), "WHISPERED", "keen hearing and listed")
}

// An empty allow-list is the old message: every player in the room but the
// excluded.
func TestMessageOnlyUserIds_EmptyIsEveryoneAsBefore(t *testing.T) {
	screens := onlyFixture(t)
	Message_SendMessage(events.Message{RoomId: onlyRoom, Text: "BELL\n", ExcludeUserIds: []int{onlyAbsent}})
	deliver(0, onlyWalkIn, onlyAbsent, onlyKnower)
	require.Contains(t, screenAfterMark(t, screens[onlyWalkIn], onlyWalkIn), "BELL")
	require.NotContains(t, screenAfterMark(t, screens[onlyAbsent], onlyAbsent), "BELL", "excluded")
	require.NotContains(t, screenAfterMark(t, screens[onlyKnower], onlyKnower), "BELL", "not in the room")
}
