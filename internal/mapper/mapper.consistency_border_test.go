package mapper

import (
	"fmt"
	"strings"
	"testing"
)

// Zone maps stop at their border (#375 phase 4, ruling 18), so the
// consistency checks a single map of the connected world made across zones
// now read the far side from its own zone's map (Task 20). Fixture ids
// 9801-9825 are used only here.

// findingsOf is CheckZone for each zone, as "zone kind room exit" lines.
func findingsOf(zones ...string) string {
	var out []string
	for _, z := range zones {
		for _, f := range CheckZone(z) {
			out = append(out, fmt.Sprintf("%s %s %d %s", f.Zone, f.Kind, f.RoomId, f.ExitName))
		}
	}
	return strings.Join(out, "; ")
}

func buildMaps(ids ...int) {
	for _, id := range ids {
		GetMapper(id)
	}
}

// A border exit with no way back is reported, as when one map held both
// rooms; before Task 20 the check skipped any exit whose far room the map did
// not hold.
func TestCheckZone_CrossZoneExitWithoutReturn(t *testing.T) {
	isolateMapperCaches(t)
	writeFixtureWorld(t, []fixtureRoom{
		{id: 9801, x: 0, zone: "zc_a", exits: map[string]int{"east": 9802}},
		{id: 9802, x: 1, zone: "zc_b", exits: map[string]int{"east": 9803}},
		{id: 9803, x: 2, zone: "zc_b", exits: map[string]int{"west": 9802}},
	})
	buildMaps(9801, 9802)
	if got, want := findingsOf("zc_a", "zc_b"), "zc_a noreciprocal 9801 east"; got != want {
		t.Fatalf("findings = %q, want %q", got, want)
	}
}

// A border exit whose far room does not sit where its direction says.
func TestCheckZone_CrossZoneDeltaMismatch(t *testing.T) {
	isolateMapperCaches(t)
	writeFixtureWorld(t, []fixtureRoom{
		{id: 9805, x: 0, zone: "zc_c", exits: map[string]int{"east": 9806}},
		{id: 9806, x: 3, zone: "zc_d", exits: map[string]int{"west": 9805}},
	})
	buildMaps(9805, 9806)
	want := "zc_c deltamismatch 9805 east; zc_d deltamismatch 9806 west"
	if got := findingsOf("zc_c", "zc_d"); got != want {
		t.Fatalf("findings = %q, want %q", got, want)
	}
}

// Two zones joined by exits, with a room of each in one cell: reported once,
// under the zone of the lower room id, as one map's collision group was.
//
//	9813 (zc_f, 1,-1)
//	9810 9811|9812 9814   9811 (zc_e) and 9812 (zc_f) both at (1,0)
func TestCheckZone_CrossZoneCollision(t *testing.T) {
	isolateMapperCaches(t)
	writeFixtureWorld(t, []fixtureRoom{
		{id: 9810, x: 0, zone: "zc_e", exits: map[string]int{"east": 9811}},
		{id: 9811, x: 1, zone: "zc_e", exits: map[string]int{"west": 9810, "north": 9813}},
		{id: 9813, x: 1, y: -1, zone: "zc_f", exits: map[string]int{"south": 9811, "southeast": 9814}},
		{id: 9814, x: 2, zone: "zc_f", exits: map[string]int{"northwest": 9813, "west": 9812}},
		{id: 9812, x: 1, zone: "zc_f", exits: map[string]int{"east": 9814}},
	})
	buildMaps(9810, 9813)
	if got, want := findingsOf("zc_e", "zc_f"), "zc_e collision 9811 "; got != want {
		t.Fatalf("findings = %q, want %q", got, want)
	}
}

// A room no exit joins to the others was never in their map, so sharing a
// cell with one of them is not a finding (shipped: the jail cell 5107,
// unplaced at the origin, and Dustwalk Road 400).
func TestCheckZone_UnjoinedRoomInTheSameCellIsNotACollision(t *testing.T) {
	isolateMapperCaches(t)
	writeFixtureWorld(t, []fixtureRoom{
		{id: 9820, x: 0, zone: "zc_g", exits: map[string]int{"east": 9821}},
		{id: 9821, x: 1, zone: "zc_h", exits: map[string]int{"west": 9820}},
		{id: 9822, x: 0, zone: "zc_i", exits: map[string]int{}},
	})
	buildMaps(9820, 9821, 9822)
	if got := findingsOf("zc_g", "zc_h", "zc_i"); got != "" {
		t.Fatalf("findings = %q, want none", got)
	}
}
