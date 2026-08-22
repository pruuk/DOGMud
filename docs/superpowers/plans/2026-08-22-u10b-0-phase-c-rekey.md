# U10b-0 Phase C: The Re-key Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the progression curve's difficulty rank the character's *trained
points* rather than a use counter, for players and mobs alike.

**Architecture:** Four rank sites swap `useCount / UsesPerRank` for
`GetStatTraining(stat)` (stats) and the skill level (skills), and the
equipment-contaminated value floor at each is deleted. `StatProgressionSoftCap`
drops 150 → 50 in the same commit, because the two are only correct together.
Separately, the mob spawn pool moves from `Training` to `Base` so `Training`
means gains-since-spawn for every creature type, with a migration for legacy
saves and gain-based mob caps replacing the value-based ones.

**Tech Stack:** Go (`internal/characters`, `internal/configs`, `internal/mobs`,
`internal/hooks`), plus `_datafiles/config.yaml`.

**Spec:** `docs/superpowers/specs/2026-08-21-u10b-0-progression-rank-from-training-design.md`
sections 4, 5, 13.2, 13.5, 13.6, 15. **Phase index:**
`2026-08-21-u10b-0-README.md` — read its Phase C block, which now records that
**two of its four original corrections are dead**.

**Branch:** `feature/u10b-0-phase-c-rekey`, stacked on
`feature/u10b-0-phase-b-safety-fixes` because the re-key *relocates* the
truncation Phase B removed. Shipping C without B re-creates the sealing bug at a
new point on the curve (spec 13.1). Rebase onto master once PR #56 merges.

---

## What is already settled, so nobody re-derives it

**Verified 2026-08-22 against post-Phase-A master. Do not re-litigate these.**

| Claim | Status |
|---|---|
| "44 mob types drop out of `hasProgression`" | **FALSE — zero do.** `ensureAllSkills` floors every skill at 1, so `len(Skills) > 0` always. Probed over all 634 statpool templates. |
| "Distribution runs before `Validate()`, so `Base++` suppresses hydration" | **MOOT.** Spawn is `mob := *m` off an already-Validated template, so `Base` already holds species + authored. |
| Both restores double-count | **CONFIRMED.** `mobs.go` and `applyCompanionState` both *assign* `Training`. |
| `MobSkillCap` 3 → 25 is a big raise | **CONFIRMED and modelled** — see Task C6. |
| The authored component of a legacy save | **EXACTLY recoverable** as `template.Base − species.Base`. Verified across all 1,764 folded rows, zero mismatches. |

**Owner ruling on the migration (2026-08-22):**
`gains = saved − (template.Base − species.Base) − template.StatPool`, floored at
0. Exact on totals; only the random per-stat share of the spawn pool is
approximated.

**Owner ruling:** the inert `hasPersistableState` gate is **filed, not fixed
here** ([[project-mob-instance-save-gate-is-inert]]).

---

## Standing rules

1. **Tasks C1 and C2 must land in ONE commit.** Any intermediate state where
   `StatProgressionSoftCap` is 50 while a site still reads the use counter puts
   every character's every stat above the soft cap, firing the old value floor
   globally. Do not split them "for reviewability".
2. **Go defaults move with shipped values.** A test binary never loads
   `config.yaml`.
3. **No absolute line numbers for code an earlier task shifts.** Locate with
   `grep` at execution time and confirm the match is the symbol you meant.
4. **`grep --include=*.go` cannot see `config.yaml` or `AGENTS.md`.**
5. **Death lowering `Training`, and therefore making a stat cheaper to regain,
   is INTENDED** (spec 13.5). Do not "fix" `Death_PlayerCleanup.go`.

---

## File Structure

**Modified:**
- `internal/characters/progression.go` — four rank sites, four value floors, mob gates
- `internal/configs/config.balance.go` — three new mob knobs
- `internal/configs/config.balance.progression.go` — `StatProgressionSoftCap` default
- `internal/configs/config.balance.mobs.go` — new mob cap defaults
- `_datafiles/config.yaml` — `StatProgressionSoftCap`, the three mob knobs
- `internal/mobs/mobs.go` — spawn pool `Training++` → `Base++`; instance restore migration
- `internal/hooks/PlayerSpawn_HandleJoin.go` — companion restore migration
- `internal/characters/context.md`, `internal/mobs/context.md` — the seam's contract

