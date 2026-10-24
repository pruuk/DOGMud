package guards

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var updateDaycycle = flag.Bool("update-lighting-daycycle", false,
	"re-record testdata/lighting_daycycle.golden")

// sampleRounds picks 12 rounds spanning the year and the day. RoundsPerDay is
// pinned to 900 below, so round = (dayOfYear-1)*900 + hour*37.5.
//
// Recorded BEFORE the celestial model exists, so the baseline shows what the
// old model did at each of these moments. A diff here after Task 8 is the
// readout of the new model, not a regression.
func sampleRounds() []struct {
	Label string
	Round uint64
} {
	type s = struct {
		Label string
		Round uint64
	}
	out := []s{}
	for _, d := range []struct {
		name string
		doy  int
	}{{"midwinter", 356}, {"equinox", 81}, {"midsummer", 172}} {
		for _, h := range []struct {
			name string
			hour float64
		}{{"midnight", 0}, {"dawn", 6}, {"noon", 12}, {"dusk", 18}} {
			// 37.5 = RoundsPerDay(900) / 24 hours per round-day. Exact only
			// when h.hour is even; an odd hour (e.g. 3am -> 112.5) would
			// truncate silently via the uint64 conversion below. Keep every
			// sampled hour even, or round explicitly.
			r := uint64(float64(d.doy-1)*900 + h.hour*37.5)
			out = append(out, s{Label: d.name + "-" + h.name, Round: r})
		}
	}
	return out
}

// TestLightingDayCycleAcrossSampleRounds is the pre-change baseline for
// graded lighting plan 3a, the celestial mechanism. It records, for every
// shipped room, the light level the UNMODIFIED tree produces at twelve
// moments spanning the year (midwinter/equinox/midsummer) and the day
// (midnight/dawn/noon/dusk).
//
// The parity golden (lighting_parity_golden_test.go) is behaviour-preserving
// and must stay byte identical. This one is the opposite: plan 3a is a
// deliberate world-wide change, so this golden is expected to move on
// nearly every room once the celestial model lands. Recording it now, against
// the unmodified tree, is what makes that later diff a readout of what the
// new model did rather than an unverifiable re-record.
func TestLightingDayCycleAcrossSampleRounds(t *testing.T) {
	// "LOW" suppresses the very noisy per-file loader logging; without any
	// logger set up at all, the loaders panic on a nil *slog.Logger.
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// Read the REAL shipped config first (mirrors
	// lighting_parity_golden_test.go): a bare test binary's configs.GetConfig()
	// returns Go zero-value defaults, and FilePaths.DataFiles defaults to
	// `_datafiles/world/default`, not the shipped `_datafiles/world/dogmud`
	// world this MUD actually ships. Without the reload, room loading walks
	// the wrong world entirely (confirmed live: it panics on a non-canonical
	// room title that only exists in the `default` fixture world).
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	// Now pin Timing explicitly on top of the real config.
	// TRAP: a test binary loads Go defaults, where NightHours is 0 and
	// IsNight() can never be true. Without this pin the baseline records a
	// world with no night in it at all.
	cfg := configs.GetConfig()
	cfg.Timing.RoundsPerDay = 900
	cfg.Timing.NightHours = 8
	cfg.Timing.RoundSeconds = 4
	cfg.Timing.Validate()
	configs.SetConfigForTest(t, cfg)

	// Order matches main.go's own boot sequence (main.go:1631, 1636, 1643,
	// 1862): biomes, then rooms, then conditions, then mutators last.
	// Mutators load after rooms in main.go too, because Room.LightLevel()
	// -> ActiveMutators() is only ever called once content is fully up, and
	// LightLevel() itself dereferences the mutator registry
	// unconditionally (internal/rooms/lighting.go:87-88).
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()
	conditions.LoadDataFiles()
	mutators.LoadDataFiles()

	originalRound := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCount(originalRound) })

	ids := rooms.GetAllRoomIds()
	if len(ids) < 1000 {
		t.Fatalf("loaded only %d rooms: the walk is not seeing the world, so a green run proves nothing", len(ids))
	}
	sort.Ints(ids)

	var b strings.Builder
	skipped := 0
	skippedIDs := map[int]bool{}
	for _, s := range sampleRounds() {
		util.SetRoundCount(s.Round)
		fmt.Fprintf(&b, "== %s (round %d)\n", s.Label, s.Round)
		for _, id := range ids {
			r := rooms.LoadRoom(id)
			if r == nil {
				// rooms.LoadRoom can return nil (failed LoadRoomInstance, or
				// a lost addRoomToMemory race). A silent `continue` here
				// would let a mass load failure shrink the golden with no
				// signal, since the recorded byte count would still clear
				// the >= 1000 sanity floor above. See
				// lighting_parity_golden_test.go's identical trap.
				skipped++
				skippedIDs[id] = true
				continue
			}
			bi := r.GetBiome()
			biomeName := ``
			if bi != nil {
				biomeName = bi.BiomeId
			}
			fmt.Fprintf(&b, "room %d biome=%s light=%d\n", id, biomeName, r.LightLevel())
		}
	}
	if skipped > 0 {
		failedIDs := make([]int, 0, len(skippedIDs))
		for id := range skippedIDs {
			failedIDs = append(failedIDs, id)
		}
		sort.Ints(failedIDs)
		t.Fatalf("%d room loads failed across %d sample rounds (room ids: %v): the golden would silently lose coverage for those rooms", skipped, len(sampleRounds()), failedIDs)
	}
	got := b.String()

	goldenPath := testdataPath("lighting_daycycle.golden")
	if *updateDaycycle {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("recorded %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (record it with -update-lighting-daycycle)", err)
	}
	if got != string(want) {
		t.Errorf("day-cycle golden moved.\n\n" +
			"This golden is NOT a never-move guard. Plan 3a changes what rooms are lit " +
			"on purpose, and moves this file several times on the way: Task 5 (night " +
			"length follows the latitude), Task 6 (biomes declare a sky fraction) and " +
			"Task 8 (LightLevel composes the real model).\n\n" +
			"What it guards is that every move is EXPLAINED. Before re-recording, work " +
			"out which sample sections should have changed and by how many rooms, and " +
			"check the diff against that prediction. A move you cannot account for " +
			"room-for-room is a defect, however plausible the totals look.\n\n" +
			"Task 5's move, for reference, was exactly 391 rooms from 70 to 60 in four " +
			"sections (midwinter and equinox dawn and dusk), all six midnights and " +
			"noons unchanged, all four midsummer sections unchanged. 391 is the " +
			"neutral-biome count.\n\n" +
			"If you have done that and the shape is right:\n" +
			"  go test ./internal/guards/ -run TestLightingDayCycleAcrossSampleRounds -update-lighting-daycycle -v")
	}
}
