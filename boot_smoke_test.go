package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	yaml "gopkg.in/yaml.v2"
)

// bootSmokeEnvVar gates the boot smoke test.
//
// It is opt-in rather than always-on for two reasons: it loads the entire world
// (~40s, and materially longer under -race), and it populates every package
// global in the process, which would leak into any other test added to package
// main later. CI runs it in a dedicated step; see
// .github/actions/codegen-and-test/action.yml.
const bootSmokeEnvVar = `DOGMUD_BOOT_SMOKE`

// TestSmoke_ServerBootsCleanWithRealData loads every YAML data file through the
// real boot path and fails if anything is malformed.
//
// This automates the manual step in CLAUDE.md's Pre-Push SOP: "Boot the server
// locally and confirm it starts cleanly past data-file loading." That step
// exists because `go build` and `go vet` cannot see content errors — a
// filename/name-field mismatch, an invalid trigger event, an ID collision or a
// bad cross-reference only surfaces when the loaders run, and they deliberately
// panic when they find one. Until now nothing verified that automatically; it
// depended on a human remembering to boot the server before pushing.
//
// It calls loadAllDataFiles, the same function main() calls, so it exercises
// the real ordering and the dependency-injected cross-validators (schedule and
// patrol room/path checks, conversation pair checks, faction holding cells,
// mutation graph, species body parts) rather than a reimplementation that could
// drift from boot.
//
// This is also the prerequisite for the yaml.v2 -> v3 migration (audit finding
// 5.1): that swap changes decoding behaviour across every content file, and is
// not safe to attempt without an automated check that the whole tree still
// loads.
func TestSmoke_ServerBootsCleanWithRealData(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the full data-file boot smoke test (~40s)", bootSmokeEnvVar)
	}

	// "LOW" maps to slog Warn, which suppresses the very noisy per-file and
	// colour-palette Info logging the loaders emit.
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed — cannot boot without config: %v", err)
	}

	// The loaders panic by design on malformed content (fail fast at boot
	// rather than silently at runtime). Capture that and report it as a test
	// failure with the stack, which is far more useful than a raw panic.
	var bootPanic any
	var bootStack string

	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				bootPanic = r
				bootStack = string(debug.Stack())
			}
		}()
		loadAllDataFiles(false)
	}()
	elapsed := time.Since(start)

	if bootPanic != nil {
		t.Fatalf("loading real data files panicked — the server would not boot:\n\n%v\n\n%s",
			bootPanic, bootStack)
	}

	t.Logf("world loaded in %s", elapsed)

	// A clean run is necessary but not sufficient: a loader that silently found
	// nothing (wrong path, empty dir) would also "not panic". Assert each major
	// category actually produced content.
	categories := []struct {
		name  string
		count int
	}{
		{"rooms", len(rooms.GetAllRoomIds())},
		{"mob templates", len(mobs.AllMobTemplates())},
		{"spells", len(spells.GetAllSpells())},
		{"conditions", len(conditions.GetAllConditionIds())},
		{"quests", len(quests.GetAllQuests())},
		{"crafting recipes", len(crafting.GetAll())},
	}

	for _, c := range categories {
		if c.count == 0 {
			t.Errorf("%s: loaded 0 entries — the loader ran without error but found nothing, "+
				"which usually means a wrong or empty data path", c.name)
			continue
		}
		t.Logf("%-18s %d loaded", c.name, c.count)
	}
}