**Created:**
- `internal/characters/progression_rank_test.go` — rank-source gates
- `internal/mobs/migration_legacy_training_test.go` — the migration's arithmetic

---

## Task C1 + C2: the re-key and the soft cap (ONE commit)

**Files:** Modify `internal/characters/progression.go`,
`internal/configs/config.balance.progression.go`, `_datafiles/config.yaml`.
Test: create `internal/characters/progression_rank_test.go`.

- [ ] **Step 1: Confirm the four sites and the four floors**

```bash
grep -n "GetSkillUseCount\|GetStatUseCount" internal/characters/progression.go | grep -v "^59[0-9]"
grep -n "StatProgressionSoftCap) && statVal > virtualRank" internal/characters/progression.go
grep -n "SkillSoftCap) && skillLevel > virtualRank" internal/characters/progression.go
```

Expect rank derivations inside `CheckSkillProgression`, `statProgressionChance`,
`CheckStatProgression` and `regenDamperFactor`; three stat value floors and one
skill floor. **If the counts differ, stop and re-cut this task.**

- [ ] **Step 2: Write the failing tests**

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// Rank is trained points, not uses. A character with an enormous use counter and
// no trained points is a BEGINNER and must roll near the base chance.
func TestStatRank_IsTrainingNotUseCount(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "ManyUsesNoGains"
	c.StatUseCount["dexterity"] = 39772 // Meirok's real counter
	c.Stats.Dexterity.Training = 0
	c.Stats.Dexterity.Recalculate()

	fresh := New()
	fresh.Name = "Fresh"

	got, want := c.statProgressionChance("dexterity", 1.0), fresh.statProgressionChance("dexterity", 1.0)
	if got != want {
		t.Errorf("use count still moves the chance: %v vs a fresh character's %v", got, want)
	}
}

// Equipment must never make a stat harder to train. This is the gear leak the
// deleted value floor caused: it keyed on GetStatValue, which includes Mods.
func TestStatRank_EquipmentDoesNotRaiseDifficulty(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "Geared"
	c.Stats.Strength.Base = 120
	c.Stats.Strength.Training = 10
	c.Stats.Strength.Recalculate()
	bare := c.statProgressionChance("strength", 1.0)

	c.Stats.Strength.SetMod(80) // a big stat item
	c.Stats.Strength.Recalculate()
	if got := c.statProgressionChance("strength", 1.0); got != bare {
		t.Errorf("equipment changed training difficulty: %v geared vs %v bare", got, bare)
	}
}

// A high Base must not either: a mob with a huge authored stat pool is not
// harder to train than a small one. This is what the whole re-key is for.
func TestStatRank_BaseDoesNotRaiseDifficulty(t *testing.T) {
	withRepoRoot(t)

	small := New()
	small.Stats.Vitality.Base = 60
	small.Stats.Vitality.Recalculate()

	huge := New()
	huge.Stats.Vitality.Base = 600
	huge.Stats.Vitality.Recalculate()

	if a, b := small.statProgressionChance("vitality", 1.0), huge.statProgressionChance("vitality", 1.0); a != b {
		t.Errorf("Base moved the chance: %v at base 60 vs %v at base 600", a, b)
	}
}

// The two documented anchors from spec section 5, which are what pin soft cap 50.
func TestStatChance_ReproducesTheDocumentedAnchors(t *testing.T) {
	withRepoRoot(t)
	b := configs.GetBalanceConfig()
	if int(b.StatProgressionSoftCap) != 50 {
		t.Fatalf("StatProgressionSoftCap is %d, want 50", int(b.StatProgressionSoftCap))
	}

	// perception has a per-stat multiplier of 1.0, so it shows the raw curve.
	fresh := New()
	if got := fresh.statProgressionChance("perception", 1.0); got < 0.26 || got > 0.28 {
		t.Errorf("fresh stat chance %v, want ~0.27 (0.12 x 2.25)", got)
	}

	trained := New()
	trained.Stats.Perception.Training = 50 // the old "stat at 150"
	trained.Stats.Perception.Recalculate()
	if got := trained.statProgressionChance("perception", 1.0); got < 0.012 || got > 0.015 {
		t.Errorf("chance at Training 50 is %v, want ~0.0134", got)
	}
}

