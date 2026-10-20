package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func installMarkerDialogue(t *testing.T) {
	t.Helper()
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(`mobid: 1
zone: SightZone
topics:
  - key: secret
    kind: info
    label: "Tell me a secret."
    text: "Under the altar."
    marked: true
`), &df))
	t.Cleanup(dialogue.InstallForTest(&df))
}

func npcMarkers(t *testing.T, viewerFixtureLamp int) ([]string, []string) {
	t.Helper()
	viewer := roomSightFixture(t, viewerFixtureLamp)
	var gm, roster []string
	for _, c := range roomInfoFor(t, viewer).Contents.Npcs {
		gm = append(gm, c.Marker)
	}
	roster = rooms.GetDetails(rooms.LoadRoom(9720), viewer).VisibleMobs
	return gm, roster
}

func TestRoomInfo_NpcCarriesTheViewersTalkMarker(t *testing.T) {
	installMarkerDialogue(t)
	require.NotNil(t, rooms.NpcMarker, "internal/conversation installs the hook")

	viewer := roomSightFixture(t, 60)
	npcs := roomInfoFor(t, viewer).Contents.Npcs
	require.Len(t, npcs, 1)
	require.Equal(t, "[*]", npcs[0].Marker)

	viewer.Character.MarkHeard("1.secret")
	npcs = roomInfoFor(t, viewer).Contents.Npcs
	require.Len(t, npcs, 1)
	require.Equal(t, "", npcs[0].Marker)
}

func TestRoomInfo_ShapesSightGivesNoMarker(t *testing.T) {
	installMarkerDialogue(t)
	gm, roster := npcMarkers(t, 30) // lamp 30 is the shapes band
	require.Len(t, gm, 1, "precondition: the occupant is listed, as a figure")
	require.Equal(t, "", gm[0])
	for _, r := range roster {
		require.NotContains(t, r, "[*]")
	}
}

func TestMarker_HiddenMobGetsNoMarkerInRosterOrGMCP(t *testing.T) {
	const cloakId = 97293
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		cloakId: {ConditionId: cloakId, Name: "Test Cloak", Flags: []conditions.Flag{conditions.Hidden}},
	}))
	installMarkerDialogue(t)
	viewer := roomSightFixture(t, 60)
	wight := mobs.GetInstance(721)

	// Positive control: unhidden, the marker shows in both.
	require.Equal(t, "[*]", conversation.Marker(viewer.UserId, wight))
	require.Contains(t, rooms.GetDetails(rooms.LoadRoom(9720), viewer).VisibleMobs[0], "[*]")
	require.Len(t, roomInfoFor(t, viewer).Contents.Npcs, 1)

	wight.Character.Conditions = conditions.New()
	wight.Character.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "gmcp_room_marker_test"}
	require.NoError(t, wight.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason))
	wight.Character.Awareness.ResolveConcealment(true, reason)
	require.NoError(t, wight.Character.AddCondition(cloakId, true))
	require.True(t, wight.Character.IsHidden(), "precondition: the wight is hidden")
	require.False(t, viewer.Character.Perceives(&wight.Character))

	require.Empty(t, rooms.GetDetails(rooms.LoadRoom(9720), viewer).VisibleMobs)
	require.Empty(t, roomInfoFor(t, viewer).Contents.Npcs)
}

func TestMarker_CharmedMobAndMissingViewerGetNoMarker(t *testing.T) {
	installMarkerDialogue(t)
	viewer := roomSightFixture(t, 60)
	wight := mobs.GetInstance(721)
	require.Equal(t, "[*]", conversation.Marker(viewer.UserId, wight), "positive control")

	require.Equal(t, "", conversation.Marker(424242, wight), "no such user")
	require.Equal(t, "", conversation.Marker(viewer.UserId, nil), "nil mob")

	wight.Character.Charmed = &characters.CharmInfo{UserId: viewer.UserId}
	require.True(t, wight.Character.IsCharmed(), "precondition")
	require.Equal(t, "", conversation.Marker(viewer.UserId, wight))
}

func TestRefreshMarkers_QueuesTheNpcListUpdate(t *testing.T) {
	events.DrainQueuedNpcMarkersChangedForTest(0)
	conversation.RefreshMarkers(0)
	require.Empty(t, events.DrainQueuedNpcMarkersChangedForTest(0), "no user, no event")

	conversation.RefreshMarkers(77)
	require.Equal(t, []events.NpcMarkersChanged{{UserId: 77}}, events.DrainQueuedNpcMarkersChangedForTest(0))

	// The module answers a GMCP user with the Npcs sub-update.
	events.DrainQueuedNpcMarkersChangedForTest(0)
	const connId = uint64(97701)
	u := users.NewTestUser(77, "markers", "Markers", connId)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{77: u})()
	gmcpModule.cache.Add(connId, GMCPSettings{GMCPAccepted: true, HelloReceived: true})
	defer gmcpModule.cache.Remove(connId)
	events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)

	res := (&GMCPRoomModule{}).npcMarkersChangedHandler(events.NpcMarkersChanged{UserId: 77})
	require.Equal(t, events.Continue, res)
	require.Equal(t, []events.Event{GMCPRoomUpdate{UserId: 77, Identifier: `Room.Info.Contents.Npcs`}},
		events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`))
}

// A player without GMCP gets no Room update queued for them.
func TestRefreshMarkers_NoGMCPQueuesNothing(t *testing.T) {
	u := users.NewTestUser(78, "plain", "Plain", 97801)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{78: u})()
	gmcpModule.cache.Remove(uint64(97801))
	events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`)

	res := (&GMCPRoomModule{}).npcMarkersChangedHandler(events.NpcMarkersChanged{UserId: 78})
	require.Equal(t, events.Continue, res)
	require.Empty(t, events.DrainQueuedByTypeForTest(`GMCPRoomUpdate`))
}
