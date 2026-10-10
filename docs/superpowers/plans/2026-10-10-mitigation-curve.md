# Mitigation Curve Implementation Plan (#465)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat 0.75 mitigation clamp with a per-channel curve (linear to a knee of 0.50, diminishing above it, a hard cap of 0.90), let the part of mitigation above the knee soften crits, and make `PowerScore` and the `status` sheet read the mitigation combat actually applies.

**Architecture:** One pure function, `combat.EffectiveMitigation(raw, MitigationCurve)`, in a new `internal/combat/mitigation_curve.go`, with `MitigationCurve.CritReduction` for the crit overflow. Twelve `Balance` knobs (four per channel; the three existing `...MitigationCap` keys keep their names and become the curve's ceiling) are read by `combat.MitigationCurveFor(channel)`, which replaces `MitigationCap(channel)`. `ApplyMitigation`, `CritOrMitigatedDamage(Scaled)` and `ReflectDamage` take the curve instead of a cap; every caller keeps its shape. The melee crit path carries the reduction as `swingDamageParams.critReduction`. `EffectiveMitigationFor(char, channel)` is the reader `PowerScore` and the new `effectiveMitigation` template function share.

**Tech Stack:** Go 1.25, testify, GoMud engine packages under `internal/`, `_datafiles/config.yaml` (skip-worktree), DOGMud templates under `_datafiles/world/dogmud/templates/`.

**Spec:** `docs/superpowers/specs/2026-10-10-mitigation-curve-design.md` (APPROVED by the owner, merged in #473). Numbers from the Mitigation Curve Lab page: knee 0.50, cap 0.90, aim 0.05, bend 0.40.

**Branch:** implement on `feat/mitigation-curve-465`, cut from master `9798f6d02` in a worktree (never the main checkout: the owner runs a server there, and its `config.yaml` carries skip-worktree). The PR body says "Refs #465" and names what it resolves in words, never with a closing keyword.

---

## How this plan was verified

The plan was first built test-first in a throwaway worktree of master `b33676790`: each new test was run against the old code and failed for the reason its step gives, then passed after its implementation step, and each null probe was sabotaged once to prove it could fail. While it was being written, #472 and #473 landed (#472 moved `combat_taunt.go`, `damage_pipeline.go`, `config.balance.go`, `config.yaml`, `PATCH_NOTES.md` and `internal/combat/context.md`), so the work was rebased onto `9798f6d02` and everything below is `9798f6d02`'s. Then the plan itself was dry-run: a small program read THIS DOCUMENT and applied every task's Create and Replace blocks, step by step, to a second fresh `9798f6d02` worktree (each "Replace in" block must match exactly once at its turn, each "Replace all N" exactly N times, as the Edit tool requires; CRLF files kept CRLF). Each "expect FAIL" command was run before its implementation step and each "expect PASS" command after; `go build ./...`, `go vet ./...` and `go test . -count=1` ran at the end of every task, and every task's `git add` list was committed and left `git status` clean. Then the whole tree was run (see "Integrated dry run"). Both worktrees were removed afterwards.

## Facts verified against source (master `9798f6d02`)

Built by grepping and reading for this plan, not copied from the spec. Line numbers are master `9798f6d02`'s; every edit below is anchored by quoted text, never by a number alone.

### The clamp and its callers

| Fact | Source |
|---|---|
| `ApplyMitigation(rawDmg, mitigationPct, cap float64) float64` clamps `pct` to `[0, cap]`, and a `cap <= 0` reads 0.75 | `internal/combat/damage_pipeline.go:105` (fallback `:110`) |
| `MitigationCap(channel DamageChannel) float64` reads `PhysicalMitigationCap` / `MagicalMitigationCap` / `ConvictionMitigationCap`; an unknown channel returns 0.75 | `damage_pipeline.go:119` (fallback `:129`) |
| `grep -rn "MitigationCap(" internal modules *.go` (non-test) prints 11 lines: the definition `damage_pipeline.go:119` and **10 call lines** in 7 files | below |
| Call lines: `actions/combat_counter.go:221` (conviction), `actions/combat_taunt.go:263` (conviction), `combat/combat_helpers.go:490` (melee), `:1967` (pets, feeds two `ApplyMitigation` calls `:1968-1969`), `combat/skill_moves.go:207` (special moves, ranged), `hooks/combat_shared_helpers.go:101` (spells; non-harm fallback `cap = 0.75` at `:104`), `:324` (riposte), `hooks/NewRound_DoCombat_unified.go:374`, `:376` (reflect, physical and magical), `usercommands/throw.go:412` | `grep -rn` |
| `CritOrMitigatedDamageScaled(rawDmg, skillRank, isCrit, mitigPct, mitigCap, bonusCritMult)`: a crit is `raw * CritDamageMultiplier(rank) * bonus`, unmitigated; a normal hit `ApplyMitigation` | `internal/combat/crit_damage.go:95`, crit `:101`, normal `:103`; `CritOrMitigatedDamage` `:116` |
| Melee crit: `critMean := sdp.rawDmgForCrit * sdp.critDmgMult`, unmitigated; `swingDamageParams` at `:120`, `buildDamageParams` `:476` (mitigation `:490`), `calcHitDamage` `:1459` | `internal/combat/combat_helpers.go:1477` |
| Armor piercing: `SkillMoveParams.MitigationMultiplier` (stomp 0.5) scales raw mitigation `mitig := p.Defender.GetPhysicalMitigation() * mitigMult` before the cap; `CritOrMitigatedDamageScaled(..., mitig, cap, ...)` | `internal/combat/skill_moves.go:94`, `:206`, `:252` |
| `ReflectDamage(damageDealt, returnPct int, mitigation, cap float64) int` | `internal/combat/reflect_damage.go:81` |
| Getters return `(gear x effectiveness + non-gear) / 100`, uncapped | `internal/characters/combat.go:155`/`:194`, `:200`/`:235`, `:241`/`:273` |
| 14 non-test lines read `Get*Mitigation()` outside `characters/combat.go`: 12 at the call sites above, `PowerScore`'s one line (three calls) and a comment in `internal/mutations/mutations.go:484` | `grep -rn` |

### Readers that skip the clamp

| Fact | Source |
|---|---|
| `PowerScore(char characters.Character)` averages the three RAW getters, weight x300 | `internal/combat/calculations.go:20`, `:101` |
| `PowerScore` callers: `actions/consider.go:36-37`, `behaviortree/conditions_combat.go:44`, `:71`, `:80`, `:85`, `modules/leaderboards/leaderboards.go:211`, `:223`, `:246`, `:258` | `grep -rn` |
| `mitigationQuality(pct)` bands: none, thin <10, light <25, moderate <40, strong <55, heavy <70, fortified from 70 | `internal/templates/templatesfunctions.go:365-383` |
| The DOGMud status sheet passes the raw getters: `(mitigationQuality (.Character.GetPhysicalMitigation))` and the magical and conviction twins on one line | `_datafiles/world/dogmud/templates/character/status.template:13` |
| `identify` passes one item's own points (`divFloat $spec.PhysicalMitigation 100`), not a character's sum | `_datafiles/world/dogmud/templates/descriptions/identify.template:10-12` |
| `internal/templates` already imports `internal/combat` and `fmt` | `templatesfunctions.go:4`, `:13` |

### Config

| Fact | Source |
|---|---|
| Fields `PhysicalMitigationCap`, `MagicalMitigationCap`, `ConvictionMitigationCap ConfigFloat` (default comment 0.75) | `internal/configs/config.balance.go:343-345` |
| Validator: each cap `<= 0 || > 1.0` reads 0.75, inside `validateCombat` (`:35`); NaN passes it | `internal/configs/config.balance.combat.go:378-386` |
| `validPositiveActionCost(v)` is the existing NaN- and Inf-safe "positive" check | `config.balance.combat.go:5` |
| No balance validator logs: `grep -c mudlog internal/configs/config.balance*.go` is 0 in every file (the grep can succeed: `internal/configs/configs.go:539` holds `mudlog.Info`). `mudlog` writes through `slogInstance`, nil until `SetupLogger`, so a log line in a lazily run validator would panic a test binary | `internal/mudlog/mudlog.go:16`, `:91` |
| Shipped: `PhysicalMitigationCap: 0.75`, `MagicalMitigationCap: 0.75`, `ConvictionMitigationCap: 0.75` under a two-line comment | `git show HEAD:_datafiles/config.yaml`, lines 1197-1201 |
| `config.yaml` is skip-worktree in the main checkout (`S`); a fresh worktree reads `H` and its disk copy equals the blob (`git diff --stat HEAD` empty); `core.autocrlf` is `true` | `git ls-files -v`; dry run |
| Shipped-file test helpers: `shippedConfigSource(t)`, `bytesContainsKey`, `loadConfig(document []byte) (Config, error)` | `internal/configs/config_fumble_outpays_win_test.go:18`, `:34`; `configs.go:508` |
| `configs.SetConfigForTest(t, c)` swaps the whole config and restores on cleanup | `internal/configs/testing_support.go:33` |

### Tests that pin the clamp or the crit bypass

| Test | Source |
|---|---|
| `TestApplyMitigation` (0.90 "capped at 75%") | `internal/combat/damage_pipeline_test.go:94-117` |
| `TestMitigationCap`: per-channel caps 0.61 / 0.62 / 0.63 via `configs.AddOverlayOverrides` with **no cleanup** (the overlay leaked into every later test in the package), unknown channel 0.75 | `damage_pipeline_test.go:204-249` (overlay `:213`) |
| `TestApplyMitigation_EdgeCases` (1.0 capped to 0.75) | `damage_pipeline_test.go:266-279` |
| `TestRegression_MitigationCapEnforced` | `internal/combat/regression_test.go:121-148` |
| `ReflectDamage(..., 0.75)` x8, `TestReflectDamage_RespectsCap` | `internal/combat/reflect_damage_test.go:16-46` (`:25`) |
| `ApplyMitigation` and `MitigationCap` in the integration tests | `internal/combat/integration_combat_test.go:46`, `:160`, `:201`, `:208`, `:214`, `:237` |
| `TestCritOrMitigatedDamage_CritBypassesMitigationAndScales` (crit at 0.75 mitigation); 11 calls with a 0.75 cap | `internal/combat/crit_damage_test.go:65`, `:78`; `:78-180` |
| Melee parity anchor: pin `PhysicalMitigationCap = 0.75` `:93`, cell `BIS-75pct-cap` `:345`, `wantDmgMean` `:404`, `critMean` (unmitigated) `:414` | `internal/combat/melee_parity_stat_test.go` |
| Riposte mean `combat.MitigationCap(combat.ChannelPhysical)` | `internal/hooks/counter_tier_test.go:99-100` |
| Fixture pins `b.PhysicalMitigationCap = 0.75` | `internal/actions/combat_fire_surprise_test.go:89`, `internal/combat/surprise_opening_strike_test.go:137` |
| `TestThrowSeam_AttackerCritBypassesMitigationAndScales` (target at 75% raw; text "mean ~7.5", "mean ~60") | `internal/usercommands/throw_seam_test.go:304`, `:318`, `:334` |
| Bounds only, still true: `PhysicalMitigationCap` etc. in `(0, 1.0]` | `internal/configs/smoke_test.go:56-67` |
| Crit tests that hold at or below the knee and need no change: spell crit vs an unmitigated target, a 50% target's crit spread, a skill-move crit | `internal/hooks/crit_damage_spell_test.go`, `internal/combat/hitroll_test.go:500-540`, `internal/combat/skill_moves_seam_test.go:140-200` |

### Prior art and content

| Fact | Source |
|---|---|
| No curve or diminishing-returns helper exists; nearest prior art `compressContestGap` (`u/(1+k*u)`) with the `!(k > 0)` NaN guard | `internal/combat/run_contest.go:91`, `:99` |
| No help page names a 75% cap: `grep -rl "75%" _datafiles/world/dogmud/templates/help` prints nothing (capable: `grep -rl "50%"` finds `reinforced-travel-pack.template`). But `help armor` says each channel "is capped so that no amount of armor makes you untouchable" | `_datafiles/world/dogmud/templates/help/armor.template:3-5` |
| Playtest mobs: The Sentinel 9552, Repair Frame 9585, Grapnel Warden 9586, Hull Sweeper 9587; North Road bandits 283 lookout, 284 fighter, 285 caster | `_datafiles/world/dogmud/mobs/eastern_highlands/9552-the_sentinel.yaml`, `crash_site_interior/9585..9587-*.yaml`, `north_road/283..285-*.yaml` |
| Python balance models that assume the 0.75 cap (not part of the game, not changed): `skill_model.py`, `u6b_model_counters_family_costs.py`, `unified_resolution_model.py` | `grep -rli "mitigationcap\|mit_cap\|mitigation cap" tools/balance` |

## Spec facts that were wrong or imprecise

- **"11 non-test call sites of `MitigationCap(`".** The grep's 11 lines are 10 call lines plus the definition. Every one of the 10 changes; nothing else calls it.
- **"falls back to the shipped defaults with a log line, as `validateCombat` does today".** `validateCombat` logs nothing, and no balance validator does (see the facts). A log line there would also dereference a nil `slogInstance` in any test binary that validates a bad value. The fallback is silent, like every other balance knob (D2).
- **"aim >= 0".** An absent key unmarshals to 0, so a validator that keeps 0 would run every test binary, and any config missing the key, on a different curve from the shipped one. Aim takes the default on `<= 0`, as `CritDamagePerSkill` does (D3).
- **"No help page states the 75% cap".** True of the number, but `help armor` describes a cap in words. Task 6 rewrites that paragraph.
- **Line numbers.** Spec `skill_moves.go:202` is `:206` (the multiply) and `:207` (the cap); the taunt call is `combat_taunt.go:263` (#472 moved it). The rest match.

## Design points where the code forced a choice (for the owner)

- **D1. Signatures.** `ApplyMitigation(raw, pct, curve MitigationCurve)`, `CritOrMitigatedDamageScaled(raw, rank, isCrit, pct, curve, bonus)`, `CritOrMitigatedDamage(raw, rank, isCrit, pct, curve)`, `ReflectDamage(dealt, pct, mitigation, curve)`. `MitigationCap` is deleted, not kept beside the curve, so nothing can clamp by the old rule.
- **D2. The validator is silent.** A bad or absent value takes the shipped default without a log line (see "Spec facts").
- **D3. Aim 0 is not expressible in YAML.** `aim <= 0` reads 0.05. The pure function still honours aim 0 (tested); an aim of 0.0001 is the approach-only curve in practice. Knee, cap and bend default on `<= 0` too.
- **D4. Joint check.** `0 < knee < cap <= 0.95`. A cap outside `(0, 0.95]` reads 0.90; a knee not below the cap reads 0.50; if the cap is at or under even 0.50, the channel takes the whole default curve.
- **D5. Fallbacks.** `MitigationCurveFor` of an unknown channel returns the physical curve (was a constant 0.75). The spell path's non-harm fallback (logged as an error, mitigation 0) passes the physical curve. A zero-value `MitigationCurve` mitigates nothing (the old `ApplyMitigation` turned a zero cap into 0.75); no caller builds one, because every curve comes from validated config.
- **D6. Melee crit.** `swingDamageParams.critReduction` is 0 in a literal built outside `buildDamageParams`, which degrades to a full crit, never to no damage. Pets have no crit path; their mean and spread take the curve.
- **D7. `status` reads a new template function,** `effectiveMitigation .Character "physical"`, which fails the render on an unknown channel name. `identify` still describes an item's own points raw: one item is not a character's protection. The default world's `status.template` and the unused `status-lite.template` (`_datafiles/world/default/`) are not loaded by DOGMud and are left alone.
- **D8. `PowerScore` moves a lot for invested tanks.** It weighs average mitigation x300. A build at raw 1.81 / 1.64 / 1.86 used to add 531; on the curve it adds about 253 (84%, 83%, 85% effective). `consider` odds, mob flee and target choice and the power leaderboard all read it, so an armoured player looks weaker to mobs than before; the old number counted protection combat never applied. Retuning PowerScore's weight is outside this change.
- **D9. A crit test moved its mitigation.** `TestCritOrMitigatedDamage_CritBypassesMitigationAndScales` held a crit against 0.75; it now holds it at 0.50, the most a crit still ignores in full. The overflow above the knee has its own tests.
- **D10. `TestMitigationCap`'s overlay leak is gone.** Its replacement pins through `SetConfigForTest`, which restores; the old test left caps of 0.61 to 0.63 in place for every later test in `internal/combat`.
- **D11. Between Tasks 2 and 3 the shipped cap knob reads 0.90 while the code still clamps.** The tasks ship in one PR; no task boundary is deployed.
- **D12. After merge, the owner's local `config.yaml` (main checkout, skip-worktree) still says 0.75 and lacks the nine new keys,** so a local server runs knee 0.50, cap 0.75 until it is resynced from the blob (`dogmud-balance-config`, "Fixing the owner's local config is Claude's job"). Prod's `config-production.yaml` and `/mud-config` overrides are the owner's to check for a `...MitigationCap` override before deploy.

## Guards and package tests the dry run tripped

- **Every pin listed under "Tests that pin the clamp"** fails to build or fails its number once Task 3 or 4 lands; each is updated in that task, not deleted.
- **`TestMeleeParityDamagePerSwing`** (200k swings per cell, about 15 seconds) pins `sdp.critReduction` in Task 4 and feeds it to the analytic crit mean.
- **`TestTemplateFreeze_StatusReadsTheBrokenLimbRecord`** renders the real DOGMud status sheet; it would catch a misspelt template function. Task 5 adds a sibling that pins the Defenses row.
- **Checked and untouched:** root guards (`go test .`) pass after every task (the `skill_moves.go` comment Task 3 adds shifts lines that no guard keys); `internal/configs/smoke_test.go` bounds hold; `tools/context_md_audit.py` prints the same list as master.

## File map and order

| Task | What | Files |
|---|---|---|
| 1 | The curve, a pure function | `internal/combat/mitigation_curve.go`, `internal/combat/mitigation_curve_test.go` |
| 2 | Twelve knobs, the validator, shipped values | `internal/configs/config_mitigation_curve_test.go`, `internal/configs/config.balance.go`, `internal/configs/config.balance.combat.go`, `_datafiles/config.yaml` |
| 3 | `MitigationCurveFor` replaces `MitigationCap`; every caller and pin | `internal/combat/damage_pipeline_test.go`, `internal/combat/regression_test.go`, `internal/combat/reflect_damage_test.go`, `internal/combat/integration_combat_test.go`, `internal/combat/crit_damage_test.go`, `internal/combat/melee_parity_stat_test.go`, `internal/combat/surprise_opening_strike_test.go`, `internal/actions/combat_fire_surprise_test.go`, `internal/hooks/counter_tier_test.go`, `internal/combat/damage_pipeline.go`, `internal/combat/crit_damage.go`, `internal/combat/reflect_damage.go`, `internal/combat/combat_helpers.go`, `internal/combat/skill_moves.go`, `internal/combat/pools.go`, `internal/actions/combat_counter.go`, `internal/actions/combat_taunt.go`, `internal/hooks/combat_shared_helpers.go`, `internal/hooks/NewRound_DoCombat_unified.go`, `internal/usercommands/throw.go` |
| 4 | Crits lose the overflow above the knee | `internal/combat/crit_overflow_test.go`, `internal/combat/crit_overflow_melee_test.go`, `internal/combat/crit_damage_test.go`, `internal/combat/melee_parity_stat_test.go`, `internal/usercommands/throw_seam_test.go`, `internal/combat/crit_damage.go`, `internal/combat/combat_helpers.go` |
| 5 | `PowerScore` and `status` read effective mitigation | `internal/combat/effective_mitigation_readers_test.go`, `internal/templates/effective_mitigation_test.go`, `internal/usercommands/template_freeze_test.go`, `internal/combat/damage_pipeline.go`, `internal/combat/calculations.go`, `internal/templates/templatesfunctions.go`, `_datafiles/world/dogmud/templates/character/status.template` |
| 6 | Docs, help and patch notes | `internal/combat/context.md`, `internal/configs/context.md`, `internal/templates/context.md`, `_datafiles/world/dogmud/templates/help/armor.template`, `docs/PATCH_NOTES.md` |
| 7 | Whole-tree verification and boot check | (no files) |
| 8 | Playtest (controller) | (no files) |

**Order.** Strictly in number order: 2 needs nothing from 1 but 3 needs both (the curve type and the knobs); 4 needs 3's signatures; 5 needs 4's `pinShippedCurve` helper; 6 documents all of them. No parallel lanes: Tasks 1, 3, 4 and 5 all touch `internal/combat`, and a half-applied task breaks the others' compile.

New files: `internal/combat/mitigation_curve.go` and the six new test files are package files; no new package, no new root guard, so no `docs/README.md` row beyond this plan's own (added with the plan).

## Rules for every task

- Work in a worktree cut from master, never the main checkout.
- Edit files with the Edit tool (or Write for a new file the step gives in full). Never `sed -i`, never a Python read-modify-write. Never `git add -A` or `git add .`: stage the named paths the task lists.
- Each "Replace in `path`" block is one Edit call: `old_string` is the first block, `new_string` the second, and the old text matches exactly once at that point. "Replace all N occurrences in `path`" is one Edit call with `replace_all: true`, and the old text matches exactly N times. Apply a file's blocks in the order given.
- `gofmt -l` on every touched Go file prints nothing; run `gofmt -w <file>` if it does.
- Run targeted tests, then the root guards: `go test . -count=1` (about a minute) at the end of every task, with `go build ./...` and `go vet ./...`.
- `grep -c` exits 1 on zero matches; run an "expect zero" check on its own line, never inside an `&&` chain.
- No em dash or en dash in any Go string, comment, YAML, template, doc or commit message. Four "Replace" old-text blocks below quote existing source lines that hold a dash (in `damage_pipeline_test.go`, `melee_parity_stat_test.go`, `damage_pipeline.go` and `throw_seam_test.go`); the edit removes each, and no new text adds one.
- Player text (Task 6): 80 columns or less, no raw numbers for protection or damage, ESL-clear (`dogmud-player-copy`).
- Balance numbers come from `git show HEAD:_datafiles/config.yaml`, never a Go default. Only Task 2 edits `config.yaml`, and it builds the change from the HEAD blob (its Step 3 gives the procedure).
- Test binaries never load `config.yaml`: the Go defaults equal the shipped curve (Task 2), and tests that depend on the curve pin it (`shippedCurveForTest`, `pinShippedCurve`).
- `context.md` is updated for every API change (Task 6 does all three; a task executed alone must still leave its package's `context.md` true, so if Task 6 is not run in the same PR, move its block for that package into the task).
- Commit messages end with a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. No "closes", "fixes" or "resolves #N" anywhere, commit or PR; write "Refs #465".
- A test fails first: each task's first run is its failing run, and the expected failure is given. If a new test passes before its implementation, stop and find out why. Pins that hold before and after, by design: `TestEffectiveMitigation_LinearUpToTheKnee` cannot fail alone (the file fails to build first), `TestCritOrMitigatedDamage_BelowTheKneeIsUnchanged`, `TestCritOrMitigatedDamage_CritBypassesMitigationAndScales` (now at the knee), `TestMitigationCurveKnobs_LegalValuesSurvive` once the fields exist.

---

### Task 1: The curve, a pure function

`EffectiveMitigation(raw, curve)` and `MitigationCurve.CritReduction(raw)` in a new file. Nothing calls them yet.

- [ ] **Step 1: Write the failing test.**

Create `internal/combat/mitigation_curve_test.go`:

```go
package combat

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// shippedCurveForTest is the curve config.yaml ships on every channel (#465):
// knee 0.50, cap 0.90, aim 0.05, bend 0.40. Written out here, not read from
// config, so the pure function is tested against the owner's numbers even in
// a test binary that never loads config.yaml.
var shippedCurveForTest = MitigationCurve{Knee: 0.50, Cap: 0.90, Aim: 0.05, Bend: 0.40}

// The spec's table (section 1): raw mitigation in, effective out, within
// half a percentage point (the table is rounded to whole percents).
func TestEffectiveMitigation_MatchesTheSpecTable(t *testing.T) {
	rows := []struct{ raw, want float64 }{
		{0.50, 0.50},
		{0.75, 0.67},
		{0.91, 0.73},
		{1.00, 0.75},
		{1.64, 0.83},
		{1.81, 0.84},
		{2.70, 0.88},
		{3.30, 0.89},
		{3.70, 0.90},
	}
	for _, r := range rows {
		got := EffectiveMitigation(r.raw, shippedCurveForTest)
		assert.InDelta(t, r.want, got, 0.005, "raw %.2f", r.raw)
	}
}

// Below the knee the curve is the identity: every point of raw mitigation
// counts in full, as it did under the old clamp.
func TestEffectiveMitigation_LinearUpToTheKnee(t *testing.T) {
	for _, raw := range []float64{0.01, 0.10, 0.25, 0.40, 0.4999, 0.50} {
		assert.InDelta(t, raw, EffectiveMitigation(raw, shippedCurveForTest), 1e-12, "raw %.4f", raw)
	}
}

// The curve has no step at the knee: a hair above it reads a hair above it.
func TestEffectiveMitigation_ContinuousAtTheKnee(t *testing.T) {
	c := shippedCurveForTest
	below := EffectiveMitigation(c.Knee-1e-9, c)
	at := EffectiveMitigation(c.Knee, c)
	above := EffectiveMitigation(c.Knee+1e-9, c)
	assert.InDelta(t, at, below, 1e-8)
	assert.InDelta(t, at, above, 1e-8)
}

// More raw mitigation never means less protection, and nothing exceeds the
// cap, across a sweep well past any shipped build (raw 0 to 10).
func TestEffectiveMitigation_IncreasingAndNeverAboveTheCap(t *testing.T) {
	c := shippedCurveForTest
	prev := 0.0
	for i := 0; i <= 10000; i++ {
		raw := float64(i) / 1000.0
		got := EffectiveMitigation(raw, c)
		assert.GreaterOrEqual(t, got, prev, "raw %.3f fell below raw %.3f", raw, raw-0.001)
		assert.LessOrEqual(t, got, c.Cap, "raw %.3f above the cap", raw)
		prev = got
	}
}

// Aim above 0 makes the cap reachable; aim 0 only approaches it.
func TestEffectiveMitigation_AimZeroNeverReachesTheCap(t *testing.T) {
	c := shippedCurveForTest
	assert.Equal(t, c.Cap, EffectiveMitigation(10.0, c), "aim 0.05 reaches the cap")

	c.Aim = 0
	for _, raw := range []float64{1, 10, 100, 1e6} {
		assert.Less(t, EffectiveMitigation(raw, c), c.Cap, "aim 0 at raw %.0f", raw)
	}
}

// Nothing negative and nothing not-a-number reaches the damage roll: both read
// as no mitigation (the guard compressContestGap uses).
func TestEffectiveMitigation_NaNAndNegativeReadAsNone(t *testing.T) {
	c := shippedCurveForTest
	assert.Equal(t, 0.0, EffectiveMitigation(-0.5, c))
	assert.Equal(t, 0.0, EffectiveMitigation(0, c))
	assert.Equal(t, 0.0, EffectiveMitigation(math.NaN(), c))
	assert.Equal(t, c.Cap, EffectiveMitigation(math.Inf(1), c))
}

// A zero curve (never built by MitigationCurveFor, which reads validated
// config) mitigates nothing rather than dividing by zero.
func TestEffectiveMitigation_ZeroCurveMitigatesNothing(t *testing.T) {
	assert.Equal(t, 0.0, EffectiveMitigation(0.6, MitigationCurve{}))
}

// The crit reduction is the part of effective mitigation above the knee:
// none at or below it, the overflow above it.
func TestCritReduction_IsTheOverflowAboveTheKnee(t *testing.T) {
	c := shippedCurveForTest
	assert.Equal(t, 0.0, c.CritReduction(0.30))
	assert.Equal(t, 0.0, c.CritReduction(0.50))
	assert.InDelta(t, EffectiveMitigation(1.00, c)-0.50, c.CritReduction(1.00), 1e-12)
	assert.InDelta(t, 0.25, c.CritReduction(1.00), 1e-12)
	assert.InDelta(t, c.Cap-c.Knee, c.CritReduction(10.0), 1e-12)
	assert.Equal(t, 0.0, c.CritReduction(math.NaN()))
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/combat/ -run "EffectiveMitigation|CritReduction" -count=1`
Expected: FAIL, build failed: `undefined: MitigationCurve` and `undefined: EffectiveMitigation`.

- [ ] **Step 3: Write the curve.**

Create `internal/combat/mitigation_curve.go`:

```go
package combat

import "math"

// MitigationCurve turns a channel's raw mitigation (the sum of every source,
// as Character.Get*Mitigation returns it, uncapped) into the fraction of
// damage actually stopped (#465). It replaced a flat clamp at 0.75, which
// shipped content stacked far past, so every source above the clamp was
// wasted.
//
//   - Knee: up to here raw mitigation counts in full.
//   - Cap:  the hard ceiling; nothing stops more than this.
//   - Aim:  where the curve would level off without the cap, as an amount
//     above it. 0 only approaches the cap; above 0 the cap is reachable.
//   - Bend: how much raw mitigation past the knee it takes to cover half the
//     distance to Cap + Aim.
//
// Each channel reads its own four knobs (MitigationCurveFor). All four ship at
// 0.50 / 0.90 / 0.05 / 0.40 on every channel.
type MitigationCurve struct {
	Knee float64
	Cap  float64
	Aim  float64
	Bend float64
}

// EffectiveMitigation is the fraction of damage stopped by raw mitigation on
// curve c: linear up to the knee, diminishing above it, never above the cap.
//
//	raw <= 0     -> 0
//	raw <= knee  -> raw
//	otherwise    -> x = raw - knee
//	                min(knee + (cap + aim - knee) * x / (x + bend), cap)
//
// The curve is continuous at the knee and never decreasing. A NaN or negative
// raw reads as no mitigation (`!(raw > 0)`, the guard compressContestGap uses:
// NaN fails every comparison, so `raw <= 0` would let it through). A zero
// curve mitigates nothing; MitigationCurveFor never returns one, because the
// config validator fills every knob.
func EffectiveMitigation(raw float64, c MitigationCurve) float64 {
	if !(raw > 0) || !(c.Cap > 0) {
		return 0
	}
	if math.IsInf(raw, 1) {
		return c.Cap
	}
	if raw <= c.Knee {
		return math.Min(raw, c.Cap)
	}
	x := raw - c.Knee
	eff := c.Knee
	if c.Bend > 0 {
		eff += (c.Cap + c.Aim - c.Knee) * x / (x + c.Bend)
	} else {
		eff = c.Cap + c.Aim
	}
	return math.Min(eff, c.Cap)
}

// CritReduction is the share a critical hit loses to raw mitigation on curve
// c: the part of effective mitigation above the knee, 0 at or below it. A
// crit lands at raw x CritDamageMultiplier x (1 - CritReduction), so a target
// at or below the knee takes full crits, as before #465, and very heavy
// protection softens them. The defence multiplier stays skipped on crits.
func (c MitigationCurve) CritReduction(raw float64) float64 {
	return math.Max(0, EffectiveMitigation(raw, c)-c.Knee)
}
```

- [ ] **Step 4: Run the tests.**

Run: `go test ./internal/combat/ -run "EffectiveMitigation|CritReduction" -count=1`
Expected: PASS. Null probe (do it once, then restore): change `return math.Min(eff, c.Cap)` to `return eff`; `TestEffectiveMitigation_IncreasingAndNeverAboveTheCap`, `..._AimZeroNeverReachesTheCap` and `TestCritReduction_IsTheOverflowAboveTheKnee` fail. Restore the line and rerun: PASS.

Run: `gofmt -l internal/combat/ && go build ./... && go vet ./internal/combat/ && go test . -count=1`
Expected: no gofmt output; `ok` for the root package.

- [ ] **Step 5: Commit.**

```bash
git add internal/combat/mitigation_curve.go internal/combat/mitigation_curve_test.go
git commit -F - <<'EOF'
feat(combat): a mitigation curve with a knee, diminishing returns and a cap

EffectiveMitigation bends raw mitigation: linear to the knee, then
knee + (cap + aim - knee) * x / (x + bend), never above the cap.
CritReduction is the part above the knee. Nothing calls them yet.

Refs #465

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 2: Twelve knobs, the validator, shipped values

Four knobs per channel. The existing `...MitigationCap` keys keep their names and become the curve's ceiling. The Go defaults equal the shipped values, so a test binary runs the shipped curve.

- [ ] **Step 1: Write the failing test.**

Create `internal/configs/config_mitigation_curve_test.go`:

```go
package configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The twelve mitigation curve knobs (#465), four per channel, in the order
// knee, cap, aim, bend for physical, magical, conviction.
var mitigationCurveKeys = []string{
	"PhysicalMitigationKnee", "PhysicalMitigationCap", "PhysicalMitigationAim", "PhysicalMitigationBend",
	"MagicalMitigationKnee", "MagicalMitigationCap", "MagicalMitigationAim", "MagicalMitigationBend",
	"ConvictionMitigationKnee", "ConvictionMitigationCap", "ConvictionMitigationAim", "ConvictionMitigationBend",
}

// The defaults equal the shipped values, so a test binary (which never loads
// config.yaml) runs the shipped curve.
var mitigationCurveDefaults = []ConfigFloat{0.50, 0.90, 0.05, 0.40}

func mitigationCurveValues(b Balance) []ConfigFloat {
	return []ConfigFloat{
		b.PhysicalMitigationKnee, b.PhysicalMitigationCap, b.PhysicalMitigationAim, b.PhysicalMitigationBend,
		b.MagicalMitigationKnee, b.MagicalMitigationCap, b.MagicalMitigationAim, b.MagicalMitigationBend,
		b.ConvictionMitigationKnee, b.ConvictionMitigationCap, b.ConvictionMitigationAim, b.ConvictionMitigationBend,
	}
}

func assertCurveDefaults(t *testing.T, b Balance, why string) {
	t.Helper()
	for i, v := range mitigationCurveValues(b) {
		assert.Equal(t, mitigationCurveDefaults[i%4], v, "%s: %s", why, mitigationCurveKeys[i])
	}
}

// An absent key reads 0 and takes the default on every channel.
func TestMitigationCurveKnobs_AbsentKeysTakeTheDefaults(t *testing.T) {
	b := Balance{}
	b.Validate()
	assertCurveDefaults(t, b, "empty Balance")
}

// Each bad value falls back to the shipped curve: a cap above 0.95 or not
// positive, a knee not below the cap, a negative aim or bend.
func TestMitigationCurveKnobs_InvalidValuesFallBack(t *testing.T) {
	cases := []struct {
		name                 string
		knee, cap, aim, bend ConfigFloat
		wantKnee, wantCap    ConfigFloat
		wantAim, wantBend    ConfigFloat
	}{
		{"cap above 0.95", 0.50, 0.99, 0.05, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"negative cap", 0.50, -0.2, 0.05, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"knee at the cap", 0.80, 0.80, 0.05, 0.40, 0.50, 0.80, 0.05, 0.40},
		{"knee above the cap", 0.85, 0.80, 0.05, 0.40, 0.50, 0.80, 0.05, 0.40},
		{"cap below the default knee", 0.60, 0.40, 0.05, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"negative aim", 0.50, 0.90, -0.1, 0.40, 0.50, 0.90, 0.05, 0.40},
		{"negative bend", 0.50, 0.90, 0.05, -1, 0.50, 0.90, 0.05, 0.40},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := Balance{
				PhysicalMitigationKnee: c.knee, PhysicalMitigationCap: c.cap,
				PhysicalMitigationAim: c.aim, PhysicalMitigationBend: c.bend,
				MagicalMitigationKnee: c.knee, MagicalMitigationCap: c.cap,
				MagicalMitigationAim: c.aim, MagicalMitigationBend: c.bend,
				ConvictionMitigationKnee: c.knee, ConvictionMitigationCap: c.cap,
				ConvictionMitigationAim: c.aim, ConvictionMitigationBend: c.bend,
			}
			b.Validate()
			want := []ConfigFloat{c.wantKnee, c.wantCap, c.wantAim, c.wantBend}
			for i, v := range mitigationCurveValues(b) {
				assert.Equal(t, want[i%4], v, mitigationCurveKeys[i])
			}
		})
	}
}

// A legal curve survives Validate untouched, including the 0.95 cap ceiling
// and a knee just under the cap.
func TestMitigationCurveKnobs_LegalValuesSurvive(t *testing.T) {
	b := Balance{
		PhysicalMitigationKnee: 0.40, PhysicalMitigationCap: 0.95,
		PhysicalMitigationAim: 0.10, PhysicalMitigationBend: 0.25,
		MagicalMitigationKnee: 0.60, MagicalMitigationCap: 0.61,
		MagicalMitigationAim: 0.01, MagicalMitigationBend: 2.0,
		ConvictionMitigationKnee: 0.30, ConvictionMitigationCap: 0.75,
		ConvictionMitigationAim: 0.05, ConvictionMitigationBend: 0.40,
	}
	b.Validate()
	assert.Equal(t, []ConfigFloat{0.40, 0.95, 0.10, 0.25, 0.60, 0.61, 0.01, 2.0, 0.30, 0.75, 0.05, 0.40},
		mitigationCurveValues(b))
}

// config.yaml names all twelve knobs at the owner's values (knee 0.50, cap
// 0.90, aim 0.05, bend 0.40 on every channel). Read from the file because a
// test binary never loads it; checked before Validate, which would otherwise
// paper over a missing or misplaced key with the same defaults.
func TestShippedConfigCarriesTheMitigationCurve(t *testing.T) {
	src := shippedConfigSource(t)
	for _, k := range mitigationCurveKeys {
		assert.True(t, bytesContainsKey(src, k+":"), "config.yaml must name %s", k)
	}
	cfg, err := loadConfig(src)
	require.NoError(t, err)
	assertCurveDefaults(t, cfg.Balance, "shipped config.yaml before Validate")
}
```

- [ ] **Step 2: Run it to see it fail.**

Run: `go test ./internal/configs/ -run "MitigationCurve" -count=1`
Expected: FAIL, build failed: `b.PhysicalMitigationKnee undefined (type Balance has no field or method PhysicalMitigationKnee)` (and the other new fields).

- [ ] **Step 3: Add the knobs and the validator.**

Replace in `internal/configs/config.balance.go`:

```go
	PhysicalMitigationCap           ConfigFloat `yaml:"PhysicalMitigationCap"`           // Max physical mitigation % (default 0.75)
	MagicalMitigationCap            ConfigFloat `yaml:"MagicalMitigationCap"`            // Max magical mitigation % (default 0.75)
	ConvictionMitigationCap         ConfigFloat `yaml:"ConvictionMitigationCap"`         // Max conviction mitigation % (default 0.75)
```

with:

```go
	// Mitigation curve (#465), four knobs per channel, read by
	// combat.MitigationCurveFor: raw mitigation counts in full up to the knee,
	// bends toward cap + aim above it, and never passes the cap. Validator:
	// 0 < knee < cap <= 0.95, aim > 0, bend > 0; a bad value falls back to the
	// default (0.50 / 0.90 / 0.05 / 0.40).
	PhysicalMitigationKnee   ConfigFloat `yaml:"PhysicalMitigationKnee"`   // Physical raw mitigation that counts in full (default 0.50)
	PhysicalMitigationCap    ConfigFloat `yaml:"PhysicalMitigationCap"`    // Hard ceiling on effective physical mitigation (default 0.90)
	PhysicalMitigationAim    ConfigFloat `yaml:"PhysicalMitigationAim"`    // How far past the cap the physical curve aims; 0 never reaches it (default 0.05)
	PhysicalMitigationBend   ConfigFloat `yaml:"PhysicalMitigationBend"`   // Raw physical mitigation past the knee that covers half the bend (default 0.40)
	MagicalMitigationKnee    ConfigFloat `yaml:"MagicalMitigationKnee"`    // Magical raw mitigation that counts in full (default 0.50)
	MagicalMitigationCap     ConfigFloat `yaml:"MagicalMitigationCap"`     // Hard ceiling on effective magical mitigation (default 0.90)
	MagicalMitigationAim     ConfigFloat `yaml:"MagicalMitigationAim"`     // How far past the cap the magical curve aims; 0 never reaches it (default 0.05)
	MagicalMitigationBend    ConfigFloat `yaml:"MagicalMitigationBend"`    // Raw magical mitigation past the knee that covers half the bend (default 0.40)
	ConvictionMitigationKnee ConfigFloat `yaml:"ConvictionMitigationKnee"` // Conviction raw mitigation that counts in full (default 0.50)
	ConvictionMitigationCap  ConfigFloat `yaml:"ConvictionMitigationCap"`  // Hard ceiling on effective conviction mitigation (default 0.90)
	ConvictionMitigationAim  ConfigFloat `yaml:"ConvictionMitigationAim"`  // How far past the cap the conviction curve aims; 0 never reaches it (default 0.05)
	ConvictionMitigationBend ConfigFloat `yaml:"ConvictionMitigationBend"` // Raw conviction mitigation past the knee that covers half the bend (default 0.40)
```

Replace in `internal/configs/config.balance.combat.go` (1 of 3):

```go
	return f > 0 && !math.IsNaN(f) && !math.IsInf(f, 0)
}
```

with:

```go
	return f > 0 && !math.IsNaN(f) && !math.IsInf(f, 0)
}

// validateMitigationCurve fills one channel's mitigation curve (#465): knee,
// ceiling (the channel's MitigationCap), aim and bend. An absent key reads 0
// and takes the default, so 0 is not expressible for any of the four (an aim
// of 0.0001 is the approach-only curve in practice). Written `!(x > 0)` so a
// NaN from YAML `.nan` falls back too. The ceiling stays at or under 0.95, so
// nothing is ever close to immune. A knee not below the ceiling takes the
// default knee; a ceiling at or under even that takes the whole default
// curve. The defaults are the shipped values, so a test binary (which never
// loads config.yaml) runs the shipped curve.
func validateMitigationCurve(knee, ceiling, aim, bend *ConfigFloat) {
	if !(*ceiling > 0) || *ceiling > 0.95 {
		*ceiling = 0.90
	}
	if !(*knee > 0) || *knee >= *ceiling {
		*knee = 0.50
	}
	if *knee >= *ceiling {
		*knee, *ceiling = 0.50, 0.90
	}
	if !validPositiveActionCost(*aim) {
		*aim = 0.05
	}
	if !validPositiveActionCost(*bend) {
		*bend = 0.40
	}
}
```

Replace in `internal/configs/config.balance.combat.go` (2 of 3):

```go
// special moves, skullduggery, darkness, damage channels, mitigation caps,
// and toxicity fields.
```

with:

```go
// special moves, skullduggery, darkness, damage channels, mitigation curves,
// and toxicity fields.
```

Replace in `internal/configs/config.balance.combat.go` (3 of 3):

```go
	if b.PhysicalMitigationCap <= 0 || b.PhysicalMitigationCap > 1.0 {
		b.PhysicalMitigationCap = 0.75
	}
	if b.MagicalMitigationCap <= 0 || b.MagicalMitigationCap > 1.0 {
		b.MagicalMitigationCap = 0.75
	}
	if b.ConvictionMitigationCap <= 0 || b.ConvictionMitigationCap > 1.0 {
		b.ConvictionMitigationCap = 0.75
	}
```

with:

```go
	validateMitigationCurve(&b.PhysicalMitigationKnee, &b.PhysicalMitigationCap,
		&b.PhysicalMitigationAim, &b.PhysicalMitigationBend)
	validateMitigationCurve(&b.MagicalMitigationKnee, &b.MagicalMitigationCap,
		&b.MagicalMitigationAim, &b.MagicalMitigationBend)
	validateMitigationCurve(&b.ConvictionMitigationKnee, &b.ConvictionMitigationCap,
		&b.ConvictionMitigationAim, &b.ConvictionMitigationBend)
```

Run: `go test ./internal/configs/ -run "MitigationCurve" -count=1`
Expected: FAIL, only `TestShippedConfigCarriesTheMitigationCurve`: nine `config.yaml must name ...` errors (the Knee, Aim and Bend keys) and twelve value mismatches (the new keys read 0, each cap `expected: 0.9 actual: 0.75`). The defaults, fallback and legal-value tests pass.

- [ ] **Step 4: Ship the values in `config.yaml`, built from the HEAD blob.**

`_datafiles/config.yaml` carries skip-worktree in the main checkout, and its disk copy there holds local-only values that must never be committed. Build the change from the committed blob:

Run: `git ls-files -v _datafiles/config.yaml`
Expected: `H _datafiles/config.yaml` (a fresh worktree). If it prints `S`, stop: you are in a checkout with local config edits; implement in a fresh worktree of master instead.

Run: `git show HEAD:_datafiles/config.yaml > _datafiles/config.yaml`
Then: `git diff --stat HEAD -- _datafiles/config.yaml`
Expected: no output (the disk copy is now the blob; with `core.autocrlf=true` git may warn "LF will be replaced by CRLF", which is harmless).

Replace in `_datafiles/config.yaml`:

```yaml
  # Mitigation caps: maximum % damage reduction from armor for each channel.
  # 0.75 = armor can block at most 75% of incoming damage in that channel.
  PhysicalMitigationCap: 0.75
  MagicalMitigationCap: 0.75
  ConvictionMitigationCap: 0.75
```

with:

```yaml
  # Mitigation curve (#465), four knobs per channel. Raw mitigation (every
  # source summed) counts in full up to the Knee, bends toward Cap + Aim above
  # it, and never stops more than Cap of the damage. Aim 0 only approaches the
  # cap; above 0 the cap is reachable. Bend is the raw mitigation past the knee
  # that covers half the distance. The part of effective mitigation above the
  # knee also softens critical hits. Validator: 0 < Knee < Cap <= 0.95, Aim and
  # Bend above 0; a bad value falls back to 0.50 / 0.90 / 0.05 / 0.40.
  # At these values: raw 0.75 stops 67%, raw 1.00 75%, raw 1.81 84%, raw 3.70 90%.
  PhysicalMitigationKnee: 0.50
  PhysicalMitigationCap: 0.90
  PhysicalMitigationAim: 0.05
  PhysicalMitigationBend: 0.40
  MagicalMitigationKnee: 0.50
  MagicalMitigationCap: 0.90
  MagicalMitigationAim: 0.05
  MagicalMitigationBend: 0.40
  ConvictionMitigationKnee: 0.50
  ConvictionMitigationCap: 0.90
  ConvictionMitigationAim: 0.05
  ConvictionMitigationBend: 0.40
```

Run: `git diff --stat HEAD -- _datafiles/config.yaml`
Expected: `_datafiles/config.yaml | 25 ++++++++++++++++++++-----` (20 insertions, 5 deletions, nothing else).

- [ ] **Step 5: Run the tests.**

Run: `go test ./internal/configs/ -count=1`
Expected: PASS (`ok`), including `TestShippedConfigCarriesTheMitigationCurve` and the existing `smoke_test.go` cap bounds.

Run: `gofmt -l internal/configs/ && go build ./... && go vet ./internal/configs/ && go test . -count=1`
Expected: no gofmt output; `ok`. (Until Task 3 the code still clamps, now at the 0.90 default; `internal/combat`, `hooks`, `actions`, `usercommands` and `templates` tests still pass in the dry run. D11.)

- [ ] **Step 6: Commit.**

```bash
git add internal/configs/config_mitigation_curve_test.go \
  internal/configs/config.balance.go \
  internal/configs/config.balance.combat.go \
  _datafiles/config.yaml
git ls-files -v _datafiles/config.yaml
git commit -F - <<'EOF'
feat(configs): twelve mitigation curve knobs, shipped at 0.50/0.90/0.05/0.40

Knee, Cap, Aim and Bend per channel. The Cap keys keep their names and
become the curve's ceiling. validateMitigationCurve enforces
0 < knee < cap <= 0.95 with aim and bend above 0, falling back to the
shipped values, which are also the Go defaults. config.yaml built from
the HEAD blob.

Refs #465

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

The `git ls-files -v` line must still print `H` (a fresh worktree); the bit is the main checkout's concern, not this branch's.

---

### Task 3: `MitigationCurveFor` replaces `MitigationCap`

`ApplyMitigation`, `CritOrMitigatedDamage(Scaled)` and `ReflectDamage` take a `MitigationCurve`; all 10 call lines pass `MitigationCurveFor(channel)`. Armor piercing still scales raw mitigation first. Crits are untouched here (Task 4). Every pin moves to the curve.

- [ ] **Step 1: Move the pins to the curve.**

Replace in `internal/combat/damage_pipeline_test.go` (1 of 4):

```go
func TestApplyMitigation(t *testing.T) {
	// No mitigation
	result := ApplyMitigation(100, 0, 0.75)
	if math.Abs(result-100.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 0, 0.75) = %f, want 100.0", result)
	}

	// 50% mitigation
	result2 := ApplyMitigation(100, 0.50, 0.75)
	if math.Abs(result2-50.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 0.50, 0.75) = %f, want 50.0", result2)
	}

	// 90% mitigation capped at 75%
	result3 := ApplyMitigation(100, 0.90, 0.75)
	if math.Abs(result3-25.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 0.90, 0.75) = %f, want 25.0 (capped)", result3)
	}

	// Negative mitigation treated as 0
	result4 := ApplyMitigation(100, -0.5, 0.75)
	if math.Abs(result4-100.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, -0.5, 0.75) = %f, want 100.0", result4)
	}
}
```

with:

```go
func TestApplyMitigation(t *testing.T) {
	c := shippedCurveForTest

	// No mitigation
	result := ApplyMitigation(100, 0, c)
	if math.Abs(result-100.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 0, curve) = %f, want 100.0", result)
	}

	// 50% mitigation, at the knee: counts in full
	result2 := ApplyMitigation(100, 0.50, c)
	if math.Abs(result2-50.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 0.50, curve) = %f, want 50.0", result2)
	}

	// 100% raw bends to 75% effective (#465): 100 -> 25
	result3 := ApplyMitigation(100, 1.00, c)
	if math.Abs(result3-25.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, 1.00, curve) = %f, want 25.0 (bent)", result3)
	}

	// Negative mitigation treated as 0
	result4 := ApplyMitigation(100, -0.5, c)
	if math.Abs(result4-100.0) > 0.01 {
		t.Errorf("ApplyMitigation(100, -0.5, curve) = %f, want 100.0", result4)
	}
}
```

Replace in `internal/combat/damage_pipeline_test.go` (2 of 4):

```go
// ─── Stage 40.2: DamageScale and MitigationCap edge cases ──────────────────
```

with:

```go
// ─── Stage 40.2: DamageScale and MitigationCurveFor edge cases ─────────────
```

Replace in `internal/combat/damage_pipeline_test.go` (3 of 4):

```go
// TestMitigationCap verifies each channel returns its OWN configured cap.
//
// This previously asserted only `0 < got <= 1.0` while its comment claimed to
// check the configured value — so an implementation returning 0.99 for every
// channel, or wiring Magical to the Physical cap, passed cleanly. The three
// caps also default to the same number (0.75), so comparing against config
// alone cannot detect cross-wiring either. The overrides below give each
// channel a distinct value so a mix-up actually fails.
func TestMitigationCap(t *testing.T) {
	err := configs.AddOverlayOverrides(map[string]any{
		"Balance.PhysicalMitigationCap":   0.61,
		"Balance.MagicalMitigationCap":    0.62,
		"Balance.ConvictionMitigationCap": 0.63,
	})
	if err != nil {
		t.Fatalf("failed to override balance config: %v", err)
	}

	tests := []struct {
		name    string
		channel DamageChannel
		want    float64
	}{
		{"Physical", ChannelPhysical, 0.61},
		{"Magical", ChannelMagical, 0.62},
		{"Conviction", ChannelConviction, 0.63},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MitigationCap(tt.channel)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("MitigationCap(%v) = %f, want %f (its own configured cap)", tt.channel, got, tt.want)
			}
			if got <= 0 || got > 1.0 {
				t.Errorf("MitigationCap(%v) = %f, outside the sane range (0, 1.0]", tt.channel, got)
			}
		})
	}

	t.Run("UnknownChannel", func(t *testing.T) {
		// Defensive default, deliberately not config-driven.
		if got := MitigationCap(DamageChannel(99)); got != 0.75 {
			t.Errorf("MitigationCap(unknown) = %f, want the 0.75 fallback", got)
		}
	})
}
```

with:

```go
// TestMitigationCurveFor verifies each channel returns its OWN configured
// curve, all four knobs.
//
// The twelve knobs ship at the same four numbers on every channel, so
// comparing against config alone cannot detect cross-wiring. The overrides
// below give every knob a distinct value so a mix-up actually fails. Pinned
// through SetConfigForTest, which restores on cleanup (the cap test this
// replaced wrote a permanent overlay that leaked into every later test).
func TestMitigationCurveFor(t *testing.T) {
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.PhysicalMitigationKnee, b.PhysicalMitigationCap = 0.41, 0.61
	b.PhysicalMitigationAim, b.PhysicalMitigationBend = 0.011, 0.31
	b.MagicalMitigationKnee, b.MagicalMitigationCap = 0.42, 0.62
	b.MagicalMitigationAim, b.MagicalMitigationBend = 0.012, 0.32
	b.ConvictionMitigationKnee, b.ConvictionMitigationCap = 0.43, 0.63
	b.ConvictionMitigationAim, b.ConvictionMitigationBend = 0.013, 0.33
	configs.SetConfigForTest(t, cfg)

	tests := []struct {
		name    string
		channel DamageChannel
		want    MitigationCurve
	}{
		{"Physical", ChannelPhysical, MitigationCurve{Knee: 0.41, Cap: 0.61, Aim: 0.011, Bend: 0.31}},
		{"Magical", ChannelMagical, MitigationCurve{Knee: 0.42, Cap: 0.62, Aim: 0.012, Bend: 0.32}},
		{"Conviction", ChannelConviction, MitigationCurve{Knee: 0.43, Cap: 0.63, Aim: 0.013, Bend: 0.33}},
		// An unknown channel falls through to the physical curve.
		{"UnknownChannel", DamageChannel(99), MitigationCurve{Knee: 0.41, Cap: 0.61, Aim: 0.011, Bend: 0.31}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MitigationCurveFor(tt.channel)
			if math.Abs(got.Knee-tt.want.Knee) > 1e-9 || math.Abs(got.Cap-tt.want.Cap) > 1e-9 ||
				math.Abs(got.Aim-tt.want.Aim) > 1e-9 || math.Abs(got.Bend-tt.want.Bend) > 1e-9 {
				t.Errorf("MitigationCurveFor(%v) = %+v, want %+v (its own configured curve)", tt.channel, got, tt.want)
			}
		})
	}
}

// With no overrides a test binary runs the Go defaults, which equal the
// shipped curve (config_mitigation_curve_test.go pins the shipped file).
func TestMitigationCurveFor_DefaultsAreTheShippedCurve(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance = configs.Balance{}
	cfg.Balance.Validate()
	configs.SetConfigForTest(t, cfg)

	for _, ch := range []DamageChannel{ChannelPhysical, ChannelMagical, ChannelConviction} {
		got := MitigationCurveFor(ch)
		if math.Abs(got.Knee-0.50) > 1e-9 || math.Abs(got.Cap-0.90) > 1e-9 ||
			math.Abs(got.Aim-0.05) > 1e-9 || math.Abs(got.Bend-0.40) > 1e-9 {
			t.Errorf("MitigationCurveFor(%v) = %+v, want the shipped 0.50 / 0.90 / 0.05 / 0.40", ch, got)
		}
	}
}
```

Replace in `internal/combat/damage_pipeline_test.go` (4 of 4):

```go
func TestApplyMitigation_EdgeCases(t *testing.T) {
	// 100% mitigation should be capped
	got := ApplyMitigation(100.0, 1.0, 0.75)
	expected := 25.0 // 100 * (1 - 0.75)
	if math.Abs(got-expected) > 0.01 {
		t.Errorf("ApplyMitigation(100, 1.0, 0.75) = %f, want %f", got, expected)
	}

	// Zero raw → zero result
	got = ApplyMitigation(0, 0.5, 0.75)
	if got != 0 {
		t.Errorf("ApplyMitigation(0, 0.5, 0.75) = %f, want 0", got)
	}
}
```

with:

```go
func TestApplyMitigation_EdgeCases(t *testing.T) {
	// Far past any build, mitigation stops at the 90% cap: 100 -> 10
	got := ApplyMitigation(100.0, 10.0, shippedCurveForTest)
	expected := 10.0 // 100 * (1 - 0.90)
	if math.Abs(got-expected) > 0.01 {
		t.Errorf("ApplyMitigation(100, 10.0, curve) = %f, want %f", got, expected)
	}

	// Zero raw → zero result
	got = ApplyMitigation(0, 0.5, shippedCurveForTest)
	if got != 0 {
		t.Errorf("ApplyMitigation(0, 0.5, curve) = %f, want 0", got)
	}
}
```

Replace in `internal/combat/regression_test.go`:

```go
// TestRegression_MitigationCapEnforced guards against bug where mitigation
// exceeding 75% was not properly capped.
func TestRegression_MitigationCapEnforced(t *testing.T) {
	testCases := []struct {
		name          string
		raw           float64
		mitigationPct float64
		cap           float64
		expected      float64
	}{
		{"under_cap", 100.0, 0.50, 0.75, 50.0},
		{"at_cap", 100.0, 0.75, 0.75, 25.0},
		{"over_cap_clamped", 100.0, 0.95, 0.75, 25.0},
		{"full_mitigation_capped", 100.0, 1.00, 0.75, 25.0},
		{"negative_mitigation_treated_as_zero", 100.0, -0.10, 0.75, 100.0},
		{"zero_mitigation", 100.0, 0.0, 0.75, 100.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyMitigation(tc.raw, tc.mitigationPct, tc.cap)
			assert.InDelta(t, tc.expected, result, 0.01,
				"ApplyMitigation(%.0f, %.2f, %.2f) should be %.0f",
				tc.raw, tc.mitigationPct, tc.cap, tc.expected)
		})
	}
}
```

with:

```go
// TestRegression_MitigationCapEnforced guards against mitigation that is not
// bounded. Since #465 the bound is a curve: linear to the knee (0.50),
// diminishing above it, never past the cap (0.90).
func TestRegression_MitigationCapEnforced(t *testing.T) {
	testCases := []struct {
		name          string
		raw           float64
		mitigationPct float64
		expected      float64
	}{
		{"under_knee", 100.0, 0.40, 60.0},
		{"at_knee", 100.0, 0.50, 50.0},
		{"over_knee_bent", 100.0, 0.75, 32.7},
		{"full_raw_bent", 100.0, 1.00, 25.0},
		{"far_past_any_build_capped", 100.0, 10.0, 10.0},
		{"negative_mitigation_treated_as_zero", 100.0, -0.10, 100.0},
		{"zero_mitigation", 100.0, 0.0, 100.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyMitigation(tc.raw, tc.mitigationPct, shippedCurveForTest)
			assert.InDelta(t, tc.expected, result, 0.05,
				"ApplyMitigation(%.0f, %.2f, curve) should be %.1f",
				tc.raw, tc.mitigationPct, tc.expected)
		})
	}
}
```

Replace in `internal/combat/reflect_damage_test.go`:

```go
func TestReflectDamage_RespectsCap(t *testing.T) {
	// 90% mitigation is clamped to the 75% cap, so 25 -> 6 (6.25 truncated).
	assert.Equal(t, 6, ReflectDamage(100, 25, 0.90, 0.75),
		"mitigation above the cap must clamp to the cap")
}
```

with:

```go
func TestReflectDamage_FollowsTheCurve(t *testing.T) {
	// 100% raw bends to 75% effective (#465), so 25 -> 6 (6.25 truncated).
	assert.Equal(t, 6, ReflectDamage(100, 25, 1.00, shippedCurveForTest),
		"mitigation above the knee must follow the curve")
	// Far past any build it stops at the 90% cap: 25 -> 2 (2.5 truncated).
	assert.Equal(t, 2, ReflectDamage(100, 25, 10.0, shippedCurveForTest),
		"mitigation must never pass the cap")
}
```

Replace all 7 occurrences in `internal/combat/reflect_damage_test.go`:

```go
, 0.75)
```

with:

```go
, shippedCurveForTest)
```

Replace in `internal/combat/integration_combat_test.go` (1 of 4):

```go
		mitigated := ApplyMitigation(rawDmg, defPhysMit, MitigationCap(ChannelPhysical))
```

with:

```go
		mitigated := ApplyMitigation(rawDmg, defPhysMit, MitigationCurveFor(ChannelPhysical))
```

Replace in `internal/combat/integration_combat_test.go` (2 of 4):

```go
// Exercises: GetPhysicalMitigation(), ApplyMitigation(), MitigationCap()
```

with:

```go
// Exercises: GetPhysicalMitigation(), ApplyMitigation(), MitigationCurveFor()
```

Replace in `internal/combat/integration_combat_test.go` (3 of 4):

```go
	mitigated := ApplyMitigation(rawDmg, physMit, 0.75)
	expected := rawDmg * (1 - physMit)
	assert.InDelta(t, expected, mitigated, 0.01,
		"Mitigated damage should be raw * (1 - mitigation%)")

	// Test mitigation cap at 75%
	heavilyArmored := 0.90 // 90% mitigation from gear
	capped := ApplyMitigation(rawDmg, heavilyArmored, 0.75)
	assert.InDelta(t, 25.0, capped, 0.01,
		"Mitigation above 75% cap should be capped to 75%")

	// Verify each channel has the correct cap from config
	for _, ch := range []DamageChannel{ChannelPhysical, ChannelMagical, ChannelConviction} {
		cap := MitigationCap(ch)
		assert.Greater(t, cap, 0.0, "Mitigation cap should be positive")
		assert.LessOrEqual(t, cap, 1.0, "Mitigation cap should be <= 1.0")
	}
```

with:

```go
	mitigated := ApplyMitigation(rawDmg, physMit, shippedCurveForTest)
	expected := rawDmg * (1 - physMit)
	assert.InDelta(t, expected, mitigated, 0.01,
		"Mitigation below the knee should be raw * (1 - mitigation%)")

	// Past the knee the curve bends (#465): raw 90% from gear stops 72.5%
	heavilyArmored := 0.90
	bent := ApplyMitigation(rawDmg, heavilyArmored, shippedCurveForTest)
	assert.InDelta(t, 100*(1-EffectiveMitigation(0.90, shippedCurveForTest)), bent, 0.01,
		"Mitigation above the knee should follow the curve")
	assert.InDelta(t, 27.5, bent, 0.01, "raw 90% stops 72.5%")

	// Every channel's configured curve is sane: 0 < knee < cap <= 0.95
	for _, ch := range []DamageChannel{ChannelPhysical, ChannelMagical, ChannelConviction} {
		c := MitigationCurveFor(ch)
		assert.Greater(t, c.Knee, 0.0, "knee should be positive")
		assert.Less(t, c.Knee, c.Cap, "knee should be below the cap")
		assert.LessOrEqual(t, c.Cap, 0.95, "cap should be <= 0.95")
	}
```

Replace in `internal/combat/integration_combat_test.go` (4 of 4):

```go
	mitigated := ApplyMitigation(rawDmg, physMitigation, MitigationCap(ChannelPhysical))
```

with:

```go
	mitigated := ApplyMitigation(rawDmg, physMitigation, MitigationCurveFor(ChannelPhysical))
```

Replace all 2 occurrences in `internal/combat/crit_damage_test.go`:

```go
0.75, 0.75)
```

with:

```go
0.75, shippedCurveForTest)
```

Replace all 2 occurrences in `internal/combat/crit_damage_test.go`:

```go
0.0, 0.75)
```

with:

```go
0.0, shippedCurveForTest)
```

Replace in `internal/combat/crit_damage_test.go`:

```go
0.50, 0.75)
```

with:

```go
0.50, shippedCurveForTest)
```

Replace all 3 occurrences in `internal/combat/crit_damage_test.go`:

```go
0.0, 0.75, 1.0)
```

with:

```go
0.0, shippedCurveForTest, 1.0)
```

Replace in `internal/combat/crit_damage_test.go`:

```go
0.0, 0.75, bonus)
```

with:

```go
0.0, shippedCurveForTest, bonus)
```

Replace in `internal/combat/crit_damage_test.go`:

```go
0.0, 0.75, 8.0)
```

with:

```go
0.0, shippedCurveForTest, 8.0)
```

Replace in `internal/combat/crit_damage_test.go`:

```go
0.0, 0.75, 0)
```

with:

```go
0.0, shippedCurveForTest, 0)
```

Replace in `internal/combat/melee_parity_stat_test.go` (1 of 4):

```go
// mitigation cells on the defender: light (0%), mid (40%), BIS (75% — the
// PhysicalMitigationCap). Mitigation is injected as a condition statmod
```

with:

```go
// mitigation cells on the defender: light (0%), mid (40%), BIS (75% raw, the
// old cap; the #465 curve bends it to about 67%, past the knee). Mitigation
// is injected as a condition statmod
```

Replace in `internal/combat/melee_parity_stat_test.go` (2 of 4):

```go
	b.PhysicalMitigationCap = 0.75
	b.StaminaPenaltyMax = 0.28
```

with:

```go
	b.PhysicalMitigationKnee = 0.50
	b.PhysicalMitigationCap = 0.90
	b.PhysicalMitigationAim = 0.05
	b.PhysicalMitigationBend = 0.40
	b.StaminaPenaltyMax = 0.28
```

Replace in `internal/combat/melee_parity_stat_test.go` (3 of 4):

```go
		{"BIS-75pct-cap", 75},
```

with:

```go
		{"BIS-75pct-raw", 75},
```

Replace in `internal/combat/melee_parity_stat_test.go` (4 of 4):

```go
			wantDmgMean := ApplyMitigation(rawDamage, float64(cell.mitPct)/100.0, 0.75)
```

with:

```go
			wantDmgMean := ApplyMitigation(rawDamage, float64(cell.mitPct)/100.0, shippedCurveForTest)
```

Replace in `internal/combat/surprise_opening_strike_test.go`:

```go
	b.PhysicalMitigationCap = 0.75
```

with:

```go
	b.PhysicalMitigationKnee = 0.50
	b.PhysicalMitigationCap = 0.90
	b.PhysicalMitigationAim = 0.05
	b.PhysicalMitigationBend = 0.40
```

Replace in `internal/actions/combat_fire_surprise_test.go`:

```go
	b.PhysicalMitigationCap = 0.75
```

with:

```go
	b.PhysicalMitigationKnee = 0.50
	b.PhysicalMitigationCap = 0.90
	b.PhysicalMitigationAim = 0.05
	b.PhysicalMitigationBend = 0.40
```

Replace in `internal/hooks/counter_tier_test.go`:

```go
		combat.MitigationCap(combat.ChannelPhysical))
	require.InDelta(t, wantMean, meanHalf, wantMean*0.10,
```

with:

```go
		combat.MitigationCurveFor(combat.ChannelPhysical))
	require.InDelta(t, wantMean, meanHalf, wantMean*0.10,
```

- [ ] **Step 2: Run them to see them fail.**

Run: `go vet ./internal/combat/`
Expected: FAIL, build failed: `cannot use shippedCurveForTest (variable of struct type MitigationCurve) as float64 value in argument to ApplyMitigation` (and to `ReflectDamage`, `CritOrMitigatedDamage`, `CritOrMitigatedDamageScaled`), `undefined: MitigationCurveFor`.

Run: `go vet ./internal/hooks/`
Expected: FAIL, build failed: `undefined: combat.MitigationCurveFor`.

- [ ] **Step 3: The curve replaces the clamp.**

Replace in `internal/combat/damage_pipeline.go`:

```go
// ApplyMitigation reduces raw damage by a mitigation percentage, capped.
// final = raw × (1 - min(mitigationPct, cap))
// mitigationPct and cap are fractions (0.0–1.0).
func ApplyMitigation(rawDmg float64, mitigationPct float64, cap float64) float64 {
	if mitigationPct < 0 {
		mitigationPct = 0
	}
	if cap <= 0 {
		cap = 0.75
	}
	if mitigationPct > cap {
		mitigationPct = cap
	}
	return rawDmg * (1.0 - mitigationPct)
}

// MitigationCap returns the configured cap for a given damage channel.
func MitigationCap(channel DamageChannel) float64 {
	bal := configs.GetBalanceConfig()
	switch channel {
	case ChannelPhysical:
		return float64(bal.PhysicalMitigationCap)
	case ChannelMagical:
		return float64(bal.MagicalMitigationCap)
	case ChannelConviction:
		return float64(bal.ConvictionMitigationCap)
	default:
		return 0.75
	}
}
```

with:

```go
// ApplyMitigation reduces raw damage by raw mitigation bent through curve
// (#465): final = raw x (1 - EffectiveMitigation(mitigationPct, curve)).
// mitigationPct is the uncapped fraction Character.Get*Mitigation returns,
// after any armor piercing has scaled it; the curve, not the caller, bounds it.
func ApplyMitigation(rawDmg float64, mitigationPct float64, curve MitigationCurve) float64 {
	return rawDmg * (1.0 - EffectiveMitigation(mitigationPct, curve))
}

// MitigationCurveFor returns the configured mitigation curve for a damage
// channel (#465), four knobs per channel. An unknown channel reads the
// physical curve, so a caller that falls through still meets a real curve.
func MitigationCurveFor(channel DamageChannel) MitigationCurve {
	bal := configs.GetBalanceConfig()
	switch channel {
	case ChannelMagical:
		return MitigationCurve{
			Knee: float64(bal.MagicalMitigationKnee),
			Cap:  float64(bal.MagicalMitigationCap),
			Aim:  float64(bal.MagicalMitigationAim),
			Bend: float64(bal.MagicalMitigationBend),
		}
	case ChannelConviction:
		return MitigationCurve{
			Knee: float64(bal.ConvictionMitigationKnee),
			Cap:  float64(bal.ConvictionMitigationCap),
			Aim:  float64(bal.ConvictionMitigationAim),
			Bend: float64(bal.ConvictionMitigationBend),
		}
	default:
		return MitigationCurve{
			Knee: float64(bal.PhysicalMitigationKnee),
			Cap:  float64(bal.PhysicalMitigationCap),
			Aim:  float64(bal.PhysicalMitigationAim),
			Bend: float64(bal.PhysicalMitigationBend),
		}
	}
}
```

Replace in `internal/combat/crit_damage.go` (1 of 3):

```go
func CritOrMitigatedDamageScaled(rawDmg float64, skillRank int, isCrit bool, mitigPct, mitigCap, bonusCritMult float64) int {
```

with:

```go
func CritOrMitigatedDamageScaled(rawDmg float64, skillRank int, isCrit bool, mitigPct float64, curve MitigationCurve, bonusCritMult float64) int {
```

Replace in `internal/combat/crit_damage.go` (2 of 3):

```go
		mean = ApplyMitigation(rawDmg, mitigPct, mitigCap)
```

with:

```go
		mean = ApplyMitigation(rawDmg, mitigPct, curve)
```

Replace in `internal/combat/crit_damage.go` (3 of 3):

```go
func CritOrMitigatedDamage(rawDmg float64, skillRank int, isCrit bool, mitigPct, mitigCap float64) int {
	return CritOrMitigatedDamageScaled(rawDmg, skillRank, isCrit, mitigPct, mitigCap, 1.0)
```

with:

```go
func CritOrMitigatedDamage(rawDmg float64, skillRank int, isCrit bool, mitigPct float64, curve MitigationCurve) int {
	return CritOrMitigatedDamageScaled(rawDmg, skillRank, isCrit, mitigPct, curve, 1.0)
```

Replace in `internal/combat/reflect_damage.go`:

```go
// fraction for the matching channel (they are the one taking it).
//
// Returns 0 for a non-positive percentage so callers can sum channels without
// guarding each one.
func ReflectDamage(damageDealt float64, returnPct int, mitigation, cap float64) int {
	if returnPct <= 0 || damageDealt <= 0 {
		return 0
	}

	raw := damageDealt * float64(returnPct) / 100.0

	return int(ApplyMitigation(raw, mitigation, cap))
```

with:

```go
// fraction for the matching channel (they are the one taking it), bent
// through that channel's curve.
//
// Returns 0 for a non-positive percentage so callers can sum channels without
// guarding each one.
func ReflectDamage(damageDealt float64, returnPct int, mitigation float64, curve MitigationCurve) int {
	if returnPct <= 0 || damageDealt <= 0 {
		return 0
	}

	raw := damageDealt * float64(returnPct) / 100.0

	return int(ApplyMitigation(raw, mitigation, curve))
```

Replace in `internal/combat/combat_helpers.go` (1 of 2):

```go
	dmgMean := ApplyMitigation(rawDmg, targetChar.GetPhysicalMitigation(), MitigationCap(ChannelPhysical))
```

with:

```go
	dmgMean := ApplyMitigation(rawDmg, targetChar.GetPhysicalMitigation(), MitigationCurveFor(ChannelPhysical))
```

Replace in `internal/combat/combat_helpers.go` (2 of 2):

```go
	petMitigationCap := MitigationCap(ChannelPhysical)
	petBaseDmg = ApplyMitigation(petBaseDmg, petMitigation, petMitigationCap)
	petVar = ApplyMitigation(petVar, petMitigation, petMitigationCap)
```

with:

```go
	petMitigationCurve := MitigationCurveFor(ChannelPhysical)
	petBaseDmg = ApplyMitigation(petBaseDmg, petMitigation, petMitigationCurve)
	petVar = ApplyMitigation(petVar, petMitigation, petMitigationCurve)
```

Replace in `internal/combat/skill_moves.go` (1 of 2):

```go
	mitig := p.Defender.GetPhysicalMitigation() * mitigMult
	cap := MitigationCap(ChannelPhysical)
```

with:

```go
	// Armor piercing scales RAW mitigation, then the curve bends it (#465):
	// a stomp halves the defender's raw sum before the knee is applied.
	mitig := p.Defender.GetPhysicalMitigation() * mitigMult
	curve := MitigationCurveFor(ChannelPhysical)
```

Replace in `internal/combat/skill_moves.go` (2 of 2):

```go
	dmg := CritOrMitigatedDamageScaled(rawDmg, p.Attack.SkillRank, result.Crit, mitig, cap, p.BonusCritMultiplier)
```

with:

```go
	dmg := CritOrMitigatedDamageScaled(rawDmg, p.Attack.SkillRank, result.Crit, mitig, curve, p.BonusCritMultiplier)
```

Replace in `internal/combat/pools.go`:

```go
// pipeline's pool: which scale knob, which mitigation cap, which stat
```

with:

```go
// pipeline's pool: which scale knob, which mitigation curve, which stat
```

Replace in `internal/actions/combat_counter.go`:

```go
		target.GetConvictionMitigation(),
		combat.MitigationCap(combat.ChannelConviction),
```

with:

```go
		target.GetConvictionMitigation(),
		combat.MitigationCurveFor(combat.ChannelConviction),
```

Replace in `internal/actions/combat_taunt.go`:

```go
		target.Char.GetConvictionMitigation(),
		combat.MitigationCap(combat.ChannelConviction),
```

with:

```go
		target.Char.GetConvictionMitigation(),
		combat.MitigationCurveFor(combat.ChannelConviction),
```

Replace in `internal/hooks/combat_shared_helpers.go` (1 of 3):

```go
		var mitigPct, cap float64
		if ch, ok := combat.MitigationChannelFor(spellData.DamageType); ok {
			switch ch {
			case combat.ChannelPhysical:
				mitigPct = target.GetPhysicalMitigation()
			case combat.ChannelMagical:
				mitigPct = target.GetMagicalMitigation()
			case combat.ChannelConviction:
				mitigPct = target.GetConvictionMitigation()
			}
			cap = combat.MitigationCap(ch)
		} else {
			mudlog.Error("calcSpellDamageForCharacter", "spell", spellData.SpellId, "error", "non-harm spell reached the damage pipeline")
			cap = 0.75
		}
```

with:

```go
		var mitigPct float64
		var curve combat.MitigationCurve
		if ch, ok := combat.MitigationChannelFor(spellData.DamageType); ok {
			switch ch {
			case combat.ChannelPhysical:
				mitigPct = target.GetPhysicalMitigation()
			case combat.ChannelMagical:
				mitigPct = target.GetMagicalMitigation()
			case combat.ChannelConviction:
				mitigPct = target.GetConvictionMitigation()
			}
			curve = combat.MitigationCurveFor(ch)
		} else {
			mudlog.Error("calcSpellDamageForCharacter", "spell", spellData.SpellId, "error", "non-harm spell reached the damage pipeline")
			curve = combat.MitigationCurveFor(combat.ChannelPhysical)
		}
```

Replace in `internal/hooks/combat_shared_helpers.go` (2 of 3):

```go
		return combat.CritOrMitigatedDamage(rawDmg, skillLevel, isCrit, mitigPct, cap)
```

with:

```go
		return combat.CritOrMitigatedDamage(rawDmg, skillLevel, isCrit, mitigPct, curve)
```

Replace in `internal/hooks/combat_shared_helpers.go` (3 of 3):

```go
		dmgMean := combat.ApplyMitigation(raw, attacker.GetPhysicalMitigation(),
			combat.MitigationCap(combat.ChannelPhysical))
```

with:

```go
		dmgMean := combat.ApplyMitigation(raw, attacker.GetPhysicalMitigation(),
			combat.MitigationCurveFor(combat.ChannelPhysical))
```

Replace in `internal/hooks/NewRound_DoCombat_unified.go`:

```go
			atkChar.GetPhysicalMitigation(), combat.MitigationCap(combat.ChannelPhysical))
		returnDmg += combat.ReflectDamage(dealt, magReturnPct,
			atkChar.GetMagicalMitigation(), combat.MitigationCap(combat.ChannelMagical))
```

with:

```go
			atkChar.GetPhysicalMitigation(), combat.MitigationCurveFor(combat.ChannelPhysical))
		returnDmg += combat.ReflectDamage(dealt, magReturnPct,
			atkChar.GetMagicalMitigation(), combat.MitigationCurveFor(combat.ChannelMagical))
```

Replace in `internal/usercommands/throw.go`:

```go
			mitCap := combat.MitigationCap(combat.ChannelPhysical)
			dmg = combat.CritOrMitigatedDamage(rawDmg, skullduggery, out.AttackerCrit, mitPct, mitCap)
```

with:

```go
			mitCurve := combat.MitigationCurveFor(combat.ChannelPhysical)
			dmg = combat.CritOrMitigatedDamage(rawDmg, skullduggery, out.AttackerCrit, mitPct, mitCurve)
```

- [ ] **Step 4: Run the tests.**

Run: `go build ./... && go vet ./...`
Expected: no output.

Run (on its own line; zero matches expected): `grep -rn "MitigationCap(" --include=*.go internal modules *.go`
Expected: no output. The grep can match: `grep -rn "MitigationCurveFor(" --include=*.go internal | wc -l` prints at least 10.

Run: `go test ./internal/combat/ ./internal/configs/ ./internal/hooks/ ./internal/actions/ ./internal/usercommands/ -count=1`
Expected: PASS, `ok` for all five (`internal/combat` about 20 s, `internal/actions` about a minute).

Run: `gofmt -l internal/ && go test . -count=1`
Expected: no gofmt output; `ok`.

- [ ] **Step 5: Commit.**

```bash
git add internal/combat/damage_pipeline_test.go internal/combat/regression_test.go \
  internal/combat/reflect_damage_test.go internal/combat/integration_combat_test.go \
  internal/combat/crit_damage_test.go internal/combat/melee_parity_stat_test.go \
  internal/combat/surprise_opening_strike_test.go internal/actions/combat_fire_surprise_test.go \
  internal/hooks/counter_tier_test.go internal/combat/damage_pipeline.go \
  internal/combat/crit_damage.go internal/combat/reflect_damage.go \
  internal/combat/combat_helpers.go internal/combat/skill_moves.go internal/combat/pools.go \
  internal/actions/combat_counter.go internal/actions/combat_taunt.go \
  internal/hooks/combat_shared_helpers.go internal/hooks/NewRound_DoCombat_unified.go \
  internal/usercommands/throw.go
git commit -F - <<'EOF'
feat(combat): mitigation bends through a curve instead of a 0.75 clamp

MitigationCurveFor(channel) replaces MitigationCap. ApplyMitigation,
CritOrMitigatedDamage(Scaled) and ReflectDamage take the curve; all ten
call lines pass their channel's curve. Armor piercing still scales raw
mitigation first. Every pin of the old clamp is moved to the curve.

Refs #465

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Crits lose the overflow above the knee

A crit skips mitigation up to the knee and is reduced by `max(0, effective - knee)`, on the shared resolver and on melee, after armor piercing. A target at or below the knee takes full crits.

- [ ] **Step 1: Write the failing resolver tests.**

Create `internal/combat/crit_overflow_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
)

