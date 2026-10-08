package main

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// TestJailedConditionNamesTheFineCommand is the second half of #298: the
// arrest line names `fine` once, at arrest. A prisoner who has scrolled past
// it finds the Jailed condition in `conditions`, whose description is the
// shipped condition 88's, so that description names `fine` too.
func TestJailedConditionNamesTheFineCommand(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	conditions.LoadDataFiles()

	spec := conditions.GetConditionSpec(88)
	if spec == nil || spec.Name != "Jailed" {
		t.Fatalf("condition 88 is %+v, want the shipped Jailed condition", spec)
	}
	if !spec.Listed() {
		t.Fatalf("Jailed is not listed, so `conditions` never shows its description")
	}
	desc := strings.Join(strings.Fields(spec.Description), " ")
	if !strings.Contains(desc, "Type fine to see what you owe.") {
		t.Errorf("Jailed description does not name the fine command: %q", desc)
	}
}

// TestJailCellsAreNeverPitchDark is the guard on #298: a prisoner could not
// even look around the cell, because every holding cell was a dungeon room
// with no lamp. 5107 is the template each arrest clones into a per-prisoner
// instance (internal/justice/arrest.go aCreateCellFn); 5105 and 5106 are the
// factions' static cells (holding_cell_room in factions/*.yaml), used when
// the instance cannot be made. Each carries a dim lamp of its own now.
//
// A normal-sighted prisoner must make out shapes (at or above
// LightBlindBelow) at every hour. The instanced cell has no sky at all and is
// also held below LightDimBelow: a cell is a dim room, not a lit one. The
// static cells keep their barred-slit sky (skylight 0.1), so only the floor
// is asserted there. The walk tries both sky extremes and both street-lamp
// states.
func TestJailCellsAreNeverPitchDark(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	// The real shipped config: a bare test binary uses Go default lighting
	// edges and walks the `default` fixture world.
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	lc := configs.GetLightingConfig()
	cells := []struct {
		roomId  int
		dimOnly bool
	}{
		{5107, true},  // instanced template: no sky, the lamp alone
		{5106, false}, // Stillwater static cell
		{5105, false}, // Thornwall static cell
	}
	for _, c := range cells {
		cell := rooms.LoadRoom(c.roomId)
		if cell == nil {
			t.Fatalf("room %d did not load: the walk is not seeing the world", c.roomId)
		}
		if cell.Lamp == nil {
			t.Errorf("room %d (%s) has no lamp: a prisoner reads pitch dark by night", cell.RoomId, cell.Title)
			continue
		}
		for _, celestial := range []float64{0, 100} {
			for _, lampsLit := range []bool{false, true} {
				level := cell.LightTermsAtForTest(celestial, lampsLit, 1).Level
				if level < lc.BlindBelow {
					t.Errorf("room %d celestial %v lampsLit %v: reads %d, below LightBlindBelow %d (pitch dark)",
						c.roomId, celestial, lampsLit, level, lc.BlindBelow)
				}
				if c.dimOnly && level >= lc.DimBelow {
					t.Errorf("room %d celestial %v lampsLit %v: reads %d, at or above LightDimBelow %d (faces, too bright for a cell)",
						c.roomId, celestial, lampsLit, level, lc.DimBelow)
				}
			}
		}
	}
}
