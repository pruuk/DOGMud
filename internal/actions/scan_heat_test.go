package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/uuid"
	"github.com/stretchr/testify/require"
)

// Lighting plan 6, owner ruling O6: infravision shows the next room's
// occupants as shapes through an exit when the light here is too poor to see
// through it, down to minus the reach in the next room. Never a name, and
// never in place of a view the light grants.
const (
	scanHeatHereId  = 9494
	scanHeatThereId = 9495
	scanHeatScoutId = 9496
	scanHeatInfraId = 9497
)

// scanHeatSent scans from a cave lit at hereLamp into one lit at thereLamp
// and darkened by thereDark (0 for none), with or without infra reach 30.
func scanHeatSent(t *testing.T, hereLamp, thereLamp int, thereDark float64, infra bool) string {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		scanHeatInfraId: {ConditionId: scanHeatInfraId, Name: "Test Heat Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
	}))
	here := &rooms.Room{RoomId: scanHeatHereId, Zone: "ScanHeat", Biome: "cave", Lamp: rooms.LampPtr(hereLamp),
		Exits: map[string]exit.RoomExit{"north": {RoomId: scanHeatThereId}}}
	there := &rooms.Room{RoomId: scanHeatThereId, Zone: "ScanHeat", Biome: "cave", Lamp: rooms.LampPtr(thereLamp),
		Exits: map[string]exit.RoomExit{"south": {RoomId: scanHeatHereId}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{scanHeatHereId: here, scanHeatThereId: there},
		map[string]*rooms.ZoneConfig{"ScanHeat": {Name: "ScanHeat", RoomId: scanHeatHereId,
			RoomIds: map[int]struct{}{scanHeatHereId: {}, scanHeatThereId: {}}}},
	))
	t.Cleanup(itemlight.ResetForTest())
	if thereDark > 0 {
		itemlight.Set(scanHeatThereId, uuid.New(), itemlight.Darkness, thereDark)
	}
	scout := newScanTestMob(scanHeatScoutId, "Midroad Scout", scanHeatThereId)
	mobs.SetInstanceForTest(scanHeatScoutId, scout)
	t.Cleanup(func() { mobs.SetInstanceForTest(scanHeatScoutId, nil) })
	there.AddMob(scanHeatScoutId)

	actor := newScanFakeActor("Scanner", here, true, 9498)
	if infra {
		if !actor.char.Conditions.AddCondition(scanHeatInfraId, true) {
			t.Fatal("could not give the scanner infra reach")
		}
		if got := actor.char.InfraReach(); got != 30 {
			t.Fatalf("scanner infra reach = %d, want 30", got)
		}
	}
	Scan(actor, ScanOptions{})
	return strings.Join(actor.sent, "\n")
}

// The occupants heat shows through an exit are the ones the room roster
// would list (rooms.GetDetails: Character.Perceives, and a mob actually in
// the room). Fixture: north holds the visible Midroad Scout, a hidden mob, a
// stale listing (a mob listed there whose RoomId says it left) and a hidden
// player. A viewer with no see-hidden senses one figure; with see-hidden,
// three; the stale mob is never one. A blinded viewer senses nothing, and a
// viewer standing in an unlit cave who sees by heat still senses through.
const (
	scanHeatHiddenMobId  = 9481
	scanHeatStaleMobId   = 9482
	scanHeatSneakUserId  = 9483
	scanHeatVeilId       = 9484
	scanHeatViewerUserId = 9498
)

func scanOccupantScene(t *testing.T, hereLamp, thereLamp int) (*scanFakeActor, *rooms.Room) {
	t.Helper()
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		scanHeatInfraId: {ConditionId: scanHeatInfraId, Name: "Test Heat Sight", RoundInterval: 1, TriggerCount: 10,
			Flags:   []conditions.Flag{conditions.InfraredVision},
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}}},
		scanHeatVeilId: {ConditionId: scanHeatVeilId, Name: "Test Veil", RoundInterval: 1, TriggerCount: 10,
			Flags: []conditions.Flag{conditions.SeeHidden}},
	}))
	here := &rooms.Room{RoomId: scanHeatHereId, Zone: "ScanHeat", Biome: "cave", Lamp: rooms.LampPtr(hereLamp),
		Exits: map[string]exit.RoomExit{"north": {RoomId: scanHeatThereId}}}
	there := &rooms.Room{RoomId: scanHeatThereId, Zone: "ScanHeat", Biome: "cave", Lamp: rooms.LampPtr(thereLamp),
		Exits: map[string]exit.RoomExit{"south": {RoomId: scanHeatHereId}}}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{scanHeatHereId: here, scanHeatThereId: there},
		map[string]*rooms.ZoneConfig{"ScanHeat": {Name: "ScanHeat", RoomId: scanHeatHereId,
			RoomIds: map[int]struct{}{scanHeatHereId: {}, scanHeatThereId: {}}}},
	))
	t.Cleanup(itemlight.ResetForTest())

	hide := func(c *characters.Character) {
		if c.Awareness == nil {
			c.Awareness = awareness.NewMachine()
		}
		r := state.TransitionReason{Trigger: "scan_heat_test"}
		require.NoError(t, c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r))
		c.Awareness.ResolveConcealment(true, r)
		require.True(t, c.IsHidden(), "fixture: %s must be hidden", c.Name)
	}
	for _, m := range []struct {
		id     int
		name   string
		roomId int
		hidden bool
	}{
		{scanHeatScoutId, "Midroad Scout", scanHeatThereId, false},
		{scanHeatHiddenMobId, "Lurking Cutpurse", scanHeatThereId, true},
		{scanHeatStaleMobId, "Departed Pedlar", scanHeatHereId + 100, false},
	} {
		mob := newScanTestMob(m.id, m.name, m.roomId)
		if m.hidden {
			hide(&mob.Character)
		}
		mobs.SetInstanceForTest(m.id, mob)
		id := m.id
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		there.AddMob(m.id)
		// AddMob moves the mob here; a stale listing is one whose mob has
		// since gone, so its RoomId is set after.
		mob.Character.RoomId = m.roomId
	}
	sneak := users.NewTestUser(scanHeatSneakUserId, "kesh", "Kesh", 99483)
	hide(sneak.Character)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{scanHeatSneakUserId: sneak}))
	there.AddPlayer(scanHeatSneakUserId)

	actor := newScanFakeActor("Scanner", here, true, scanHeatViewerUserId)
	require.True(t, actor.char.Conditions.AddCondition(scanHeatInfraId, true), "fixture: infra reach")
	return actor, there
}