// #465: a crit skips mitigation up to the curve's knee and loses the part of
// effective mitigation above it (MitigationCurve.CritReduction). These tests
// pin that on the shared spell, taunt, throw and skill-move resolver;
// crit_overflow_melee_test.go pins the melee path.

// pinShippedCurve pins the shipped curve on all three channels, restored on
// cleanup, so these tests do not depend on what an earlier test left behind.
func pinShippedCurve(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	b := &cfg.Balance
	b.PhysicalMitigationKnee, b.PhysicalMitigationCap = 0.50, 0.90
	b.PhysicalMitigationAim, b.PhysicalMitigationBend = 0.05, 0.40
	b.MagicalMitigationKnee, b.MagicalMitigationCap = 0.50, 0.90
	b.MagicalMitigationAim, b.MagicalMitigationBend = 0.05, 0.40
	b.ConvictionMitigationKnee, b.ConvictionMitigationCap = 0.50, 0.90
	b.ConvictionMitigationAim, b.ConvictionMitigationBend = 0.05, 0.40
	cfg.GamePlay.UseSkillProgression = false
	configs.SetConfigForTest(t, cfg)
}

// critMeanOver is the mean crit damage of the shared resolver at one raw
// mitigation.
func critMeanOver(samples int, rawDmg float64, rank int, mitig float64) float64 {
	total := 0.0
	for i := 0; i < samples; i++ {
		total += float64(CritOrMitigatedDamage(rawDmg, rank, true, mitig, shippedCurveForTest))
	}
	return total / float64(samples)
}