// TestSmoke_AllDialogueFilesParse eagerly parses every dialogue file.
//
// Dialogue is the one major content type loadAllDataFiles does NOT cover: it is
// lazy-loaded per mob on first interaction (dialogue.Load). That laziness is
// what made the documented "bare-scalar list field mutes a whole NPC" incident
// so expensive — internal/dialogue/loader.go discards the ENTIRE file on any
// unmarshal error and permanently caches a nil sentinel, so a single malformed
// field silently mutes that NPC for the life of the process, and nothing at
// boot or in CI notices. It only surfaces when a player talks to them.
//
// Parsing every file up front turns that from a silent production failure into
// a CI failure. Note this deliberately checks parsing only, mirroring exactly
// what dialogue.Load does — it is not a semantic validation of the content.
func TestSmoke_AllDialogueFilesParse(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the dialogue parse sweep", bootSmokeEnvVar)
	}

	root := filepath.Join(`_datafiles`, `world`, `dogmud`, `dialogue`)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("dialogue directory not found at %s: %v", root, err)
	}

	var checked, failed, greetingFiles int

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// A test elsewhere can create and remove a temp file under
				// the tree while packages test in parallel; a vanished
				// entry has nothing to scan.
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, `.yaml`) {
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("%s: unreadable: %v", path, readErr)
			failed++
			return nil
		}

		var df dialogue.DialogueFile
		// Strict, exactly as dialogue.Load decodes: an unknown key or a
		// malformed field would mute this NPC at runtime (Load discards the
		// whole file), and an unknown key is a value silently dropped.
		if unmarshalErr := yaml.UnmarshalStrict(raw, &df); unmarshalErr != nil {
			t.Errorf("%s: would mute this NPC at runtime — dialogue.Load discards the whole "+
				"file on this error:\n    %v", path, unmarshalErr)
			failed++
			return nil
		}

		if len(df.Greetings) > 0 {
			greetingFiles++
		}

		checked++
		return nil
	})
	if err != nil {
		t.Fatalf("walking dialogue tree: %v", err)
	}

	if checked == 0 {
		t.Fatal("parsed 0 dialogue files — wrong path?")
	}
	// Measured 2026-07-25 by strict-unmarshal probe: 186 of 302 files author
	// a greetings block. If this number DROPS without content changes, the
	// Greetings field has stopped matching the authored YAML shape.
	if greetingFiles < 186 {
		t.Errorf("files with a non-empty greetings block: got %d, want >= 186", greetingFiles)
	}
	t.Logf("dialogue files parsed cleanly: %d (failed: %d, with greetings: %d)", checked, failed, greetingFiles)
}

// TestSmoke_ShippedDialoguePassesTheValidators runs the boot door's validators
// (validateShippedDialogue: dialogue.RegistryWorld plus dialogue.ValidateWorld)
// over every shipped dialogue file, against the loaded quests, items and
// behavior trees. A file written by hand used to meet no validator at all:
// Hadwen's 9100.yaml shipped a ';' in spoken text, which the command splitter
// treats as a separator, cutting his answer short in play.
func TestSmoke_ShippedDialoguePassesTheValidators(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the shipped-dialogue validator sweep", bootSmokeEnvVar)
	}

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}

	func() {
		defer func() { _ = recover() }() // boot failures are the boot test's job
		loadAllDataFiles(false)
	}()
	if len(quests.GetAllQuests()) == 0 {
		t.Fatal("no quests loaded; the quest-token checks would all fail for the wrong reason")
	}

	files, err := filepath.Glob(filepath.Join(`_datafiles`, `world`, `dogmud`, `dialogue`, `*`, `*.yaml`))
	if err != nil || len(files) == 0 {
		t.Fatalf("found no dialogue files (err %v); the sweep cannot run", err)
	}

	// No baseline: every problem is an error. This is the same call the boot
	// door makes (validateShippedDialogue), so CI and boot cannot disagree.
	probs := validateShippedDialogue()
	for _, p := range probs {
		t.Errorf("%s", p.String())
	}
	t.Logf("dialogue files validated: %d (problems: %d)", len(files), len(probs))

	// The behavior-topic rules read each tree at the path TryMobBehavior
	// loads. Seven shipped trees carry topic_is (Hadwen, Esk, Sable, the
	// Threshold Keeper and three ferry agents); finding none would mean the
	// index read the wrong paths and every behavior check passed vacuously.
	perMob, _, _ := behaviorTopicIndex(mobs.AllMobTemplates())
	if len(perMob) < 7 {
		t.Errorf("the behavior-topic index found topic_is in %d trees, want at least 7: %v", len(perMob), perMob)
	}
}

