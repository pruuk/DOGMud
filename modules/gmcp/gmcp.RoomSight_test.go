package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// roomSightFixture seeds one sky-less room lit by exactly lamp (0 is pitch
// dark, 30 shapes, 60 faces), with an exit, a floor item, a mob (Grave
// Wight) and a second player (Kesh), and returns the viewer standing in it.
func roomSightFixture(t *testing.T, lamp int) *users.UserRecord {
	t.Helper()
	mudlog.SetupLogger(nil, ``, ``, false) // the mapper logs when it builds a zone
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default"},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		999981: {ItemId: 999981, Name: "Grey Pebble", Type: items.Object},
	}))
	zero := 0.0
	room := &rooms.Room{RoomId: 9720, Zone: "SightZone", Biome: "default", SkyLight: &zero,
		Title: "Cellar", Description: "A low cellar smelling of damp stone.",
		Exits: map[string]exit.RoomExit{"north": {RoomId: 9721}},
		Items: []items.Item{items.New(999981)}}
	if lamp > 0 {
		room.Lamp = rooms.LampPtr(lamp)
	}
	north := &rooms.Room{RoomId: 9721, Zone: "SightZone", Biome: "default"}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9720: room, 9721: north},
		map[string]*rooms.ZoneConfig{"SightZone": {Name: "SightZone", RoomId: 9720,
			RoomIds: map[int]struct{}{9720: {}, 9721: {}}}},
	))

	wight := &mobs.Mob{MobId: 1, InstanceId: 721, Zone: "SightZone"}
	wight.Character.Name = "Grave Wight"
	wight.Character.RoomId = 9720
	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "SightZone"}},
		map[int]*mobs.Mob{721: wight},
	))
	room.AddMob(721)

	viewer := users.NewTestUser(9722, "viewer", "Viewer", 97722)
	viewer.Character.RoomId = 9720
	kesh := users.NewTestUser(9723, "kesh", "Kesh", 97723)
	kesh.Character.RoomId = 9720
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9722: viewer, 9723: kesh}))
	room.AddPlayer(9722)
	room.AddPlayer(9723)
	return viewer
}

func roomInfoFor(t *testing.T, viewer *users.UserRecord) GMCPRoomModule_Payload {
	t.Helper()
	data, name := (&GMCPRoomModule{}).GetRoomNode(viewer, `Room.Info`)
	require.Equal(t, `Room.Info`, name)
	p, ok := data.(GMCPRoomModule_Payload)
	require.True(t, ok, "Room.Info payload is %T", data)
	return p
}

// #252: a viewer who sees nothing gets the room's identity (id, area,
// coordinates, so the map can place them) and nothing the light would show:
// no title, description, exits, items or occupants. Empty, not omitted, so a
// client merging payloads drops the last sighted reading (see Char.Enemies).
func TestRoomInfo_DarkViewerGetsNoSightedDetail(t *testing.T) {
	p := roomInfoFor(t, roomSightFixture(t, 0))
	require.Equal(t, 9720, p.Id)
	require.Equal(t, ``, p.Name)
	require.Equal(t, ``, p.Description)
	require.Empty(t, p.Exits)
	require.Empty(t, p.ExitsV2)
	require.NotNil(t, p.Exits, "exits must be an empty map, not omitted")
	require.Empty(t, p.Contents.Items)
	require.Empty(t, p.Contents.Players)
	require.Empty(t, p.Contents.Npcs)
}

// #252: at shapes the room reads (title, description, exits, items) but every
// occupant is the anonymous figure with no id or adjectives, as the room
// roster shows them.
func TestRoomInfo_ShapesViewerGetsFiguresNotNames(t *testing.T) {
	p := roomInfoFor(t, roomSightFixture(t, 30))
	require.Equal(t, `Cellar`, p.Name)
	require.NotEmpty(t, p.Description)
	require.Contains(t, p.Exits, `north`)
	require.Len(t, p.Contents.Items, 1)
	require.Len(t, p.Contents.Players, 1)
	require.Len(t, p.Contents.Npcs, 1)
	for _, c := range append(p.Contents.Players, p.Contents.Npcs...) {
		require.Equal(t, `a figure`, c.Name)
		require.Equal(t, ``, c.Id)
		require.Empty(t, c.Adjectives)
	}
}

