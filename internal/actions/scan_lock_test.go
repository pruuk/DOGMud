package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// #427 (owner ruling 2026-10-07): a locked door blocks scan exactly as it
// blocks look, by sight and by heat. #428: scan names the next room's title
// only when the scanner sees out by light.
const (
	scanLockHereId  = 9470
	scanLockThereId = 9471
	scanLockScoutId = 9472
	scanLockInfraId = 9473
	scanLockUserId  = 9474
	scanLockTitle   = "Midroad Guardpost"
)

// scanLockScene seeds a cave lit at hereLamp whose north exit (locked or not)
// leads to a bright room holding the Midroad Scout, and returns a scanner
// standing in it, with infra reach 30 when infra is set.
func scanLockScene(t *testing.T, hereLamp int, locked, infra, isPlayer bool) *scanFakeActor {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		scanLockInfraId: {ConditionId: scanLockInfraId, Name: "Test Heat Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	north := exit.RoomExit{RoomId: scanLockThereId}
	if locked {
		north.Lock = gamelock.Lock{Difficulty: 5}
	}
	here := &rooms.Room{RoomId: scanLockHereId, Zone: "ScanLock", Biome: "cave", Lamp: rooms.LampPtr(hereLamp),
		Exits: map[string]exit.RoomExit{"north": north}}
	there := &rooms.Room{RoomId: scanLockThereId, Zone: "ScanLock", Biome: "cave", Lamp: rooms.LampPtr(90),
		Title: scanLockTitle,
		Exits: map[string]exit.RoomExit{"south": {RoomId: scanLockHereId}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{scanLockHereId: here, scanLockThereId: there},
		map[string]*rooms.ZoneConfig{"ScanLock": {Name: "ScanLock", RoomId: scanLockHereId,
			RoomIds: map[int]struct{}{scanLockHereId: {}, scanLockThereId: {}}}},
	))
	scout := newScanTestMob(scanLockScoutId, "Midroad Scout", scanLockThereId)
	mobs.SetInstanceForTest(scanLockScoutId, scout)
	t.Cleanup(func() { mobs.SetInstanceForTest(scanLockScoutId, nil) })
	there.AddMob(scanLockScoutId)

	var actor *scanFakeActor
	if isPlayer {
		actor = newScanFakeActor("Scanner", here, true, scanLockUserId)
	} else {
		actor = newScanMobActor("Scanner", here, scanLockScoutId+100)
	}
	if infra {
		require.True(t, actor.char.Conditions.AddCondition(scanLockInfraId, true), "fixture: infra reach")
	}
	return actor
}

func TestScan_LockedExitBlocksSightAndHeat(t *testing.T) {
	for _, c := range []struct {
		name     string
		hereLamp int
		infra    bool
		lookKind LookKind
	}{
		{"light sees out: locked, nobody named", 90, false, LookExitLocked},
		{"too dark to see out, heat reaches: locked, no figure", 30, true, LookExitLocked},
	} {
		t.Run(c.name, func(t *testing.T) {
			actor := scanLockScene(t, c.hereLamp, true, c.infra, true)
			result := Scan(actor, ScanOptions{})
			sent := strings.Join(actor.sent, "\n")
			require.NotContains(t, sent, "Midroad Scout")
			require.NotContains(t, sent, "a figure")
			require.NotContains(t, sent, scanLockTitle, "a locked exit shows no room title")
			require.Contains(t, sent, `<ansi fg="exit">north</ansi>: the exit is locked`)
			require.Len(t, result.Sightings, 1)
			require.True(t, result.Sightings[0].Locked)
			require.Empty(t, result.Sightings[0].Mobs)
			require.Empty(t, result.Sightings[0].Players)
			// look north reads the same exit as locked.
			require.Equal(t, c.lookKind, ResolveLook(actor, "north").Kind)
		})
	}
}

// With neither light nor heat reaching through, look calls a locked exit too
// dark (LookExitTooDark), and so does scan.
func TestScan_LockedExitInTheDarkIsTooDark(t *testing.T) {
	actor := scanLockScene(t, 30, true, false, true)
	Scan(actor, ScanOptions{})
	sent := strings.Join(actor.sent, "\n")
	require.NotContains(t, sent, "Midroad Scout")
	require.NotContains(t, sent, "locked")
	require.Contains(t, sent, "too dark to make anything out")
	require.Equal(t, LookExitTooDark, ResolveLook(actor, "north").Kind)
}

// A mob scanning through a locked door gets the direction, marked locked,
// and nobody behind it.
func TestScan_LockedExitHidesOccupantsFromMobs(t *testing.T) {
	actor := scanLockScene(t, 90, true, false, false)
	result := Scan(actor, ScanOptions{HostileOnly: true})
	require.Len(t, result.Sightings, 1)
	require.True(t, result.Sightings[0].Locked)
	require.Empty(t, result.Sightings[0].Mobs)
	require.Empty(t, result.Sightings[0].Players)

	open := scanLockScene(t, 90, false, false, false)
	result = Scan(open, ScanOptions{HostileOnly: true})
	require.Len(t, result.Sightings, 1)
	require.False(t, result.Sightings[0].Locked)
	require.Len(t, result.Sightings[0].Mobs, 1, "an open exit still shows the mob caller who is there")
}