// knownSilentlyIgnoredKeys is the accepted baseline of YAML keys that appear in
// content files but map to no field on their target type, so the lenient
// decoder discards them.
//
// This exists as a DRIFT GATE, not an approval. Flipping the loaders to strict
// decoding outright would fail the boot with thousands of violations, the vast
// bulk benign: `coord` is authored room data the engine abandoned in favour of
// crawled exit-delta positions, `level` is legacy from the level/XP system
// being removed.
//
// Quest YAML used to contribute 17 entries here — an artifact of being parsed
// by two type systems (the legacy `quests` package and `questengine`), each
// seeing the other's fields as unknown. The 5c-pre unification (2026-07-27,
// docs/superpowers/specs/completed/2026-07-27-quest-unification-5c-pre-design.md) gave
// quest files ONE explicitly-tagged parse, and those entries are gone; a novel
// quest key now means a real tag mistake or content typo, never re-baseline it.
//
// Baselining the distinct field/type pairs means a NEW mistyped or unsupported
// key — the `hostile:` failure mode, where an authored value silently does
// nothing — fails CI immediately, without requiring the existing backlog to be
// cleared first.
//
// Several entries below are genuine content bugs worth fixing, listed in the
// audit doc rather than silently accepted:
//   - `scriptag` on mobs.Mob: almost certainly a typo for `scripttag`.
//   - `visible` / `sequential` / `expireMessage` on conditions.ConditionSpec.
//   - `zone` on exit.RoomExit (136x): authored values doing nothing.
//
// `cooldown` on rooms.SpawnInfo was cleared 2026-07-25: 59 authored lines
// deleted rather than converted to `respawnrate`, since their values are in
// rounds while every live respawnrate is in real minutes — converting would
// have slowed those spawn points by 1.3-11x. See the spawn-list editor spec.
//
// Note for future cleanups: the first pass here found only 36, because it
// anchored its grep at one indentation depth. Spawn lists are authored both
// flush ("- mobid:") and indented ("  - mobid:"), so a depth-specific pattern
// silently misses half the world. This gate is what caught the shortfall.
//
// To clear an entry: fix the content (or add the field to the struct), confirm
// the count drops, and delete the line.
var knownSilentlyIgnoredKeys = map[string]bool{
	"coord|rooms.Room":                       true,
	"level|characters.Character":             true,
	"zone|exit.RoomExit":                     true,
	"items|mobs.Mob":                         true,
	"tactics|characters.Character":           true,
	"long|rooms.Container":                   true,
	"scriptag|mobs.Mob":                      true,
	"allow_recall|rooms.Room":                true,
	"visible|conditions.ConditionSpec":       true,
	"sequential|conditions.ConditionSpec":    true,
	"expireMessage|conditions.ConditionSpec": true,
}

var unknownKeyRe = regexp.MustCompile(`field ([A-Za-z_0-9]+) not found in type ([A-Za-z_.]+)`)

// TestSmoke_NoNewSilentlyIgnoredYAMLKeys fails when content introduces a YAML
// key that maps to no field on its target type and is not already baselined.
//
// This closes the `hostile:` incident class going forward: an authored value
// that silently does nothing now fails CI instead of shipping to production and
// being discovered months later.
func TestSmoke_NoNewSilentlyIgnoredYAMLKeys(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the unknown-YAML-key drift gate", bootSmokeEnvVar)
	}

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}

	var mu sync.Mutex
	found := map[string][]string{} // "field|type" -> example paths

	fileloader.StrictDecodeProbe = func(path string, err error) {
		mu.Lock()
		defer mu.Unlock()
		for _, m := range unknownKeyRe.FindAllStringSubmatch(err.Error(), -1) {
			key := m[1] + "|" + m[2]
			if len(found[key]) < 3 {
				found[key] = append(found[key], path)
			}
		}
	}
	defer func() { fileloader.StrictDecodeProbe = nil }()

	func() {
		defer func() { _ = recover() }() // boot failures are the other test's job
		loadAllDataFiles(false)
	}()

	var novel []string
	for key := range found {
		if !knownSilentlyIgnoredKeys[key] {
			novel = append(novel, key)
		}
	}
	sort.Strings(novel)

	for _, key := range novel {
		t.Errorf("new silently-ignored YAML key %q — this value is authored but does nothing, "+
			"because no field on that type matches it. Fix the key, add the field, or (if "+
			"deliberate) add it to knownSilentlyIgnoredKeys.\n    e.g. %v",
			key, found[key])
	}

	t.Logf("distinct silently-ignored keys: %d (baselined: %d, new: %d)",
		len(found), len(knownSilentlyIgnoredKeys), len(novel))
}

