package rooms

import (
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// heardFixture is a room a (3290) with a neighbour b (3291) whose exit back
// is "crack", secret when secret is true, and two listeners in b: Kesh
// (97701), who has found the crack, and Mira (97702), who has not.
func heardFixture(t *testing.T, secret bool) (a, b *Room, kesh, mira *users.UserRecord) {
	t.Helper()
	a = &Room{RoomId: 3290, Exits: map[string]exit.RoomExit{"north": {RoomId: 3291}}}
	b = &Room{RoomId: 3291, Exits: map[string]exit.RoomExit{"crack": {RoomId: 3290, Secret: secret}}}
	t.Cleanup(SeedRoomsForTest(map[int]*Room{3290: a, 3291: b}, map[string]*ZoneConfig{}))
	kesh = users.NewTestUser(97701, "kesh", "Kesh", 97701)
	mira = users.NewTestUser(97702, "mira", "Mira", 97702)
	kesh.Character.RoomId, mira.Character.RoomId = 3291, 3291
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{97701: kesh, 97702: mira}))
	b.AddPlayer(97701)
	b.AddPlayer(97702)
	kesh.Character.LearnSecretExit(3291, "crack")
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	events.DrainAllQueuedEventsForTest()
	return a, b, kesh, mira
}

// readersOf is the text each of uids reads of the RoomId messages queued for
// roomId: every message that does not exclude them.
func readersOf(roomId int, uids ...int) map[int][]string {
	got := map[int][]string{}
	for _, m := range events.DrainQueuedRoomMessagesForTest(roomId) {
		for _, uid := range uids {
			if !slices.Contains(m.ExcludeUserIds, uid) {
				got[uid] = append(got[uid], strings.TrimSpace(m.Text))
			}
		}
	}
	return got
}

// Owner ruling 16 (#781): a sound heard through a secret exit names it only
// to a listener who has found it; any other reads "(From nearby)". The
// existing exclude list rides on both lines.
func TestSendTextToExits_ASecretExitReadsNearbyToAStranger(t *testing.T) {
	a, _, kesh, mira := heardFixture(t, true)

	a.SendTextToExits("You hear someone talking.", true, 555)
	msgs := events.DrainQueuedRoomMessagesForTest(3291)
	for _, m := range msgs {
		require.True(t, m.IsQuiet, "isQuiet rides on both lines")
		require.Contains(t, m.ExcludeUserIds, 555, "the caller's excludes ride on both lines")
	}
	got := map[int][]string{}
	for _, m := range msgs {
		for _, uid := range []int{kesh.UserId, mira.UserId} {
			if !slices.Contains(m.ExcludeUserIds, uid) {
				got[uid] = append(got[uid], strings.TrimSpace(m.Text))
			}
		}
	}
	require.Equal(t, []string{`(From <ansi fg="exit">crack</ansi>) You hear someone talking.`}, got[kesh.UserId],
		"the listener who found the crack reads its name")
	require.Equal(t, []string{`(From nearby) You hear someone talking.`}, got[mira.UserId],
		"the listener who has not found it reads nearby")
	require.False(t, mira.Character.KnowsSecretExit(3291, "crack"), "hearing teaches nothing")
}

// An ordinary exit reads its name for everyone, in one message, as before.
func TestSendTextToExits_AnOrdinaryExitReadsItsNameForEveryone(t *testing.T) {
	a, _, kesh, mira := heardFixture(t, false)

	a.SendTextToExits("You hear someone talking.", true)
	got := readersOf(3291, kesh.UserId, mira.UserId)
	want := []string{`(From <ansi fg="exit">crack</ansi>) You hear someone talking.`}
	require.Equal(t, want, got[kesh.UserId])
	require.Equal(t, want, got[mira.UserId])
}

// HeardExitName is the per-listener form (the GMCP shout's sender): the name
// for a knower or an ordinary exit, nearby for a stranger at a secret one.
func TestHeardExitName(t *testing.T) {
	_, b, kesh, mira := heardFixture(t, true)
	require.Equal(t, "crack", b.HeardExitName(kesh.Character, "crack"))
	require.Equal(t, HeardFromNearby, b.HeardExitName(mira.Character, "crack"))

	_, b, kesh, mira = heardFixture(t, false)
	require.Equal(t, "crack", b.HeardExitName(kesh.Character, "crack"))
	require.Equal(t, "crack", b.HeardExitName(mira.Character, "crack"))
}

// FindKnownExitTo answers a temporary exit by its key, the word `go` and
// `shoot` match, never its title (#781 Task 5c, the archer's line of fire).
func TestFindKnownExitTo_ATemporaryExitAnswersItsKey(t *testing.T) {
	t.Cleanup(keywords.SeedKeywordsForTest())
	r := &Room{RoomId: 3292, ExitsTemp: map[string]exit.TemporaryRoomExit{
		"glade-7": {RoomId: 3293, Title: "glade-7 rift"},
	}}
	require.Equal(t, "glade-7", r.FindKnownExitTo(nil, 3293))
	name, roomId := r.FindKnownExitByName(nil, "glade-7")
	require.Equal(t, 3293, roomId, "premise: the key is what a typed exit word matches")
	require.NotEmpty(t, name)
}