// Below the knee a crit is unchanged: it lands on the same mean as a crit
// against no mitigation at all.
func TestCritOrMitigatedDamage_BelowTheKneeIsUnchanged(t *testing.T) {
	const samples, rawDmg, rank = 20000, 60.0, 40
	bare := critMeanOver(samples, rawDmg, rank, 0.0)
	light := critMeanOver(samples, rawDmg, rank, 0.30)
	assert.InDelta(t, 1.0, light/bare, 0.03, "a crit against 30%% raw mitigation must equal a crit against none")
}

// Above the knee a crit loses the overflow: raw 100% bends to 75% effective,
// 25 points over the knee, so the crit keeps three quarters.
func TestCritOrMitigatedDamage_AboveTheKneeLosesTheOverflow(t *testing.T) {
	const samples, rawDmg, rank = 20000, 60.0, 40
	bare := critMeanOver(samples, rawDmg, rank, 0.0)
	heavy := critMeanOver(samples, rawDmg, rank, 1.00)
	assert.InDelta(t, 0.75, heavy/bare, 0.03, "a crit against 100%% raw mitigation keeps 1 - (0.75 - 0.50)")

	// At the cap the crit keeps 1 - (0.90 - 0.50) = 0.60, and is still worth
	// several normal hits: a normal hit there keeps 0.10.
	capped := critMeanOver(samples, rawDmg, rank, 10.0)
	assert.InDelta(t, 0.60, capped/bare, 0.03)
}
```

Replace in `internal/combat/crit_damage_test.go` (1 of 2):

```go
	// The two halves of a crit in the spell and conviction channels: it ignores
	// the defender's mitigation entirely AND multiplies by skill. Holding
	// mitigation at a punishing 0.75 proves the bypass; the ratio proves the
	// scaling.
