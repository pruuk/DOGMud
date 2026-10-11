package hooks

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// #480 reference player (owner ruling 2026-10-10): the real playtest `mid`
// profile (tools/playtest/profiles/mid.yaml), effectively UNARMOURED at raw
// 0.02 physical. A missed discharge must never one-shot them at full health,
// so the highest normal (non-crit) roll has to stay under their pool.
const (
	coreDischargeMidProfile  = "tools/playtest/profiles/mid.yaml"
	coreDischargeGuardianId  = 9562
	coreDischargeWardenId    = 9561
	coreDischargeSpawns      = 40
	coreDischargeRollsPerMob = 500

	// The non-crit roll is dice.RollStat: a normal around the mean with
	// sd = mean x RollSpread and no upper clamp, so it has no true maximum.
	// The bound uses z = 3, a roll one hit in about 740 exceeds.
	coreDischargeRollZ = 3.0
	// coreDischargeRunAllowance covers what moves a live boss's raw damage
	// above the spawn-averaged mean this test measures: one spawn's random
	// willpower (sd about 17 on 580 at pool 2800, 3%) and the spellcasting
	// the Guardian trains on its own drains during the fight. The
	// re-playtest of cdc2250de logged raw 489.7 to 493 against a mean of
	// 470, so +4.2%; 5% covers it.
	coreDischargeRunAllowance = 1.05
	// coreDischargeBoundMargin keeps the highest normal roll a little under
	// a full pool rather than exactly at it.
	coreDischargeBoundMargin = 0.97
)

// midReferencePlayer is the `mid` playtest profile's character, validated
// the way the game builds one, so its pool and mitigation are the game's
// own (about 491 HP; raw 0.02 physical from its body and feet).
func midReferencePlayer(t *testing.T) *characters.Character {
	t.Helper()
	raw, err := os.ReadFile(coreDischargeMidProfile)
	if err != nil {
		t.Fatalf("reading the mid profile: %v", err)
	}
	var profile struct {
		Character characters.Character `yaml:"character"`
	}
	if err := yaml.Unmarshal(raw, &profile); err != nil {
		t.Fatalf("parsing the mid profile: %v", err)
	}
	c := &profile.Character
	c.Validate()
	c.Health = c.HealthMax.Value
	return c
}

