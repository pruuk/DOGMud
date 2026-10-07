package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/uuid"
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
