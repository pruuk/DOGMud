package mobs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
)

// TestSomeShippedMobSensesHeat is the dead-surface guard for Graded Lighting
// Plan 2 Task 8. Before this task, nothing in shipped content granted
// condition 85 (InfraredVision), so Character.InfraReach() was reachable in
// code but never actually nonzero for anything a player could meet. Sentry
// Drift (mob 9555, Crash Site Interior) now carries `conditionids: [85]`,
// and this test proves that grant survives the real spawn path
// (mobs.NewMobByIdFresh -> Character.Validate -> reapplyPermanentConditions
// -> AddCondition(85, true)) rather than only existing as an unread YAML key.
//
// It reads Sentry Drift's REAL shipped YAML file rather than a synthetic
// fixture, because the thing under test is that the shipped author-facing
// key actually reaches the runtime character, not that the mechanism can be
// exercised in the abstract.
//
// Proven capable of failing: run with the mob's `conditionids: [85]` line
// removed (or with the assertion's mob id pointed at one that never had the
// grant), and this test reports InfraReach() == 0.
func TestSomeShippedMobSensesHeat(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// configs.ReloadConfig() and fileloader both resolve "_datafiles/..."
	// relative to the process CWD, which a shared test binary does not
	// reliably set to this package's directory (dogmud-writing-tests: "CWD
	// is not the package dir"). Anchor on this file's own path and chdir to
	// the repo root for the duration of the test, restoring it after.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("os.Chdir(%q): %v", repoRoot, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// Read the REAL shipped config, matching internal/guards/lighting_parity_golden_test.go's
	// setup: a bare test binary never reads config.yaml, so condition 0
	// (Meditating)'s TriggerCount would come back as the Go default 0 and
	// conditions.LoadDataFiles would panic validating it.
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	// conditions.LoadDataFiles() and species.LoadDataFiles() both replace
	// their package's entire map with no restore of their own (unlike
	// configs.SetConfigForTest above). Left alone, loading the real shipped
	// data here would permanently overwrite whatever minimal test fixtures
	// other tests in this shared binary seeded via
	// species.SeedSpeciesForTest / conditions.SeedConditionsForTest, leaking
	// forward into every test that runs after this one. Both seed helpers
	// capture the map that was live at the moment they are called and
	// restore exactly that on cleanup, so calling them with a throwaway
	// value first (immediately overwritten by the real Load below) captures
	// the pre-test snapshot for a genuine restore.
	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))

	conditions.LoadDataFiles()
	species.LoadDataFiles()

	const sentryDriftId = 9555
	dataPath := string(configs.GetFilePathsConfig().DataFiles) + `/mobs/crash_site_interior`
	tmpMobs, err := fileloader.LoadAllFlatFiles[int, *Mob](dataPath)
	if err != nil {
		t.Fatalf("loading shipped crash_site_interior mob files: %v", err)
	}

	spec, ok := tmpMobs[sentryDriftId]
	if !ok {
		t.Fatalf("shipped mob %d (Sentry Drift) not found under %s", sentryDriftId, dataPath)
	}
	if len(spec.ConditionIds) == 0 {
		t.Fatalf("Sentry Drift (mob %d) shipped conditionids is empty; expected it to include 85", sentryDriftId)
	}

	// Seed just this one real spec into the package's mob registry, the same
	// way save_test.go's seedMob helper installs a template, so
	// NewMobByIdFresh finds it without requiring a full mobs.LoadDataFiles()
	// (which would also cross-check every shipped mob's schedule_id and
	// patrol_id against DI validators this test does not wire up).
	mobsMu.Lock()
	prev, hadPrev := mobs[sentryDriftId]
	cp := *spec
	mobs[sentryDriftId] = &cp
	mobsMu.Unlock()
	t.Cleanup(func() {
		mobsMu.Lock()
		if hadPrev {
			mobs[sentryDriftId] = prev
		} else {
			delete(mobs, sentryDriftId)
		}
		mobsMu.Unlock()
	})

	inst := NewMobByIdFresh(MobId(sentryDriftId), 0)
	if inst == nil {
		t.Fatalf("NewMobByIdFresh(%d) returned nil", sentryDriftId)
	}

	if got := inst.Character.InfraReach(); got <= 0 {
		t.Fatalf("Sentry Drift (mob %d) InfraReach() = %d, want > 0: condition 85's grant must survive spawn, or infrared vision has no consumer again", sentryDriftId, got)
	}
}