// Clear sight is unchanged: names and ids as before.
func TestRoomInfo_SightedViewerGetsNames(t *testing.T) {
	p := roomInfoFor(t, roomSightFixture(t, 60))
	require.Equal(t, `Cellar`, p.Name)
	require.Len(t, p.Contents.Players, 1)
	require.Equal(t, `Kesh`, p.Contents.Players[0].Name)
	require.NotEqual(t, ``, p.Contents.Players[0].Id)
	require.Len(t, p.Contents.Npcs, 1)
	require.Equal(t, `Grave Wight`, p.Contents.Npcs[0].Name)
}

// The sub-module requests take the same gate: a client asking for only the
// players list in the dark gets an empty list.
func TestRoomInfoContentsPlayers_DarkViewerGetsNone(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	data, _ := (&GMCPRoomModule{}).GetRoomNode(viewer, `Room.Info.Contents.Players`)
	got, ok := data.([]GMCPRoomModule_Payload_Contents_Character)
	require.True(t, ok, "payload is %T", data)
	require.Empty(t, got)
}

// A band change maps the room the player now makes out (#252, R5): lighting a
// torch in a room entered in the dark puts it on the map.
func TestRoomSightBandChanged_MapsTheRoomOnceSeen(t *testing.T) {
	viewer := roomSightFixture(t, 30)
	require.False(t, viewer.Character.HasVisitedRoom("SightZone", 9720))
	(&GMCPRoomModule{}).sightBandChangedHandler(events.SightBandChanged{UserId: viewer.UserId})
	require.True(t, viewer.Character.HasVisitedRoom("SightZone", 9720))
}

// Still dark: the band changed but nothing is seen, so nothing is mapped.
func TestRoomSightBandChanged_DarkMapsNothing(t *testing.T) {
	viewer := roomSightFixture(t, 0)
	(&GMCPRoomModule{}).sightBandChangedHandler(events.SightBandChanged{UserId: viewer.UserId})
	require.False(t, viewer.Character.HasVisitedRoom("SightZone", 9720))
}

// A hidden container stays out of Room.Info until the viewer discovers it,
// the rule text look and search use (look.go, search_feature.go).
func TestRoomInfo_HiddenContainerListedOnlyOnceDiscovered(t *testing.T) {
	viewer := roomSightFixture(t, 60)
	room := rooms.LoadRoom(9720)
	room.Containers = map[string]rooms.Container{
		"crate":       {},
		"loose stone": {Hidden: true},
	}
	names := func() []string {
		out := []string{}
		for _, c := range roomInfoFor(t, viewer).Contents.Containers {
			out = append(out, c.Name)
		}
		return out
	}
	require.ElementsMatch(t, []string{"crate"}, names())

	viewer.Character.AddDiscovery(9720, "loose stone")
	require.ElementsMatch(t, []string{"crate", "loose stone"}, names())
}

// Room.Info's rosters use the text roster's rule, Character.Perceives
// (rooms.GetDetails): a hidden occupant is left out unless the viewer has
// see-hidden, and then it is listed.
func TestRoomInfo_SeeHiddenViewerListsAHiddenMob(t *testing.T) {
	const veilId, cloakId = 97291, 97292
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		veilId:  {ConditionId: veilId, Name: "Test Veil", Flags: []conditions.Flag{conditions.SeeHidden}},
		cloakId: {ConditionId: cloakId, Name: "Test Cloak", Flags: []conditions.Flag{conditions.Hidden}},
	}))
	viewer := roomSightFixture(t, 60)
	wight := mobs.GetInstance(721)
	wight.Character.Conditions = conditions.New()
	wight.Character.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "gmcp_room_sight_test"}
	require.NoError(t, wight.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	wight.Character.Awareness.ResolveConcealment(true, reason)
	require.NoError(t, wight.Character.AddCondition(cloakId, true))
	require.True(t, wight.Character.IsHidden(), "precondition: the wight is hidden")

	npcNames := func() []string {
		out := []string{}
		for _, c := range roomInfoFor(t, viewer).Contents.Npcs {
			out = append(out, c.Name)
		}
		return out
	}
	require.Empty(t, npcNames(), "no see-hidden: the hidden wight is not listed")

	require.NoError(t, viewer.Character.AddCondition(veilId, true))
	require.Equal(t, []string{"Grave Wight"}, npcNames(), "see-hidden lists the hidden wight")
}