```

with:

```go
	// The two halves of a crit in the spell and conviction channels: it ignores
	// the defender's mitigation up to the curve's knee AND multiplies by skill.
	// Holding mitigation at the knee (0.50, the most a crit still ignores in
	// full since #465) proves the bypass; the ratio proves the scaling.
```

Replace in `internal/combat/crit_damage_test.go` (2 of 2):

```go
		critTotal += float64(CritOrMitigatedDamage(rawDmg, rank, true, 0.75, shippedCurveForTest))
		normalTotal += float64(CritOrMitigatedDamage(rawDmg, rank, false, 0.0, shippedCurveForTest))
	}

	// Normal + zero mitigation lands on rawDmg, so the ratio isolates the
	// multiplier even though the crit sample faced 75% mitigation.
```

with:

```go
		critTotal += float64(CritOrMitigatedDamage(rawDmg, rank, true, 0.50, shippedCurveForTest))
		normalTotal += float64(CritOrMitigatedDamage(rawDmg, rank, false, 0.0, shippedCurveForTest))
	}

	// Normal + zero mitigation lands on rawDmg, so the ratio isolates the
	// multiplier even though the crit sample faced 50% mitigation.
```

- [ ] **Step 2: Run them to see them fail.**

Run: `go test ./internal/combat/ -run "CritOrMitigatedDamage" -count=1`
Expected: FAIL, only `TestCritOrMitigatedDamage_AboveTheKneeLosesTheOverflow`, at `crit_overflow_test.go:56` and `:61`: `Max difference between 0.75 and ...` with the ratio near 1.0 (and the same against 0.60). The crit still ignores all mitigation. `..._BelowTheKneeIsUnchanged` and `..._CritBypassesMitigationAndScales` pass (pins).

- [ ] **Step 3: The shared resolver takes the overflow.**

Replace in `internal/combat/crit_damage.go` (1 of 3):

```go
// `CritDamageMultiplier(skill) / (1 - mitigation)`. The bypass is what lets a
// crit answer heavy armour; the multiplier is what makes skill investment show
// up in the payoff.
```

with:

```go
// `CritDamageMultiplier(skill) / (1 - mitigation)`. The bypass is what lets a
// crit answer heavy armour; the multiplier is what makes skill investment show
// up in the payoff.
//
// #465 narrowed the bypass: a crit now skips mitigation up to the curve's
// knee and is reduced by the part of effective mitigation above it
// (MitigationCurve.CritReduction), so very heavy protection softens crits
// too. A target at or below the knee takes full crits, as before. At 85%
// effective, a 2.0x crit lands near 1.3x raw against a 0.15x normal hit:
// still about nine normal hits.
```

Replace in `internal/combat/crit_damage.go` (2 of 3):

```go
// On a crit it bypasses mitigation entirely and scales by
// CritDamageMultiplier(skillRank) * bonusCritMult; on a normal hit it applies
// mitigation and bonusCritMult plays no part at all. Either way the
```

with:

```go
// On a crit it skips mitigation up to the curve's knee and scales by
// CritDamageMultiplier(skillRank) * bonusCritMult * (1 - the curve's
// CritReduction), the overflow above the knee (#465); on a normal hit it
// applies mitigation and bonusCritMult plays no part at all. Either way the
```

Replace in `internal/combat/crit_damage.go` (3 of 3):

```go
		mean *= CritDamageMultiplier(skillRank) * bonusCritMult
	} else {
```

with:

```go
		mean *= CritDamageMultiplier(skillRank) * bonusCritMult * (1 - curve.CritReduction(mitigPct))
	} else {
```

Run: `go test ./internal/combat/ -run "CritOrMitigatedDamage" -count=1`
Expected: PASS.

- [ ] **Step 4: Write the failing melee tests.**

Create `internal/combat/crit_overflow_melee_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #465 on the melee path: buildDamageParams carries the same crit reduction
// the shared resolver applies (crit_overflow_test.go), and a melee crit keeps
// the same share of its unmitigated mean as a spell-path crit.

const critOverflowConditionId = 9465

// critOverflowDefender is a parity combatant carrying rawPoints of physical
// mitigation as a condition statmod (melee_parity_stat_test.go's injection).
func critOverflowDefender(t *testing.T, rawPoints int) *characters.Character {
	t.Helper()
	d := parityCombatant(t, "overflow defender")
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		critOverflowConditionId: {
			ConditionId:  critOverflowConditionId,
			Name:         "overflow mitigation",
			TriggerCount: 1,
			StatMods:     statmods.StatMods{"physical_mitigation": rawPoints},
		},
	}))
	d.Conditions.List = append(d.Conditions.List, &conditions.Condition{
		ConditionId: critOverflowConditionId, TriggersLeft: 1000000000,
	})
	d.Conditions.Validate(true)
	require.InDelta(t, float64(rawPoints)/100.0, d.GetPhysicalMitigation(), 1e-9)
	return d
}

