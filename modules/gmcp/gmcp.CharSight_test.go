package gmcp

import (
	"encoding/json"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// sightViewer seeds one sky-less room lit by exactly its lamp (0 for an unlit
// cave) and one viewer standing in it.
func sightViewer(t *testing.T, lamp int) *users.UserRecord {
	t.Helper()
	zero := 0.0
	room := &rooms.Room{RoomId: 9710, Zone: "SightZone", SkyLight: &zero}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9710: room},
		map[string]*rooms.ZoneConfig{"SightZone": {Name: "SightZone", RoomId: 9710, RoomIds: map[int]struct{}{9710: {}}}},
	))
	viewer := users.NewTestUser(9711, "sighted", "Sighted", 97711)
	viewer.Character.RoomId = 9710
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9711: viewer}))
	room.AddPlayer(9711)
	return viewer
}

// Char.Sight carries only the player's own band, as messaging.Band names it
// (lighting plan 5d, ruling D7).
func TestCharSightCarriesThePlayersBand(t *testing.T) {
	cases := []struct {
		lamp int
		want string
	}{
		{0, "dark"},
		{30, "shapes"},
		{60, "faces"},
		{90, "dazzled"},
	}
	g := &GMCPCharModule{}
	for _, c := range cases {
		viewer := sightViewer(t, c.lamp)
		data, moduleName := g.GetCharNode(viewer, `Char.Sight`)
		require.Equal(t, `Char.Sight`, moduleName)
		sight, ok := data.(*GMCPCharModule_Payload_Sight)
		require.True(t, ok, "Char.Sight payload must be *GMCPCharModule_Payload_Sight, got %T", data)
		require.Equal(t, c.want, sight.Band, "lamp %d", c.lamp)

		raw, err := json.Marshal(sight)
		require.NoError(t, err)
		require.JSONEq(t, `{"band":"`+c.want+`"}`, string(raw), "Char.Sight must be exactly one field")
	}
}

// A viewer whose room is not loaded reads "faces", LightBand's answer for a
// nil room, and does not panic: sightBand must hand LightBand an untyped nil,
// never a nil *rooms.Room (plan fact F25).
func TestCharSightWithNoLoadedRoom(t *testing.T) {
	viewer := sightViewer(t, 60)
	viewer.Character.RoomId = 9799 // not seeded
	data, _ := (&GMCPCharModule{}).GetCharNode(viewer, `Char.Sight`)
	sight, ok := data.(*GMCPCharModule_Payload_Sight)
	require.True(t, ok, "Char.Sight payload type %T", data)
	require.Equal(t, "faces", sight.Band)
}

// A full Char push carries Sight too, so a client that logs in or refreshes
// draws the border without waiting for a band change.
func TestFullCharPayloadCarriesSight(t *testing.T) {
	viewer := sightViewer(t, 60)
	data, moduleName := (&GMCPCharModule{}).GetCharNode(viewer, `Char`)
	require.Equal(t, `Char`, moduleName)
	payload, ok := data.(GMCPCharModule_Payload)
	require.True(t, ok, "Char payload type %T", data)
	require.NotNil(t, payload.Sight)
	require.Equal(t, "faces", payload.Sight.Band)
}

// The SightBandChanged event becomes one Char.Sight update for that player.
func TestSightBandChangedQueuesCharSight(t *testing.T) {
	events.ProcessEvents() // drop anything queued before the capture
	var got []GMCPCharUpdate
	id := events.RegisterListener(GMCPCharUpdate{}, func(e events.Event) events.ListenerReturn {
		if u, ok := e.(GMCPCharUpdate); ok && u.UserId == 9712 {
			got = append(got, u)
		}
		return events.Continue
	})
	t.Cleanup(func() { events.UnregisterListener(GMCPCharUpdate{}, id) })

	(&GMCPCharModule{}).sightBandChangedHandler(events.SightBandChanged{UserId: 9712})
	events.ProcessEvents()
	require.Len(t, got, 1)
	require.Equal(t, `Char.Sight`, got[0].Identifier)
}
