package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Zone maps stop at their zone's border and GetPath searches across maps
// (#375 phase 4, spec ruling 18, section 10). The split changes cost, not
// behaviour, so on shipped data:
//
//   - every cross-zone patrol and schedule leg still paths (boot validators
//     panic otherwise, so a failure here is a server that will not boot), and
//   - 200 seeded room pairs give a path no longer than master's, and exactly
//     master's set of unreachable pairs.
//
// The master lengths live in testdata/mapper_route_lengths.golden, recorded
// on 8534cf6a1, whose internal/mapper is master's (one map crawling 1,209
// rooms across 38 zones), with
//
//	DOGMUD_BOOT_SMOKE=1 DOGMUD_ROUTE_GOLDEN_WRITE=1 go test . -run TestShippedRoutesSurviveZoneBoundedMaps
//
// Do not re-record it to make a failure pass: a longer route or a new
// failure is the regression this test exists to catch.

const routeGoldenPath = `testdata/mapper_route_lengths.golden`

const routeGoldenPairs = 200

// crossZoneLegs are the shipped patrol and schedule legs whose ends lie in
// different zones (traced 2026-10-28): patrols caravan_thornwall_stillwater
// (4 legs), marsh_forager_delivery, np_docks_runner_circuit (5 legs),
// steppe_forager_delivery; schedule np_crafting_halvard (2 legs).
var crossZoneLegs = [][2]int{
	{4038, 465}, {4038, 4109}, {4109, 4038}, {465, 4038},
	{4143, 4198},
	{5903, 5506}, {5506, 5709}, {5717, 5802}, {5812, 6010}, {6010, 5903},
	{483, 3040},
	{5605, 5709}, {5709, 5605},
}

// routePairs is the seeded sample: 200 ordered pairs of distinct template
// rooms, drawn from every room id in ascending order.
func routePairs() [][2]int {
	all := rooms.GetAllRoomIds()
	sort.Ints(all)
	rng := rand.New(rand.NewSource(375))
	out := make([][2]int, 0, routeGoldenPairs)
	for len(out) < routeGoldenPairs {
		a, b := all[rng.Intn(len(all))], all[rng.Intn(len(all))]
		if a == b {
			continue
		}
		out = append(out, [2]int{a, b})
	}
	return out
}

// routeLength is the step count of GetPath(a, b), or -1 when it fails.
func routeLength(a, b int) int {
	p, err := mapper.GetPath(a, b)
	if err != nil {
		return -1
	}
	return len(p)
}

func TestShippedRoutesSurviveZoneBoundedMaps(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the shipped-route comparison (~40s)", bootSmokeEnvVar)
	}
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	loadAllDataFiles(false)
	mapper.PreCacheMaps()

	for _, leg := range crossZoneLegs {
		za, zb := rooms.TemplateRoomZone(leg[0]), rooms.TemplateRoomZone(leg[1])
		if za == `` || za == zb {
			t.Errorf("leg %d -> %d is no longer cross-zone (%q -> %q); update crossZoneLegs", leg[0], leg[1], za, zb)
		}
		if _, err := mapper.GetPath(leg[0], leg[1]); err != nil {
			t.Errorf("cross-zone leg %d (%s) -> %d (%s) does not path: %v", leg[0], za, leg[1], zb, err)
		}
	}

	pairs := routePairs()

	if os.Getenv(`DOGMUD_ROUTE_GOLDEN_WRITE`) != `` {
		var b strings.Builder
		b.WriteString("# from to length (-1 = no path); GetPath on shipped data, see mapper_zone_routes_test.go\n")
		for _, p := range pairs {
			fmt.Fprintf(&b, "%d %d %d\n", p[0], p[1], routeLength(p[0], p[1]))
		}
		if err := os.MkdirAll(filepath.Dir(routeGoldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(routeGoldenPath, []byte(b.String()), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d pairs)", routeGoldenPath, len(pairs))
		return
	}

	f, err := os.Open(routeGoldenPath)
	if err != nil {
		t.Fatalf("open golden: %v", err)
	}
	defer f.Close()
	want := map[[2]int]int{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == `` || strings.HasPrefix(line, `#`) {
			continue
		}
		var a, b, n int
		if _, err := fmt.Sscan(line, &a, &b, &n); err != nil {
			t.Fatalf("golden line %q: %v", line, err)
		}
		want[[2]int{a, b}] = n
	}

	same, shorter, fails := 0, 0, 0
	for _, p := range pairs {
		w, ok := want[p]
		if !ok {
			t.Fatalf("pair %d -> %d is not in the golden; the room set changed, so the sample did too", p[0], p[1])
		}
		got := routeLength(p[0], p[1])
		switch {
		case w == -1 && got == -1:
			fails++
		case w == -1:
			t.Errorf("%d -> %d: master had no path, now %d steps", p[0], p[1], got)
		case got == -1:
			t.Errorf("%d -> %d: master pathed in %d steps, now no path", p[0], p[1], w)
		case got > w:
			t.Errorf("%d -> %d: %d steps, longer than master's %d", p[0], p[1], got, w)
		case got < w:
			shorter++
		default:
			same++
		}
	}
	t.Logf("route lengths vs master: %d same, %d shorter, %d unreachable on both (of %d)",
		same, shorter, fails, len(pairs))
	t.Logf("cross-zone legs checked: %d", len(crossZoneLegs))
}
