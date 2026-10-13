# Messaging M6 Slice 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close ledger rows 36 to 59 of #370: dark-combat prose for blind readers, plain words for six idioms, one pool per special-move event, the anonymity helper, and no dashes in combat, narration, rooms or mobs.

**Architecture:** A new store package `internal/darkcombat` (the `internal/lightnotice` pattern) holds the dark lines; `internal/combat.hideIdentitiesInPersonalLines` swaps a blind reader's personal lines for one dark line per `SwingEvent`, gated on optics (`messaging.ParticipantSight`), not on the sleep-gated `ReaderSight`. Special-move `player_*` events merge into their plain events in `_datafiles/world/dogmud/narration/special-moves/`. Copy edits are hand rewrites guarded by root-package tests.

**Tech Stack:** Go 1.x, YAML stores loaded by `internal/fileloader`, `internal/narration` render core, golden snapshots in `internal/narration/testdata/stores/`.

**Spec:** `docs/superpowers/specs/2026-10-12-messaging-m6-slice2-combat-copy-design.md`

---

## Facts verified against source (master `61da8280b`, 2026-10-12)

| Fact | Where |
|---|---|
| `SwingEvent` fields: `Hit, Crit, CritSource, Fumble, DoubleFumble, DefenseCrit, Damage, DamageReduced, DefenseUsed, AttackZScore, DefenseZScore, AttackType` | `internal/combat/attackresult.go:65` |
| `SwingEvent` is appended once per swing at `internal/combat/combat.go:593`, with `DefenseUsed: attackResult.DefenseUsed` | `combat.go:593-606` |
| `attackResult.DefenseUsed` is set only on a defensive win (`combat_helpers.go:1374`) and never cleared, so it is STICKY across later swings of the same round. The per-swing truth is `hitResolution.defenceWon()` (`combat_helpers.go:915`: `r.defended \|\| r.defenseCrit`), available as `res` at the append site | `combat_helpers.go:1374,915` |
| `hideIdentitiesInPersonalLines(result *AttackResult, sourceChar, targetChar *characters.Character, ctx combatContext)` is called once at `combat.go:653`, after every swing | `combat.go:653,712` |
| `combatContext` fields: `sourceSight, targetSight messaging.SightDecision; sourceDark, sourceBright, targetDark, targetBright float64; omitAttackSkill, forceCrit bool` | `combat_helpers.go:35-60` |
| `newCombatContext(source, target *characters.Character, room messaging.RoomVisibility, forceCrit bool) combatContext` uses `messaging.ReaderSight` (sleep-gated) | `combat_helpers.go:71-83` |
| `SightDecision` order: `SightFull = 0, SightShapes = 1, SightNone = 2`. A zero-valued `combatContext` therefore reads `SightFull` for every field | `predicates.go:204-207` comment |
| `messaging.ParticipantSight(observer *characters.Character, room RoomVisibility) SightDecision` is optics only: `Blinded` perception gives `SightNone`; no sleep test | `predicates.go:56-62` |
| `TaggedMessage{Category messaging.Category; Text string}` | `attackresult.go:133` |
| Deleted dark literals and bucketing (the parity reference) | `git show 6370977fd^:internal/hooks/NewRound_DoCombat_helpers.go` (function `replaceDarknessMessages`) |
| Combat test fixtures: `verdictLight(int)` (a `RoomVisibility`), `verdictObserver(t, nightVision, infraredVision, asleep, blind bool)` | `internal/combat/darkness_penalty_verdict_test.go:41,62` |
| Store pattern: `lightnotice.LoadFrom(dir) error`, `LoadLightNoticeFiles()` panics, `fileloader.LoadAllFlatFiles[string, *T](dir)` keyed by `Id()`, `Filepath()` | `internal/lightnotice/store.go` |
| Boot loaders run in `main.go:1977-1980` (`movenarration.LoadMoveNarrationFiles()`, `lightnotice.LoadLightNoticeFiles()`) | `main.go:1978-1979` |
| Golden snapshot harness: `checkGolden(t, name, text)`, `-update` flag, `dogmudDataDir(t)`; light notices built by `buildLightNoticesGolden` | `internal/narration/snapshot_test.go:111,854,1562` |
| `movenarration.EventMessages{Actor, Actee, Observer, RemoteObserver []string}` (yaml `actor`, `actee`, `observer`, `remote_observer`); `Validate` runs `narration.ValidateVariants(ev.variants(), 1)` which fails on unequal role lengths | `internal/movenarration/store.go:70-115` |
| Mob `sendMoveEvent` sends EVERY authored role (`internal/mobcommands/move_narration.go:71-82`) | that file |
| Mob vs player event shapes per verb (actor/actee/observer/remote counts) | table in Task 5 |
| Go callers using `player_*` keys: `internal/usercommands/{bash,drain,gore,grapple,kick,maul,pounce,rake,shoot,throttle,throw,trip}.go` (the full line list is in Task 5) | `grep -rn 'player_' internal/usercommands/<verb>.go` |
| Tests naming `player_*` keys: root `move_narration_migration_guard_test.go` (26 lines incl. `intentionallySilentRoles` at :519), `internal/usercommands/{special_move_net_test,usercommands_test,cast_interrupt_sound_test,shoot_vertical_narration_test}.go`, `internal/mobcommands/special_move_net_test.go`, `internal/hooks/hidden_player_area_drain_test.go`, `internal/mobs/mobs_test.go`, `internal/actions/command_readiness_drift_test.go` | grep |
| The two `special_move_net_test.go` files are the M4e byte-identity nets against `testdata/pre_migration_literals.json`, generated by `tools/move_narration_net.py` | those files |
| `drain.yaml:37` carries `'{actor}''''s touch drains you, ...'` (renders two apostrophes) | `_datafiles/world/dogmud/narration/special-moves/drain.yaml:37` |
| Stealth literal `` `Someone` `` at `internal/mobcommands/shoot.go:97` and `internal/usercommands/shoot.go:441` | those files |
| `UnseenNoun`, `SpeakerNoun` live in `internal/messaging/hidenames.go` | `hidenames.go:30,77` |
| Dash counts (non-comment lines). Three forms count as a dash: em/en dash, a spaced hyphen ` - `, and a spaced double hyphen ` -- ` (owner 2026-10-12; no unspaced `word--word` exists in scope). Rooms: 3,840 em/en + 1,015 ` -- ` = 4,857 lines in 1,174 files across 48 zone dirs. Mobs: 317 em/en + 496 ` -- ` = 813 lines in 461 files. `messaging/` 109 (99 in `grapple_outcomes.yaml`). `narration/` 12 (9 em, 2 ` - `, 1 ` -- `). `combat-messages/` 171 ` - `, 0 others | `grep -rnP "[\x{2014}\x{2013}]\|[A-Za-z,.!?'\"] --? [A-Za-z]"` |
| Go player strings with ` -- `: 12 sites (listed in Task 9). `internal/usercommands/admin.fact.go:158` uses `--` as a CLI separator and is not prose | grep |
| `docs/PATCH_NOTES.md` takes a dated `## YYYY-MM-DD: Title` entry, player voice, 72-column bullets | that file |

## Working rules for every task

- Worktree: `C:/tmp/dogmud-m6s2` on branch `feat/m6-slice2-combat-copy` (PR 1) and `C:/tmp/dogmud-m6s2-world` on `feat/m6-slice2-world-dashes` (PR 2). Both are named here so the tmp sweep keeps them; remove with `git worktree remove` after merge.
- Every text edit goes through the Edit tool. No `sed -i`, no Python that writes files, no `yaml.dump`. Read-only Python and grep for counting are fine.
- Every test gate runs the touched packages AND `go test .` at the repo root.
- `git add` named paths only.
- Player copy follows `dogmud-player-copy`: 80 visible columns, no numbers, plain ESL-clear words, no em or en dashes, no spaced hyphen used as a dash.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

# PR 1: combat copy

### Task 0: Worktree and baseline

**Files:** none

- [ ] **Step 1: Create the worktree**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/m6-slice2-combat-copy C:/tmp/dogmud-m6s2 origin/master
```

- [ ] **Step 2: Baseline gate**

```bash
cd C:/tmp/dogmud-m6s2
go build ./... && go test . ./internal/combat/... ./internal/movenarration/... ./internal/narration/... ./internal/usercommands/... ./internal/mobcommands/...
```

Expected: all `ok`. Record any pre-existing failure before touching code.

---

### Task 1: Per-swing `Defended` on `SwingEvent`

The dark bucketing needs to know whether THIS swing's defence won. `DefenseUsed` is sticky across a round, so the old function mislabelled a later clean hit as "defended" once any earlier swing had been defended.

**Files:**
- Modify: `internal/combat/attackresult.go` (struct `SwingEvent`, around line 65)
- Modify: `internal/combat/combat.go:593-606` (the append)
- Test: `internal/combat/swing_event_defended_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package combat

import (
	"reflect"
	"testing"
)

// The dark-combat bucketing (M6 slice 2) reads a per-swing defence result.
// AttackResult.DefenseUsed is set on a defensive win and never cleared, so it
// cannot answer "did THIS swing's defence win". SwingEvent.Defended does.
func TestSwingEvent_HasPerSwingDefended(t *testing.T) {
	f, ok := reflect.TypeOf(SwingEvent{}).FieldByName("Defended")
	if !ok {
		t.Fatal("SwingEvent has no Defended field")
	}
	if f.Type.Kind() != reflect.Bool {
		t.Fatalf("SwingEvent.Defended is %v, want bool", f.Type)
	}
}