// The melee path carries the same reduction the shared resolver applies:
// none below the knee, the overflow above it.
func TestBuildDamageParams_CritReductionIsTheOverflow(t *testing.T) {
	pinShippedCurve(t)
	attacker := parityCombatant(t, "overflow attacker")

	for _, tc := range []struct {
		points int
		want   float64
	}{
		{0, 0},
		{40, 0},
		{50, 0},
		{100, 0.25},
	} {
		defender := critOverflowDefender(t, tc.points)
		plan := buildAttackPlan(attacker, defender)
		require.NotEmpty(t, plan.weapons)
		sdp := buildDamageParams(attacker, defender, plan.weapons[0], 0, User)
		assert.InDelta(t, tc.want, sdp.critReduction, 1e-9, "raw %d points", tc.points)
		assert.InDelta(t, MitigationCurveFor(ChannelPhysical).CritReduction(float64(tc.points)/100.0),
			sdp.critReduction, 1e-12, "melee and the curve agree at raw %d points", tc.points)
	}
}

// Melee and the shared resolver agree on the crit itself: against the same
// raw mitigation, a melee crit and a spell-path crit keep the same share of
// their unmitigated crit mean.
func TestMeleeCritAndSpellCritAgreeAboveTheKnee(t *testing.T) {
	pinShippedCurve(t)
	const samples = 20000
	attacker := parityCombatant(t, "overflow attacker")
	defender := critOverflowDefender(t, 100)

	plan := buildAttackPlan(attacker, defender)
	require.NotEmpty(t, plan.weapons)
	sdp := buildDamageParams(attacker, defender, plan.weapons[0], 0, User)
	fullCrit := sdp.rawDmgForCrit * sdp.critDmgMult

	meleeTotal := 0.0
	for i := 0; i < samples; i++ {
		dmg, _ := calcHitDamage(&AttackResult{}, true, false, sdp)
		meleeTotal += float64(dmg)
	}
	meleeShare := meleeTotal / samples / fullCrit

	rank := attacker.GetCombatSkillLevel()
	spellBare := 0.0
	spellHeavy := 0.0
	curve := MitigationCurveFor(ChannelPhysical)
	for i := 0; i < samples; i++ {
		spellBare += float64(CritOrMitigatedDamage(sdp.rawDmgForCrit, rank, true, 0, curve))
		spellHeavy += float64(CritOrMitigatedDamage(sdp.rawDmgForCrit, rank, true, defender.GetPhysicalMitigation(), curve))
	}
	spellShare := spellHeavy / spellBare

	assert.InDelta(t, 0.75, meleeShare, 0.03, "melee crit keeps 1 - overflow")
	assert.InDelta(t, 0.75, spellShare, 0.03, "spell-path crit keeps 1 - overflow")
	assert.InDelta(t, meleeShare, spellShare, 0.03, "melee and spell agree")
}
```

Replace in `internal/combat/melee_parity_stat_test.go`:

```go
				t.Fatalf("critDmgMult = %.6f, want %.6f", sdp.critDmgMult, critDmgMult)
			}
			critMean := sdp.rawDmgForCrit * sdp.critDmgMult