// Skills key on the skill level, which they effectively already did above the
// soft cap. Below it, the use counter must stop mattering.
func TestSkillRank_IsLevelNotUseCount(t *testing.T) {
	withRepoRoot(t)

	c := New()
	c.Name = "SkillUses"
	c.Skills["weapon-combat"] = 5
	c.SkillUseCount["weapon-combat"] = 40000

	fresh := New()
	fresh.Skills["weapon-combat"] = 5

	if got, want := c.skillProgressionChance("weapon-combat", 1.0), fresh.skillProgressionChance("weapon-combat", 1.0); got != want {
		t.Errorf("skill use count still moves the chance: %v vs %v", got, want)
	}
}
```

**Note the last test needs `skillProgressionChance` to exist.** The skill path
currently assembles its chance inline inside `CheckSkillProgression`. Extract it
to a `skillProgressionChance(skillName string, bonusMultiplier float64) float64`
mirroring `statProgressionChance`, and have `CheckSkillProgression` call it.
That extraction is required anyway: Phase E needs the expression exported, and
an inline copy is what let the dashboard drift.

- [ ] **Step 3: Verify they fail**

```bash
GOTMPDIR=C:/gotmp go test ./internal/characters/ -run "TestStatRank|TestSkillRank|TestStatChance_Reproduces" -v
```

- [ ] **Step 4: Re-key the stat sites**

In `statProgressionChance`, `CheckStatProgression` and `regenDamperFactor`,
replace:

```go
	virtualRank := c.GetStatUseCount(statName) / int(b.UsesPerRank)
	if statVal := c.GetStatValue(statName); statVal > int(b.StatProgressionSoftCap) && statVal > virtualRank {
		virtualRank = statVal
	}
```

with:

```go
	// Rank is TRAINED POINTS, not uses and not the final value. Uses measured
	// how often you swung, which punished frequency; the value included Base
	// (the baseline you started with) and Mods (equipment), so gear made a stat
	// harder to train and a big mob was harder to train than a small one.
	// Training is exactly what progression has added, so difficulty depends only
	// on gains actually made.
	//
	// The old anti-exploit value floor is deleted with the counter: it existed
	// because a counter could be low while the value was high, which cannot
	// happen when the rank IS the gains.
	virtualRank := c.GetStatTraining(statName)
```

`CheckStatProgression` keeps its copy only for the debug log; it must read the
same expression, so leave the single line rather than re-deriving.

- [ ] **Step 5: Re-key the skill site**

In the extracted `skillProgressionChance`, replace the `progressMult`-adjusted
counter block and the skill value floor with:

```go
	// Rank is the skill LEVEL. The use-count normalisation that used to sit here
	// existed to stop frequently-fired skills exhausting the curve faster than
	// rare ones; with rank as the level, a skill's difficulty depends on how far
	// it has actually come, which is frequency-independent by construction.
	actualSkillName := resolveSkillName(skillName)
	virtualRank := c.Skills[actualSkillName]