// TestSmoke_EveryQuestStepHasMarkerDecision asserts that every quest step has a
// deliberate minimap-marker decision: it either resolves a marker room (explicit
// map_target > 0, or a room_enter trigger the resolver infers) or is explicitly
// map_target: -1 ("no marker", audited). A step that is neither — map_target 0
// with nothing to infer — is undecided, and fails.
//
// Since a marker is now an expectation, a marker-less step must be a choice, not
// an oversight. This gate makes that choice mandatory: a new quest step that is
// neither markable-and-resolved nor explicitly -1 fails CI until someone eyeballs
// it. See docs/superpowers/specs/completed/2026-07-21-quest-minimap-marker-audit-design.md.
func TestSmoke_EveryQuestStepHasMarkerDecision(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the quest-marker decision gate", bootSmokeEnvVar)
	}

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}

	func() {
		defer func() { _ = recover() }() // boot failures are the boot test's job
		loadAllDataFiles(false)
	}()

	eng := questengine.GetEngine()
	var violations []string
	for _, q := range eng.AllQuests() {
		if q.QuestId == 1000000 {
			continue // generic reference template, not live content
		}
		for _, step := range q.Steps {
			if step.Id == "end" {
				continue // resolver returns 0 for "end" by design
			}
			if step.MapTarget == -1 {
				continue // audited deliberate no-marker
			}
			if eng.ResolveQuestTarget(q.QuestId, step.Id) > 0 {
				continue // explicit or inferred marker
			}
			violations = append(violations, fmt.Sprintf(
				"  quest %d %q step %q: no marker (map_target 0, no room_enter inference) — "+
					"set a room target or -1 with a reason comment", q.QuestId, q.Name, step.Id))
		}
	}
	sort.Strings(violations)

	if len(violations) > 0 {
		t.Fatalf("%d quest step(s) have no marker decision:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
	t.Logf("all quest steps have a marker decision")
}

// TestSmoke_KaelGreetingFollowsQuest11Branch walks Hermit Kael's (mob 240)
// root variants across the quest 11, 12 and 13 states against the real quest
// registry. HasQuest counts a finished player as holding every earlier step,
// so a variant gated only on 11-start matches everyone past it: when "Have you
// chosen yet?" sat first, every player who had ever started quest 11 saw it
// forever (#611). It must stay below the two sided-with variants, because a
// flag map cannot say "no 11-branch yet".
func TestSmoke_KaelGreetingFollowsQuest11Branch(t *testing.T) {
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the Kael greeting walk", bootSmokeEnvVar)
	}

	mudlog.SetupLogger(nil, `LOW`, ``, false)
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	func() {
		defer func() { _ = recover() }() // boot failures are the boot test's job
		loadAllDataFiles(false)
	}()
	if quests.GetQuest(`11-start`) == nil || quests.GetQuest(`13-extraction`) == nil {
		t.Fatal("quests 11-13 not loaded; the walk would fail for the wrong reason")
	}

	raw, err := os.ReadFile(filepath.Join(`_datafiles`, `world`, `dogmud`, `dialogue`, `ironwind_steppe`, `240.yaml`))
	if err != nil {
		t.Fatalf("read Kael's dialogue: %v", err)
	}
	var df dialogue.DialogueFile
	if err := yaml.Unmarshal(raw, &df); err != nil {
		t.Fatalf("parse Kael's dialogue: %v", err)
	}

	cases := []struct {
		name     string
		progress map[int]string
		branch   string
		want     string // a phrase unique to the expected greeting
	}{
		{"no quest", map[int]string{}, ``, `A weathered man in patched leather`},
		{"11-start, no side", map[int]string{11: `start`}, ``, `Have you chosen yet?`},
		{"11-start, sylara", map[int]string{11: `start`}, `sylara`, `You sided with Sylara.`},
		{"11-start, rhett", map[int]string{11: `start`}, `rhett`, `You sided with Rhett.`},
		{"11-end, sylara", map[int]string{11: `end`}, `sylara`, `You made your choice.`},
		{"11-end, rhett", map[int]string{11: `end`}, `rhett`, `You made your choice.`},
		{"12-start", map[int]string{11: `end`, 12: `start`}, `sylara`, `Walking the Windwarden's path`},
		{"12-ritual", map[int]string{11: `end`, 12: `ritual`}, `sylara`, `Walking the Windwarden's path`},
		{"12-end", map[int]string{11: `end`, 12: `end`}, `sylara`, `The spirits walk with you now.`},
		{"13-start", map[int]string{11: `end`, 13: `start`}, `rhett`, `Walking the scholar's path.`},
		{"13-extraction", map[int]string{11: `end`, 13: `extraction`}, `rhett`, `Walking the scholar's path.`},
		{"13-end", map[int]string{11: `end`, 13: `end`}, `rhett`, `Rhett got his crystal.`},
	}
	for _, c := range cases {
		ch := &characters.Character{QuestProgress: c.progress}
		branch := c.branch
		ps := &dialogue.PlayerState{
			HasQuest: ch.HasQuest,
			GetQuestFlag: func(key string) string {
				if key == `11-branch` {
					return branch
				}
				return ``
			},
		}
		// #375: talk opens with OpenerFor (the first variant whose gates
		// pass); the narrator's half of the greeting is the opener hint.
		text, hints := df.OpenerFor(ps)
		if got := strings.Join(strings.Fields(text+" "+hints), " "); !strings.Contains(got, c.want) {
			t.Errorf("%s: Kael opens with %q, want the opener containing %q", c.name, got, c.want)
		}
	}
}