```

with:

```go
				t.Fatalf("critDmgMult = %.6f, want %.6f", sdp.critDmgMult, critDmgMult)
			}
			// #465: the crit loses the mitigation above the knee (0 in the
			// light and mid cells, about 0.17 in the BIS cell).
			wantCritReduction := shippedCurveForTest.CritReduction(float64(cell.mitPct) / 100.0)
			if math.Abs(sdp.critReduction-wantCritReduction) > 1e-9 {
				t.Fatalf("critReduction = %.6f, want %.6f", sdp.critReduction, wantCritReduction)
			}
			critMean := sdp.rawDmgForCrit * sdp.critDmgMult * (1 - sdp.critReduction)
```

Run: `go vet ./internal/combat/`
Expected: FAIL, build failed: `sdp.critReduction undefined (type swingDamageParams has no field or method critReduction)`.

- [ ] **Step 5: Melee carries the reduction.**

Replace in `internal/combat/combat_helpers.go` (1 of 4):

```go
	critDmgMult    float64 // chunk 5.11g: skill-scaled crit worth, applied to rawDmgForCrit only
	critConditions []int
```

with:

```go
	critDmgMult    float64 // chunk 5.11g: skill-scaled crit worth, applied to rawDmgForCrit only
	critConditions []int

	// critReduction is the share a crit loses to the target's physical
	// mitigation above the curve's knee (#465, MitigationCurve.CritReduction).
	// 0 at or below the knee, and 0 in a literal built outside
	// buildDamageParams, which degrades to a full crit, never to no damage.
	critReduction float64
```

Replace in `internal/combat/combat_helpers.go` (2 of 4):

```go
	// Apply target's physical mitigation
	dmgMean := ApplyMitigation(rawDmg, targetChar.GetPhysicalMitigation(), MitigationCurveFor(ChannelPhysical))
```

with:

```go
	// Apply target's physical mitigation, bent through the curve (#465). A
	// crit skips it up to the knee and loses the overflow above it.
	physMit := targetChar.GetPhysicalMitigation()
	physCurve := MitigationCurveFor(ChannelPhysical)
	dmgMean := ApplyMitigation(rawDmg, physMit, physCurve)
```

Replace in `internal/combat/combat_helpers.go` (3 of 4):

```go
		critDmgMult:   CritDamageMultiplier(combatSkillLevel),
		openingStrikeMult: OpeningStrikeMultiplier(sourceChar,
```

with:

```go
		critDmgMult:   CritDamageMultiplier(combatSkillLevel),
		critReduction: physCurve.CritReduction(physMit),
		openingStrikeMult: OpeningStrikeMultiplier(sourceChar,
```

Replace in `internal/combat/combat_helpers.go` (4 of 4):

```go
		// same mean for the same reason.
		critMean := sdp.rawDmgForCrit * sdp.critDmgMult
```

with:

```go
		// same mean for the same reason. #465: mitigation above the curve's
		// knee softens the crit (critReduction), as on every other channel.
		critMean := sdp.rawDmgForCrit * sdp.critDmgMult * (1 - sdp.critReduction)
```

Run: `go test ./internal/combat/ -run "CritReduction|MeleeCritAndSpell|MeleeParity|CritOrMitigated" -count=1`
Expected: PASS (`TestMeleeParityDamagePerSwing` takes about 15 s).

- [ ] **Step 6: The throw test's numbers follow.**

The throw target carries 75% raw: a normal hit's mean moves from about 7.5 to about 10, a crit's from about 60 to about 50. Both assertions still hold; their text is made true.

Replace in `internal/usercommands/throw_seam_test.go` (1 of 3):

```go
// tier — a crit bypasses the target's 75% physical mitigation and scales by
// CritDamageMultiplier, where a normal hit is mitigated to a quarter.
```

with:

```go
// tier. The target carries 75% raw physical mitigation, which the #465 curve
// bends to about 67%: a normal hit keeps about a third (mean ~10), and a crit
// skips the mitigation up to the knee, loses the overflow above it (about a
// sixth) and scales by CritDamageMultiplier (mean ~50).
```

Replace in `internal/usercommands/throw_seam_test.go` (2 of 3):

```go
"a non-crit hit must respect 75%% mitigation (mean ~7.5)")
```

with:

```go
"a non-crit hit must respect the target's mitigation (mean ~10)")
```

Replace in `internal/usercommands/throw_seam_test.go` (3 of 3):

```go
			"a crit must bypass mitigation and scale by CritDamageMultiplier (mean ~60)")
```

with:

```go
			"a crit must skip mitigation to the knee and scale by CritDamageMultiplier (mean ~50)")
```

- [ ] **Step 7: Run the packages.**

Run: `gofmt -l internal/ && go build ./... && go vet ./...`
Expected: no output.

Run: `go test ./internal/combat/ ./internal/hooks/ ./internal/actions/ ./internal/usercommands/ -count=1`
Expected: PASS, `ok` for all four.

Run: `go test . -count=1`
Expected: `ok`.

- [ ] **Step 8: Commit.**

```bash
git add internal/combat/crit_overflow_test.go internal/combat/crit_overflow_melee_test.go \
  internal/combat/crit_damage_test.go internal/combat/melee_parity_stat_test.go \
  internal/usercommands/throw_seam_test.go internal/combat/crit_damage.go \
  internal/combat/combat_helpers.go
