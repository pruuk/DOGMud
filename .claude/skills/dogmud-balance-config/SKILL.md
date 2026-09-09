---
name: dogmud-balance-config
description: Use before hardcoding any balance number, or when retuning how something feels. Covers that 375 balance knobs are declared in internal/configs/config.balance.go and surfaced through _datafiles/config.yaml, that retuning is a config edit rather than a code change, that a Go default is never a live value because several shipped knobs differ sharply, that an absent key is meaningful because 0 is a legal shipped value, and that config.yaml carries skip-worktree so it desyncs in both directions.
---

## Look for the knob before editing a literal

Lifted verbatim from CLAUDE.md, "Balance Lives in config.yaml, Not in Code"
(lines 454-487 as of this skill's authoring):

**Before hardcoding any balance number, check whether a knob already exists.**
There are **352 balance knobs**, all declared in the single file
`internal/configs/config.balance.go` (466 `Config*`-typed fields across the
whole config package), surfaced through a 1506 line `_datafiles/config.yaml`.
The seven sibling `config.balance.*.go` files (`combat`, `discovery`, `misc`,
`mobs`, `progression`, `shops`, `spells`) declare **no fields at all**; they
hold only defaulting and validation logic, so look in `config.balance.go` for
the field and in the sibling named for its subsystem (`config.balance.shops.go`
for shop knobs, and so on) for its default. If you cannot tell which subsystem
owns a knob, grep its name across `config.balance.*.go`.
Damage scales, mitigation caps, regen percentages,
progression rates, resource penalty curves, shop pricing, toxicity, salvage
odds, conversation and schedule pacing are all tunable without a rebuild.

Three rules follow from this:

1. **Retuning is a config edit, not a code change.** If you find yourself
   editing a literal in `internal/` to change how something feels, stop and
   look for the knob. If there genuinely is not one, adding a knob is usually
   the better change than editing the literal.
2. **Never quote a Go default as a live value.** Defaults are fallbacks applied
   only when the key is absent from `config.yaml`. Several shipped values differ
   sharply from their defaults (`SpellDamageScale` ships at 3.12 against a
   default of 1.0). Read `config.yaml` for what the game actually does.
3. **Absence is meaningful.** A knob left out of `config.yaml` falls back to its
   Go default, and `0` is a legal shipped value (`StaminaPerStrength: 0`). Do
   not assume a missing key means "unset" or "zero".

This is also worth surfacing to readers of the public diagrams page: "the
combat model is data, not code" is a genuinely interesting property to an
engineering audience.

## Never quote a Go default as a live value

The point above is easy to skim past, so state it plainly: a default in Go
code is a fallback, applied only when `config.yaml` omits the key. It is
never proof of what the shipped game does. Verified today (2026-09-08):
`SpellDamageScale` ships at `3.12` in `_datafiles/config.yaml:947`, against a
Go default of `1.0` set in `internal/configs/config.balance.spells.go:31`.
Quoting the default instead of the shipped value would be off by more than
3x on a live combat number.

**Absence is meaningful, and that rule has two halves.** The lifted block
above illustrates only the first half with `StaminaPerStrength`: that key is
present in `_datafiles/config.yaml:1142`, set to `0`, which shows that zero
is a legal shipped value. It does not demonstrate absence, because the key
is right there in the file.

The other half, and the more dangerous one, is a key that is not in
`config.yaml` at all. `CarryCapacityMultiplier` is genuinely absent, verified
2026-09-08: grep for both `CarryCapacityMultiplier` and its snake_case form
`carry_capacity_multiplier` returns zero hits in `_datafiles/config.yaml`.
The field is declared at `internal/configs/config.balance.go:770`, and its
Go default silently applies at `0.65`, set in
`internal/configs/config.balance.misc.go:157`. Nothing in `config.yaml`
tells you this knob exists. Someone reading only the config file would
conclude carry capacity has no multiplier at all, when in fact one is
live at 0.65. That is the case the rule actually warns about: do not treat
a key's absence from `config.yaml` as evidence a system is disabled or
unset, and do not assume the config file is a complete list of what is
tunable, because a key can be silently governed by its Go default with no
trace in the shipped YAML.

This is the same discipline this skill's own numbers below are held to: a
remembered count that nobody re-checks against source is exactly how a stale
figure like the ones in the next section survives past the change that
invalidated it. `[[reference-clean-hit-rate-is-mislabelled]]` is a sharper
version of the same trap outside config: a combat analytics script printed
the plain hit rate under the label "CLEAN-HIT RATE," and a balance change
was tuned against the mislabelled number before anyone re-derived it from
source.

## Where knobs are declared

All balance knob fields live in the single file
`internal/configs/config.balance.go`. The seven sibling files
(`config.balance.combat.go`, `config.balance.discovery.go`,
`config.balance.misc.go`, `config.balance.mobs.go`,
`config.balance.progression.go`, `config.balance.shops.go`,
`config.balance.spells.go`) declare zero `Config*`-typed fields between them,
verified by grep; they hold only defaulting and validation logic (the
`Validate()` and `applyDefaults()`-style functions that clamp or backfill a
value after YAML load). Plain field-name grep across the siblings is enough
to find which one owns a knob's default, since each sibling references the
field directly as `b.FieldName`.

**Grep the YAML tag, not the Go field name, when searching INSIDE
`config.yaml` itself.** Most fields share their PascalCase Go name with
their yaml tag, but not all of them do, and a field-name grep against
`config.yaml` finds nothing for the ones that differ. Verified 2026-09-08:
of the 375 `Config*`-typed fields in `config.balance.go`, exactly **8**
carry a yaml tag that does not match the Go field name, all snake_case:
`ReachStandingGrappleRadius` (`internal/configs/config.balance.go:191`)
carries the tag `` yaml:"reach_standing_grapple_radius" ``, and
`config.yaml:1064` holds only the snake_case form
`reach_standing_grapple_radius: 0.5`. Grepping the PascalCase field name
against `config.yaml` for that knob returns nothing, even though the knob is
present and shipped. The other seven are `ReachGroundGrappleRadius`,
`ReachUtilityFloor`, `SubmissionAttemptAlpha`, `SubmissionAttemptCritZ`,
`SubBadZThreshold`, `SubGoldLossFraction`, and `BrokenLimbBuffDuration`, all
in the same grapple/submission/broken-limb cluster.

**CLAUDE.md's counts are stale, verified against source 2026-09-08:**

| Claim | CLAUDE.md figure | Actual, verified 2026-09-08 |
|---|---|---|
| Fields in `config.balance.go` | 352 | **375** `Config*`-typed fields (`grep -cE '^\s*[A-Za-z_]+\s+Config[A-Za-z]+\b' internal/configs/config.balance.go`) |
| `Config*`-typed fields across the whole `internal/configs` package | 466 | **508** (515 raw matches of the same pattern across all non-test `.go` files in the package, minus 7 false positives in `config_types.go` where the pattern matches the `type ConfigInt int` style declarations, not struct fields) |
| `_datafiles/config.yaml` line count | 1506 | **2296** lines in the committed blob (`git show HEAD:_datafiles/config.yaml \| wc -l`); the on-disk working copy reads 2300, a few lines longer from the local, uncommitted skip-worktree divergence described below |
| Seven sibling `config.balance.*.go` files declare no fields | (same claim) | **Confirmed correct**, 0 fields in each of the seven, matching CLAUDE.md exactly |

The 352/466/1506 figures were accurate when CLAUDE.md's balance section was
written but the config surface has grown since. Do not carry the old numbers
forward, and do not assume today's verified numbers stay correct either: the
whole point of this skill is that a knob count is exactly the kind of fact
that must be re-read from source, not recalled, every time it matters.

## config.yaml has skip-worktree

`_datafiles/config.yaml` carries the git skip-worktree bit
(`git ls-files -v _datafiles/config.yaml` shows `S`), which hides a
deliberate local-dev divergence (locally the file diverges from the tracked
blob, for example a different `HttpPort` and log settings). Consequences:

- `git status` shows the file as clean even when it has local edits.
- `git add` on it fails with a misleading error about "sparse-checkout
  definition." The worktree is not sparse; that is just modern git's wording
  for a skip-worktree path.
- The disk copy and the committed blob can drift arbitrarily far apart in
  both directions, because normal `git status`/`git diff` never surfaces the
  drift while the bit is set.

**Build any commit to this file from the committed blob, never from disk.**
Use `git show HEAD:_datafiles/config.yaml` as the base, not the working-tree
copy, since the working-tree copy may hold local-only values that must never
land in a commit (for example a dev-only port).

**`git update-index --cacheinfo` clears the skip-worktree bit.** A
hash-object-plus-cacheinfo procedure for staging a config change rewrites
the index entry from scratch, which silently clears the bit. The symptom
shows up several commits later: `git status` starts reporting the file as
modified, and switching branches refuses with an uncommitted-changes error
on a file that is supposed to be invisible. Check with
`git ls-files -v _datafiles/config.yaml` after any procedure that touches
the index for this file: `S` means the bit is still set, `H` means it was
cleared and needs `git update-index --skip-worktree _datafiles/config.yaml`
to restore it. [[reference_config_yaml_skip_worktree]]

## Sources

Lifted verbatim from CLAUDE.md:
- "Balance Lives in config.yaml, Not in Code" (lines 454-487)

Folded memory files:
- [[reference_config_yaml_skip_worktree]] (skip-worktree desync, both
  directions; the `--cacheinfo` trap; build commits from the `git show HEAD:`
  blob)

Cited, not folded:
- [[reference-clean-hit-rate-is-mislabelled]] (a dated incident where a
  combat analytics tool mislabelled hit rate as clean-hit rate; used above
  only as an example of the same class of trap, not folded for its own
  content)

Deliberately left out of this skill: the skip-worktree memory file's
2026-08-22 measured drift table (which specific knobs were missing from a
particular local disk copy on that date) and its 2026-08-27 recovery
walkthrough for restoring a cleared bit step by step. Those are dated,
situational incident logs rather than standing procedure; the standing
procedure (build from the blob, watch for `--cacheinfo` clearing the bit,
check with `git ls-files -v`) is folded above.
