package guards

import (
	"sort"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// nonCartesianAllowed lists the zones that may carry `non_cartesian: true`
// in their zone-config, each with its reason. The flag switches off the boot
// map check (collisions, one-way returns, exit deltas), so it is only for a
// zone whose space is folded or moving on purpose, never for a zone that was
// laid out loosely. A zone that can be drawn on a grid is checked like any
// other. The 2026-07-30 audit (memory project_cross_zone_mapping_plan) set
// the first list; the Crash Site Interior left it 2026-10-24 when its two
// overlaps were fixed (owner reported the Drift Vault and the Sealed Shuttle
// Bay sharing one spot).
var nonCartesianAllowed = map[string]string{
	"The Foldweave":            "folded space by design",
	"Endless Trashheap":        "the loot goblin's yard, not player exploration",
	"Ferries":                  "moving transport rooms",
	"Instance Planar Oasis":    "transient instance",
	"Labyrinth of Low Tunnels": "audit: FIX, two bad exits (not yet straightened)",
	"Newcomer Antechamber":     "audit: FIX, already Cartesian (not yet unflagged)",
}

func loadWorldRooms(t *testing.T) []string {
	t.Helper()
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	zones := rooms.GetAllZoneNames()
	if len(zones) < 20 {
		t.Fatalf("only %d zones loaded: the test is not seeing the world", len(zones))
	}
	sort.Strings(zones)
	return zones
}

// TestNonCartesianFlagOnlyWhereAllowed fails when a zone carries the
// non_cartesian flag without an entry in nonCartesianAllowed.
func TestNonCartesianFlagOnlyWhereAllowed(t *testing.T) {
	zones := loadWorldRooms(t)

	flagged := 0
	for _, z := range zones {
		if !rooms.IsZoneNonCartesian(z) {
			continue
		}
		flagged++
		if _, ok := nonCartesianAllowed[z]; !ok {
			t.Errorf("zone %q is flagged non_cartesian, which turns off its boot map check; "+
				"fix its layout instead, or add it to nonCartesianAllowed with the reason its space is folded or moving", z)
		}
	}
	if flagged == 0 {
		t.Fatalf("no zone reads as non_cartesian: the probe cannot see the flag")
	}
}

// TestEveryZoneRootIsInItsZone: the boot map check crawls each zone from
// rooms.GetZoneRoot. Before 2026-10-24 an instanced zone (which names its
// first room as `entry_room`, not `roomid`) resolved to room 0, so the check
// crawled the wrong map and never saw the zone at all; that is how the Crash
// Site Interior's overlaps went unflagged.
func TestEveryZoneRootIsInItsZone(t *testing.T) {
	zones := loadWorldRooms(t)

	for _, z := range zones {
		root, err := rooms.GetZoneRoot(z)
		if err != nil {
			t.Errorf("GetZoneRoot(%q): %v", z, err)
			continue
		}
		room := rooms.LoadRoom(root)
		if room == nil {
			t.Errorf("zone %q: root room %d does not load; the boot map check cannot crawl this zone", z, root)
			continue
		}
		if room.Zone != z {
			t.Errorf("zone %q: root room %d belongs to zone %q; the boot map check crawls the wrong map", z, root, room.Zone)
		}
	}
}

// TestCrashSiteInteriorIsCartesian pins the 2026-10-24 fix: the ship's two
// decks map onto one grid with no two rooms on the same spot.
func TestCrashSiteInteriorIsCartesian(t *testing.T) {
	loadWorldRooms(t)

	const zone = "Crash Site Interior"
	root, err := rooms.GetZoneRoot(zone)
	if err != nil {
		t.Fatalf("GetZoneRoot(%q): %v", zone, err)
	}
	m := mapper.GetMapper(root)
	if m == nil {
		t.Fatalf("no mapper for room %d", root)
	}
	if !m.HasRoom(6400) || !m.HasRoom(6401) {
		t.Fatalf("the crawl from room %d does not reach the upper deck: the check would see nothing", root)
	}
	for _, f := range mapper.FilterFindingsToZone(m.CheckConsistency(zone, false), zone) {
		if f.Severity == "error" {
			t.Errorf("map consistency: %s", f)
		}
	}
}
