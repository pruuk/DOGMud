package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const (
	followMiraId    = 8951
	followBrannocId = 8952
	followWitnessId = 8953
	followRoomId    = 8970
)

var followTag = regexp.MustCompile(`<[^>]*>`)

func followScene(t *testing.T, lamp int) (mira, brannoc *users.UserRecord, room *rooms.Room) {
	t.Helper()
	people := map[int]*users.UserRecord{
		followMiraId:    users.NewTestUser(followMiraId, "mira", "Mira", 98951),
		followBrannocId: users.NewTestUser(followBrannocId, "brannoc", "Brannoc", 98952),
		followWitnessId: users.NewTestUser(followWitnessId, "wren", "Wren", 98953),
	}
	t.Cleanup(users.SeedUsersForTest(people))
	room = &rooms.Room{RoomId: followRoomId, SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(lamp)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{followRoomId: room}, map[string]*rooms.ZoneConfig{}))
	for id, u := range people {
		u.Character.RoomId = followRoomId
		room.AddPlayer(id)
	}
	for id := range people {
		events.DrainQueuedMessageEventsForTest(id)
	}
	return people[followMiraId], people[followBrannocId], room
}

func followRead(uid int) []string {
	var out []string
	for _, m := range events.DrainQueuedMessageEventsForTest(uid) {
		out = append(out, strings.TrimSpace(followTag.ReplaceAllString(m.Text, "")))
	}
	return out
}

func TestBreakOffMenuToFollow_ClosesAndSpeaksThreeLines(t *testing.T) {
	mira, brannoc, room := followScene(t, 60)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(mira.Character, room))
	_, ok := mira.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}},
		menu.Session{Topic: "talk with Kael", TopicNames: []string{"Kael"}})
	require.True(t, ok)
	events.DrainQueuedMessageEventsForTest(followMiraId) // the menu render

	BreakOffMenuToFollow(mira, brannoc.Character, followBrannocId, room)

	require.False(t, mira.HasOpenMenu())
	require.Equal(t, []string{"You break off your talk with Kael to follow Brannoc."}, followRead(followMiraId))
	require.Equal(t, []string{"Mira breaks off a talk with Kael to follow you."}, followRead(followBrannocId))
	require.Equal(t, []string{"Mira breaks off a talk with Kael and follows Brannoc."}, followRead(followWitnessId))
}

// The leader has already left the room when the lines are spoken, and a mover
// is seen by their own light on the way out (owner rule 2026-10-09): in a dark
// room a lantern-carrying leader is named to an unlit follower, to the leader
// reading the follower, and to an unlit witness.
func TestBreakOffMenuToFollow_LitLeaderIsNamedInADarkRoom(t *testing.T) {
	const lanternId = 8979
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		lanternId: {ConditionId: lanternId, Name: "Test Lantern", TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 54}}},
	}))
	mira, brannoc, room := followScene(t, 0)
	require.True(t, brannoc.Character.Conditions.AddCondition(lanternId, true))
	room.RemovePlayer(followBrannocId) // the leader has already walked out
	require.NotEqual(t, messaging.SightFull, messaging.ParticipantSight(mira.Character, room),
		"fixture premise: the room alone is dark to the follower")
	_, ok := mira.OpenMenu(menu.Menu{Entries: []menu.Entry{{Id: "a", Label: "A"}}}, menu.Session{})
	require.True(t, ok)
	for _, id := range []int{followMiraId, followBrannocId, followWitnessId} {
		events.DrainQueuedMessageEventsForTest(id)
	}

	BreakOffMenuToFollow(mira, brannoc.Character, followBrannocId, room)

	require.Equal(t, []string{"You stop what you were doing to follow Brannoc."}, followRead(followMiraId))
	require.Equal(t, []string{"Mira stops what they were doing to follow you."}, followRead(followBrannocId))
	require.Equal(t, []string{"Mira stops what they were doing and follows Brannoc."}, followRead(followWitnessId))
}

func TestBreakOffMenuToFollow_NothingOpenSaysNothing(t *testing.T) {
	mira, brannoc, room := followScene(t, 60)

	BreakOffMenuToFollow(mira, brannoc.Character, followBrannocId, room)

	require.Empty(t, followRead(followMiraId))
	require.Empty(t, followRead(followBrannocId))
	require.Empty(t, followRead(followWitnessId))
	require.Empty(t, events.DrainQueuedRoomMessagesForTest(followRoomId))
}