// loadShippedCrashSiteBosses points the registries at the REAL shipped
// config, species, items, spells and the two crash-site boss templates, and
// restores every one of them on cleanup. A test binary otherwise runs on Go
// defaults (SpellDamageScale 1.0 against a shipped 3.12), which is the trap
// this test exists to avoid. Files are read by path anchored on this source
// file, because the hooks TestMain points FilePaths.DataFiles at a temp dir.
func loadShippedCrashSiteBosses(t *testing.T) {
	t.Helper()
	drainEventQueueOnCleanup(t) // spawning boss templates may queue events
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

// meanCoreDischargeHit is the mean NORMAL (non-crit) landed discharge against
// target, averaged over fresh spawns of mobId (the stat pool is randomly
// distributed per spawn) and many damage rolls each.
func meanCoreDischargeHit(t *testing.T, sd *spells.SpellData, mobId int, statPool int, target *characters.Character) (mean float64, willpower float64) {
	t.Helper()
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

// spawnMultiplier reads the instance stat-pool multiplier a crash-site room
// authors for mobId (spawninfo statpool; the live pool is gold paid times
// this, rooms.ScaleSpawnStatPools), so the reference run follows the content.
func spawnMultiplier(t *testing.T, roomId, mobId int) int {
	t.Helper()
	path := filepath.Join("_datafiles", "world", "dogmud", "rooms", "crash_site_interior", strconv.Itoa(roomId)+".yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading room %d: %v", roomId, err)
	}
	var room struct {
		SpawnInfo []struct {
			MobId    int `yaml:"mobid"`
			StatPool int `yaml:"statpool"`
		} `yaml:"spawninfo"`
	}
	if err := yaml.Unmarshal(raw, &room); err != nil {
		t.Fatalf("parsing room %d: %v", roomId, err)
	}
	for _, s := range room.SpawnInfo {
		if s.MobId == mobId && s.StatPool > 0 {
			return s.StatPool
		}
	}
	t.Fatalf("room %d spawns no mob %d with a statpool multiplier", roomId, mobId)
	return 0
}

// TestCrashSiteDischarges_ShippedMeansHitTheirBands pins #480's retune
// against the `mid` playtest player in a 400g run (the Core Guardian's
// authored pool, 2800, is exactly that run). Two checks per boss:
//
//   - the bound (owner ruling: never a one-shot): mean x the roll's z = 3
//     multiple x the live-run allowance stays under 97% of the pool;
//   - the mean band: 58-64% of the pool for the Core Guardian, 30-40% for
//     Warden-Prime, the teaching boss.
//
// It runs the real calcSpellDamageForCharacter on the shipped spells, boss
// templates, Core Matrix, mid profile and config, so a later change to
// SpellDamageScale, RollSpread, GlobalDamageMultiplier, the mitigation curve,
// a boss's stats or a spell's damage_multiplier that moves either fails here.
func TestCrashSiteDischarges_ShippedMeansHitTheirBands(t *testing.T) {
	loadShippedCrashSiteBosses(t)
	if got := float64(configs.GetBalanceConfig().SpellDamageScale); got == 1.0 {
		t.Fatal("SpellDamageScale is the Go default 1.0: the shipped config.yaml was not read")
	}
	ref := midReferencePlayer(t)
	pool := float64(ref.HealthMax.Value)
	if raw := ref.GetPhysicalMitigation(); math.Abs(raw-0.02) > 1e-9 {
		t.Fatalf("the mid profile's raw physical mitigation is %.3f, want 0.02 (effectively unarmoured)", raw)
	}
	spread := float64(configs.GetBalanceConfig().RollSpread)
	highRoll := 1 + coreDischargeRollZ*spread
	t.Logf("mid reference player: %d HP, raw physical 0.02; RollSpread %.2f, so a z=%.0f roll is x%.3f",
		ref.HealthMax.Value, spread, coreDischargeRollZ, highRoll)

	const referenceGold = 400
	for _, tc := range []struct {
		boss, spellId string
		mobId, roomId int
		low, high     float64
		// wantWil is the YAML comment's willpower: 30 (automaton species) +
		// pool x 0.125 (the fighting archetype's per-mental-stat share,
		// 0.15 / 1.2) + 200 from the Guardian's Core Matrix. Pinning it means
		// a drift in the stat inputs fails here by name, not only as a
		// moved damage band.
		wantWil float64
	}{
		{"Core Guardian", "core-discharge", coreDischargeGuardianId, 6395, 0.58, 0.64, 30 + 2800*0.125 + 200},
		{"Warden-Prime", "warden-discharge", coreDischargeWardenId, 6392, 0.30, 0.40, 30 + 2000*0.125},
	} {
		t.Run(tc.boss, func(t *testing.T) {
			sd := spells.GetSpell(tc.spellId)
			if sd == nil {
				t.Fatalf("%s did not load from the shipped spells", tc.spellId)
			}
			mult := spawnMultiplier(t, tc.roomId, tc.mobId)
			mean, wil := meanCoreDischargeHit(t, sd, tc.mobId, referenceGold*mult, ref)
			frac := mean / pool
			high := mean * highRoll * coreDischargeRunAllowance
			t.Logf("%s, %dg run (pool %d): willpower %.1f, mean hit %.1f = %.1f%% of %.0f HP; "+
				"high normal roll %.0f = %.1f%% (damage_multiplier %.3f)",
				tc.boss, referenceGold, referenceGold*mult, wil, mean, frac*100, pool, high, high/pool*100, sd.DamageMultiplier)
			if high >= coreDischargeBoundMargin*pool {
				t.Errorf("%s: mean %.1f x z=%.0f roll %.3f x run allowance %.2f = %.0f, want under %.0f%% of the "+
					"%.0f HP pool: a normal roll could one-shot an unarmoured player", tc.spellId, mean,
					coreDischargeRollZ, highRoll, coreDischargeRunAllowance, high, coreDischargeBoundMargin*100, pool)
			}
			// The pool spreads at random per spawn: one spawn's willpower has
			// a spread of about 17 at pool 2800, so the 40-spawn mean is
			// within about 3, and 15 is a five-sigma margin.
			if math.Abs(wil-tc.wantWil) > 15 {
				t.Errorf("%s willpower %.1f, want %.0f +/- 15 (species + pool share + Core Matrix); "+
					"the YAML arithmetic is stale", tc.boss, wil, tc.wantWil)
			}
			if frac < tc.low || frac > tc.high {
				t.Errorf("%s mean hit is %.1f%% of the %.0f HP reference player, want %.0f-%.0f%%; "+
					"retune damage_multiplier in %s.yaml (or revisit the knob that moved)",
					tc.spellId, frac*100, pool, tc.low*100, tc.high*100, tc.spellId)
			}
		})
	}
}

// TestCrashSiteDischarges_TreesCastTheirOwnWindupSpell pins the wiring: each
// boss tree casts its own discharge, and every telegraphed spell it casts
// carries the two-round min_cast_rounds windup and a held-round warning.
func TestCrashSiteDischarges_TreesCastTheirOwnWindupSpell(t *testing.T) {
	loadShippedCrashSiteBosses(t)
	for tree, want := range map[string]string{
		"9562-the_core_guardian.yaml": "core-discharge",
		"9561-warden_prime.yaml":      "warden-discharge",
	} {
		raw, err := os.ReadFile(filepath.Join("_datafiles", "world", "dogmud", "behaviors", "crash_site_interior", tree))
		if err != nil {
			t.Fatalf("reading %s: %v", tree, err)
		}
		var doc any
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parsing %s: %v", tree, err)
		}
		cast := map[string]bool{}
		collectCastSpells(doc, cast)
		// #480 review: the windup's warning is the spell's own sight-aware
		// cast and wait lines. A send_room_text beside the cast is plain
		// room.SendText: it names the boss in the dark and prints even when
		// the cast then fails.
		if n := roomTextBesideCast(doc); n > 0 {
			t.Errorf("%s: %d send_room_text node(s) share a sequence with a cast; warn through the spell's lines", tree, n)
		}
		if !cast[want] {
			t.Errorf("%s casts %v, want it to cast %s", tree, cast, want)
		}
		for id := range cast {
			if strings.HasSuffix(id, "-discharge") && id != want {
				t.Errorf("%s casts %s; each boss casts only its own discharge", tree, id)
			}
		}
		// Every telegraphed boss cast the tree makes (no_damage_interrupt:
		// the discharges and Core Drain) carries the two-round windup its
		// help page promises, and a warning line for the held round.
		for id := range cast {
			sd := spells.GetSpell(id)
			if sd == nil {
				t.Fatalf("%s casts %s, which did not load", tree, id)
			}
			if !sd.NoDamageInterrupt {
				continue
			}
			if sd.MinCastRounds != 2 || sd.WaitRoomText == "" {
				t.Errorf("%s: min_cast_rounds %d, wait_observer %q; want 2 and a warning line", id, sd.MinCastRounds, sd.WaitRoomText)
			}
		}
	}
}

// roomTextBesideCast counts send_room_text actions in any children list that
// also holds a `do: cast` action.
func roomTextBesideCast(node any) int {
	n := 0
	switch v := node.(type) {
	case map[string]any:
		if kids, ok := v["children"].([]any); ok {
			hasCast, texts := false, 0
			for _, k := range kids {
				if m, ok := k.(map[string]any); ok {
					hasCast = hasCast || m["do"] == "cast"
					if m["do"] == "send_room_text" {
						texts++
					}
				}
			}
			if hasCast {
				n += texts
			}
		}
		for _, c := range v {
			n += roomTextBesideCast(c)
		}
	case []any:
		for _, c := range v {
			n += roomTextBesideCast(c)
		}
	}
	return n
}

// collectCastSpells gathers the spell of every `do: cast` node in a parsed tree.
func collectCastSpells(node any, out map[string]bool) {
	switch n := node.(type) {
	case map[string]any:
		if n["do"] == "cast" {
			if s, ok := n["spell"].(string); ok {
				out[s] = true
			}
		}
		for _, v := range n {
			collectCastSpells(v, out)
		}
	case []any:
		for _, v := range n {
			collectCastSpells(v, out)
		}
	}
}
