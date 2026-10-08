package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #251: the structured ScanResult, which scout behaviour trees act on, follows
// the scanner's sight by the same rule the player's scan text uses
// (scanReach, then listedOccupants). A mob scanning out of a dark room, or
// into one, sees nobody; a hidden player is never a sighting. Under lighting
// 5d ruling D8 a mob acts on a shape, so a dim next room still counts.
const (
	scanMobHereId  = 9530
	scanMobThereId = 9531
	scanMobScoutId = 9532
	scanMobSelfId  = 9533
)

func scanMobSightings(t *testing.T, hereLamp, thereLamp int) ScanResult {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	here := &rooms.Room{RoomId: scanMobHereId, Zone: "ScanMob", Biome: "cave", Lamp: rooms.LampPtr(hereLamp),
		Exits: map[string]exit.RoomExit{"north": {RoomId: scanMobThereId}}}
	there := &rooms.Room{RoomId: scanMobThereId, Zone: "ScanMob", Biome: "cave", Lamp: rooms.LampPtr(thereLamp),
		Exits: map[string]exit.RoomExit{"south": {RoomId: scanMobHereId}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{scanMobHereId: here, scanMobThereId: there},
		map[string]*rooms.ZoneConfig{"ScanMob": {Name: "ScanMob", RoomId: scanMobHereId,
			RoomIds: map[int]struct{}{scanMobHereId: {}, scanMobThereId: {}}}},
	))
	scout := newScanTestMob(scanMobScoutId, "Midroad Scout", scanMobThereId)
	mobs.SetInstanceForTest(scanMobScoutId, scout)
	t.Cleanup(func() { mobs.SetInstanceForTest(scanMobScoutId, nil) })
	there.AddMob(scanMobScoutId)

	actor := newScanMobActor("Scanner", here, scanMobSelfId)
	return Scan(actor, ScanOptions{HostileOnly: true})
}

func TestScan_MobSightingsFollowTheScannersSight(t *testing.T) {
	for _, c := range []struct {
		name        string
		here, there int
		wantMobs    int
	}{
		{"both rooms bright: a sighting", 90, 90, 1},
		{"next room dim: a shape still counts (D8)", 90, 35, 1},
		{"next room dark: nobody", 90, -50, 0},
		{"own room too dark to see out: nobody", 10, 90, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			result := scanMobSightings(t, c.here, c.there)
			require.Len(t, result.Sightings, 1, "the exit is still listed")
			require.Len(t, result.Sightings[0].Mobs, c.wantMobs)
			require.Empty(t, result.Sightings[0].Players)
		})
	}
}

// The structured result lists whom the roster would list: no hidden player,
// no hidden mob, no stale listing. Fixture from scanOccupantScene: north holds
// the Midroad Scout, a hidden mob, a stale mob listing and the hidden Kesh.
func TestScan_MobSightingsSkipHiddenAndStale(t *testing.T) {
	actor, _ := scanOccupantScene(t, 90, 90)
	actor.isPlayer, actor.userId, actor.mobInstId = false, 0, scanMobSelfId
	actor.char.Conditions = conditions.New() // no infra, no see-hidden

	result := Scan(actor, ScanOptions{HostileOnly: true})
	require.Len(t, result.Sightings, 1)
	require.Empty(t, result.Sightings[0].Players, "a hidden player is never a scout's sighting")
	require.Len(t, result.Sightings[0].Mobs, 1)
	require.Equal(t, "Midroad Scout", result.Sightings[0].Mobs[0].Name)
}

// A heat-sensing scout standing where the light is too poor to see out still
// makes out the next room's occupants by heat, by the rule a player's scan
// uses (scanReach's SensesHeatThroughExit branch, lighting plan 6 ruling O6):
// whom FiguresSensedIn would show a player, and no hidden player. Without
// infra the same scout makes out nobody. Fixture from scanOccupantScene, both
// rooms lit at 30.
func TestScan_HeatSensingScoutSightsThroughTheDark(t *testing.T) {
	for _, infra := range []bool{true, false} {
		actor, there := scanOccupantScene(t, 30, 30)
		actor.isPlayer, actor.userId, actor.mobInstId = false, 0, scanMobSelfId
		if !infra {
			actor.char.Conditions = conditions.New()
		}
		require.Equal(t, infra, actor.char.InfraReach() > 0, "fixture: infra %v", infra)

		result := Scan(actor, ScanOptions{HostileOnly: true})
		require.Len(t, result.Sightings, 1, "the exit is still listed")
		require.Empty(t, result.Sightings[0].Players, "a hidden player is never a scout's sighting")
		if !infra {
			require.Empty(t, result.Sightings[0].Mobs, "no infra, too dark to see out: nobody")
			continue
		}
		require.Len(t, result.Sightings[0].Mobs, 1, "heat shows the scout the next room's creature")
		require.Equal(t, "Midroad Scout", result.Sightings[0].Mobs[0].Name)
		require.Len(t, FiguresSensedIn(actor.char, there, 0), 1,
			"the scout senses whom a player's heat would show")
	}
}