```

**`skills.GetProgressionMultiplier(skillName)` stays** as a chance multiplier —
it is the per-skill pace dial and Phase D re-derives it. Only its use as a
*use-count normaliser* goes.

- [ ] **Step 6: Move the soft cap, both places**

```bash
grep -n "StatProgressionSoftCap: 150" _datafiles/config.yaml
grep -n "b.StatProgressionSoftCap = 150" internal/configs/config.balance.progression.go
```

Set both to **50**, and rewrite the config comment: it currently describes
virtual ranks from use counts and the anti-exploit floor, neither of which
exists after this task. Say instead that it is the trained-points rank at which
progression slows sharply, that a fresh stat is ~27% and a stat with 50 trained
points ~1.3%, and that it is **not** a cap on stat values.

`SkillSoftCap` stays **50** — skills already keyed on level above it.

`config.yaml` carries the git skip-worktree bit. Build the commit from
`git show HEAD:_datafiles/config.yaml`, never from disk, per
`reference_config_yaml_skip_worktree`.

- [ ] **Step 7: Sweep the prose that describes the retired model**

`grep --include=*.go` cannot see these. Check each:

```bash
grep -rn "UsesPerRank\|virtual rank\|use count" _datafiles/config.yaml AGENTS.md CLAUDE.md docs/ internal/characters/context.md | grep -v superpowers/plans
```

`UsesPerRank` becomes dead for decay purposes but the counters stay in the save
as telemetry (spec section 7). Say that where it is documented rather than
deleting the knob.

- [ ] **Step 8: Verify and commit**

```bash
gofmt -l internal/ && GOTMPDIR=C:/gotmp go build ./... && GOTMPDIR=C:/gotmp go test ./... -count=1 2>&1 | grep -E "^(FAIL|--- FAIL)"
```

`TestIntegration_StatSoftCapCurve` () is a known casualty: with the cap at 50 its
`belowCap` and `atCap` fixtures become the same call. Re-pick its fixtures
rather than deleting it.

```bash
git add internal/characters/ internal/configs/config.balance.progression.go _datafiles/config.yaml
git commit -m "feat(u10b-0): progression rank is trained points, not use counts"
```

---

## Task C3: mob spawn pool moves to `Base`

**Files:** Modify `internal/mobs/mobs.go`. Test: extend
`internal/mobs/statdump_test.go` usage (no new test file).

- [ ] **Step 1: Make the change**

```bash
grep -n "Stats.Strength.Training++" internal/mobs/mobs.go
```

Change all six `.Training++` to `.Base++` in that distribution block, and add
above it:

```go
			// Pool points land in Base, not Training. Training is
			// gains-since-spawn for players, and U10b-0 makes the progression
			// curve read it as difficulty rank — so a mob whose authored pool
			// sat there would start partway down the decay curve and could be
			// frozen outright by MobStatTrainingCap. Safe with respect to
			// species hydration: this runs on a copy of a template already
			// Validated at load, so Base already carries species + authored.
