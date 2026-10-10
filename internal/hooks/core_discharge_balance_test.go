package hooks

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// #480 reference player: the mid-geared playtester Core Discharge one-shot,
// 532 HP at raw 0.56 physical mitigation (55.9% after the #465 curve).
const (
	coreDischargeRefHP       = 530
	coreDischargeRefRawMitig = 0.56
	coreDischargeGuardianId  = 9562
	coreDischargeWardenId    = 9561
	coreDischargeSpawns      = 40
	coreDischargeRollsPerMob = 500
	coreDischargeBandLow     = 0.60
	coreDischargeBandHigh    = 0.70
)

// loadShippedCrashSiteBosses points the registries at the REAL shipped
// config, species, items, spells and the two crash-site boss templates, and
// restores every one of them on cleanup. A test binary otherwise runs on Go
// defaults (SpellDamageScale 1.0 against a shipped 3.12), which is the trap
// this test exists to avoid. Files are read by path anchored on this source
// file, because the hooks TestMain points FilePaths.DataFiles at a temp dir.
func loadShippedCrashSiteBosses(t *testing.T) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	world := filepath.Join(repoRoot, "_datafiles", "world", "dogmud")
	t.Chdir(repoRoot)

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	t.Cleanup(func() { mudlog.SetupLogger(nil, "", "", false) })
	// The WithLookups form, not plain SetConfigForTest: ReloadConfig rebuilds
	// the key lookups and override tables, and a later SetVal test in this
	// binary fails if those leak. It also points CONFIG_PATH at a scratch
	// file, so a developer's local config-overrides.yaml cannot move the pin.
	configs.SetConfigWithLookupsForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	sp, err := fileloader.LoadAllFlatFiles[int, *species.Species](filepath.Join(world, "species"))
	if err != nil {
		t.Fatalf("loading species: %v", err)
	}
	t.Cleanup(species.SeedSpeciesForTest(sp))

	it, err := fileloader.LoadAllFlatFiles[int, *items.ItemSpec](filepath.Join(world, "items"))
	if err != nil {
		t.Fatalf("loading items: %v", err)
	}
	t.Cleanup(items.SeedItemsForTest(it))

	sps, err := fileloader.LoadAllFlatFiles[string, *spells.SpellData](filepath.Join(world, "spells"))
	if err != nil {
		t.Fatalf("loading spells: %v", err)
	}
	t.Cleanup(spells.SeedSpellsForTest(sps))

	ms, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](filepath.Join(world, "mobs", "crash_site_interior"))
	if err != nil {
		t.Fatalf("loading crash site mobs: %v", err)
	}
	t.Cleanup(mobs.SeedMobsForTest(ms, map[int]*mobs.Mob{}))
}

// coreDischargeReferenceTarget is the #480 reference player: a bare
// character whose only protection is one piece carrying raw 0.56 physical.
func coreDischargeReferenceTarget() *characters.Character {
	target := &characters.Character{Name: "Reference"}
	target.HealthMax.Value = coreDischargeRefHP
	target.Health = coreDischargeRefHP
	target.Equipment.Body = items.Item{ItemId: 1, Spec: &items.ItemSpec{
		PhysicalMitigation: int(coreDischargeRefRawMitig * 100),
	}}
	return target
}

// meanCoreDischargeHit is the mean NORMAL (non-crit) landed Core Discharge
// against the reference target, averaged over fresh spawns of mobId (the
// stat pool is randomly distributed per spawn) and many damage rolls each.
// statPool 0 spawns at the template's own statpool.
func meanCoreDischargeHit(t *testing.T, sd *spells.SpellData, mobId int, statPool int) (mean float64, willpower float64) {
	t.Helper()
	target := coreDischargeReferenceTarget()
	total, wil := 0.0, 0.0
	for s := 0; s < coreDischargeSpawns; s++ {
		mob := mobs.NewMobByIdFresh(mobs.MobId(mobId), 0, statPool)
		if mob == nil {
			t.Fatalf("NewMobByIdFresh(%d) returned nil", mobId)
		}
		wil += float64(sd.CasterStatValue(mob.Character.Stats))
		for r := 0; r < coreDischargeRollsPerMob; r++ {
			total += float64(calcSpellDamageForCharacter(sd, &mob.Character, target, sd.EffectMagnitude, false))
		}
	}
	return total / float64(coreDischargeSpawns*coreDischargeRollsPerMob), wil / coreDischargeSpawns
}

// TestCoreDischarge_ShippedMeanLeavesReferencePlayerStanding pins #480's
// retune: an uninterrupted Core Discharge from the Core Guardian at its
// authored statpool must take a mean 60 to 70% of the reference mid player's
// health on a normal hit. It runs the real calcSpellDamageForCharacter on the
// shipped spell, boss template, Core Matrix and config, so a later change to
// SpellDamageScale, GlobalDamageMultiplier, the mitigation curve, the boss's
// stats or the spell's damage_multiplier that moves the number fails here.
func TestCoreDischarge_ShippedMeanLeavesReferencePlayerStanding(t *testing.T) {
	loadShippedCrashSiteBosses(t)

	if got := float64(configs.GetBalanceConfig().SpellDamageScale); got == 1.0 {
		t.Fatal("SpellDamageScale is the Go default 1.0: the shipped config.yaml was not read")
	}
	sd := spells.GetSpell("core-discharge")
	if sd == nil {
		t.Fatal("core-discharge did not load from the shipped spells")
	}

	mean, wil := meanCoreDischargeHit(t, sd, coreDischargeGuardianId, 0)
	frac := mean / coreDischargeRefHP
	t.Logf("Core Guardian: willpower %.1f, mean hit %.1f = %.1f%% of %d HP (damage_multiplier %.2f)",
		wil, mean, frac*100, coreDischargeRefHP, sd.DamageMultiplier)

	if frac < coreDischargeBandLow || frac > coreDischargeBandHigh {
		t.Errorf("Core Discharge mean hit is %.1f%% of a %d HP reference player, want %.0f-%.0f%%; "+
			"retune damage_multiplier in core-discharge.yaml (or revisit the knob that moved)",
			frac*100, coreDischargeRefHP, coreDischargeBandLow*100, coreDischargeBandHigh*100)
	}

	wMean, wWil := meanCoreDischargeHit(t, sd, coreDischargeWardenId, 0)
	t.Logf("Warden-Prime: willpower %.1f, mean hit %.1f = %.1f%% of %d HP",
		wWil, wMean, wMean/coreDischargeRefHP*100, coreDischargeRefHP)
	for _, gold := range []int{200, 300, 400} {
		g, gw := meanCoreDischargeHit(t, sd, coreDischargeGuardianId, gold*7)
		w, ww := meanCoreDischargeHit(t, sd, coreDischargeWardenId, gold*5)
		t.Logf("%dg instance: Guardian wil %.0f mean %.0f (%.0f%%); Warden-Prime wil %.0f mean %.0f (%.0f%%)",
			gold, gw, g, g/coreDischargeRefHP*100, ww, w, w/coreDischargeRefHP*100)
	}
}
