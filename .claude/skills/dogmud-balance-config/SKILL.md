---
name: dogmud-balance-config
description: Use before hardcoding any balance number, or when retuning how something feels. Covers that 472 balance knobs are declared in internal/configs/config.balance.go and surfaced through _datafiles/config.yaml, that retuning is a config edit rather than a code change, that a Go default is never a live value because several shipped knobs differ sharply, that an absent key is meaningful because 0 is a legal shipped value, and that config.yaml carries skip-worktree so it desyncs in both directions.
---

## Look for the knob before editing a literal

Lifted from CLAUDE.md's former "Balance Lives in config.yaml, Not in Code"
section, with the counts recounted 2026-10-08:

**Before hardcoding any balance number, check whether a knob already exists.**
There are **472 balance knobs**, all declared in the single file
`internal/configs/config.balance.go` (612 `Config*`-typed fields across the
whole config package), surfaced through a 2802 line `_datafiles/config.yaml`.
The nine sibling `config.balance.*.go` files (`baubles`, `combat`, `discovery`,
`lighting`, `misc`, `mobs`, `progression`, `shops`, `spells`) declare **no
fields at all**; they
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
`SpellDamageScale` ships at `3.12` in `_datafiles/config.yaml:978`, against a
Go default of `1.0` set in `internal/configs/config.balance.spells.go:31`.
Quoting the default instead of the shipped value would be off by more than
3x on a live combat number.

**Absence is meaningful, and that rule has two halves.** The lifted block
above illustrates only the first half with `StaminaPerStrength`: that key is
present in `_datafiles/config.yaml:1173`, set to `0`, which shows that zero
is a legal shipped value. It does not demonstrate absence, because the key
is right there in the file.

The other half, and the more dangerous one, is a key that is not in
`config.yaml` at all. `CarryCapacityMultiplier` is genuinely absent, verified
2026-09-08: grep for both `CarryCapacityMultiplier` and its snake_case form
`carry_capacity_multiplier` returns zero hits in `_datafiles/config.yaml`.
The field is declared at `internal/configs/config.balance.go:785`, and its
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

## The `< 0 || > 1.0` validator can never default a knob

An absent YAML key unmarshals to **0**. Zero is neither negative nor above
1.0, so a validator written `if x < 0 || x > 1.0 { x = default }` never takes
its defaulting branch, and the knob stays at **0.0** forever rather than at
the default the comment advertises. The five `SurpriseAttack*Penalty` knobs
advertised 0.10 through 0.70 and every one of them ran at 0.0, which is why
the pre-U10d surprise burst auto-hit every limb; they have since been deleted
(`3abb94585`). For any knob whose legitimate range includes 0, validate with
`if x <= 0 { x = default }`, and reserve the range-check shape for knobs where
0 is a meaningful shipped value.

The dangerous pair is specifically a **non-zero advertised default** on a key
**absent from `config.yaml`**; the shape alone is often harmless
(`MinAttackCritChance` and `EquipmentDropChance` both use it and both ship
explicitly). A sweep after the source note was corrected found no live
instance left, and that correction is itself the cautionary tale: the false
finding came from grepping the Go field name `SubGoldLossFraction` against a
YAML file whose key is the snake_case tag `sub_gold_loss_fraction`, which is
the same "grep the tag, not the identifier" rule stated below.
[[project-dead-state-machine-registry-and-validator-trap]]

## Fixing the owner's local config is Claude's job

When the owner's local `_datafiles/config.yaml` needs a change (a renamed key,
a stale knob), make the change rather than handing them the chore. Deploys are
the owner's job ([[feedback-owner-does-the-deploys]]); local dev config is not
a deploy. Diff the disk copy against `git show HEAD:_datafiles/config.yaml`,
separate the owner's real local edits from lag behind HEAD, back the disk copy
up to the scratchpad, rebuild from the HEAD blob, reapply only the real edits,
and confirm the `S` bit survived. As of 2026-09-15 the real local edits are
`HttpPort: 8090`, `LogLevel: "info"`, and the `Playtest:` block. Prod's
`config-production.yaml` on the droplet stays the owner's to change.
[[feedback-fix-local-config-yourself]]

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
`config.yaml` finds nothing for the ones that differ. Verified 2026-09-08,
count rechecked 2026-10-08: of the 472 `Config*`-typed fields in
`config.balance.go`, exactly **7**
carry a yaml tag that does not match the Go field name, all snake_case:
`ReachStandingGrappleRadius` (`internal/configs/config.balance.go:191`)
carries the tag `` yaml:"reach_standing_grapple_radius" ``, and
`config.yaml:1095` holds only the snake_case form
`reach_standing_grapple_radius: 0.5`. Grepping the PascalCase field name
against `config.yaml` for that knob returns nothing, even though the knob is
present and shipped. The other six are `ReachGroundGrappleRadius`,
`ReachUtilityFloor`, `SubmissionAttemptAlpha`, `SubmissionAttemptCritZ`,
`SubBadZThreshold`, and `SubGoldLossFraction`, all in the same
grapple/submission cluster.

**How the counts were taken (recounted 2026-10-08, on master `c7520f387`):**

| Claim | Old figure | Actual, 2026-10-08 |
|---|---|---|
| Fields in `config.balance.go` | 352, then 389 (2026-09-15) | **472** (`grep -cE '^\s*[A-Za-z_]+\s+Config[A-Za-z]+\b' internal/configs/config.balance.go`) |
| `Config*`-typed fields across the whole `internal/configs` package | 466, then 520 | **612**: 620 raw matches of the same pattern across the non-test `.go` files, minus 7 false positives in `config_types.go` (the `type ConfigInt int` style declarations) and 1 in `config.balance.lighting.go:207` (a local `def ConfigFloat`) |
| `_datafiles/config.yaml` line count | 1506, then 2327 | **2802** lines in the committed blob (`git show HEAD:_datafiles/config.yaml \| wc -l`) |
| Sibling `config.balance.*.go` files declare no fields | seven siblings | **Nine** siblings now (`baubles` and `lighting` added); still 0 fields in each |

Each earlier figure was accurate when written, and the config surface keeps
growing. Do not carry the old numbers
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
- [[project-dead-state-machine-registry-and-validator-trap]] (its part 2 only:
  the `< 0 || > 1.0` validator shape, the dangerous non-zero-default-plus-
  absent-key pair, and the grep-the-tag correction. Its part 1, the state
  machine registry, is obsolete: the registry was replaced by an on-demand
  resolver on 2026-08-30 and the trap it described no longer exists)
- [[feedback-fix-local-config-yourself]] (resync the owner's local
  `config.yaml` yourself; cites [[feedback-owner-does-the-deploys]] for the
  boundary against deploys, which stays the owner's)

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
