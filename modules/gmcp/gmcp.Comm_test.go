package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// sayFixture seeds one sky-less room lit by exactly lamp (0 is pitch dark, 30
// shapes, 60 faces) holding the speaker Kesh (9741), a listener Ana (9742), a
// deafened listener Bo (9743) and a mob, the Crier (instance 741).
func sayFixture(t *testing.T, lamp int) {
	t.Helper()
	zero := 0.0
	room := &rooms.Room{RoomId: 9740, Zone: "SayZone", SkyLight: &zero}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9740: room},
		map[string]*rooms.ZoneConfig{"SayZone": {Name: "SayZone", RoomId: 9740, RoomIds: map[int]struct{}{9740: {}}}},
	))
	crier := &mobs.Mob{MobId: 1, InstanceId: 741, Zone: "SayZone"}
	crier.Character.Name = "Crier"
	crier.Character.RoomId = 9740
	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "SayZone"}},
		map[int]*mobs.Mob{741: crier},
	))
	room.AddMob(741)

	kesh := users.NewTestUser(9741, "kesh", "Kesh", 97741)
	ana := users.NewTestUser(9742, "ana", "Ana", 97742)
	bo := users.NewTestUser(9743, "bo", "Bo", 97743)
	bo.Deafened = true
	for _, u := range []*users.UserRecord{kesh, ana, bo} {
		u.Character.RoomId = 9740
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9741: kesh, 9742: ana, 9743: bo}))
	room.AddPlayer(9741)
	room.AddPlayer(9742)
	room.AddPlayer(9743)
}

// sendersByUser runs sayDeliveries and maps each recipient to the Sender it reads.
func sendersByUser(evt events.Communication) map[int]string {
	got := map[int]string{}
	for _, d := range sayDeliveries(evt, GMCPCommModule_Payload{Channel: `say`, Sender: evt.Name}) {
		got[d.UserId] = d.Payload.Sender
	}
	return got
}

func keshSays() events.Communication {
	return events.Communication{SourceUserId: 9741, CommType: `say`, Name: `Kesh`, Message: `hello`}
}

// #252: the sender follows each listener's sight, as the text lane does; the
// deafened listener gets no player chatter; the speaker reads their own name.
func TestSayDeliveries_SenderFollowsListenerSight(t *testing.T) {
	cases := []struct {
		lamp int
		want string
	}{
		{60, `Kesh`},
		{30, `A figure`},
		{0, `Someone`},
	}
	for _, c := range cases {
		sayFixture(t, c.lamp)
		got := sendersByUser(keshSays())
		require.Equal(t, map[int]string{9741: `Kesh`, 9742: c.want}, got, "lamp %d", c.lamp)
	}
}

// A speaker still hidden after speaking is "Someone" to every listener, even
// in full light; the speaker still reads their own name.
func TestSayDeliveries_HiddenSpeakerIsSomeone(t *testing.T) {
	sayFixture(t, 60)
	evt := keshSays()
	evt.SpeakerHidden = true
	got := sendersByUser(evt)
	require.Equal(t, map[int]string{9741: `Kesh`, 9742: `Someone`}, got)
}

// An NPC's words are authored, so they reach a deafened listener too (ruling 6).
func TestSayDeliveries_MobSpeechReachesTheDeafened(t *testing.T) {
	sayFixture(t, 60)
	got := sendersByUser(events.Communication{SourceMobInstanceId: 741, CommType: `say`, Name: `Crier`, Message: `hear ye`})
	require.Equal(t, map[int]string{9741: `Crier`, 9742: `Crier`, 9743: `Crier`}, got)
}

// A mob's sayto to one player reaches only that player, not the room.
func TestSayDeliveries_TargetedSayReachesOnlyTheTarget(t *testing.T) {
	sayFixture(t, 60)
	got := sendersByUser(events.Communication{SourceMobInstanceId: 741, TargetUserId: 9742,
		CommType: `say`, Name: `Crier`, Message: `psst`, SpeakerHidden: true})
	require.Equal(t, map[int]string{9742: `Someone`}, got)
}