git commit -F - <<'EOF'
feat(combat): heavy protection softens crits by the overflow above the knee

A crit skips mitigation up to the curve's knee and loses
max(0, effective - knee): CritOrMitigatedDamageScaled for spells, taunt,
counter, throw and skill moves, swingDamageParams.critReduction for melee.
A target at or below the knee takes full crits, as before.

Refs #465

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 5: `PowerScore` and `status` read effective mitigation

`EffectiveMitigationFor(char, channel)` is the value combat applies. `PowerScore` averages it; the status sheet shows it through a new `effectiveMitigation` template function.

- [ ] **Step 1: Write the failing tests.**

Create `internal/combat/effective_mitigation_readers_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
)

// #465: the readers that used to see raw mitigation with no cap at all now
// read what combat applies.

// armoredChar carries raw mitigation as worn points on one body item.
func armoredChar(physical, magical, conviction int) characters.Character {
	c := *characters.New()
	c.Stats.Strength.ValueAdj = 100
	c.Stats.Dexterity.ValueAdj = 100
	c.Stats.Willpower.ValueAdj = 100
	c.Stats.Charisma.ValueAdj = 100
	c.HealthMax.Value = 100
	if physical+magical+conviction > 0 {
		c.Equipment.Body = items.Item{ItemId: 1, Spec: &items.ItemSpec{
			PhysicalMitigation:   physical,
			MagicalMitigation:    magical,
			ConvictionMitigation: conviction,
		}}
	}
	return c
}

// EffectiveMitigationFor is each channel's raw sum bent through that
// channel's own curve.
func TestEffectiveMitigationFor_BendsEachChannel(t *testing.T) {
	pinShippedCurve(t)
	c := armoredChar(40, 100, 370)
	assert.InDelta(t, 0.40, EffectiveMitigationFor(&c, ChannelPhysical), 1e-9, "below the knee: raw")
	assert.InDelta(t, 0.75, EffectiveMitigationFor(&c, ChannelMagical), 1e-9, "raw 1.00 bends to 0.75")
	assert.InDelta(t, 0.90, EffectiveMitigationFor(&c, ChannelConviction), 1e-9, "raw 3.70 reaches the cap")
}

// PowerScore weighs the average effective mitigation (x300): 370 raw points
// on one channel add 300 x 0.90 / 3 = 90, not the 370 the raw sum used to add.
func TestPowerScore_ReadsEffectiveMitigation(t *testing.T) {
	pinShippedCurve(t)
	bare := armoredChar(0, 0, 0)
	heavy := armoredChar(370, 0, 0)
	assert.InDelta(t, 90.0, PowerScore(heavy)-PowerScore(bare), 0.01)

	// Below the knee nothing changes: 40 points add 300 x 0.40 / 3 = 40.
	light := armoredChar(40, 0, 0)
	assert.InDelta(t, 40.0, PowerScore(light)-PowerScore(bare), 0.01)
}
```

Create `internal/templates/effective_mitigation_test.go`:

```go
package templates

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderMitigationBand runs the real template engine over the status sheet's
// Defenses expression, so a typo in the registered name, an arity mismatch or
// a bad argument type surfaces here rather than on a player's `status`.
func renderMitigationBand(t *testing.T, c *characters.Character, channel string) (string, error) {
	t.Helper()
	out, err := ProcessText(
		`{{ mitigationQuality (effectiveMitigation .C "`+channel+`") }}`,
		map[string]any{"C": c},
	)
	return strings.TrimRight(out, "\r\n"), err
}

// #465: the status sheet describes the protection combat applies, not the
// raw sum. 75 raw points used to read "fortified" (the old 75% cap); the
// curve bends them to about 67%, which reads "heavy". 370 points reach the
// 90% cap and read "fortified". Below the knee nothing changes.
func TestEffectiveMitigation_StatusBandsReadTheCurve(t *testing.T) {
	c := characters.New()
	c.Equipment.Body = items.Item{ItemId: 1, Spec: &items.ItemSpec{
		PhysicalMitigation:   75,
		MagicalMitigation:    30,
		ConvictionMitigation: 370,
	}}

	got, err := renderMitigationBand(t, c, "physical")
	require.NoError(t, err)
	assert.Equal(t, "heavy", got, "75 raw points bend to about 67%%")

	got, err = renderMitigationBand(t, c, "magical")
	require.NoError(t, err)
	assert.Equal(t, "moderate", got, "30 raw points are below the knee and read as before")

	got, err = renderMitigationBand(t, c, "conviction")
	require.NoError(t, err)
	assert.Equal(t, "fortified", got, "370 raw points reach the cap")
}

// A channel name the function does not know fails the render rather than
// silently describing some other channel.
func TestEffectiveMitigation_UnknownChannelFailsTheRender(t *testing.T) {
	_, err := renderMitigationBand(t, characters.New(), "physcial")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "physcial")
}
```

Replace in `internal/usercommands/template_freeze_test.go`:

```go
func TestTemplateFreeze_IdentifyReadsConditionIds(t *testing.T) {
```

with:

```go
// #465: the status sheet's Defenses row describes effective mitigation. 75
// raw physical points read "fortified" under the old raw reading and "heavy"
// on the curve (about 67%).
func TestTemplateFreeze_StatusDefensesReadTheCurve(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)

	user, _ := getTestUserAndRoom(t)
	originalBody := user.Character.Equipment.Body
	t.Cleanup(func() { user.Character.Equipment.Body = originalBody })
	user.Character.Equipment.Body = items.Item{ItemId: 99465, Spec: &items.ItemSpec{
		ItemId: 99465, Name: "probe plate", PhysicalMitigation: 75,
	}}

	out, err := templates.Process("character/status", user, user.UserId)
	require.NoError(t, err)
	assert.Contains(t, out, "heavy", "75 raw points bend to about 67%%")
	assert.NotContains(t, out, "fortified", "the raw sum must not reach the status sheet")
}

func TestTemplateFreeze_IdentifyReadsConditionIds(t *testing.T) {
```

- [ ] **Step 2: Run them to see them fail.**

Run: `go test ./internal/combat/ -run "EffectiveMitigationFor|PowerScore_ReadsEffective" -count=1`
Expected: FAIL, build failed: `undefined: EffectiveMitigationFor`.

Run: `go test ./internal/templates/ -run "EffectiveMitigation" -count=1`
Expected: FAIL: `function "effectiveMitigation" not defined` in both tests.

Run: `go test ./internal/usercommands/ -run "TemplateFreeze_StatusDefenses" -count=1`
Expected: FAIL: the rendered sheet's Defenses row reads `Physical:   </ansi>fortified`, so `does not contain "heavy"` and `should not contain "fortified"`.

- [ ] **Step 3: The reader.**

Replace in `internal/combat/damage_pipeline.go`:

```go
// MitigationCurveFor returns the configured mitigation curve for a damage
```

with:

```go
// EffectiveMitigationFor is the fraction of a channel's damage char actually
// stops: its raw sum (Character.Get*Mitigation) bent through the channel's
// curve. The reader for anything that describes or weighs a character's
// protection (PowerScore, the status sheet), so it sees what combat applies.
// An unknown channel reads physical, as MitigationCurveFor does.
func EffectiveMitigationFor(char *characters.Character, channel DamageChannel) float64 {
	switch channel {
	case ChannelMagical:
		return EffectiveMitigation(char.GetMagicalMitigation(), MitigationCurveFor(channel))
	case ChannelConviction:
		return EffectiveMitigation(char.GetConvictionMitigation(), MitigationCurveFor(channel))
	default:
		return EffectiveMitigation(char.GetPhysicalMitigation(), MitigationCurveFor(ChannelPhysical))
	}
}

// MitigationCurveFor returns the configured mitigation curve for a damage
```

Run: `go test ./internal/combat/ -run "EffectiveMitigationFor|PowerScore_ReadsEffective" -count=1`
Expected: FAIL, only `TestPowerScore_ReadsEffectiveMitigation` at `effective_mitigation_readers_test.go:48`: `Max difference between 90 and 370 allowed is 0.01, but difference was -280` (PowerScore still weighs the raw sum). `TestEffectiveMitigationFor_BendsEachChannel` passes.

- [ ] **Step 4: `PowerScore` reads it.**

Replace in `internal/combat/calculations.go`:

```go
	avgMit := (char.GetPhysicalMitigation() + char.GetMagicalMitigation() + char.GetConvictionMitigation()) / 3.0
```

with:

```go
	// Effective, not raw (#465): consider, mob flee and target choice and the
	// power leaderboard weigh the protection combat actually applies.
	avgMit := (EffectiveMitigationFor(&char, ChannelPhysical) +
		EffectiveMitigationFor(&char, ChannelMagical) +
		EffectiveMitigationFor(&char, ChannelConviction)) / 3.0
```

Run: `go test ./internal/combat/ -run "EffectiveMitigationFor|PowerScore" -count=1`
Expected: PASS (the existing `TestPowerScore_*` too).

- [ ] **Step 5: The template function.**

Replace in `internal/templates/templatesfunctions.go`:

```go
		"mitigationQuality": func(pct float64) string {
```

with:

```go
		// effectiveMitigation is a character's mitigation on one channel
		// ("physical", "magical" or "conviction") bent through that channel's
		// curve (#465), the value combat applies. The status sheet passes it
		// to mitigationQuality. An unknown name fails the render.
		"effectiveMitigation": func(c *characters.Character, channel string) (float64, error) {
			switch channel {
			case "physical":
				return combat.EffectiveMitigationFor(c, combat.ChannelPhysical), nil
			case "magical":
				return combat.EffectiveMitigationFor(c, combat.ChannelMagical), nil
			case "conviction":
				return combat.EffectiveMitigationFor(c, combat.ChannelConviction), nil
			}
			return 0, fmt.Errorf("effectiveMitigation: unknown channel %q", channel)
		},
		"mitigationQuality": func(pct float64) string {
```

Run: `go test ./internal/templates/ -run "EffectiveMitigation" -count=1`
Expected: PASS.

- [ ] **Step 6: The status sheet uses it.**

Replace in `_datafiles/world/dogmud/templates/character/status.template`:

```
{{ padRight 13 (mitigationQuality (.Character.GetPhysicalMitigation)) }}<ansi fg="yellow">Magical:    </ansi>{{ padRight 13 (mitigationQuality (.Character.GetMagicalMitigation)) }}<ansi fg="yellow">Conviction: </ansi>{{ padRight 13 (mitigationQuality (.Character.GetConvictionMitigation)) }}
```

with:

```
{{ padRight 13 (mitigationQuality (effectiveMitigation .Character "physical")) }}<ansi fg="yellow">Magical:    </ansi>{{ padRight 13 (mitigationQuality (effectiveMitigation .Character "magical")) }}<ansi fg="yellow">Conviction: </ansi>{{ padRight 13 (mitigationQuality (effectiveMitigation .Character "conviction")) }}
```

Run: `go test ./internal/usercommands/ -run "TemplateFreeze" -count=1`
Expected: PASS, including `TestTemplateFreeze_StatusReadsTheBrokenLimbRecord` (the whole sheet still renders).

- [ ] **Step 7: Run the packages.**

Run: `gofmt -l internal/ && go build ./... && go vet ./...`
Expected: no output.

Run: `go test ./internal/combat/ ./internal/templates/ ./internal/usercommands/ ./internal/actions/ ./internal/behaviortree/ -count=1`
Expected: PASS (`consider` and the mob AI read `PowerScore`; `modules/leaderboards`, the third reader, has no test files and is covered by `go build`).

Run: `go test . -count=1`
Expected: `ok`.

- [ ] **Step 8: Commit.**

```bash
git add internal/combat/effective_mitigation_readers_test.go \
  internal/templates/effective_mitigation_test.go \
  internal/usercommands/template_freeze_test.go \
  internal/combat/damage_pipeline.go internal/combat/calculations.go \
  internal/templates/templatesfunctions.go \
  _datafiles/world/dogmud/templates/character/status.template
git commit -F - <<'EOF'
feat(combat): PowerScore and status read effective mitigation

EffectiveMitigationFor(char, channel) bends a character's raw sum through
the channel's curve. PowerScore averages it, so consider, mob flee and
target choice and the power leaderboard weigh what combat applies; the
status sheet shows it through the effectiveMitigation template function.

Refs #465

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 6: Docs, help and patch notes

- [ ] **Step 1: `internal/combat/context.md`.**

Replace in `internal/combat/context.md` (1 of 8):

```markdown
   - Z-score >= 2.0 = crit (double damage, bypasses mitigation).
