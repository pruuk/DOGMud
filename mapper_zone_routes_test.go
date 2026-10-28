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
//   - each (from, to, length) triple in the golden gives a path no longer than
//     the recorded one, and a recorded failure (-1) still fails.
//
// The golden, testdata/mapper_route_lengths.golden, was recorded from
// master's mapper (8534cf6a1, whose internal/mapper is master's: one map
// crawling 1,209 rooms across 38 zones). It is a regression baseline, not a
// snapshot of the current tree. It stores explicit triples, so a content PR
// that adds or removes rooms leaves the other triples standing; a triple
// whose room no longer exists is skipped and the count is logged.
//
// It may be re-recorded only with a stated reason, which the write mode
// requires and puts in the golden's header:
//
//	DOGMUD_BOOT_SMOKE=1 DOGMUD_ROUTE_GOLDEN_WRITE="<reason>" go test . -run TestShippedRoutesSurviveZoneBoundedMaps
//
// Never re-record it to make a failure pass: a longer route or a new failure
// is the regression this test exists to catch.

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

// routeTriple is one golden line: GetPath(from, to) took length steps on the
// recording tree (-1: no path).
type routeTriple struct{ from, to, length int }

// routeComparison is the outcome of holding the current tree to the golden.
type routeComparison struct {
	same, shorter, fails, skipped int
	problems                      []string
}

// compareRouteGolden holds length (the current tree's GetPath step count, -1
// on failure) to each golden triple whose rooms both still exist: no longer
// than recorded, and failing exactly where the recording failed.
func compareRouteGolden(golden []routeTriple, exists func(roomId int) bool, length func(from, to int) int) routeComparison {
	var c routeComparison
	for _, g := range golden {
		if !exists(g.from) || !exists(g.to) {
			c.skipped++
			continue
		}
		got := length(g.from, g.to)
		switch {
		case g.length == -1 && got == -1:
			c.fails++
		case g.length == -1:
			c.problems = append(c.problems, fmt.Sprintf("%d -> %d: the recording had no path, now %d steps", g.from, g.to, got))
		case got == -1:
			c.problems = append(c.problems, fmt.Sprintf("%d -> %d: the recording pathed in %d steps, now no path", g.from, g.to, g.length))
		case got > g.length:
			c.problems = append(c.problems, fmt.Sprintf("%d -> %d: %d steps, longer than the recorded %d", g.from, g.to, got, g.length))
		case got < g.length:
			c.shorter++
		default:
			c.same++
		}
	}
	return c
}

// readRouteGolden reads the golden's triples, skipping blank and # lines.
func readRouteGolden(path string) ([]routeTriple, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []routeTriple
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == `` || strings.HasPrefix(line, `#`) {
			continue
		}
		var g routeTriple
		if _, err := fmt.Sscan(line, &g.from, &g.to, &g.length); err != nil {
			return nil, fmt.Errorf("golden line %q: %w", line, err)
		}
		out = append(out, g)
	}
	return out, sc.Err()
}

// The comparison skips a triple whose room is gone, so a content PR that
// removes or renumbers rooms does not fail it, and still catches a longer
// route and a changed failure on the triples that remain.
func TestCompareRouteGolden(t *testing.T) {
	golden := []routeTriple{
		{1, 2, 5},  // same
		{1, 3, 5},  // shorter now
		{1, 4, -1}, // fails on both
		{1, 9, 5},  // room 9 is gone: skipped
		{9, 1, -1}, // room 9 is gone: skipped
		{2, 3, 4},  // longer now
		{2, 4, -1}, // paths now
		{3, 4, 6},  // fails now
	}
	now := map[[2]int]int{{1, 2}: 5, {1, 3}: 4, {1, 4}: -1, {2, 3}: 7, {2, 4}: 3, {3, 4}: -1}
	exists := func(id int) bool { return id != 9 }
	length := func(a, b int) int {
		n, ok := now[[2]int{a, b}]
		if !ok {
			t.Fatalf("length asked for %d -> %d, a skipped or unknown pair", a, b)
		}
		return n
	}
	c := compareRouteGolden(golden, exists, length)
	if c.same != 1 || c.shorter != 1 || c.fails != 1 || c.skipped != 2 || len(c.problems) != 3 {
		t.Errorf("got same %d, shorter %d, fails %d, skipped %d, problems %q; want 1, 1, 1, 2 and three problems",
			c.same, c.shorter, c.fails, c.skipped, c.problems)
	}
}

// routePairs is the seeded sample the write mode records: 200 ordered pairs
// of distinct template rooms, drawn from every room id in ascending order.
// Only the write mode uses it; the check reads the triples from the golden.
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

	// The consistency checks see across borders again (#375 phase 4 Task
	// 20); shipped data has none, as on master, or boot (panic mode in
	// config.yaml) would not come up.
	for _, zone := range rooms.GetAllZoneNames() {
		for _, f := range mapper.CheckZone(zone) {
			t.Errorf("consistency finding on shipped data: %s", f)
		}
	}

	for _, leg := range crossZoneLegs {
		za, zb := rooms.TemplateRoomZone(leg[0]), rooms.TemplateRoomZone(leg[1])
		if za == `` || za == zb {
			t.Errorf("leg %d -> %d is no longer cross-zone (%q -> %q); update crossZoneLegs", leg[0], leg[1], za, zb)
		}
		if _, err := mapper.GetPath(leg[0], leg[1]); err != nil {
			t.Errorf("cross-zone leg %d (%s) -> %d (%s) does not path: %v", leg[0], za, leg[1], zb, err)
		}
	}

	if reason := strings.TrimSpace(os.Getenv(`DOGMUD_ROUTE_GOLDEN_WRITE`)); reason != `` {
		if reason == `1` {
			t.Fatalf("DOGMUD_ROUTE_GOLDEN_WRITE must state the reason for re-recording, not 1")
		}
		var b strings.Builder
		b.WriteString("# from to length (-1 = no path); GetPath on shipped data, see mapper_zone_routes_test.go\n")
		fmt.Fprintf(&b, "# re-recorded: %s\n", reason)
		for _, p := range routePairs() {
			fmt.Fprintf(&b, "%d %d %d\n", p[0], p[1], routeLength(p[0], p[1]))
		}
		if err := os.MkdirAll(filepath.Dir(routeGoldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(routeGoldenPath, []byte(b.String()), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", routeGoldenPath)
		return
	}

	golden, err := readRouteGolden(routeGoldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	exists := func(id int) bool { return rooms.LoadRoom(id) != nil }
	c := compareRouteGolden(golden, exists, routeLength)
	for _, p := range c.problems {
		t.Error(p)
	}
	if c.skipped > 0 {
		t.Logf("skipped %d golden triples whose rooms no longer exist", c.skipped)
	}
	t.Logf("route lengths vs the recording: %d same, %d shorter, %d unreachable on both, %d skipped (of %d)",
		c.same, c.shorter, c.fails, c.skipped, len(golden))
	t.Logf("cross-zone legs checked: %d", len(crossZoneLegs))
}
