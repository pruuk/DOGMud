package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/mutators"
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

// giveKeenEars gives u the superhearing flag, so u hears quiet lines
// (Character.HearsQuiet).
func giveKeenEars(t *testing.T, u *users.UserRecord) {
	t.Helper()
	const keenEars = 97790
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		keenEars: {ConditionId: keenEars, Name: "Test Keen Ears", TriggerCount: 1000000,
			Flags: []conditions.Flag{conditions.SuperHearing}},
	}))
	require.NoError(t, u.Character.AddCondition(keenEars, true))
	require.True(t, u.Character.HearsQuiet(), "fixture: keen ears")
}

// readersOf is the text each of uids reads of the RoomId messages queued for
// roomId: every message that does not exclude them.
func readersOf(roomId int, uids ...int) map[int][]string {
	got := map[int][]string{}
	for _, m := range events.DrainQueuedRoomMessagesForTest(roomId) {
		for _, uid := range uids {
			if m.AddressedTo(uid) {
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
	giveKeenEars(t, mira) // so the quiet line is not skipped as unheard

	a.SendTextToExits("You hear someone talking.", true, 555)
	msgs := events.DrainQueuedRoomMessagesForTest(3291)
	for _, m := range msgs {
		require.True(t, m.IsQuiet, "isQuiet rides on both lines")
		require.Contains(t, m.ExcludeUserIds, 555, "the caller's excludes ride on both lines")
	}
	const walkIn = 97709 // not in the room when the sound was queued
	got := map[int][]string{}
	for _, m := range msgs {
		for _, uid := range []int{kesh.UserId, mira.UserId, walkIn} {
			if m.AddressedTo(uid) {
				got[uid] = append(got[uid], strings.TrimSpace(m.Text))
			}
		}
	}
	require.Equal(t, []string{`(From <ansi fg="exit">crack</ansi>) You hear someone talking.`}, got[kesh.UserId],
		"the listener who found the crack reads its name")
	require.Equal(t, []string{`(From nearby) You hear someone talking.`}, got[mira.UserId],
		"the listener who has not found it reads nearby")
	require.Equal(t, []string{`(From nearby) You hear someone talking.`}, got[walkIn],
		"a walk-in is addressed by the nearby line only (D43)")
	require.False(t, mira.Character.KnowsSecretExit(3291, "crack"), "hearing teaches nothing")
}

// An ordinary exit reads its name for everyone, in one message, as before.
func TestSendTextToExits_AnOrdinaryExitReadsItsNameForEveryone(t *testing.T) {
	a, _, kesh, mira := heardFixture(t, false)

	a.SendTextToExits("You hear someone talking.", false)
	got := readersOf(3291, kesh.UserId, mira.UserId)
	want := []string{`(From <ansi fg="exit">crack</ansi>) You hear someone talking.`}
	require.Equal(t, want, got[kesh.UserId])
	require.Equal(t, want, got[mira.UserId])
}

// The split itself takes the exit's own secret flag: knowers and strangers
// at a secret exit; nothing at an ordinary one, where no listener is looked
// at. HeardLines turns it into the lines to send.
func TestHeardThroughExit(t *testing.T) {
	_, b, kesh, mira := heardFixture(t, true)
	crack, ok := b.FirstExitTo(3290)
	require.True(t, ok)
	require.Equal(t, ExitTo{Key: "crack", Name: "crack", Secret: true}, crack)
	knowers, strangers, secret := b.HeardThroughExit(crack)
	require.True(t, secret)
	require.Equal(t, []int{kesh.UserId}, knowers)
	require.Equal(t, []int{mira.UserId}, strangers)
	require.Equal(t, []HeardLine{
		{Text: "NAMED", Only: []int{kesh.UserId}, Exclude: []int{9, mira.UserId}},
		{Text: "NEARBY", Exclude: []int{9, kesh.UserId}},
	}, b.HeardLines(crack, "NAMED", "NEARBY", []int{9}))

	_, b, _, _ = heardFixture(t, false)
	crack, _ = b.FirstExitTo(3290)
	knowers, strangers, secret = b.HeardThroughExit(crack)
	require.False(t, secret)
	require.Nil(t, knowers)
	require.Nil(t, strangers)
	require.Equal(t, []HeardLine{{Text: "NAMED", Exclude: []int{9}}}, b.HeardLines(crack, "NAMED", "NEARBY", []int{9}))
}

// The split runs on the exit's own flag, not on the printed name: a
// temporary exit back whose title happens to equal a secret exit's key is
// still an ordinary way back, heard by name by everyone.
func TestHeardLines_ATemporaryExitTitledLikeASecretExitIsOrdinary(t *testing.T) {
	a, b, kesh, mira := heardFixture(t, true)
	delete(b.Exits, "crack")
	b.Exits["crack"] = exit.RoomExit{RoomId: 3299, Secret: true} // a secret exit elsewhere
	b.ExitsTemp = map[string]exit.TemporaryRoomExit{"rift-9": {RoomId: 3290, Title: "crack"}}

	a.SendTextToExits("You hear someone talking.", false)
	got := readersOf(3291, kesh.UserId, mira.UserId)
	want := []string{`(From <ansi fg="exit">crack</ansi>) You hear someone talking.`}
	require.Equal(t, want, got[kesh.UserId])
	require.Equal(t, want, got[mira.UserId], "a temporary exit is never secret, whatever its title")
}

// A quiet line is skipped for a neighbour where no player hears quiet lines
// (Character.HearsQuiet, the delivery hook's rule): nothing is queued and no
// split is done. With a keen-eared listener there, it goes as before.
func TestSendTextToExits_AQuietLineSkipsARoomNobodyHears(t *testing.T) {
	a, _, kesh, _ := heardFixture(t, true)
	a.SendTextToExits("You hear someone talking.", true)
	require.Empty(t, events.DrainQueuedRoomMessagesForTest(3291), "nobody next door hears quiet lines")

	giveKeenEars(t, kesh)
	a.SendTextToExits("You hear someone talking.", true)
	require.NotEmpty(t, events.DrainQueuedRoomMessagesForTest(3291), "a keen-eared listener is there")

	a.SendTextToExits("A bell rings.", false)
	require.NotEmpty(t, events.DrainQueuedRoomMessagesForTest(3291), "a line that is not quiet always goes")
}

// #781: with a plain exit and a secret exit both leading back, every line
// names the plain one: FindExitTo ranks plain, then temporary, then secret,
// A to Z, whatever order the maps walk in. A room with two exits back reads
// one heard line.
func TestFindExitTo_APlainExitBackBeatsASecretOne(t *testing.T) {
	a, b, kesh, mira := heardFixture(t, true)
	b.Exits["south"] = exit.RoomExit{RoomId: 3290}
	b.Exits["alcove"] = exit.RoomExit{RoomId: 3290, Secret: true}
	for range 50 {
		require.Equal(t, "south", b.FindExitTo(3290))
	}
	require.Equal(t, []ExitTo{
		{Key: "south", Name: "south"},
		{Key: "alcove", Name: "alcove", Secret: true},
		{Key: "crack", Name: "crack", Secret: true},
	}, b.ExitsTo(3290))

	a.SendTextToExits("You hear someone talking.", false)
	got := readersOf(3291, kesh.UserId, mira.UserId)
	want := []string{`(From <ansi fg="exit">south</ansi>) You hear someone talking.`}
	require.Equal(t, want, got[kesh.UserId], "one line per room, naming the plain exit")
	require.Equal(t, want, got[mira.UserId])

	b.ExitsTemp = map[string]exit.TemporaryRoomExit{"rift-1": {RoomId: 3290, Title: "shimmering rift"}}
	delete(b.Exits, "south")
	require.Equal(t, "shimmering rift", b.FindExitTo(3290), "a temporary exit beats a secret one and prints its title")
}

// A mutator whose spec is unknown (a content typo) has a nil spec; the exit
// readers skip it rather than panic.
func TestExitReaders_SkipAMutatorWithNoSpec(t *testing.T) {
	here := &Room{RoomId: 3297, Zone: "SpeclessZone", Exits: map[string]exit.RoomExit{"north": {RoomId: 3298}},
		Mutators: mutators.MutatorList{{MutatorId: "no-such-mutator"}}}
	there := &Room{RoomId: 3298, Zone: "SpeclessZone", Exits: map[string]exit.RoomExit{"south": {RoomId: 3297}},
		Mutators: mutators.MutatorList{{MutatorId: "no-such-mutator"}}}
	t.Cleanup(SeedRoomsForTest(map[int]*Room{3297: here, 3298: there},
		map[string]*ZoneConfig{"SpeclessZone": {Name: "SpeclessZone", RoomId: 3297}}))
	n := 0
	for range here.ActiveMutators {
		n++
	}
	require.Equal(t, 1, n, "premise: the specless mutator is active")

	info, ok := here.GetExitInfo("north")
	require.True(t, ok)
	require.Equal(t, 3298, info.RoomId)
	visited := map[int]string{}
	here.ForEachAdjacentRoom(func(other *Room, back ExitTo) { visited[other.RoomId] = back.Name })
	require.Equal(t, map[int]string{3298: "south"}, visited)
}

// The fast path keeps ExitsTo's A to Z tie-break among plain exits.
func TestFindExitTo_PlainExitsTieBreakAToZ(t *testing.T) {
	r := &Room{RoomId: 3294, Exits: map[string]exit.RoomExit{
		"zigzag": {RoomId: 3295}, "stair": {RoomId: 3295}, "arch": {RoomId: 3295}, "north": {RoomId: 3296},
	}}
	for range 50 {
		require.Equal(t, "arch", r.FindExitTo(3295))
	}
	require.Equal(t, "arch", r.ExitsTo(3295)[0].Name, "the same answer as the ranked list")
}

// FindExitTo's fast path (plain room exits only) must step aside for an
// active mutator: a mutator west takes the place of the room's west, so the
// room's west leads nowhere and the way to the mutator's room is west.
func TestFindExitTo_AMutatorExitTakesThePlaceOfTheRoomExit(t *testing.T) {
	r, _ := knownExitsRoom(t, true)
	r.Exits["west"] = exit.RoomExit{RoomId: 5}
	require.Equal(t, "west", r.FindExitTo(5), "premise: before the mutator")
	t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
		MutatorId: "test-find-exit-to-wall",
		Exits:     map[string]exit.RoomExit{"west": {RoomId: 9}},
	}))
	require.True(t, GetZoneConfig("TestZone").Mutators.Add("test-find-exit-to-wall"), "fixture: the mutator attaches")
	require.Equal(t, "", r.FindExitTo(5), "the room's west is shadowed")
	require.Equal(t, "west", r.FindExitTo(9))
}

// FindKnownExitTo answers a temporary exit by its key, the word `go` and
// `shoot` match, never its title, so an NPC heading there or an archer
// shooting there can name it in a command (#781).
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