func TestHitResolution_DefenceWonFeedsDefended(t *testing.T) {
	cases := []struct {
		r    hitResolution
		want bool
	}{
		{hitResolution{}, false},
		{hitResolution{defended: true}, true},
		{hitResolution{defenseCrit: true}, true},
	}
	for _, c := range cases {
		if got := swingEventFor(c.r).Defended; got != c.want {
			t.Fatalf("swingEventFor(%+v).Defended = %v, want %v", c.r, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/combat/ -run 'TestSwingEvent_HasPerSwingDefended|TestHitResolution_DefenceWonFeedsDefended' -count=1`
Expected: FAIL, compile error `undefined: swingEventFor` / no field `Defended`.

- [ ] **Step 3: Add the field**

In `attackresult.go`, in `SwingEvent`, directly after `DefenseCrit   bool`:

```go
	// Defended is this swing's own defensive win (hitResolution.defenceWon:
	// a plain win or a defence crit). AttackResult.DefenseUsed cannot answer
	// that: it is stamped on a win and never cleared, so every later swing of
	// the round would read as defended. The dark-combat lines (M6 slice 2)
	// bucket on this field.
	Defended bool
```

- [ ] **Step 4: Extract the per-swing fields and set `Defended`**

In `combat.go`, add near `hideIdentitiesInPersonalLines`:

```go
// swingEventFor fills the per-swing outcome fields of a SwingEvent from one
// swing's resolution. The caller adds the round-scoped fields (damage, the
// z-scores, DefenseUsed, AttackType).
func swingEventFor(r hitResolution) SwingEvent {
	return SwingEvent{
		Hit:          r.hit,
		Crit:         r.crit,
		CritSource:   r.critSource,
		Fumble:       r.fumble,
		DoubleFumble: r.doubleFumble,
		DefenseCrit:  r.defenseCrit,
		Defended:     r.defenceWon(),
	}
}
```

Replace the append at `combat.go:593-606` with:

```go
			// Record per-swing analytics
			se := swingEventFor(res)
			se.Damage = attackTargetDamage
			se.DamageReduced = attackTargetReduction
			se.DefenseUsed = attackResult.DefenseUsed
			se.AttackZScore = attackResult.AttackZScore
			se.DefenseZScore = attackResult.DefenseZScore
			se.AttackType = swingAtkType
			attackResult.SwingEvents = append(attackResult.SwingEvents, se)
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/combat/... . -count=1`
Expected: PASS. Null probe: change `Defended: r.defenceWon()` to `Defended: r.defended`, rerun, confirm `TestHitResolution_DefenceWonFeedsDefended` fails on the `defenseCrit` case, restore.

- [ ] **Step 6: Commit**

```bash
git add internal/combat/attackresult.go internal/combat/combat.go internal/combat/swing_event_defended_test.go
git commit -m "feat(combat): SwingEvent records its own defensive win

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: The `darkcombat` store

**Files:**
- Create: `internal/darkcombat/store.go`
- Create: `internal/darkcombat/store_test.go`
- Create: `internal/darkcombat/context.md`
- Create: `_datafiles/world/dogmud/narration/dark-combat/attacker.yaml`
- Create: `_datafiles/world/dogmud/narration/dark-combat/defender.yaml`
- Modify: `main.go:1979` (boot loader)

Layout note: the spec named one file, `narration/dark-combat.yaml`. `fileloader.LoadAllFlatFiles` loads a directory keyed by `Id()`, so the store is a directory with one file per viewpoint, matching `light-notices/`. Same content, same rule.

- [ ] **Step 1: Write the store files**

`_datafiles/world/dogmud/narration/dark-combat/attacker.yaml`:

```yaml
# Dark-combat lines, attacker viewpoint (M6 slice 2, ledger rows 36 to 41).
# Read by a player who cannot see at all (optics, not sleep) in place of the
# composed swing line. One line per swing outcome, written from touch and
# sound. Never name anyone, never claim sight, no {tokens}, no numbers, no
# dashes, at most 80 visible columns. Variant 0 of each list is the line the
# game shipped before M4d PR 2.
viewpoint: attacker
outcomes:
  fumble:
    - '<ansi fg="fumble-text">!!!</ansi> <ansi fg="yellow">You stumble badly in the darkness!</ansi> <ansi fg="fumble-text">!!!</ansi>'
    - '<ansi fg="fumble-text">!!!</ansi> <ansi fg="yellow">Your foot catches and you lurch off balance!</ansi> <ansi fg="fumble-text">!!!</ansi>'
    - '<ansi fg="fumble-text">!!!</ansi> <ansi fg="yellow">You overreach in the dark and nearly fall!</ansi> <ansi fg="fumble-text">!!!</ansi>'
    - '<ansi fg="fumble-text">!!!</ansi> <ansi fg="yellow">Your blind swing goes wrong and throws you off!</ansi> <ansi fg="fumble-text">!!!</ansi>'
  crit:
    - '<ansi fg="crit-text">***</ansi> <ansi fg="attack-good">You land a devastating blow in the dark!</ansi> <ansi fg="crit-text">***</ansi>'
    - '<ansi fg="crit-text">***</ansi> <ansi fg="attack-good">You feel your blow strike deep into something!</ansi> <ansi fg="crit-text">***</ansi>'
    - '<ansi fg="crit-text">***</ansi> <ansi fg="attack-good">Your blow lands with a heavy, sickening thud!</ansi> <ansi fg="crit-text">***</ansi>'
    - '<ansi fg="crit-text">***</ansi> <ansi fg="attack-good">Your arm jolts as your blow smashes home!</ansi> <ansi fg="crit-text">***</ansi>'
  deflected:
    - '<ansi fg="attack-bad">Something turns your blow aside, but you feel it land!</ansi>'
    - '<ansi fg="attack-bad">Your strike is pushed aside, but it still connects!</ansi>'
    - '<ansi fg="attack-bad">Something blocks most of your blow, but not all of it!</ansi>'
    - '<ansi fg="attack-bad">You feel your blow meet resistance, then scrape through!</ansi>'
  defended:
    - '<ansi fg="attack-bad">Your attack is turned aside by something!</ansi>'
    - '<ansi fg="attack-bad">Your blow meets something hard and stops dead!</ansi>'
    - '<ansi fg="attack-bad">Something moves away and your strike finds only air!</ansi>'
    - '<ansi fg="attack-bad">You feel your blow knocked aside in the dark!</ansi>'
  hit:
    - '<ansi fg="attack-good">You strike blindly and connect!</ansi>'
    - '<ansi fg="attack-good">Your blind swing thuds into something!</ansi>'
    - '<ansi fg="attack-good">You feel your blow land on something solid!</ansi>'
    - '<ansi fg="attack-good">You hear a grunt as your strike connects!</ansi>'
  miss:
    - '<ansi fg="yellow">You swing wildly in the darkness!</ansi>'
    - '<ansi fg="yellow">Your swing cuts through empty air!</ansi>'
    - '<ansi fg="yellow">You strike at a sound and hit nothing!</ansi>'
    - '<ansi fg="yellow">Your blow finds nothing in the dark!</ansi>'
```

`_datafiles/world/dogmud/narration/dark-combat/defender.yaml`:

```yaml
# Dark-combat lines, defender viewpoint (M6 slice 2, ledger rows 42 to 47).
# Read by a player who cannot see at all (optics, not sleep) in place of the
# composed swing line. Same rules as attacker.yaml.
viewpoint: defender
outcomes:
  fumble:
    - '<ansi fg="yellow">You hear your attacker stumble!</ansi>'
    - '<ansi fg="yellow">You hear something lurch and scrape close by!</ansi>'
    - '<ansi fg="yellow">Something stumbles near you with a clatter!</ansi>'
    - '<ansi fg="yellow">You hear a scuffle as something loses its balance!</ansi>'
  crit:
    - '<ansi fg="crit-text">***</ansi> <ansi fg="red">Something hits you hard in the dark!</ansi> <ansi fg="crit-text">***</ansi>'
    - '<ansi fg="crit-text">***</ansi> <ansi fg="red">A crushing blow slams into you out of the dark!</ansi> <ansi fg="crit-text">***</ansi>'
    - '<ansi fg="crit-text">***</ansi> <ansi fg="red">Something strikes you with terrible force!</ansi> <ansi fg="crit-text">***</ansi>'
    - '<ansi fg="crit-text">***</ansi> <ansi fg="red">Pain bursts through you as a heavy blow lands!</ansi> <ansi fg="crit-text">***</ansi>'
  deflected:
    - '<ansi fg="defense-good">You fend off something in the dark, but it still catches you!</ansi>'
    - '<ansi fg="defense-good">You block something blindly, but it still gets through!</ansi>'
    - '<ansi fg="defense-good">You turn something aside, but not quite in time!</ansi>'
    - '<ansi fg="defense-good">You feel a blow coming and soften it, but it lands!</ansi>'
  defended:
    - '<ansi fg="defense-good">You fend off something in the dark!</ansi>'
    - '<ansi fg="defense-good">You hear something coming and block it!</ansi>'
    - '<ansi fg="defense-good">You twist away from something in the dark!</ansi>'
    - '<ansi fg="defense-good">You feel a blow coming and turn it aside!</ansi>'
  hit:
    - '<ansi fg="red">Something strikes you in the dark!</ansi>'
    - '<ansi fg="red">A blow lands on you out of nowhere!</ansi>'
    - '<ansi fg="red">You feel something hit you hard!</ansi>'
    - '<ansi fg="red">Something you cannot see strikes you!</ansi>'
  miss:
    - '<ansi fg="yellow">You hear something whoosh past!</ansi>'
    - '<ansi fg="yellow">Something rushes past you in the dark!</ansi>'
    - '<ansi fg="yellow">You feel the air move as something misses you!</ansi>'
    - '<ansi fg="yellow">You hear a swing pass close by!</ansi>'
```

- [ ] **Step 2: Write the failing store tests**

`internal/darkcombat/store_test.go`:

```go
package darkcombat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func shippedDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "narration", "dark-combat")
}

func TestShippedStoreLoads(t *testing.T) {
	t.Cleanup(UnloadForTest)
	if err := LoadFrom(shippedDir(t)); err != nil {
		t.Fatalf("shipped dark-combat store: %v", err)
	}
	for _, v := range Viewpoints() {
		for _, o := range Outcomes() {
			if n := len(Pool(v, o)); n < MinVariants {
				t.Fatalf("%s/%s holds %d lines, want at least %d", v, o, n, MinVariants)
			}
		}
	}
}

func TestLine_UnloadedIsSilent(t *testing.T) {
	UnloadForTest()
	if _, ok := Line(Attacker, Hit, func(int) int { return 0 }); ok {
		t.Fatal("an unloaded store must say nothing, so callers keep their composed lines")
	}
}

func TestLine_PicksFromThePool(t *testing.T) {
	t.Cleanup(UnloadForTest)
	if err := LoadFrom(shippedDir(t)); err != nil {
		t.Fatal(err)
	}
	got, ok := Line(Defender, Miss, func(int) int { return 0 })
	if !ok || !strings.Contains(got, "You hear something whoosh past!") {
		t.Fatalf("Line(defender, miss, first) = %q, %v", got, ok)
	}
}

// Each validation rule must be able to fire. One bad file per rule.
func TestValidate_Rejects(t *testing.T) {
	good := func() *ViewpointGroup {
		g := &ViewpointGroup{Viewpoint: Attacker, Outcomes: map[Outcome][]string{}}
		for _, o := range Outcomes() {
			g.Outcomes[o] = []string{"You hit a.", "You hit b.", "You hit c.", "You hit d."}
		}
		return g
	}
	if err := good().Validate(); err != nil {
		t.Fatalf("control: a well-formed group failed: %v", err)
	}
	cases := map[string]func(g *ViewpointGroup){
		"unknown viewpoint": func(g *ViewpointGroup) { g.Viewpoint = "watcher" },
		"missing outcome":   func(g *ViewpointGroup) { delete(g.Outcomes, Crit) },
		"unknown outcome":   func(g *ViewpointGroup) { g.Outcomes["parry"] = g.Outcomes[Hit] },
		"too few":           func(g *ViewpointGroup) { g.Outcomes[Hit] = g.Outcomes[Hit][:3] },
		"token":             func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit {actee}." },
		"number":            func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit 3 times." },
		"em dash":           func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit \u2014 hard." },
		"spaced hyphen":     func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit - hard." },
		"double hyphen":     func(g *ViewpointGroup) { g.Outcomes[Hit][1] = "You hit -- hard." },
		"too wide":          func(g *ViewpointGroup) { g.Outcomes[Hit][1] = strings.Repeat("a", 81) },
	}
	for name, mutate := range cases {
		g := good()
		mutate(g)
		if err := g.Validate(); err == nil {
			t.Errorf("%s: Validate passed a bad group", name)
		}
	}
}

func TestLoadFrom_MissingViewpointFile(t *testing.T) {
	t.Cleanup(UnloadForTest)
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(shippedDir(t), "attacker.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "attacker.yaml"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadFrom(dir); err == nil {
		t.Fatal("a store with no defender file must fail to load")
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/darkcombat/ -count=1`
Expected: FAIL, package has no non-test Go files / undefined symbols.

- [ ] **Step 4: Write the package**

`internal/darkcombat/store.go`:

```go
// Package darkcombat holds the combat lines a player reads when they cannot
// see at all: one line per swing outcome, from the attacker's or the
// defender's side, written from touch and sound.
//
// Go decides WHEN a dark line replaces a composed one and which outcome it
// narrates (internal/combat, hideIdentitiesInPersonalLines). Go holds no
// wording: every line lives in
// _datafiles/world/dogmud/narration/dark-combat/<viewpoint>.yaml.
package darkcombat

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/pkg/errors"
)

// Viewpoint is whose personal line a dark line replaces.
type Viewpoint string

const (
	Attacker Viewpoint = "attacker"
	Defender Viewpoint = "defender"
)

// Outcome is one swing's result as a blind reader feels it. The buckets and
// their order are the retired replaceDarknessMessages' (M4d PR 2).
type Outcome string

const (
	Fumble    Outcome = "fumble"    // the attacker fumbled
	Crit      Outcome = "crit"      // the attacker crit
	Deflected Outcome = "deflected" // the defence won but damage still landed
	Defended  Outcome = "defended"  // the defence won clean
	Hit       Outcome = "hit"
	Miss      Outcome = "miss"
)

var allViewpoints = []Viewpoint{Attacker, Defender}

var allOutcomes = []Outcome{Fumble, Crit, Deflected, Defended, Hit, Miss}

// Viewpoints and Outcomes return copies of the vocabularies in a fixed order,
// for tests and the narration snapshot harness.
func Viewpoints() []Viewpoint { return append([]Viewpoint(nil), allViewpoints...) }
func Outcomes() []Outcome     { return append([]Outcome(nil), allOutcomes...) }

// MinVariants is the fewest lines one pool may hold. A blind fight repeats
// the same outcome many times; four keeps it from reading as one sentence.
const MinVariants = 4

const maxLineColumns = 80

var (
	ansiTag = regexp.MustCompile(`<[^>]*>`)
	// spacedHyphen is a hyphen or double hyphen standing in for a dash.
	spacedHyphen = regexp.MustCompile(`[A-Za-z,.!?'"] --? [A-Za-z]`)
)

// ViewpointGroup is one viewpoint's file.
type ViewpointGroup struct {
	Viewpoint Viewpoint             `yaml:"viewpoint"`
	Outcomes  map[Outcome][]string `yaml:"outcomes"`
}

func (g *ViewpointGroup) Id() string       { return string(g.Viewpoint) }
func (g *ViewpointGroup) Filepath() string { return string(g.Viewpoint) + ".yaml" }

// Validate fails the boot on a malformed file. Every outcome is reachable
// from combat, so a missing one would be silence in a dark fight.
func (g *ViewpointGroup) Validate() error {
	if !slices.Contains(allViewpoints, g.Viewpoint) {
		return errors.Errorf("unknown viewpoint %q", g.Viewpoint)
	}
	for o := range g.Outcomes {
		if !slices.Contains(allOutcomes, o) {
			return errors.Errorf("viewpoint %q declares unknown outcome %q", g.Viewpoint, o)
		}
	}
	for _, o := range allOutcomes {
		pool, ok := g.Outcomes[o]
		if !ok {
			return errors.Errorf("viewpoint %q is missing outcome %q", g.Viewpoint, o)
		}
		if err := narration.ValidateVariants(narration.Variants{Actor: pool}, MinVariants, narration.RoleActor); err != nil {
			return errors.Wrapf(err, "viewpoint %q outcome %q", g.Viewpoint, o)
		}
		for i, text := range pool {
			if err := validateLine(text); err != nil {
				return errors.Wrapf(err, "viewpoint %q outcome %q variant %d", g.Viewpoint, o, i)
			}
		}
	}
	return nil
}

// validateLine enforces dogmud-player-copy on a line this store owns: at most
// 80 visible columns, no numbers, no dashes, no {tokens} (nothing fills them,
// and a dark line names no one).
func validateLine(text string) error {
	plain := ansiTag.ReplaceAllString(text, "")
	if n := utf8.RuneCountInString(plain); n > maxLineColumns {
		return errors.Errorf("is %d visible columns, over %d", n, maxLineColumns)
	}
	if strings.ContainsAny(plain, "0123456789") {
		return errors.New("shows a number")
	}
	if strings.ContainsAny(plain, "\u2014\u2013") || spacedHyphen.MatchString(plain) {
		return errors.New("uses a dash")
	}
	if strings.ContainsAny(plain, "{}") {
		return errors.New("carries a token; this store fills none")
	}
	return nil
}

var loaded map[string]*ViewpointGroup

// LoadFrom loads the store from an explicit directory and returns the error.
// Tests use it because a test binary never reads config.yaml.
func LoadFrom(dir string) error {
	got, err := fileloader.LoadAllFlatFiles[string, *ViewpointGroup](dir)
	if err != nil {
		return errors.Wrap(err, "loading dark-combat lines")
	}
	for _, v := range allViewpoints {
		if got[string(v)] == nil {
			return errors.Errorf("dark-combat lines: no file for viewpoint %q", v)
		}
	}
	loaded = got
	return nil
}

// LoadDarkCombatFiles loads the store at boot and panics on any failure,
// matching lightnotice.LoadLightNoticeFiles.
func LoadDarkCombatFiles() {
	dir := string(configs.GetFilePathsConfig().DataFiles) + `/narration/dark-combat`
	if err := LoadFrom(dir); err != nil {
		panic(err)
	}
}

// UnloadForTest empties the store. Test-only by name.
func UnloadForTest() { loaded = nil }

// Pool returns one viewpoint's lines for one outcome, or nil when the store is
// unloaded.
func Pool(v Viewpoint, o Outcome) []string {
	g := loaded[string(v)]
	if g == nil {
		return nil
	}
	return g.Outcomes[o]
}

// Line picks one dark line. ok is false while the store is unloaded, which is
// what keeps every test package that never loads it on its composed lines.
func Line(v Viewpoint, o Outcome, pick narration.Picker) (string, bool) {
	pool := Pool(v, o)
	if len(pool) == 0 {
		return "", false
	}
	return narration.Render(narration.Variants{Actor: pool}, nil, pick).Actor, true
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/darkcombat/ -count=1`
Expected: PASS. Null probe: add a ninth lone `- 'x'` line to `defender.yaml`'s `crit` with the text `You hit 3 times.`, rerun, confirm `TestShippedStoreLoads` fails naming `defender` / `crit` / `shows a number`, then delete that line.

- [ ] **Step 6: Wire the boot loader**

In `main.go`, directly after `lightnotice.LoadLightNoticeFiles()` (line 1979):

```go
	darkcombat.LoadDarkCombatFiles()
```

and add `"github.com/GoMudEngine/GoMud/internal/darkcombat"` to the import block (alphabetical, beside the other `internal/` imports).

Run: `go build ./... && go vet . ./internal/darkcombat/`
Expected: no output.

- [ ] **Step 7: Write `internal/darkcombat/context.md`**

```markdown
# internal/darkcombat

The combat lines a player reads when they cannot see at all. Added by
messaging M6 slice 2 (#370 ledger rows 36 to 47), replacing the twelve Go
literals M4d PR 2 retired with `replaceDarknessMessages`.

Go decides WHEN: `internal/combat.hideIdentitiesInPersonalLines` swaps a
reader's personal lines for one dark line per `combat.SwingEvent` when that
reader's OPTICS verdict (`messaging.ParticipantSight`) is `SightNone`. Not
`ReaderSight`: that one counts a sleeper as blind, and a sleeper hit in a lit
room must not read "in the dark". Shapes readers (infravision) keep the
composed line with "a figure".

Go holds no wording: `_datafiles/world/dogmud/narration/dark-combat/`
`attacker.yaml` and `defender.yaml`, one list per `Outcome`.

| Symbol | Use |
|---|---|
| `Viewpoint` (`Attacker`, `Defender`) | whose personal line is replaced |
| `Outcome` (`Fumble`, `Crit`, `Deflected`, `Defended`, `Hit`, `Miss`) | the swing's bucket |
| `Viewpoints()`, `Outcomes()` | fixed-order vocabularies |
| `ViewpointGroup` | one file; `Validate` fails boot on a missing outcome, fewer than `MinVariants` (4) lines, a token, a number, a dash, or over 80 visible columns |
| `LoadFrom(dir) error`, `LoadDarkCombatFiles()` | test and boot loaders |
| `Pool(v, o)`, `Line(v, o, pick)` | read; both are empty while unloaded, so callers keep their composed lines |
| `UnloadForTest()` | test reset |

The bucketing (which `Outcome` a swing is) lives in `internal/combat`
(`darkOutcome`), because this package cannot import `combat`.
```

- [ ] **Step 8: Commit**

```bash
git add internal/darkcombat/store.go internal/darkcombat/store_test.go internal/darkcombat/context.md _datafiles/world/dogmud/narration/dark-combat/attacker.yaml _datafiles/world/dogmud/narration/dark-combat/defender.yaml main.go
git commit -m "feat(darkcombat): store of dark-combat lines for blind readers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Blind readers read dark lines

**Files:**
- Modify: `internal/combat/combat_helpers.go:35-83` (`combatContext`, `newCombatContext`)
- Modify: `internal/combat/combat.go` (`hideIdentitiesInPersonalLines`, new `darkOutcome`, `darkLines`)
- Modify: `internal/combat/context.md`
- Test: `internal/combat/dark_combat_lines_test.go` (create)

- [ ] **Step 1: Write the failing tests**

`internal/combat/dark_combat_lines_test.go`:

```go
package combat

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/darkcombat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

func loadDarkCombatForTest(t *testing.T) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "narration", "dark-combat")
	if err := darkcombat.LoadFrom(dir); err != nil {
		t.Fatalf("loading dark-combat store: %v", err)
	}
	t.Cleanup(darkcombat.UnloadForTest)
}

// The bucket order is the retired replaceDarknessMessages', except that
// "defended" reads the per-swing Defended, not the sticky DefenseUsed.
func TestDarkOutcome_Buckets(t *testing.T) {
	cases := []struct {
		name string
		se   SwingEvent
		want darkcombat.Outcome
		ok   bool
	}{
		{"double fumble says nothing", SwingEvent{DoubleFumble: true, Fumble: true}, "", false},
		{"fumble", SwingEvent{Fumble: true}, darkcombat.Fumble, true},
		{"crit", SwingEvent{Hit: true, Crit: true, Damage: 9}, darkcombat.Crit, true},
		{"deflected", SwingEvent{Defended: true, Damage: 3}, darkcombat.Deflected, true},
		{"defence crit is defended even with damage", SwingEvent{Defended: true, DefenseCrit: true, Damage: 3}, darkcombat.Defended, true},
		{"defended clean", SwingEvent{Defended: true}, darkcombat.Defended, true},
		{"hit", SwingEvent{Hit: true, Damage: 4}, darkcombat.Hit, true},
		{"miss", SwingEvent{}, darkcombat.Miss, true},
		{"sticky DefenseUsed alone is not a defence", SwingEvent{Hit: true, Damage: 4, DefenseUsed: "dodge"}, darkcombat.Hit, true},
	}
	for _, c := range cases {
		got, ok := darkOutcome(c.se)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: darkOutcome = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestDarkLines_CategoriesMatchTheRetiredFunction(t *testing.T) {
	loadDarkCombatForTest(t)
	events := []SwingEvent{{Defended: true}, {Hit: true, Damage: 2}, {DoubleFumble: true, Fumble: true}}
	got := darkLines(events, darkcombat.Defender)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2 (the double fumble says nothing)", len(got))
	}
	if got[0].Category != messaging.CategoryDodge || got[1].Category != messaging.CategoryHitMelee {
		t.Fatalf("categories = %v, %v; want CategoryDodge, CategoryHitMelee", got[0].Category, got[1].Category)
	}
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Defended), got[0].Text) {
		t.Fatalf("line 0 %q is not from defender/defended", got[0].Text)
	}
}

func darkResult() *AttackResult {
	return &AttackResult{
		SwingEvents:      []SwingEvent{{Hit: true, Damage: 5}},
		MessagesToSource: []TaggedMessage{{Category: messaging.CategoryHitMelee, Text: "Reading their stance, you strike Shade with practiced precision!"}},
		MessagesToTarget: []TaggedMessage{{Category: messaging.CategoryHitMelee, Text: "Grimwald slashes you with their Rusted Cleaver!"}},
	}
}

func TestHideIdentities_BlindAttackerReadsADarkLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, true) // blind
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), false)

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if len(r.MessagesToSource) != 1 || !slices.Contains(darkcombat.Pool(darkcombat.Attacker, darkcombat.Hit), r.MessagesToSource[0].Text) {
		t.Fatalf("blind attacker read %q, want an attacker/hit dark line", r.MessagesToSource)
	}
	if strings.Contains(r.MessagesToTarget[0].Text, "something") || !strings.Contains(r.MessagesToTarget[0].Text, "Grimwald") {
		t.Fatalf("the sighted defender's line changed: %q", r.MessagesToTarget[0].Text)
	}
}

func TestHideIdentities_DarkRoomDefenderReadsADarkLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, false)
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(0), false) // both blind in the dark

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Hit), r.MessagesToTarget[0].Text) {
		t.Fatalf("defender in the dark read %q, want a defender/hit dark line", r.MessagesToTarget[0].Text)
	}
}

func TestHideIdentities_ShapesReaderKeepsTheComposedLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, true, false, false) // infrared
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(0), false)
	if ctx.sourceOptics != messaging.SightShapes {
		t.Fatalf("control: infrared attacker optics = %v, want SightShapes", ctx.sourceOptics)
	}

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if got := r.MessagesToSource[0].Text; !strings.Contains(got, "practiced precision") || strings.Contains(got, "Shade") {
		t.Fatalf("shapes attacker should read the composed line with the name hidden, got %q", got)
	}
}

func TestHideIdentities_SleeperInALitRoomKeepsTheComposedLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, false)
	def := verdictObserver(t, false, false, true, false) // asleep
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), true)

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	got := r.MessagesToTarget[0].Text
	if slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Hit), got) {
		t.Fatalf("a sleeper in a lit room read a dark line: %q", got)
	}
	if strings.Contains(got, "Grimwald") || !strings.Contains(strings.ToLower(got), "something") {
		t.Fatalf("the sleeper should read the composed line with \"something\", got %q", got)
	}
}

func TestHideIdentities_NoSwingsKeepsTheComposedLines(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, true)
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), false)

	r := darkResult()
	r.SwingEvents = nil
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if !strings.Contains(r.MessagesToSource[0].Text, "practiced precision") {
		t.Fatalf("with no swing to bucket the composed line must stay, got %q", r.MessagesToSource[0].Text)
	}
}

// A zero-valued combatContext must never trigger dark lines: dozens of
// existing tests build one by hand.
func TestHideIdentities_ZeroContextIsSighted(t *testing.T) {
	loadDarkCombatForTest(t)
	r := darkResult()
	hideIdentitiesInPersonalLines(r, &characters.Character{Name: "Grimwald"}, &characters.Character{Name: "Shade"}, combatContext{})
	if !strings.Contains(r.MessagesToSource[0].Text, "practiced precision") {
		t.Fatalf("zero context replaced the line: %q", r.MessagesToSource[0].Text)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/combat/ -run 'TestDarkOutcome|TestDarkLines|TestHideIdentities_' -count=1`
Expected: FAIL, `undefined: darkOutcome`, `darkLines`, `ctx.sourceOptics`.

- [ ] **Step 3: Add the optics fields**

In `combat_helpers.go`, in `combatContext`, after `targetSight messaging.SightDecision`:

```go
	// sourceOptics and targetOptics are messaging.ParticipantSight: optics
	// only, NO sleep test. They decide one thing: whether a side's personal
	// lines become dark lines (internal/darkcombat). The sight fields above
	// cannot, because ReaderSight counts a sleeper as blind and a sleeper hit
	// in a lit room must not read "in the dark" (the 2026-08-31 defect).
	sourceOptics messaging.SightDecision
	targetOptics messaging.SightDecision
```

In `newCombatContext`'s returned literal, after `targetSight: ...`:

```go
		sourceOptics: messaging.ParticipantSight(source, room),
		targetOptics: messaging.ParticipantSight(target, room),
```

Extend the function's doc comment with one sentence: `The optics pair is ParticipantSight, the dark-line gate, which a sleeper does not trip.`

- [ ] **Step 4: Add the bucketing and the swap**

In `combat.go`, after `swingEventFor`:

```go
// darkOutcome buckets one swing for a reader who cannot see, in the order the
// retired replaceDarknessMessages used. ok is false for a double fumble,
// which is narrated by its own line and gets no dark line. "Defended" reads
// the swing's own Defended, not AttackResult.DefenseUsed, which is sticky.
func darkOutcome(se SwingEvent) (darkcombat.Outcome, bool) {
	switch {
	case se.DoubleFumble:
		return "", false
	case se.Fumble:
		return darkcombat.Fumble, true
	case se.Crit:
		return darkcombat.Crit, true
	case se.Defended && !se.DefenseCrit && se.Damage > 0:
		return darkcombat.Deflected, true
	case se.Defended:
		return darkcombat.Defended, true
	case se.Hit:
		return darkcombat.Hit, true
	}
	return darkcombat.Miss, true
}

// darkLines is one dark line per swing for viewpoint v. A clean defence is
// CategoryDodge; every other bucket, a deflected swing included, is
// CategoryHitMelee, so the damage-to-viewer verbosity floor still applies.
// Empty when the store is unloaded or there is no swing to narrate.
func darkLines(events []SwingEvent, v darkcombat.Viewpoint) []TaggedMessage {
	var out []TaggedMessage
	for _, se := range events {
		o, ok := darkOutcome(se)
		if !ok {
			continue
		}
		text, ok := darkcombat.Line(v, o, narration.DefaultPicker)
		if !ok {
			return nil
		}
		cat := messaging.CategoryHitMelee
		if o == darkcombat.Defended {
			cat = messaging.CategoryDodge
		}
		out = append(out, TaggedMessage{Category: cat, Text: text})
	}
	return out
}
```

At the top of `hideIdentitiesInPersonalLines`'s body, before the first `for`:

```go
	// M6 slice 2: a reader who cannot see at all reads one dark line per
	// swing in place of every composed personal line. Optics, not the sight
	// fields, so a sleeper keeps the composed line. Shapes readers keep it too.
	if ctx.sourceOptics == messaging.SightNone {
		if dark := darkLines(result.SwingEvents, darkcombat.Attacker); len(dark) > 0 {
			result.MessagesToSource = dark
		}
	}
	if ctx.targetOptics == messaging.SightNone {
		if dark := darkLines(result.SwingEvents, darkcombat.Defender); len(dark) > 0 {
			result.MessagesToTarget = dark
		}
	}
```

Add imports `github.com/GoMudEngine/GoMud/internal/darkcombat` and `github.com/GoMudEngine/GoMud/internal/narration` to `combat.go` if absent. Update the function's doc comment: replace the sentence about "rather than replaceDarknessMessages' approach of throwing the composed line away" context with a short paragraph naming the dark-line swap and the optics gate.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/combat/... ./internal/hooks/... . -count=1`
Expected: PASS. Null probes, one at a time, restoring after each:
1. Gate on `ctx.sourceSight` instead of `ctx.sourceOptics`: `TestHideIdentities_SleeperInALitRoomKeepsTheComposedLine` must stay green only for the target branch, so also switch the target gate to `ctx.targetSight` and confirm the sleeper test FAILS.
2. Change `case se.Defended:` to `case se.DefenseUsed != "":` and confirm `TestDarkOutcome_Buckets` fails on "sticky DefenseUsed alone is not a defence".

- [ ] **Step 6: Update `internal/combat/context.md`**

Find the section on darkness and identity hiding (`grep -n "hideIdentitiesInPersonalLines\|HideNames" internal/combat/context.md`) and add:

```markdown
**Dark lines (M6 slice 2).** When a reader's optics verdict
(`combatContext.sourceOptics` / `targetOptics`, `messaging.ParticipantSight`)
is `SightNone`, `hideIdentitiesInPersonalLines` replaces that side's personal
lines with one `internal/darkcombat` line per `SwingEvent` (`darkOutcome`,
`darkLines`). A sleeper trips `ReaderSight` but not optics, so it keeps the
composed line with "something". `SwingEvent.Defended` is the per-swing
defensive win the buckets read; `AttackResult.DefenseUsed` is sticky across a
round and must not be used for per-swing questions.
```

- [ ] **Step 7: Commit**

```bash
git add internal/combat/combat.go internal/combat/combat_helpers.go internal/combat/dark_combat_lines_test.go internal/combat/context.md
git commit -m "feat(combat): a reader who cannot see reads dark-combat lines

Closes ledger rows 36 to 47 (#370).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Dark-combat golden

**Files:**
- Modify: `internal/narration/snapshot_test.go` (new subtest + builder beside `buildLightNoticesGolden`)
- Create: `internal/narration/testdata/stores/dark_combat.golden` (via `-update`)

- [ ] **Step 1: Add the builder and subtest**

Beside the `light_notices` subtest (line 854):

```go
	t.Run("dark_combat", func(t *testing.T) {
		checkGolden(t, "dark_combat.golden", buildDarkCombatGolden(t))
	})
```

Beside `buildLightNoticesGolden`:

```go
func buildDarkCombatGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "narration", "dark-combat")
	groups, err := fileloader.LoadAllFlatFiles[string, *darkcombat.ViewpointGroup](dir)
	if err != nil {
		t.Fatalf("loading %s: %v", dir, err)
	}
	if len(groups) == 0 {
		t.Fatal("no dark-combat files loaded; the golden would be vacuous")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# dark-combat store snapshot (M6 slice 2)\n")
	fmt.Fprintf(&b, "# dimensions: viewpoint x outcome x index\n\n")
	for _, v := range darkcombat.Viewpoints() {
		g := groups[string(v)]
		if g == nil {
			t.Fatalf("viewpoint %q has no file", v)
		}
		for _, o := range darkcombat.Outcomes() {
			for i, line := range g.Outcomes[o] {
				fmt.Fprintf(&b, "%s|%s|%d= %s\n", v, o, i, line)
			}
		}
	}
	return b.String()
}
```

Add the `internal/darkcombat` import.

- [ ] **Step 2: Record and verify**

```bash
go test ./internal/narration/ -run 'TestSnapshotStores/dark_combat' -update -count=1
go test ./internal/narration/ -count=1
```

Expected: the first writes `dark_combat.golden`; the second passes. Read the golden: 48 rows, 24 per viewpoint.

- [ ] **Step 3: Commit**

```bash
git add internal/narration/snapshot_test.go internal/narration/testdata/stores/dark_combat.golden
git commit -m "test(narration): golden for the dark-combat store

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: One pool per special-move event (row 56)

**Files:**
- Modify: all 12 files with `player_*` keys under `_datafiles/world/dogmud/narration/special-moves/` (bash, drain, gore, grapple, kick, maul, pounce, rake, shoot, throttle, throw, trip)
- Modify: `internal/usercommands/{bash,drain,gore,grapple,kick,maul,pounce,rake,shoot,throttle,throw,trip}.go`
- Modify: root `move_narration_migration_guard_test.go`, `internal/usercommands/usercommands_test.go`, `internal/usercommands/cast_interrupt_sound_test.go`, `internal/usercommands/shoot_vertical_narration_test.go`, `internal/hooks/hidden_player_area_drain_test.go`, `internal/mobs/mobs_test.go`, `internal/actions/command_readiness_drift_test.go` (only where they name a `player_*` event key; check each hit, some `player_` matches are unrelated, e.g. `player_enter`)
- Delete: `internal/usercommands/special_move_net_test.go`, `internal/usercommands/testdata/pre_migration_literals.json`, `internal/mobcommands/special_move_net_test.go`, `internal/mobcommands/testdata/pre_migration_literals.json`, `tools/move_narration_net.py`
- Modify: `internal/narration/testdata/stores/special_moves.golden` (re-record)
- Modify: `internal/movenarration/context.md`

Shapes today (`a`=actor, `e`=actee, `o`=observer, `r`=remote_observer, digits are pool sizes):

| Verb | Event | Mob | Player |
|---|---|---|---|
| bash | hit, knockdown, miss, partial | e1 o1 | a1 e1 o1 |
| drain | hit | e1 o1 | a4 e4 o4 |
| drain | miss / partial | e1 o1 | a3 e3 o3 / a2 e2 o2 |
| drain | hit_heal / partial_heal | none | a3 / a2 (actor only) |
| gore, pounce | hit, knockdown / miss / partial | e1 o1 | a4 e4 o4 / a3 / a2 |
| grapple | fail, success | e1 o1 | a1 e1 o1 |
| grapple | prone_penalty, defense_exposed | none | a1 (actor only) |
| grapple | crit_failure, disarm | a1 e1 o1 | none |
| kick | stomp/knee/standard hit, knockdown, miss, partial | e1 o1 | a1 to a6 matched |
| maul, rake | hit / miss / partial | e1 o1 | a5 / a3 / a2 matched |
| shoot | hit, miss, partial | e1 | a1 e1 |
| shoot | fire_announce, fire_depart | o1 | o1 |
| shoot | arrival_{known,unknown}_{hit,miss,partial} | r1 | r1 |
| shoot | surprise_hit, surprise_miss | none | a1 |
| throttle | hit / miss / partial | e1 o1 | a4 / a3 / a2 matched |
| throttle | cast_interrupt | e1 | a1 e1 o1 |
| throw | cast_interrupt, fumble, hurl | none | a1 o1 |
| throw | partial_hit | none | a1 |
| trip | tailsweep/trip hit, knockdown, miss, partial | e1 o1 | a1 e1 o1 |
| charge, hamstring, trip whiff_on_prone | all | mob only | none |

- [ ] **Step 1: Merge rule (apply per event)**

For each event that exists as both `X` (mob) and `player_X`:
1. The merged event `X` keeps the mob entries first, then appends the player entries, index by index, in every role.
2. Every role present in EITHER side must be present at every index of the merged event. Where the mob side lacks a role the player side has, author the missing line for each mob index:
   - **actor** (missing on every mob event that has a player twin): a "You ..." line describing the same moment as that index's actee and observer, using `{actee}` and the same `{damage}` / `{heal}` / `{position}` tokens the actee line carries. Count: bash 4, drain 3, gore 4, grapple 2, kick 10, maul 3, pounce 4, rake 3, shoot 3, throttle 4, trip 8, about 48 lines.
   - **observer** on `throttle/cast_interrupt` index 0: one room line for the mob's interrupt, matching the player line's style.
3. A `player_X` with no mob `X` is renamed `X` unchanged.
4. Mob-only events are unchanged.
5. Delete the `player_*` comment blocks that explain option B and the squaring; replace with one comment at the top of each file: `# Mobs and players draw from one pool per event (M6 slice 2, ledger row 56). Variant N of every role describes one moment.`

Authored lines obey `dogmud-player-copy` and the Task 7 word list (no "clip", "slip", "sliver", "footing", "goes wide", "overrun").

Do the files one verb at a time. After each verb run:

```bash
go test ./internal/movenarration/ -count=1
```

Expected: PASS (the store's `Validate` fails boot on unequal role lengths, so a missed actor line shows up here).

- [ ] **Step 2: Behaviour change to record, not avoid**

Mob `sendMoveEvent` sends every authored role. After the merge:
- `throttle/cast_interrupt` gains an observer role, so a MOB's throttle that breaks a cast is now told to the room, as the player's already is. This is the point of row 56; record it in the PR body.
- Mob events that gain an actor role send that line to the mob actor, which has no client. No change in play.

Update the `intentionallySilentRoles` map in `move_narration_migration_guard_test.go`:
- Remove every `player_*` entry.
- Remove `"throttle/cast_interrupt"` (it now has an observer).
- Keep the `shoot/*` entries (their shapes do not change: `shoot/hit` etc. become actor plus actee, still no observer). Edit their text where it names `player_fire_announce` to name `fire_announce`.
- Keep `grapple/prone_penalty` and `grapple/defense_exposed` (renamed from `player_`), and add `drain/hit_heal`, `drain/partial_heal`, `shoot/surprise_hit`, `shoot/surprise_miss`, `throw/partial_hit` under their new names with the reasons the `player_` entries carried (read each reason off the old entry or the call site comment).

- [ ] **Step 3: Switch the Go callers**

In each `internal/usercommands/<verb>.go`, replace every `"player_X"` / `` `player_X` `` event key with `"X"`. The complete list (from grep at `61da8280b`):

```
bash.go:97,99,105,127   drain.go:84,95,101,119,130   gore.go:85,87,93,114
grapple.go:130,139,152,156   kick.go:100,103,107,109,118,120,122,148,151,154
maul.go:84,89,110   pounce.go:92,94,100,121   rake.go:84,89,110
shoot.go:419,421,423,545,562,590,592,594,596,598,600,646,662
throttle.go:96,104,111,132   throw.go:333,365,395,443
trip.go:106,108,112,114,124,127,160,162
```

Also update comments that name `player_*` events (`drain.go:90`, `throttle.go:21,26`, `throw.go:158,316`) to the plain names.

Then:

```bash
grep -rn 'player_\(hit\|miss\|partial\|knockdown\|success\|fail\|prone\|defense\|stomp\|knee\|standard\|fire\|arrival\|surprise\|cast\|hurl\|fumble\|tailsweep\|trip\)' internal/usercommands --include=*.go
```

Expected: no output.

- [ ] **Step 4: Retire the byte-identity nets**

The two `special_move_net_test.go` files proved the M4e migration changed no byte. M6 changes wording on purpose, and `special_moves.golden` stays as the record of what ships.

```bash
git rm internal/usercommands/special_move_net_test.go internal/usercommands/testdata/pre_migration_literals.json internal/mobcommands/special_move_net_test.go internal/mobcommands/testdata/pre_migration_literals.json tools/move_narration_net.py
```

Then remove the `tools/move_narration_net.py` row from `docs/README.md` (grep for it) or mark it retired in place if the README lists retired tools that way (match the surrounding convention).

- [ ] **Step 5: Fix the remaining tests**

```bash
go test . ./internal/usercommands/... ./internal/mobcommands/... ./internal/hooks/... ./internal/mobs/... ./internal/actions/... -count=1
```

For each failure that names a `player_*` event key, rename the key. Do not change any assertion's meaning. Any failure that is NOT a key rename gets reported to the controller before it is touched.

- [ ] **Step 6: Re-record the special-moves golden**

```bash
go test ./internal/narration/ -run 'TestSnapshotStores/special_moves' -update -count=1
go test ./internal/narration/ -count=1
git diff --stat internal/narration/testdata/stores/special_moves.golden
```

Read the diff: `player_*` rows become plain-key rows, mob events gain actor rows and player indices. No row's text may change in this task except row 58 (Task 6) and lines changed later; if any existing line's text changed here, it is a merge mistake.

- [ ] **Step 7: Update `internal/movenarration/context.md`**

Replace the section describing `player_*` keys and option B with: mobs and players share one pool per event (M6 slice 2, row 56); a merged event lists mob variants then player variants; every role present is present at every index; the mob sender sends every authored role, so a role added for the player twin is also sent for the mob.

- [ ] **Step 8: Gate and commit**

```bash
go build ./... && go test . ./internal/... -count=1
git add _datafiles/world/dogmud/narration/special-moves/*.yaml internal/usercommands/bash.go internal/usercommands/drain.go internal/usercommands/gore.go internal/usercommands/grapple.go internal/usercommands/kick.go internal/usercommands/maul.go internal/usercommands/pounce.go internal/usercommands/rake.go internal/usercommands/shoot.go internal/usercommands/throttle.go internal/usercommands/throw.go internal/usercommands/trip.go move_narration_migration_guard_test.go internal/narration/testdata/stores/special_moves.golden internal/movenarration/context.md docs/README.md
git add <each test file Step 5 changed, by name>
git commit -m "feat(narration): mobs and players share one pool per special move

Closes ledger row 56 (#370). Retires the M4e byte-identity nets.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Pairing audit and rows 57, 58, 59

**Files:**
- Modify: special-move YAML files as the audit finds
- Create: `docs/superpowers/audits/2026-10-12-special-move-pairing-audit.md`
- Modify: `internal/narration/testdata/stores/special_moves.golden`

- [ ] **Step 1: Row 58**

In `drain.yaml`, the line `'{actor}''''s touch drains you, pulling vitality through their grasp! ...'` becomes `'{actor}''s touch drains you, pulling vitality through their grasp! ...'` (keep the rest of the line byte for byte).

- [ ] **Step 2: Audit every index**

For every event of every one of the 14 files, and every index, read the roles side by side. Print them with this read-only helper (scratchpad, not committed):

```bash
python -I - "_datafiles/world/dogmud/narration/special-moves" <<'EOF'
import sys, glob, os, yaml
for f in sorted(glob.glob(sys.argv[1] + "/*.yaml")):
    d = yaml.safe_load(open(f, encoding="utf-8"))
    for ev, roles in d["events"].items():
        n = max(len(roles.get(r) or []) for r in ("actor", "actee", "observer", "remote_observer"))
        for i in range(n):
            print(f"== {os.path.basename(f)[:-5]}/{ev}/{i}")
            for r in ("actor", "actee", "observer", "remote_observer"):
                p = roles.get(r) or []
                if p:
                    print(f"   {r:15} {p[i]}")
EOF
```

A row passes when (a) every role describes the same moment and (b) a room line (observer or remote_observer) states the same outcome as the personal lines: a hit or partial room line says the blow landed; a miss says it missed. Fix every failing row by reordering lines within the event or rewriting the room line; author a line only where none fits.

Known failures that must be fixed:
- **Row 57, `drain`:** the room line that reads "tears into ... feeds hungrily!" belongs at the index whose actor and actee read "tear into and feed"; move it there, and move the displaced line to the index whose actor reads "touch drains ... pulling vitality".
- **Row 59, `kick` (player standard hit, now `standard_hit` after the merge):** reorder the observer lines so each matches its actor and actee; author the missing observer line for the actor "Your boot connects with {actee}!" moment (for example `{actor}'s boot connects with {actee}!`); the observer "{actor} snaps a kick at {actee}!" states no outcome and is rewritten to state the hit (for example `{actor} snaps a kick into {actee}!`); fix the actee drift at index 4 the same way.

- [ ] **Step 3: Write the audit record**

`docs/superpowers/audits/2026-10-12-special-move-pairing-audit.md`: a header (date, master SHA, rule (a) and (b) above), then one table per verb with columns `event | index | verdict (ok / reordered / rewritten / authored) | note`. Every index of every event gets a row. End with a count line: rows checked, rows fixed.

- [ ] **Step 4: Re-record and gate**

```bash
go test ./internal/movenarration/ -count=1
go test ./internal/narration/ -run 'TestSnapshotStores/special_moves' -update -count=1
go test . ./internal/narration/... ./internal/usercommands/... ./internal/mobcommands/... -count=1
```

Expected: PASS. Read the golden diff: only reorders, the row 58 apostrophe, and the audited rewrites.

- [ ] **Step 5: Commit**

```bash
git add _datafiles/world/dogmud/narration/special-moves/*.yaml docs/superpowers/audits/2026-10-12-special-move-pairing-audit.md internal/narration/testdata/stores/special_moves.golden
git commit -m "fix(narration): every special-move index describes one moment

Closes ledger rows 57, 58, 59 (#370).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Plain words for idioms (rows 48 to 53)

**Files:**
- Modify: `_datafiles/world/dogmud/narration/special-moves/*.yaml`
- Modify: `_datafiles/world/dogmud/combat-messages/*.yaml`, `_datafiles/world/dogmud/defense-messages/*.yaml` (where hits exist)
- Create: `combat_idiom_guard_test.go` (repo root)
- Modify: goldens `special_moves.golden`, `combat_messages.golden`, `defense_messages.golden`

- [ ] **Step 1: Write the failing guard**

`combat_idiom_guard_test.go`:

```go
package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// M6 slice 2 (ledger rows 48 to 53, owner 2026-10-12): six idioms that do not
// translate for players reading English as a second language are retired
// from combat text. Replacements: barely hits, misses / flies past, dodge
// most of it, hurts a little, stay on your feet, lose your balance, runs
// over / runs right past.
var retiredCombatIdiom = regexp.MustCompile(`(?i)\bclip(s|ped)?\b|goes wide|\bslips? most\b|\bsliver\b|\bfooting\b|\boverruns?\b|carries straight past`)

func TestCombatTextHasNoRetiredIdioms(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "_datafiles", "world", "dogmud")
	dirs := []string{"combat-messages", "defense-messages", filepath.Join("narration", "special-moves")}
	scanned := 0
	for _, d := range dirs {
		files, err := filepath.Glob(filepath.Join(root, d, "*.yaml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no yaml under %s (err %v); the guard would check nothing", d, err)
		}
		for _, f := range files {
			fh, err := os.Open(f)
			if err != nil {
				t.Fatal(err)
			}
			sc := bufio.NewScanner(fh)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				scanned++
				if m := retiredCombatIdiom.FindString(line); m != "" {
					t.Errorf("%s:%d uses retired idiom %q: %s", f, n, m, strings.TrimSpace(line))
				}
			}
			fh.Close()
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no lines")
	}
}
```

Run: `go test . -run TestCombatTextHasNoRetiredIdioms -count=1`
Expected: FAIL listing every current hit (about 120 lines).

- [ ] **Step 2: Rewrite each hit**

Use the owner-approved word list:

| Now | Becomes |
|---|---|
| clips you / gets clipped / still clips you / clip X | barely hits you / is barely hit / still barely hits you / barely hit X |
| shot goes wide / blow goes wide / attack goes wide | shot misses (or flies past {actee}) / blow misses / attack misses |
| you slip most of it | you dodge most of it |
| still catches a sliver of you | still hurts you a little |
| keep your footing / keep their footing | stay on your feet / stay on their feet |
| lose your footing / lose their footing / lose footing | lose your balance / lose their balance |
| read(s) X's footing | watch(es) X's stance |
| overruns them / carries straight past | runs over them / runs right past them |

Rewrite the sentence around the plain word when a straight swap reads badly. Keep every line at its index. Keep each line within 80 visible columns after tokens are filled with a typical name (check the longest by eye against its siblings). Use the Edit tool for every change.

- [ ] **Step 3: Run the guard and the goldens**

```bash
go test . -run TestCombatTextHasNoRetiredIdioms -count=1
go test ./internal/narration/ -run 'TestSnapshotStores/(special_moves|combat_messages|defense_messages)' -update -count=1
go test . ./internal/narration/... ./internal/combat/... ./internal/movenarration/... -count=1
```

Expected: all PASS. Null probe: put `clips` back into one line, confirm the guard names that file and line, restore.

- [ ] **Step 4: Commit**

```bash
git add combat_idiom_guard_test.go _datafiles/world/dogmud/narration/special-moves/*.yaml _datafiles/world/dogmud/combat-messages/*.yaml internal/narration/testdata/stores/special_moves.golden internal/narration/testdata/stores/combat_messages.golden
git add <defense-messages files and golden only if changed>
git commit -m "copy(combat): plain words for six idioms

Closes ledger rows 48 to 53 (#370).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: No dashes in combat and narration text (row 55, PR 1 part)

**Files:**
- Create: `prose_dash_guard_test.go` (repo root)
- Modify: `_datafiles/world/dogmud/narration/**/*.yaml` (9 em dashes, 2 spaced hyphens)
- Modify: `_datafiles/world/dogmud/messaging/*.yaml` (109 lines, 99 in `grapple_outcomes.yaml`)
- Modify: `_datafiles/world/dogmud/combat-messages/*.yaml` (171 spaced hyphens)
- Modify: goldens that cover those stores (`special_moves`, `combat_messages`, `messaging_grapple`, `light_notices` if touched)

- [ ] **Step 1: Write the failing guard**

`prose_dash_guard_test.go`:

```go
package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// House style (dogmud-player-copy): no em dash, no en dash, and no spaced
// hyphen or spaced double hyphen (" - ", " -- ") standing in for one, in
// prose a player reads. M6 slice 2 (ledger
// row 55). Quests, dialogue and conversations are exempt until the quest and
// dialogue rework arc (#375).
//
// proseDashDirs grows in PR 2 to add rooms and mobs.
var proseDashDirs = []string{
	"combat-messages",
	"defense-messages",
	"narration",
	"messaging",
}

var (
	emOrEnDash   = regexp.MustCompile("[\u2014\u2013]")
	spacedHyphen = regexp.MustCompile(`[A-Za-z,.!?'"] --? [A-Za-z]`)
)

func TestPlayerProseHasNoDashes(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "_datafiles", "world", "dogmud")
	scanned := 0
	for _, d := range proseDashDirs {
		base := filepath.Join(root, d)
		if _, err := os.Stat(base); err != nil {
			t.Fatalf("%s: %v; the guard would check nothing there", base, err)
		}
		err := filepath.WalkDir(base, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() || filepath.Ext(path) != ".yaml" {
				return err
			}
			fh, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fh.Close()
			sc := bufio.NewScanner(fh)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			n := 0
			for sc.Scan() {
				n++
				line := sc.Text()
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				scanned++
				if emOrEnDash.MatchString(line) || spacedHyphen.MatchString(line) {
					t.Errorf("%s:%d uses a dash: %s", path, n, strings.TrimSpace(line))
				}
			}
			return sc.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no lines")
	}
}
```

Run: `go test . -run TestPlayerProseHasNoDashes -count=1`
Expected: FAIL listing about 292 lines (narration 12, messaging 109, combat-messages 171). Null probe for the double-hyphen form: the narration hit with ` -- ` must be among them. If a hit is not prose (a YAML key or an ID), report it to the controller rather than weakening the regex.

- [ ] **Step 2: Rewrite each hit by hand**

Rule: read the sentence, then choose a period, a comma, a colon, or "and". Examples:
- `Your horned charge carries you past {actee} — they step aside!` → `Your horned charge carries you past {actee}. They step aside!`
- `Their stolen vitality flows into you — {heal}.` → `Their stolen vitality flows into you: {heal}.`
- `opens {actee} completely - you ANNIHILATE them!` → `opens {actee} completely, and you ANNIHILATE them!`

`grapple_outcomes.yaml` uses folded multi-line strings (`"...\` continuations); keep the folding and the indentation, change only the dash and its spacing. Keep every line at its index.

- [ ] **Step 3: Goldens and gate**

```bash
go test . -run TestPlayerProseHasNoDashes -count=1
go test ./internal/narration/ -run TestSnapshotStores -update -count=1
git diff --stat internal/narration/testdata/stores/
go test . ./internal/... -count=1
```

Expected: PASS. Read each changed golden: only dash rewrites. Null probe: put one `—` back, confirm the guard names the file and line, restore.

- [ ] **Step 4: Commit**

```bash
git add prose_dash_guard_test.go internal/narration/testdata/stores/*.golden
git add <each changed yaml under narration/, messaging/, combat-messages/ by name>
git commit -m "copy: no dashes in combat, narration or messaging text

Part of ledger row 55 (#370).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: No dashes in player-facing Go text

**Files (each line verified at `61da8280b`):**

| File | Lines |
|---|---|
| `internal/caravan/arrival_listener.go` | 244 |
| `internal/ferry/controller.go` | 104 |
| `internal/hooks/companion_summon.go` | 102, 240 |
| `internal/hooks/NewRound_AutoHeal.go` | 118 |
| `internal/hooks/NewRound_Bloom.go` | 97 |
| `internal/hooks/pinnacle_tick.go` | 359 |
| `internal/hooks/spell_foldrecall.go` | 66, 86 |
| `internal/items/aging.go` | 109, 115, 117, 127, 139, 141 |
| `internal/mutations/describe.go` | 62, 139, 141, 143, 145, 149, 151, 153 |
| `internal/rooms/instances.go` | 179, 199 |
| `internal/usercommands/achievements.go` | 54, 80, 83 |
| `internal/usercommands/ban.go` | 93 |
| `internal/usercommands/channel.go` | 87 |
| `internal/usercommands/craft.go` | 176, 361, 478, 508, 646, 650 |
| `internal/usercommands/enchant_slot.go` | 44 |
| `internal/usercommands/guild.go` | 96, 168, 185 |
| `internal/usercommands/inbox.go` | 56 |
| `internal/usercommands/mail.go` | 201 |
| `internal/usercommands/petition.go` | 30, 37, 50, 73 |
| `internal/usercommands/picklock.go` | 51 |
| `internal/usercommands/renameself.go` | 89 |
| `internal/usercommands/set.go` | 430 |
| `internal/usercommands/skill.disenchant.go` | 23, 73 |
| `modules/auctions/auctions.go` | 434, 952 |

Spaced double hyphen (` -- `), same treatment:

| File | Lines |
|---|---|
| `internal/actions/drink.go` | 370 |
| `internal/actions/sleeping_target.go` | 54 |
| `internal/actions/track.go` | 126 |
| `internal/hooks/RoomChange_ShadowFollow.go` | 20 |
| `internal/hooks/StorageFee_MonthlyCharge.go` | 69 |
| `internal/mutations/describe.go` | 54 |
| `internal/skills/skills.go` | 54, 58, 60 |
| `internal/usercommands/list.go` | 115 |
| `internal/usercommands/start.go` | 98 |
| `internal/usercommands/throw.go` | 276 |

`internal/usercommands/admin.fact.go:158` is a CLI `--` separator in an admin usage line, not prose; leave it.

Excluded on purpose: log lines (`mudlog.*`), error values, validator and builder output (`internal/behaviortree`, `modules/gmcp`, `internal/mapper`, `internal/migration`, schedule/patrol loaders), admin commands (`admin.*.go`, `modules/weather/weather_commands.go`), Go comments, `internal/util/util.go` (the dash-folding table), `internal/lightnotice/store.go:149` (the validator), and the quest, dialogue and conversation packages (#375).

- [ ] **Step 1: Re-verify the list**

```bash
grep -rnP '[\x{2014}\x{2013}]|[a-z] -- [a-zA-Z]' <each file in both tables> | grep -vP '^\S+:\d+:\s*//'
```

Expected: exactly the lines listed (line numbers may shift by a few if earlier tasks touched the file; `usercommands/shoot.go` is not in the list).

- [ ] **Step 2: Write the Go guard (it fails now)**

The text stays in Go (owner 2026-10-12; M7 #371 decides any store question), so a guard reading Go source keeps dashes from coming back. Create `go_prose_dash_guard_test.go` at the repo root:

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// House style for player text held in Go (M6 slice 2, ledger row 55): no em
// dash, en dash or spaced double hyphen in a string literal. Spaced single
// hyphens are not checked here: Go uses " - " in format and usage strings
// too often for a pattern to tell them from a dash.
//
// Skipped: string literals inside log, error, panic and test-failure calls;
// and the paths below, which hold builder, admin, validator or migration
// output, the dash-folding table, or quest and dialogue text (#375).
var goDash = regexp.MustCompile("[—–]|[A-Za-z,.!?] -- [A-Za-z]")

var goDashExemptPrefixes = []string{
	"internal/behaviortree/",
	"internal/conversations/",
	"internal/dialogue/",
	"internal/mapper/",
	"internal/migration/",
	"internal/questengine/",
	"internal/quests/",
	"modules/gmcp/",
}

var goDashExemptFiles = map[string]string{
	"internal/actions/divergences.go":          "registry audit log text",
	"internal/caravan/visit.go":                "log text",
	"internal/darkcombat/store.go":             "the validator's own pattern",
	"internal/ferry/route.go":                  "log text",
	"internal/ferry/tradecircuit.go":           "loader error text",
	"internal/hooks/Quest_HandleQuestUpdate.go": "quest text, #375",
	"internal/lightnotice/store.go":            "the validator's own pattern",
	"internal/mobs/patrol_loader.go":           "loader log text",
	"internal/mobs/schedule_loader.go":         "loader log text",
	"internal/state/position/validation.go":    "invariant error text",
	"internal/util/util.go":                    "the dash-folding table",
	"modules/weather/weather_commands.go":      "admin command output",
}

func goDashExempt(rel string) bool {
	for _, p := range goDashExemptPrefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	if _, ok := goDashExemptFiles[rel]; ok {
		return true
	}
	return strings.HasPrefix(filepath.Base(rel), "admin.")
}

// quietCall reports a call whose string arguments are not player text.
func quietCall(c *ast.CallExpr) bool {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name == "panic"
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok {
			switch x.Name {
			case "mudlog", "errors", "log", "slog":
				return true
			case "fmt":
				return f.Sel.Name == "Errorf"
			}
		}
	}
	return false
}

func TestGoPlayerStringsHaveNoDashes(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(here)
	scanned := 0
	for _, top := range []string{"internal", "modules"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if goDashExempt(rel) {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			scanned++
			var quiet [][2]token.Pos
			ast.Inspect(f, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok && quietCall(c) {
					quiet = append(quiet, [2]token.Pos{c.Pos(), c.End()})
				}
				return true
			})
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				for _, q := range quiet {
					if lit.Pos() >= q[0] && lit.End() <= q[1] {
						return true
					}
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					s = lit.Value
				}
				if goDash.MatchString(s) {
					t.Errorf("%s:%d: dash in a string: %s", rel, fset.Position(lit.Pos()).Line, s)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned only %d Go files; the walk is broken", scanned)
	}
}
```

Run: `go test . -run TestGoPlayerStringsHaveNoDashes -count=1`
Expected: FAIL, listing the sites in the two tables above, plus anything the grep's filters missed (a string concatenated across lines, a struct literal). Classify every hit that is not in the tables: player text gets fixed in Step 3; text no player reads gets an entry in `goDashExemptFiles` with its reason, and the controller is told. Never widen `quietCall` or the prefixes to clear a hit you have not read.

- [ ] **Step 3: Rewrite each string by hand**

Same rule as Task 8. In a list row such as `` `  [x] %s (%d) — %s` `` use a colon: `` `  [x] %s (%d): %s` ``. Keep `%` verbs and their order. Then `go test . -run TestGoPlayerStringsHaveNoDashes -count=1` → PASS. Null probe: put `—` back into one `aging.go` string, confirm the guard names that file and line, restore.

- [ ] **Step 4: Gate**

```bash
go build ./... && go test . ./internal/... ./modules/... -count=1
```

Expected: PASS. A test that pinned the old string is updated to the new string in the same commit; name it in the commit body.

- [ ] **Step 5: Commit**

```bash
git add go_prose_dash_guard_test.go <each file in both tables by name> <each test updated by name>
git commit -m "copy: no dashes in player-facing Go text

Part of ledger row 55 (#370). Quest, dialogue and conversation text waits for #375.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: One name for an unseen stealth shooter (row 54)

**Files:**
- Modify: `internal/messaging/hidenames.go` (after `SpeakerNoun`)
- Modify: `internal/mobcommands/shoot.go:97`, `internal/usercommands/shoot.go:441`
- Test: `internal/messaging/hidenames_test.go` (append) or a new `stealth_actor_test.go` in `internal/messaging`

- [ ] **Step 1: Failing test**

```go
func TestStealthActor_IsTheOneSpelling(t *testing.T) {
	if StealthActor != "Someone" {
		t.Fatalf("StealthActor = %q, want %q", StealthActor, "Someone")
	}
}
```

Run: `go test ./internal/messaging/ -run TestStealthActor -count=1` → FAIL, undefined.

- [ ] **Step 2: Add the constant**

```go
// StealthActor is what every reader calls a shooter firing from stealth. It is
// not a sight word: a sneaking shooter is hidden from everyone whatever their
// sight, so the name never enters the line (shoot, both twins). Engine
// vocabulary, kept in Go beside UnseenNoun and SpeakerNoun by decision (M6
// slice 2, ledger row 54).
const StealthActor = "Someone"
```

- [ ] **Step 3: Use it**

`internal/mobcommands/shoot.go:97`: `` actor = `Someone` `` → `actor = messaging.StealthActor`.
`internal/usercommands/shoot.go:441`: `` targetIds.Actor = `Someone` `` → `targetIds.Actor = messaging.StealthActor`.

```bash
grep -rn '`Someone`' internal/mobcommands/shoot.go internal/usercommands/shoot.go
go test . ./internal/messaging/... ./internal/mobcommands/... ./internal/usercommands/... -count=1
```

Expected: grep prints nothing; tests PASS.

- [ ] **Step 4: Update `internal/messaging/context.md`** with one row for `StealthActor` beside `UnseenNoun` / `SpeakerNoun` (find them with `grep -n "SpeakerNoun" internal/messaging/context.md`).

- [ ] **Step 5: Commit**

```bash
git add internal/messaging/hidenames.go internal/messaging/context.md internal/mobcommands/shoot.go internal/usercommands/shoot.go <test file>
git commit -m "refactor(messaging): one constant names a stealth shooter

Closes ledger row 54 (#370): anonymity words stay engine vocabulary in Go.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Docs for PR 1

**Files:**
- Modify: `docs/superpowers/audits/messaging-m6-content-ledger.md` (rows 36 to 54, 56 to 59; row 55 partial)
- Modify: `docs/README.md`
- Modify: `docs/PATCH_NOTES.md`

- [ ] **Step 1: Ledger.** Set the Status cell of rows 36 to 54 and 56 to 59 to `done (PR #___, M6 slice 2)` with a short note (row 54: "kept in Go by decision; `messaging.StealthActor`"; rows 36 to 47: "dark-combat store, optics-gated"). Row 55: `partly done (PR #___): combat, narration, messaging, player Go text; rooms and mobs in PR #___; quests and dialogue moved to #375`. Fill the PR number once the PR exists (amend the docs commit before merge).

- [ ] **Step 2: README.** Add rows for: `internal/darkcombat/context.md`, `_datafiles/world/dogmud/narration/dark-combat/`, `combat_idiom_guard_test.go`, `prose_dash_guard_test.go`, `go_prose_dash_guard_test.go`, `docs/superpowers/audits/2026-10-12-special-move-pairing-audit.md`. Match the neighbouring rows' style.

- [ ] **Step 3: Patch notes.** A new top entry:

```markdown
## Unreleased: Fighting blind, and plainer combat words

- When you cannot see at all, combat now tells you what you feel and hear:
  a blow landing, something whooshing past, your own swing finding nothing.
  You no longer read lines about watching your foe's stance. With
  infravision you still see shapes and read the fight as before.
- Creatures and players now share the same wording for special moves, so
  a wolf's pounce and a player's pounce read alike and both have more
  variety. What the room sees always matches what happened.
- Combat text uses plainer words: "barely hits" instead of "clips", "stay
  on your feet" instead of "keep your footing", and similar.
- A creature that throttles a spellcaster now tells the room the spell was
  broken, as a player's throttle already did.
```

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/messaging-m6-content-ledger.md docs/README.md docs/PATCH_NOTES.md
git commit -m "docs: M6 slice 2 PR 1 ledger, index and patch notes

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: PR 1 gate and pull request

- [ ] **Step 1:** Load `dogmud-shipping` and run its full pre-push gate in order (build, vet, root package, full `go test ./...`, `-race` on the touched packages, the detached-worktree boot check). The boot log must show the dark-combat store loading with no panic.
- [ ] **Step 2:** Push and open the PR with `--repo pruuk/DOGMud`. Body: what changed per ledger row, the throttle behaviour change, the retired byte-identity nets, the Defended fix (a later clean hit is no longer narrated as defended). Use "Part of #370", never a closing keyword for #370 (the issue stays open for PR 2 and the playtest). End with the attribution line.
- [ ] **Step 3:** Request code review (`superpowers:requesting-code-review`), fix findings, merge after green CI.

---

# PR 2: world dash sweep

### Task 13: Worktree

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add -b feat/m6-slice2-world-dashes C:/tmp/dogmud-m6s2-world origin/master
```

Start after PR 1 merges, so `prose_dash_guard_test.go` exists on master.

### Task 14: Rooms, one zone per commit

**Files:** `_datafiles/world/dogmud/rooms/<zone>/*.yaml` (48 zone directories, 1,174 files, 4,857 lines: 3,840 em/en dashes, 1,015 ` -- `, and 1 ` - `).

- [ ] **Step 1: List zones by hit count**

```bash
cd C:/tmp/dogmud-m6s2-world
for z in _datafiles/world/dogmud/rooms/*/; do n=$(grep -rnP "[\x{2014}\x{2013}]|[A-Za-z,.!?'\"] --? [A-Za-z]" "$z" --include=*.yaml | grep -vP '^\S+?:\d+:\s*#' | wc -l); [ "$n" -gt 0 ] && echo "$n $(basename "$z")"; done | sort -nr
```

- [ ] **Step 2: Per zone (subagent, one zone or a small batch of small zones per dispatch)**

For each file: rewrite every dash by hand (period, comma, colon, "and"), keep each description's line wrap within 80 columns (re-wrap the paragraph if a rewrite changes its length), change nothing else. Then:

```bash
grep -rnP "[\x{2014}\x{2013}]|[A-Za-z,.!?'\"] --? [A-Za-z]" _datafiles/world/dogmud/rooms/<zone> --include=*.yaml | grep -vP '^\S+?:\d+:\s*#'
go test ./internal/rooms/... -count=1
```

Expected: no grep output; tests PASS. The controller reads the zone's diff before the commit.

```bash
git add _datafiles/world/dogmud/rooms/<zone>
git commit -m "copy(rooms): no dashes in <zone>

Part of ledger row 55 (#370).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(`git add` of one zone directory is a named path, which the rule allows.)

### Task 15: Mobs

**Files:** `_datafiles/world/dogmud/mobs/**/*.yaml` (461 files, 813 lines: 317 em/en dashes, 496 ` -- `). Same procedure and grep as Task 14, batched by subdirectory, one commit per batch, `go test ./internal/mobs/... -count=1` after each.

### Task 16: Widen the guard

- [ ] **Step 1:** In `prose_dash_guard_test.go`, add `"rooms"` and `"mobs"` to `proseDashDirs` and update the comment (`proseDashDirs grows in PR 2` becomes the exempt-list sentence only).
- [ ] **Step 2:** `go test . -run TestPlayerProseHasNoDashes -count=1` → PASS. Null probe: one `—` into any room description, confirm the guard names it, restore.
- [ ] **Step 3:** Commit `test: the dash guard covers rooms and mobs`.

### Task 17: PR 2 docs, gate and pull request

- [ ] Ledger row 55 → `done (PR #___ and PR #___); quests and dialogue moved to #375`. Comment on #375 that quest and dialogue dashes are now its scope.
- [ ] `dogmud-shipping` full gate, boot check, push with `--repo pruuk/DOGMud`, "Part of #370", code review, merge.

---

# After both PRs: the M6 playtest

Load `dogmud-playtesting`. Ephemeral goals file and `--checkout` of master after PR 2. Cases:
1. A blind player (blinded condition) fights a mob in a lit room: every personal line is a dark line; the blind notice speaks each round.
2. A player in a dark room without infravision is attacked: defender dark lines.
3. A player with infravision fights in the dark: composed lines with "a figure" and "weapon".
4. A sleeping player in a lit room is attacked: the first hit reads "something", not a dark line.
5. Each player special move once, and a mob using kick, pounce and throttle on a caster: room lines match outcomes; the mob throttle interrupt reaches the room.
6. `look` in five swept rooms across different zones: no dashes, wrap intact.

File findings as GitHub issues on `pruuk/DOGMud`.