func TestScan_HeatAndLookCountTheRostersOccupants(t *testing.T) {
	for _, c := range []struct {
		name        string
		hereLamp    int
		seeHidden   bool
		blinded     bool
		wantFigures int
	}{
		{"too dark to see out, no see-hidden: the visible scout only", 30, false, false, 1},
		{"too dark to see out, see-hidden: the hidden mob and player too", 30, true, false, 3},
		{"standing in an unlit cave, seeing by heat: still senses through", 0, false, false, 1},
		{"blinded: nothing", 30, false, true, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			actor, there := scanOccupantScene(t, c.hereLamp, 30)
			if c.seeHidden {
				require.True(t, actor.char.Conditions.AddCondition(scanHeatVeilId, true))
			}
			if c.blinded {
				actor.char.Perception = characters.New().Perception
				require.NoError(t, actor.char.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
			}
			Scan(actor, ScanOptions{})
			sent := strings.Join(actor.sent, "\n")
			if got := strings.Count(sent, "a figure"); got != c.wantFigures {
				t.Errorf("scan shows %d figures, want %d: %q", got, c.wantFigures, sent)
			}
			for _, name := range []string{"Midroad Scout", "Lurking Cutpurse", "Departed Pedlar", "Kesh"} {
				if strings.Contains(sent, name) {
					t.Errorf("scan named %s through heat: %q", name, sent)
				}
			}
			if c.blinded && !strings.Contains(sent, "too dark to make anything out") {
				t.Errorf("a blinded scan should make nothing out: %q", sent)
			}

			// look <exit> asks the same two questions: may heat show this
			// room (LookAt's LookExitShapes), and who would it show.
			res := ResolveLook(actor, "north")
			if c.blinded {
				if res.Kind == LookExitShapes {
					t.Errorf("a blinded look got LookExitShapes")
				}
				return
			}
			if res.Kind != LookExitShapes {
				t.Fatalf("look north: Kind %v, want LookExitShapes", res.Kind)
			}
			if got := len(FiguresSensedIn(actor.char, there, scanHeatViewerUserId)); got != c.wantFigures {
				t.Errorf("look north senses %d figures, want %d", got, c.wantFigures)
			}
		})
	}
}

// With light enough to see out, scan names whom the roster would name: the
// hidden mob only to a viewer with see-hidden, the stale listing never.
func TestScan_NamesFollowTheRoster(t *testing.T) {
	for _, seeHidden := range []bool{false, true} {
		actor, _ := scanOccupantScene(t, 90, 90)
		if seeHidden {
			require.True(t, actor.char.Conditions.AddCondition(scanHeatVeilId, true))
		}
		Scan(actor, ScanOptions{})
		sent := strings.Join(actor.sent, "\n")
		require.Contains(t, sent, "Midroad Scout")
		require.NotContains(t, sent, "Departed Pedlar", "a stale listing is never named")
		if got := strings.Contains(sent, "Lurking Cutpurse"); got != seeHidden {
			t.Errorf("see-hidden %v: hidden mob named = %v: %q", seeHidden, got, sent)
		}
		if got := strings.Contains(sent, "Kesh"); got != seeHidden {
			t.Errorf("see-hidden %v: hidden player named = %v: %q", seeHidden, got, sent)
		}
	}
}

func TestScan_HeatShowsShapesThroughAnExit(t *testing.T) {
	cases := []struct {
		name            string
		here, there     int
		thereDark       float64
		infra           bool
		named, figure   bool
		tooDarkToMakeIt bool
	}{
		{"no infra, too dark to see out: nobody", 30, 30, 0, false, false, false, true},
		{"infra, too dark to see out: a figure by heat", 30, 30, 0, true, false, true, false},
		{"infra into an unlit cave: a figure by heat", 30, 0, 0, true, false, true, false},
		{"infra into darkness within reach: a figure", 30, 0, 20, true, false, true, false},
		{"infra into darkness beyond reach: nobody", 30, 0, 40, true, false, false, true},
		{"infra, light here sees out: the name, never downgraded", 90, 90, 0, true, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sent := scanHeatSent(t, c.here, c.there, c.thereDark, c.infra)
			if got := strings.Contains(sent, "Midroad Scout"); got != c.named {
				t.Errorf("named = %v, want %v: %q", got, c.named, sent)
			}
			if got := strings.Contains(sent, "a figure"); got != c.figure {
				t.Errorf("figure = %v, want %v: %q", got, c.figure, sent)
			}
			if got := strings.Contains(sent, "too dark to make anything out"); got != c.tooDarkToMakeIt {
				t.Errorf("too dark = %v, want %v: %q", got, c.tooDarkToMakeIt, sent)
			}
		})
	}
}