```

- [ ] **Step 2: Prove the mob's stats did not move**

`Value = Base + Training + Mods`, so moving a point between Base and Training
must leave every stat identical. Templates carry no spawn pool, so the template
dump cannot see this — spawn real mobs instead:

Write a throwaway test that spawns each of ~20 templates with a fixed
`forceStatPool`, sums `Value` across the six stats, and asserts the total equals
`species + authored + pool` both before and after. Delete it once it passes.

- [ ] **Step 3: Commit**

---

## Task C4: the legacy-save migration

**This is the task that sank two previous plans. The arithmetic is settled;
implement exactly it.**

Both restores currently *assign*, and after C3 a fresh spawn has already rolled
the pool into `Base`. A legacy saved value is three things fused:

```
saved = authored + spawnPool + gains
```

`authored` is exactly recoverable as `template.Base − species.Base` (verified
across all 1,764 folded rows). `spawnPool` is `template.StatPool`, exact in
total. So:

```
gains = max(0, saved − (template.Base − species.Base) − spawnPoolShare)
```

**Files:** Modify `internal/mobs/mobs.go`,
`internal/hooks/PlayerSpawn_HandleJoin.go`. Test: create
`internal/mobs/migration_legacy_training_test.go`.

- [ ] **Step 1: Version the save**

Add `SchemaVersion int` to `MobInstanceData` and to `CompanionInfo`. Absent
(zero) means legacy and triggers the conversion; new saves write 1 and are
restored as-is. Without this the migration runs twice on the second load and
subtracts the pool again.

- [ ] **Step 2: Write the failing tests**

Cover, per stat and in total:
- a never-progressed legacy pet → **exactly 0 gains**
- a legacy pet with known gains → those gains back, within the pool-split
  approximation
- a saved value smaller than authored + pool → floored at 0, never negative
- a **version-1** save → restored verbatim, migration not applied
- running the migration twice → the same answer as running it once

- [ ] **Step 3: Implement**

A single shared helper, because two call sites doing this independently is how
the double-count arrived in the first place:

```go
// LegacyTrainingToGains converts a pre-U10b-0 saved Training value into
// gains-since-spawn. See the plan's Task C4 for the derivation.
func LegacyTrainingToGains(saved, templateBase, speciesBase, poolShare int) int
```

- [ ] **Step 4: Re-point both restores at it, and commit**

---

## Task C5: gain-based mob stat cap

Today `statProgressionChance` gates on `GetStatValue(statName) >= MobStatCap`
(200). That is a **value** cap, so a mob with base 250 can gain nothing while a
mob with base 180 can gain 20 — the asymmetry spec 13.2 removes.

- [ ] Add `MobStatTrainingCap` (default **50**) to the balance config and
  `config.yaml`, with a `<= 0` safety default.
- [ ] Replace the gate with `c.GetStatTraining(statName) >= int(b.MobStatTrainingCap)`
      at **BOTH** sites. There are two, not one:
      `statProgressionChance` and `CheckRegenProgression` each carry their own
      copy (`grep -n "MobStatCap" internal/characters/progression.go`). Missing
      the regen one leaves a mob capped by value on the faucet that actually
      drives its vitality.
- [ ] Decide `MobStatCap`'s fate explicitly: it has other readers
      (`grep -rn "MobStatCap" --include=*.go internal/`). Leave it for those or
      retire it, but do not leave a dead knob undocumented.
- [ ] Test: a mob at base 250 can still gain; a mob with 50 trained points cannot.

---

## Task C6: mob skill caps — MODEL BEFORE SHIPPING

`MobSkillCap` is **3** today. Spec 13.2 raises it to a soft 20 / hard 25.
**Modelled 2026-08-22 with the shipped config** (`SkillMultiplierBase` 1.0,
`Max` 3.0, `SkillSoftCap` 50, `SkillWeight` 5.0):

| mob skill rank | damage multiplier | contest contribution |
|---|---|---|
| 3 (today's cap) | 1.490 | +15 |
| 20 (proposed soft) | 2.265 | +100 |
| 25 (proposed hard) | 2.414 | +125 |

So the ceiling raise is **1.62× damage and +110 to the contest term**. For
scale, +110 is **24% of Meirok's melee score of 455**. This is a large mob buff,
not a cap repair.

It also compounds: under the re-key a mob's skill rank *is* its level, so a mob
at skill 1 sits at nearly the maximum progression chance and climbs steadily,
where before it stopped at 3.

### The pacing, modelled — the cap is a ceiling, not a start

**Every mob starts at skill 1.** Verified: all 124 authored skill entries across
the 79 templates with a `skills:` block are level `1`, and `ensureAllSkills`
floors every other skill at 1. Nothing spawns near the cap.

Climbing is slow. With the shipped config (`MobProgressionRate` 0.5,
`weapon-combat`/`unarmed-combat` multiplier 0.23, soft cap 20), a mob swinging
once per round needs:

| to reach skill | combat rounds |
|---|---|
| 3 (today's cap) | ~180 |
| 10 | ~1,500 |
| 20 | ~8,500 |
| 25 | ~17,500 |

So an ordinary mob never approaches it — it dies in a handful of rounds and
respawns at 1. The raise lands only on creatures that persist AND fight
constantly, which in practice means companions. That is the "favourite pet"
dynamic the owner asked for, so this is a ceiling raise rather than a broad mob
buff, and needs no separate sign-off — only accurate patch notes.

### Audit: skill progression cannot survive death (verified 2026-08-22)

Because this raise makes the invariant load-bearing, the whole chain was
audited rather than assumed:

- `characters/die.go` routes mob death through `Life.TransitionToDead`, so the
  `Death_MobInstanceCleanup` observer fires. **Its "wired but dormant pending
  Task 10" comment is STALE** — Task 10 landed. Fix the comment in this task.
- The observer fires on the **instance**: `mobs.go` calls
  `ResetForMobInstance()` between the shallow copy and `Validate()`, nilling the
  Life machine and clearing `combatPhaseWired` so callbacks re-fire closed over
  the instance. Without it the observer would see `MobInstanceId == 0` and
  early-return, and nothing would ever be cleaned up.
- `scheduleMobDespawnFromLife` deletes the instance file *before* destroying the
  in-memory record, so a respawn reads no file.
- All 15 `DestroyInstance` call sites were checked. Every one that skips the
  delete is legitimate: bounty hunters (`SaveMobInstance` early-returns on
  `bh_target_user_id`, no file exists), companions and charmed mobs
  (early-returns on `IsCharmed()`, state lives on `CompanionInfo`), and room
  unload (`roommanager.go` saves *then* destroys — the mob is not dead).
- A mob that never dies does keep its skills across room unload/reload, which is
  correct. It is bounded: `main.go` calls
  `PruneStaleInstances(MobInstanceMaxAgeDays)` at boot, so a file untouched for
  7 days is removed and that mob reverts to template.

- [ ] **Add a permanent gate for it.** A test that a mob whose skills were
      raised above template, saved, then put through the death-path delete,
      respawns reading from template. The audit above is a point-in-time check;
      this is what keeps it true.
- [ ] **Fix the stale "dormant" comment** on `wireMobInstanceCleanup`.

- [ ] Add `MobSkillSoftCap` (20) and `MobSkillTrainingCap` (25) with `<= 0`
      defaults, in both the Go defaults and `config.yaml`.
- [ ] Use `MobSkillSoftCap` as the curve's soft cap for mobs and
      `MobSkillTrainingCap` as the hard gate, replacing `MobSkillCap`.
Owner reviewed the pacing 2026-08-22 and accepted the raise. If it ever needs
walking back, the lever is `MobSkillTrainingCap`, not the multiplier curve,
which is shared with players.

---

## Task C7: Phase gates

- [ ] `gofmt -l internal/ modules/` prints nothing.
- [ ] `go build ./...` and the full `go test ./...` are green. Note
      `internal/usercommands` is independently flaky on master.
- [ ] **Re-probe the real character.** Rebuild the Meirok probe from Phase B
      (`statProgressionChance` per stat, from `users/3.yaml`) and record the new
      chances. Under Training-rank his dexterity should sit near ~2% rather than
      at the floor.
- [ ] **Exercise the migration before wiping instance saves.** The smoke-test
      SOP's wipe discards exactly the legacy saves this phase migrates, so a
      wipe-then-test proves nothing. Copy `mobs.instances/` aside, boot, confirm
      a known mob's stats, then wipe.
- [ ] Boot test in an isolated detached worktree (`mkdir -p _datafiles/logs`
      first; exit **124** is success; never grep the bare word `panic`).
- [ ] Patch notes: player-facing framing, no raw numbers, no em dashes, 80
      columns. The honest statement is that how hard something is to improve now
      depends on how far you have taken it, not on how often you have used it,
      and that equipment no longer makes a stat harder to train.
- [ ] Adversarial playtest. **The `veteran` profile's seeded use counters become
      irrelevant after this phase**, which makes it a *better* instrument than it
      was for Phase B: its stats have low Training, so progression should fire
      visibly. That is itself the headline check.
- [ ] Ship via PR to `pruuk/DOGMud`, hand over, do not merge.

---

## Self-Review

**Spec coverage:** 4 and 5 (rank = Training, soft cap 50) → C1+C2 · 13.2 (mobs
mirror players; the three caps) → C3, C5, C6 · 13.5 (death penalty intended) →
standing rule 5 · 13.6 (admin write path) → already cut in Phase A's C4 · 15
(authored `training:`) → landed in Phase A.

**Not in this phase, deliberately:** all balance retuning, including the
per-stat and per-skill multipliers and vitality. That is Phase D. C6's cap
choice is a ceiling, not a pace.

**Known-weak points, stated rather than hidden:**
- C4's per-stat split of the spawn pool is an approximation and always will be —
  the original roll is random and unrecorded. Only the totals are exact. The
  tests must assert totals exactly and per-stat within a tolerance, not pretend
  otherwise.
- C6 is a genuine balance change wearing a cap-repair costume. It is flagged for
  an explicit owner decision rather than assumed.
- C2's soft-cap move makes the two documented anchors the only pinned points on
  the new curve. Everything between them is unverified until Phase D measures
  pace.