```

with:

```markdown
   - Z-score >= 2.0 = crit (skill-scaled damage; skips mitigation up to the
     curve's knee and loses the overflow above it, #465).
```

Replace in `internal/combat/context.md` (2 of 8):

```markdown
dmgMean = ApplyMitigation(rawDmg, mob.GetPhysicalMitigation(), 0.75)
  e.g., mob has 20% phys mitigation: dmgMean = 81.8 * 0.80 = 65.4
```

with:

```markdown
dmgMean = ApplyMitigation(rawDmg, mob.GetPhysicalMitigation(),
                          MitigationCurveFor(ChannelPhysical))
  e.g., mob has 20% phys mitigation (below the knee, so it counts in full):
  dmgMean = 81.8 * 0.80 = 65.4
critReduction = MitigationCurveFor(ChannelPhysical).CritReduction(mitigation)
  (0 at or below the knee; see "Mitigation curve (#465)")
```

Replace in `internal/combat/context.md` (3 of 8):

```markdown
  CRIT: mean = rawDmgForCrit * critDmgMult                   // PRE-mitigation!
```

with:

```markdown
  CRIT: mean = rawDmgForCrit * critDmgMult * (1 - critReduction)
                                         // PRE-mitigation, less the overflow
                                         // above the curve's knee (#465)
```

Replace in `internal/combat/context.md` (4 of 8):

```markdown
`ResourceMultiplier`, `MitigationCap`, `DamageScale`; `GetConvictionDamageDescription`
```

with:

```markdown
`ResourceMultiplier`, `MitigationCurveFor`, `EffectiveMitigationFor`, `DamageScale`; `GetConvictionDamageDescription`
```

Replace in `internal/combat/context.md` (5 of 8):

```markdown
crit_damage.go` | `CritDamageMultiplier`, `CritOrMitigatedDamage` |
```

with:

```markdown
crit_damage.go` | `CritDamageMultiplier`, `CritOrMitigatedDamage`, `CritOrMitigatedDamageScaled` (crit reduced by the curve's overflow, #465) |
| `combat/mitigation_curve.go` | `MitigationCurve`, `EffectiveMitigation`, `MitigationCurve.CritReduction` (#465) |
```

Replace in `internal/combat/context.md` (6 of 8):

```markdown
| `damage_pipeline.go` | The unified three-channel damage + mitigation pipeline |
```

with:

```markdown
| `damage_pipeline.go` | The unified three-channel damage + mitigation pipeline |
| `mitigation_curve.go` | The mitigation curve (#465): `MitigationCurve`, `EffectiveMitigation`, `CritReduction`. See "Mitigation curve (#465)" |
```

Replace in `internal/combat/context.md` (7 of 8):

```markdown
summed by `char.Get*Mitigation()` |
```

with:

```markdown
summed by `char.Get*Mitigation()`, bent through each channel's curve by `EffectiveMitigationFor` (#465) |
```

Replace in `internal/combat/context.md` (8 of 8):

````markdown
## Position hit modifiers (chunk 4e)
````

with:

````markdown
## Mitigation curve (#465)

Every channel sums its raw mitigation from all sources
(`Character.GetPhysicalMitigation` / `GetMagicalMitigation` /
`GetConvictionMitigation`, uncapped) and bends it through that channel's
`MitigationCurve` (`mitigation_curve.go`), read by
`MitigationCurveFor(channel)` from four knobs per channel
(`PhysicalMitigationKnee`, `...Cap`, `...Aim`, `...Bend`, and the same for
`Magical` and `Conviction`; all ship at 0.50 / 0.90 / 0.05 / 0.40):

```
EffectiveMitigation(raw, c)
  raw <= 0 or NaN -> 0
  raw <= knee     -> raw
  otherwise       -> x = raw - knee
                     min(knee + (cap + aim - knee) * x / (x + bend), cap)
```

At the shipped numbers raw 0.75 stops 67%, 1.00 75%, 1.81 84%, 3.70 90%.
It replaced a flat clamp at 0.75 (`MitigationCap(channel)`, deleted).

- **Normal hits:** `ApplyMitigation(raw, pct, curve)` is
  `raw * (1 - EffectiveMitigation(pct, curve))`. Every caller passes
  `MitigationCurveFor` of its channel: melee (`buildDamageParams`), pets,
  `ExecuteSkillMove`, spells (`calcSpellDamageForCharacter`), taunt, counter,
  riposte, throw, and `ReflectDamage`.
- **Armor piercing** (`SkillMoveParams.MitigationMultiplier`, stomp 0.5)
  scales RAW mitigation before the curve.
- **Crits:** a crit skips mitigation up to the knee and loses the overflow
  above it, `curve.CritReduction(pct) = max(0, EffectiveMitigation(pct) -
  knee)`. `CritOrMitigatedDamageScaled` multiplies the crit mean by
  `1 - CritReduction`; melee carries it as `swingDamageParams.critReduction`
  (0 in a literal built elsewhere: a full crit, never no damage). The defence
  multiplier stays skipped on crits.
- **Readers:** `EffectiveMitigationFor(char, channel)` is the value combat
  applies; `PowerScore` averages it over the three channels and the status
  sheet shows it (`effectiveMitigation` template function, `internal/templates`).
  `identify` still describes one item's own points, raw.

## Position hit modifiers (chunk 4e)
````

- [ ] **Step 2: `internal/configs/context.md` and `internal/templates/context.md`.**

Replace in `internal/configs/context.md` (1 of 2):

```markdown
| `config.balance.combat.go` | Combat maths, mitigation caps, defence floor |
```

with:

```markdown
| `config.balance.combat.go` | Combat maths, mitigation curves (`validateMitigationCurve`, #465), defence floor |
```

Replace in `internal/configs/context.md` (2 of 2):

```markdown
### Banned Name Validation
```

with:

```markdown
### Mitigation curve knobs (#465)
Twelve `Balance` knobs, four per channel, read by `combat.MitigationCurveFor`:
`PhysicalMitigationKnee`, `PhysicalMitigationCap`, `PhysicalMitigationAim`,
`PhysicalMitigationBend`, and the same for `Magical` and `Conviction`. The
`...Cap` keys kept their names; they are the curve's hard ceiling now, not a
clamp. `validateMitigationCurve` (`config.balance.combat.go`) enforces
`0 < knee < cap <= 0.95`, `aim > 0`, `bend > 0`, written `!(x > 0)` so a NaN
falls back too; a bad or absent value takes the default 0.50 / 0.90 / 0.05 /
0.40, which is also what `config.yaml` ships, so a test binary runs the
shipped curve. An absent key reads 0, so aim 0 is not expressible (0.0001 is
the approach-only curve in practice). `config_mitigation_curve_test.go` pins
the defaults, every fallback and the shipped file.

### Banned Name Validation
```

Replace in `internal/templates/context.md`:

```markdown
### Markdown Integration
- **processMarkdown(in string) string**: Processes markdown content
```

with:

```markdown
### Mitigation on the status sheet (#465)
- **effectiveMitigation(char, channel)** (`templatesfunctions.go`): a
  character's mitigation on `"physical"`, `"magical"` or `"conviction"`, bent
  through that channel's curve by `combat.EffectiveMitigationFor`, the value
  combat applies. An unknown channel name fails the render. The DOGMud
  `character/status` template passes it to **mitigationQuality**, which maps a
  fraction to a word band ("none" to "fortified", "fortified" from 70%).
  `descriptions/identify` still passes one item's own points to
  `mitigationQuality`, raw.

### Markdown Integration
- **processMarkdown(in string) string**: Processes markdown content
```

- [ ] **Step 3: `help armor` and the patch notes (player copy: no numbers, no dashes, 80 columns).**

Replace in `_datafiles/world/dogmud/templates/help/armor.template`:

```
<ansi fg="command">Armor</ansi> provides damage mitigation across three channels.
Each channel has its own mitigation percentage, and each is
capped so that no amount of armor makes you untouchable.
```

with:

```
<ansi fg="command">Armor</ansi> provides damage mitigation across three channels.
Each channel has its own mitigation. The first layers of
protection count in full. Past that, each extra layer helps
less, and no amount of armor makes you untouchable. Very heavy
protection also softens critical hits.
```

Replace in `docs/PATCH_NOTES.md`:

```markdown
  `killstats` and to the achievements that count kills. New kills are
  counted from now on; kills from before the fix cannot be recovered.
```

with:

```markdown
  `killstats` and to the achievements that count kills. New kills are
  counted from now on; kills from before the fix cannot be recovered.
- Armor keeps paying off past the point where it used to stop. The first
  layers of protection count in full. Past that, each extra layer helps
  less, but it always helps, and nothing makes you completely immune.
- Very heavy protection now also softens critical hits. Against lighter
  protection a critical hit lands in full, as before.
- The Defenses part of `status` now describes the protection you really
  have, so a heavily armored character may read one step lower than
  before. `consider`, and how creatures size you up, use the same measure.
```

(The anchor is the last bullet of the current "Unreleased" section on master `9798f6d02`. If that section has been released by the time this runs, put the three bullets under the new top "Unreleased" heading instead, creating one if needed.)

- [ ] **Step 4: Check the docs.**

Run: `python tools/context_md_audit.py`
Expected: the same list as on master (14 packages with phantom symbols, none of them `internal/combat` or `internal/templates`; `internal/configs` lists only its two pre-existing `server_Config` and `Get`).

Run (on its own line; zero matches expected): `git diff HEAD -- docs/PATCH_NOTES.md _datafiles/world/dogmud/templates/help/armor.template | grep '^+' | grep -nP '[\x{2013}\x{2014}]|[0-9]+%'`
Expected: no output (no dash, no percentage in player copy).

Run: `grep -n "2026-10-10-mitigation-curve" docs/README.md`
Expected: the spec's row (on master since #473) and this plan's row (it arrives with the plan's own commit). No new package or root guard, so nothing to add.

Run: `go test ./internal/usercommands/ ./internal/templates/ -count=1 && go test . -count=1`
Expected: `ok` (the help guards and the root copy guards, `copy_no_dash_test.go` among them).

- [ ] **Step 5: Commit.**

```bash
git add internal/combat/context.md internal/configs/context.md internal/templates/context.md \
  _datafiles/world/dogmud/templates/help/armor.template docs/PATCH_NOTES.md
git commit -F - <<'EOF'
docs: the mitigation curve in context.md, help armor and patch notes

Refs #465

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 7: Whole-tree verification and boot check

- [ ] **Step 1: Format.**

Run: `gofmt -l ./internal ./modules *.go`
Expected: no output.

- [ ] **Step 2: Build and vet.**

Run: `go build ./... && go vet ./...`
Expected: no output.

- [ ] **Step 3: Full suite.**

Run: `go test ./... -count=1` (about a minute and a half)
Expected: every package `ok`. A failure is a finding: fix it in the task it belongs to.

- [ ] **Step 4: Lint, new issues only.**

Run: `golangci-lint run --new-from-merge-base=origin/master ./...`
Expected: `0 issues.`

- [ ] **Step 5: No dashes in what the branch added.**

Run (on its own line; zero matches expected): `git diff $(git merge-base origin/master HEAD) -- '*.go' '*.yaml' '*.template' '*.md' | grep '^+' | grep -nP '[\x{2013}\x{2014}]'`

Diff against the merge base, not `master`: master moves while the branch is open, and a plain diff would report its new lines as yours.
Expected: no output.

- [ ] **Step 6: The config bit and the config diff.**

Run: `git diff $(git merge-base origin/master HEAD) --stat -- _datafiles/config.yaml`
Expected: `_datafiles/config.yaml | 25 ++++++++++++++++++++-----` (Task 2's block and nothing else).

Run: `git diff $(git merge-base origin/master HEAD) -- _datafiles/config.yaml | grep '^[-+] ' | grep -v Mitigation`
Expected: only comment lines (`#`); every changed key is a mitigation knob.

- [ ] **Step 7: Boot check** (`dogmud-shipping`, step 6 of the pre-push SOP), on ports clear of the owner's server. Write a scratch override file (session scratchpad, not `C:/tmp`) holding

```yaml
Network:
  TelnetPort: [43333]
  LocalPort: 19999
  HttpPort: 18090
  AIPort: 45555
```

then, in a detached worktree of the branch head (`git worktree add --detach C:/tmp/dogmud-465-boot HEAD`): `go build -o boot-check.exe .` and `CONFIG_PATH=<that file> timeout 180 ./boot-check.exe > boot.log 2>&1` (write `boot.log` to the scratchpad too).
Expected: exit 124 (the server stayed up); `grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log` prints 0; `grep -c "Server Ready" boot.log` prints 1; the "Starting http server" and Telnet "listening" lines name ports 18090, 43333, 19999 and 45555 and no other (it bound the scratch ports, not the owner's). Kill nothing by name: the timeout ends the process. Remove the worktree afterwards (`git worktree remove --force C:/tmp/dogmud-465-boot`; if Windows holds a lock, PowerShell `Remove-Item -Recurse -Force`, then `git worktree prune`).

---

### Task 8: Playtest (executed by the controller, not a subagent)

Combat feel changed for every armoured character, so the change closes with the playtest the spec asks for. Run it against a local build of the branch with the harness, following `dogmud-playtesting` (wipe instance saves with the server down; kill only your own server, by PID). Harness lessons from recent runs are in memory (`project-sight-wrapup-playtest-harness-lessons-2026-10-09`). Goals, not mechanics:

- **An invested tank.** Gear a character to roughly the spec's heavily invested build (raw physical well past 100%) and fight The Sentinel (9552, eastern highlands) and the North Road bandits (283 lookout, 284 fighter, 285 caster). The tank takes noticeably less than before from ordinary blows, still takes real damage, and is never untouchable. `status` Defenses reads "fortified" only well past the knee.
- **A crit-heavy build against a high-mitigation mob.** A high-skill melee attacker and a high-Spellcasting caster against The Sentinel (raw 75% physical, 97% magical, 93% conviction): crits still land as big hits, visibly smaller than before against the magical channel, and still worth several normal hits.
- **The crash site interior** (Repair Frame 9585, Grapnel Warden 9586, Hull Sweeper 9587; about 35% physical, 25% magical, below the knee): their own mitigation is unchanged by the curve; the owner expects the area may feel harder for players whose gear stacked past the old clamp anyway. Report how it feels.
- **`consider`** on The Sentinel with the tank: the odds move toward the mob compared with master (D8), and read sensibly.
- **`help armor`** reads true and fits 80 columns.

File every finding as a GitHub issue on `pruuk/DOGMud` (`--repo pruuk/DOGMud`). A retune is a `config.yaml` edit (the twelve knobs), not a code change. The PR body says "Refs #465" and lists what it resolves in words, without a closing keyword.

---

## Integrated dry run

Run on master `9798f6d02` from this document (see "How this plan was verified").

- **From this document.** All 85 blocks of Tasks 1 to 6 applied at their turn: 7 Create blocks and 78 Replace blocks (4 of them "Replace all", 14 matches in all), each matching exactly as many times as stated. Each task's commit staged exactly its changes (`git status` clean after every commit). The resulting tree is byte-identical (`git diff`, empty) to the tree the tasks were first built in.
- **Every "Expected: FAIL" run failed, 11 of 11, for the stated reason:** Task 1 build (`undefined: MitigationCurve`); Task 2 build (`b.PhysicalMitigationKnee undefined`), then only `TestShippedConfigCarriesTheMitigationCurve` (9 missing keys, 12 values); Task 3 `go vet` of `internal/combat` (`cannot use shippedCurveForTest (variable of struct type MitigationCurve) as float64 value in argument to CritOrMitigatedDamage`) and of `internal/hooks` (`undefined: combat.MitigationCurveFor`); Task 4 `TestCritOrMitigatedDamage_AboveTheKneeLosesTheOverflow` at `crit_overflow_test.go:56` and `:61`, then the melee build (`sdp.critReduction undefined`); Task 5 the combat build (`undefined: EffectiveMitigationFor`), both template tests (`function "effectiveMitigation" not defined`), the status sheet (`fortified` in the Defenses row), then only `TestPowerScore_ReadsEffectiveMitigation` (`Max difference between 90 and 370 allowed is 0.01`). Task 1's null probe failed the three tests it names.
- **Every "Expected: PASS" run passed, 23 of 23,** and `go test . -count=1` passed at the end of every task. Between Tasks 2 and 3 the `combat`, `hooks`, `actions`, `usercommands` and `templates` packages passed with the code still clamping at the new 0.90 default (D11).
- **Whole tree (Task 7).** `gofmt -l` printed nothing; `go build ./...` and `go vet ./...` were clean; `go test ./... -count=1` passed with 132 packages `ok` and none failing; `golangci-lint run --new-from-merge-base=origin/master ./...` reported `0 issues.`; the dash scan of the added lines found nothing; the `config.yaml` diff is 20 insertions and 5 deletions, every non-comment line a mitigation knob; `tools/context_md_audit.py` printed the same list as master. Boot check on ports 43333 / 19999 / 18090 / 45555: exit 124, 0 panics, 1 `Server Ready`, and the log shows all four scratch ports bound.

Building the tasks and dry-running this document forced these corrections, all folded into the tasks above (no design change):

- **#472 landed mid-plan** and moved the taunt call (`combat_taunt.go:263`), the `damage_pipeline.go` row of `internal/combat/context.md` (it gained the `GetConvictionDamageDescription` text, so Task 6 anchors on a substring of that row) and the top of `PATCH_NOTES.md`.
- **Task 3:** the six `CritOrMitigatedDamageScaled` calls in `crit_damage_test.go` were first one "Replace all" on a pattern ending in a space; a Markdown editor can strip that, so they are four blocks keyed on the bonus argument.
- **Task 4:** the crit tests were first one file whose melee half failed to build before the resolver half could fail on its numbers; they are split (`crit_overflow_test.go`, `crit_overflow_melee_test.go`) so each half fails for its own reason.
- **Task 5:** `EffectiveMitigationFor` lands one step before `PowerScore` uses it, so `TestPowerScore_ReadsEffectiveMitigation` fails on its number (370 against 90), not only on a build error. `modules/leaderboards` has no test files; it is left out of the test run.
- **Task 7:** the first boot override left the AI telnet port at the shipped 55555, which the owner's server may hold; the override now moves it (`AIPort: 45555`).

## Self-review against the spec

- **Section 1, the curve:** Task 1 (`EffectiveMitigation`, continuity, monotonic, cap, the table within rounding, NaN and negative, aim 0). Table check: 0.50 / 0.75 / 0.91 / 1.00 / 1.64 / 1.81 / 2.70 / 3.30 / 3.70 give 50 / 67 / 73 / 75 / 83 / 84 / 88 / 89 / 90%.
- **Section 2, twelve knobs:** Task 2 (fields, validator, defaults equal shipped, shipped values read from the file before `Validate`). Deviations D2, D3, D4.
- **Section 3, where the curve applies:** Task 3 (all 10 call lines, `ReflectDamage`, armor piercing first) and Task 5 (`PowerScore`, `status`).
- **Section 4, crits:** Task 4 (resolver and melee, after armor piercing, below the knee unchanged, melee and spell agree; the defence multiplier stays skipped on crits: `skill_moves.go` and `throw.go` still multiply only non-crits).
- **Section 5, player text:** Task 6 (patch notes in words; `help armor` was found to describe the cap and is rewritten).
- **Testing section:** curve (Task 1), config (Task 2), combat pins moved not deleted (Tasks 3, 4), crits (Task 4), readers (Task 5), Python models noted (facts table), playtest (Task 8).
- **Placeholders:** none; every step carries its code or its command. **Names:** `MitigationCurve`, `EffectiveMitigation`, `CritReduction`, `MitigationCurveFor`, `EffectiveMitigationFor`, `validateMitigationCurve`, `critReduction`, `shippedCurveForTest`, `pinShippedCurve`, `effectiveMitigation` are each defined in exactly one task and used only after it.
- **Out of scope, per spec:** retuning gear, wards, mutations or mobs; per-source curves; changing armor piercing; PowerScore's x300 weight (D8).
