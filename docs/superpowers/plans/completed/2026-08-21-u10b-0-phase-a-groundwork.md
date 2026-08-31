# U10b-0 Phase A: Groundwork Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Put the accessors and the data in the shape the rest of U10b-0 needs,
without changing how anything progresses.

**Architecture:** Two accessors (`GetStatTraining`, `StatPoolTotal`) and one data
migration that folds authored mob `training:` into `base:` across 599 templates.
After this phase, `Training` means "gains since spawn" for mob templates, which
is the invariant every later phase depends on and which the approved spec
wrongly assumed already held.

**Tech Stack:** Go (`internal/characters`, `internal/hooks`,
`internal/usercommands`, `internal/mobs`, `modules/gmcp`), plus a line-based
Python migration script over `_datafiles/world/dogmud/mobs/`.

**Spec:** `docs/superpowers/specs/completed/2026-08-21-u10b-0-progression-rank-from-training-design.md`
sections 13 and 14. **Phase index:** `2026-08-21-u10b-0-README.md`.

**Branch:** `feature/u10b-0-phase-a-groundwork`, cut from `master`.

**Nothing in this phase changes progression.** If any progression rate moves,
something is wrong — stop and diagnose rather than adjusting a knob.

---

## Standing rules

1. **No absolute line numbers for code an earlier task shifts.** Locate with
   `grep` at execution time, and **verify the grep matched the symbol you meant**
   — a previous plan version's grep for a pool reader matched `poolMax` (the
   ceiling) instead of `PoolValue` (the current value), which would have silently
   disabled regen progression.
2. **Never edit a file with a Python read-modify-write.** Project rule: it
   truncates before the write evaluates and has destroyed files twice. Task A3's
   script reads one path and writes a *different* path, then a separate step
   moves them.
3. **Never round-trip mob YAML through a YAML library.** It reformats the file
   and destroys `#` comments. Task A3 is a line-based transform, like
   `tools/id_inventory.py`.
4. **`characters.New()` calls `Validate()`**, which populates `Value` and
   hydrates `Base` from the species record.

---

## File Structure

**Modified:**
- `internal/characters/skills.go` — `GetStatTraining`, `StatPoolTotal`
- `internal/usercommands/assess.go` — essence bands read `StatPoolTotal`
- `internal/hooks/companion_summon.go` — `corpseRaisePool`
- `internal/hooks/charm_spell.go` — charm resistance
- `internal/hooks/NewRound_MobRoundTick.go` — charm re-roll contest
- `modules/gmcp/gmcp.Mob.go` — mob web editor reads/writes `Base`
- 599 files under `_datafiles/world/dogmud/mobs/` — `training:` → `base:`
- `docs/schemas/mob.md` — the authoring convention changes

**Created:**
- `tools/fold_mob_training_to_base.py` — the migration
- `internal/mobs/template_training_test.go` — the permanent gate

---

## Task A1: `GetStatTraining` accessor

**Files:** Modify `internal/characters/skills.go`. Test: create
`internal/characters/stat_accessors_test.go`.

- [ ] **Step 1: Write the failing test**

```go
package characters

import "testing"

// GetStatTraining reads .Training only. Once Phase A folds authored mob
// training into base and Phase C moves spawn pools there too, Training means
// gains-since-spawn for every character type — so the progression curve keyed to
// it is gear-free and baseline-free by construction.
func TestGetStatTraining_ReadsTrainingOnly(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 115
	c.Stats.Strength.Training = 21
	c.Stats.Strength.SetMod(40)
	c.Stats.Strength.Recalculate()

	if got := c.GetStatTraining("strength"); got != 21 {
		t.Errorf("GetStatTraining(strength) = %d, want 21 (Base and Mods must not leak)", got)
	}
	if got := c.GetStatValue("strength"); got != 176 {
		t.Errorf("sanity: GetStatValue(strength) = %d, want 176", got)
	}
}

func TestGetStatTraining_AllSixStats(t *testing.T) {
	c := New()
	c.Stats.Strength.Training = 1
	c.Stats.Dexterity.Training = 2
	c.Stats.Perception.Training = 3
	c.Stats.Vitality.Training = 4
	c.Stats.Willpower.Training = 5
	c.Stats.Charisma.Training = 6

	for name, want := range map[string]int{
		"strength": 1, "dexterity": 2, "perception": 3,
		"vitality": 4, "willpower": 5, "charisma": 6,
	} {
		if got := c.GetStatTraining(name); got != want {
			t.Errorf("GetStatTraining(%q) = %d, want %d", name, got, want)
		}
	}
	if got := c.GetStatTraining("nonsense"); got != 0 {
		t.Errorf("GetStatTraining(nonsense) = %d, want 0", got)
	}
}
```

- [ ] **Step 2: Verify it fails**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
go test ./internal/characters/ -run TestGetStatTraining -v
```

Expected: FAIL, `c.GetStatTraining undefined`.

- [ ] **Step 3: Add the accessor**

```bash
grep -n "func (c \*Character) GetStatValue" internal/characters/skills.go
```

Insert after that function's closing brace:

```go
// GetStatTraining returns the trained component of the named stat, or 0 if
// unrecognised.
//
// This is the progression curve's rank input (U10b-0). Unlike GetStatValue it
// excludes Base (the baseline the character or mob started with) and Mods
// (equipment and spells), so difficulty depends only on gains actually made.
// Equipping a stat item must never make a stat harder to train, and a mob with a
// large authored stat pool must not be harder to train than a small one.
func (c *Character) GetStatTraining(statName string) int {
	switch statName {
	case "strength":
		return c.Stats.Strength.Training
	case "dexterity":
		return c.Stats.Dexterity.Training
	case "perception":
		return c.Stats.Perception.Training
	case "vitality":
		return c.Stats.Vitality.Training
	case "willpower":
		return c.Stats.Willpower.Training
	case "charisma":
		return c.Stats.Charisma.Training
	}
	return 0
}
```

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/characters/ -run TestGetStatTraining -v
git add internal/characters/skills.go internal/characters/stat_accessors_test.go
git commit -m "feat(u10b-0): GetStatTraining accessor, the new rank input

Reads .Training only. Base is the baseline the character started with and Mods
is equipment; neither may influence how hard a stat is to raise."
```

---

## Task A2: `StatPoolTotal()` — name the concept four systems open-code

Four sites sum a creature's `.Training` as "how much creature is there". Phase C
moves mob spawn pools to `Base` and would zero all four — three of them
player-visible. The accessor lands first so that move becomes invisible to them.

**The formula is invariant across both data changes**, which is why it can land
early:

| | `sum(Base)` | `sum(Training)` | `sum(Base) − speciesBase + sum(Training)` |
|---|---|---|---|
| today | speciesBase | authored + pool | **authored + pool** |
| after Task A3 | speciesBase + authored | pool | **authored + pool** |
| after Phase C | speciesBase + authored + pool | gains | **authored + pool + gains** |

**Files:** Modify `internal/characters/skills.go`,
`internal/usercommands/assess.go`, `internal/hooks/companion_summon.go`,
`internal/hooks/charm_spell.go`, `internal/hooks/NewRound_MobRoundTick.go`.
Test: `internal/characters/stat_accessors_test.go`.

- [ ] **Step 1: Find every consumer**

```bash
grep -rn "Strength.Training" --include=*.go internal/ modules/ | grep -v _test.go
```

Expect these four summing sites plus persistence and accessor sites:
`usercommands/assess.go`, `hooks/companion_summon.go`, `hooks/charm_spell.go`,
`hooks/NewRound_MobRoundTick.go`. (`modules/gmcp/gmcp.Mob.go` reads the six stats
individually rather than summed — Task A4 handles it.)

**If this grep returns a summing site not in that list, stop and add it to the
plan before proceeding.** A previous plan version told the worker to "expect
five" from a grep that returns four, which is exactly the kind of instruction
that gets ignored rather than questioned.

- [ ] **Step 2: Write the failing tests**

Append to `internal/characters/stat_accessors_test.go`:

```go
// StatPoolTotal is "how much creature is there": the authored pool plus gains,
// excluding the species baseline and excluding equipment.
//
// This invariance is what lets the accessor land before the data moves. The same
// creature expressed either way must report the same pool.
func TestStatPoolTotal_UnchangedByWhereThePoolLives(t *testing.T) {
	// Species-1 baseline is 100/stat. "Before": baseline in Base, pool in
	// Training. "After": both in Base.
	before := New()
	before.SpeciesId = 1
	before.Stats.Strength.Base, before.Stats.Strength.Training = 100, 30
	before.Stats.Dexterity.Base, before.Stats.Dexterity.Training = 100, 20
	before.Validate()

	after := New()
	after.SpeciesId = 1
	after.Stats.Strength.Base, after.Stats.Strength.Training = 130, 0
	after.Stats.Dexterity.Base, after.Stats.Dexterity.Training = 120, 0
	after.Validate()

	if before.StatPoolTotal() != after.StatPoolTotal() {
		t.Errorf("pool total moved with the representation: %d vs %d",
			before.StatPoolTotal(), after.StatPoolTotal())
	}
}

func TestStatPoolTotal_CountsGains(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 130
	c.Validate()
	baseline := c.StatPoolTotal()

	c.Stats.Strength.Training = 7
	c.Validate()
	if got := c.StatPoolTotal(); got != baseline+7 {
		t.Errorf("StatPoolTotal = %d after 7 gains, want %d", got, baseline+7)
	}
}

func TestStatPoolTotal_IgnoresEquipment(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 130
	c.Validate()
	baseline := c.StatPoolTotal()

	c.Stats.Strength.SetMod(50)
	c.Stats.Strength.Recalculate()
	if got := c.StatPoolTotal(); got != baseline {
		t.Errorf("equipment leaked into StatPoolTotal: %d, want %d", got, baseline)
	}
}

// Species baselines are NOT uniform — they range from 0 (20-orb has no stats
// block at all) to 6000 (99-ascended); only species 1 sums to 600. An unknown
// species contributes no baseline, which is the right fallback because such a
// character's Base was never hydrated either.
func TestStatPoolTotal_UnknownSpeciesContributesNoBaseline(t *testing.T) {
	c := New()
	c.SpeciesId = 999999
	c.Stats.Strength.Base = 42
	if got := c.StatPoolTotal(); got != 42 {
		t.Errorf("StatPoolTotal = %d for an unknown species, want 42", got)
	}
}
```

- [ ] **Step 3: Verify it fails, then add the accessor**

```bash
go test ./internal/characters/ -run TestStatPoolTotal -v   # expect: undefined
```

`internal/species` is already imported in `skills.go` — confirm with
`grep -n "internal/species" internal/characters/skills.go` before relying on it.

```go
// StatPoolTotal returns "how much creature is there": the authored stat pool
// plus everything progression has added, excluding the species baseline and
// excluding equipment.
//
// Four systems need this number — assess's essence bands, corpseRaisePool, charm
// resistance, and the charm re-roll contest — and all four used to open-code it
// as a sum of .Training. That worked only while a mob's authored stats and spawn
// pool both lived in Training. U10b-0 moves both to Base so Training can mean
// gains-since-spawn for mobs exactly as it does for players.
//
// Subtracting the species baseline is what keeps the answer stable across that
// move, and it is why the bands in assess.go did not need recalibrating. Species
// baselines are not uniform (0 to 6000 across the roster), so this must be a
// per-species lookup, not a constant.
//
// A nil species record contributes no baseline: such a character's Base was
// never hydrated either, so the two cancel.
func (c *Character) StatPoolTotal() int {
	total := c.Stats.Strength.Base + c.Stats.Dexterity.Base +
		c.Stats.Perception.Base + c.Stats.Vitality.Base +
		c.Stats.Willpower.Base + c.Stats.Charisma.Base +
		c.Stats.Strength.Training + c.Stats.Dexterity.Training +
		c.Stats.Perception.Training + c.Stats.Vitality.Training +
		c.Stats.Willpower.Training + c.Stats.Charisma.Training

	if sp := species.GetSpecies(c.SpeciesId); sp != nil {
		total -= sp.Stats.Strength.Base + sp.Stats.Dexterity.Base +
			sp.Stats.Perception.Base + sp.Stats.Vitality.Base +
			sp.Stats.Willpower.Base + sp.Stats.Charisma.Base
	}
	if total < 0 {
		total = 0
	}
	return total
}
```

- [ ] **Step 4: Repoint the four sites**

```bash
grep -n "Strength.Training" internal/usercommands/assess.go internal/hooks/companion_summon.go internal/hooks/charm_spell.go internal/hooks/NewRound_MobRoundTick.go
```

- `assess.go` — `totalTraining := corpse.Character.StatPoolTotal()`. Update the
  comment above it ("Sum all stat training values as a proxy for the creature's
  total power") to name the accessor. **Band thresholds do not change.**
- `companion_summon.go` — `corpseRaisePool` becomes
  `return c.Character.StatPoolTotal()`. Keep its note that it must agree with
  `assess`; that invariant is now enforced by both calling one function.
- `charm_spell.go` — `statTrainingTotal := targetMob.Character.StatPoolTotal()`.
- `NewRound_MobRoundTick.go` — `targetPool := mob.Character.StatPoolTotal()`.

- [ ] **Step 5: Record the one behaviour change this causes**

`assess.go` calls `room.FindCorpse(rest)` with **no `UserId` filter**, unlike
`selectRaiseCorpse`, so it works on *player* corpses too. For a player, `Base` is
a gaussian roll made at character creation, not the species baseline
(`Death_PlayerCleanup.go` says so explicitly), so `StatPoolTotal` differs from
the old `sum(Training)`:

| character | old (`sum Training`) | new (`StatPoolTotal`) |
|---|---|---|
| Meirok | 163 | 187 |
| user 24 | 39 | 196 |

Meirok stays in the same band; user 24 moves from "faint residual essence" to
"substantial essence".

**This is a deliberate, accepted change** — a player corpse's essence should
reflect the whole character, not only what they trained. Do not try to preserve
the old numbers. Note it in the commit and in the Phase F patch notes, and put it
in front of the playtest gate.

- [ ] **Step 6: Verify and commit**

```bash
gofmt -l internal/ modules/
go build ./... && go test ./... 2>&1 | grep -Ev "^ok|no test files"
```

Expected: no output for mobs. If a *player-corpse* assess test exists it may
move a band — check before assuming a failure is unrelated.

```bash
git add internal/characters/ internal/usercommands/assess.go internal/hooks/
git commit -m "refactor(u10b-0): StatPoolTotal, the concept four systems open-coded

assess's essence bands, corpseRaisePool, charm resistance and the charm re-roll
all summed a creature's .Training as 'how much creature is there'. That works
only while a mob's authored stats and spawn pool both live in Training, which
Phase A and Phase C change.

StatPoolTotal is sum(Base) - speciesBase + sum(Training), which returns the
identical number across both moves — so this lands first and the data changes
become invisible to all four. Without it, assess would report every corpse as
barely a trace of essence and raise would reject every corpse.

Species baselines are not uniform (0 to 6000 across the roster), so the
subtraction is a per-species lookup rather than a constant.

One accepted behaviour change: assess does not filter player corpses, and a
player's Base is a gaussian roll rather than the species baseline, so player
corpses now report their whole character rather than only trained points. One
sampled character moves up a band. A player corpse's essence reflecting the
whole character is the more defensible reading."
```

---

## Task A3: Fold authored mob `training:` into `base:` — 599 templates

**This is the task the approved spec did not know it needed.** Spec 13.2 assumed
a mob's `Training` came only from the runtime stat-pool distribution. It does
not: 599 of 641 mob files author `training:` directly under `character.stats`,
with values up to 188. Left in place, `MobStatTrainingCap` (Phase C) would freeze
the mobs whose authored value already exceeds it, at spawn — the exact failure
the owner's ruling exists to remove.

**The arithmetic, per stat, per mob:**

```
base_new = (existing base, else the species base for that stat) + authored training
```

then delete the `training:` key. Value is unchanged: today the loader hydrates
`Base` from species (only when `Base == 0`) and adds authored `training`;
afterwards `base:` carries both and hydration correctly skips a non-zero `Base`.

**Files:** Create `tools/fold_mob_training_to_base.py`,
`internal/mobs/template_training_test.go`. Modify 599 files under
`_datafiles/world/dogmud/mobs/`, `docs/schemas/mob.md`.

- [ ] **Step 1: Survey before changing anything**

```bash
grep -rl "training:" _datafiles/world/dogmud/mobs/ | wc -l        # expect 599
find _datafiles/world/dogmud/mobs -name "*.yaml" | wc -l          # expect 641
grep -rn "base:" _datafiles/world/dogmud/mobs/ | wc -l            # expect few
grep -rn "speciesid:" _datafiles/world/dogmud/mobs/ | wc -l       # expect 641
```

Record the actual numbers. **If any mob file lacks `speciesid:`, the script must
fail on it rather than guess** — a wrong species baseline silently changes that
mob's power.

- [ ] **Step 2: Write the permanent gate first**

`internal/mobs/template_training_test.go`:

```go
package mobs

import "testing"

// After U10b-0 Phase A, no mob TEMPLATE may carry authored stat Training.
// Training means gains-since-spawn; a template has not spawned yet, so any
// nonzero value there is authored baseline in the wrong field.
//
// This is a permanent gate, not a migration check: it fails if a builder or the
// mob web editor reintroduces the old convention. Before Phase A there were 599
// such templates.
func TestNoMobTemplateCarriesAuthoredTraining(t *testing.T) {
	// Load every mob template the way the engine does, then assert each one's
	// six .Training values are zero. Name every offender in the failure, not
	// just a count — a builder needs to know which file to fix.
}
```

Write the body against the package's real loading entry point:

```bash
grep -n "func LoadDataFiles\|func GetAllMobs\|allMobs\b" internal/mobs/mobs.go | head
grep -rn "LoadAllFlatFiles" --include=*.go internal/ | head -3
```

Run it now — it must **fail**, naming 599 files. That failure is the migration's
to-do list and its completion criterion.

- [ ] **Step 3: Write the migration script**

Full path: `C:\Users\Calabe Davis\workspace\DOGMud\tools\fold_mob_training_to_base.py`

**Two hard constraints:**

- **Line-based only.** Do not use a YAML library. Round-tripping reformats the
  file and destroys `#` comments — the mob web editor already has that bug and it
  is not to be replicated. `tools/id_inventory.py` is the precedent: a
  filename/line parser with no YAML dependency.
- **Never read and write the same path.** Project rule: a Python
  read-modify-write truncates before the write evaluates and has destroyed files
  twice. Read `X.yaml`, write `X.yaml.new`, and let Step 5 do the move.

Behaviour:

1. Parse `_datafiles/world/dogmud/species/*.yaml` into
   `{speciesid: {stat: base}}`. A species with no `stats:` block contributes 0
   for every stat (`20-orb` is real; do not treat it as an error).
2. For each mob file: find `speciesid:` and the `stats:` block under
   `character:`. Track indentation — `stats:` appears nested, and a naive search
   for the string `stats:` will also match other blocks.
3. For each stat in that block, collect any `base:` and any `training:` line.
   - `training` absent → leave the stat untouched.
   - `training: 0` → **delete the line only.** Do not write an explicit `base:`;
     leaving the stat absent preserves species hydration, so that stat still
     tracks a future species rebalance.
   - `training: N` (N > 0) → write `base: (existing_base or species_base) + N`
     in place of the first of the two lines, and delete the other.
4. Preserve every other line byte-for-byte, including comments and blank lines.
5. Emit a CSV report: `file, stat, old_base, old_training, species_base,
   new_base` — this is the evidence for the commit and the artefact to check if a
   mob later feels wrong.
6. Support `--dry-run` (report only, write nothing) and `--verify` (recompute and
   assert `old_species_base + old_training == new_base` for every row).

- [ ] **Step 4: Dry-run and read the report**

```bash
python tools/fold_mob_training_to_base.py --dry-run > /tmp/fold-report.csv
wc -l /tmp/fold-report.csv
```

**Read it, do not skim.** Specifically check:
- the ~15 mobs with `training >= 50` (sump_dweller 120, training_post 100,
  windscour_wyrm 100, stone_beetle_queen 75 among them)
- `13-loot_goblin.yaml`, the only file with pre-existing `base:` lines — confirm
  they are summed, not overwritten
- any mob on a species with a 0 baseline
- the largest `new_base` values, for anything implausible

- [ ] **Step 5: Apply, then move**

```bash
python tools/fold_mob_training_to_base.py            # writes *.yaml.new
find _datafiles/world/dogmud/mobs -name "*.yaml.new" | wc -l
```

Spot-check one diff by hand before moving anything:

```bash
diff _datafiles/world/dogmud/mobs/stillwater/332-sump_dweller.yaml{,.new}
```

Expect: `training: 22` → `base: 102` for strength (species 23 = 80),
`training: 120` → `base: 240` for vitality (species 23 = 120), `training: 0`
lines deleted, everything else identical.

Then move them into place:

```bash
find _datafiles/world/dogmud/mobs -name "*.yaml.new" -exec sh -c 'mv "$1" "${1%.new}"' _ {} \;
find _datafiles/world/dogmud/mobs -name "*.yaml.new" | wc -l   # want 0
```

- [ ] **Step 6: Verify by construction, then by boot**

```bash
python tools/fold_mob_training_to_base.py --verify
go test ./internal/mobs/ -run TestNoMobTemplateCarriesAuthoredTraining -v
```

The gate from Step 2 must now **pass**.

Then a boot test — 599 changed data files is exactly the case where YAML parses
but the world does not come up:

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Exit code 124 is the success case. Do not grep for the bare word `panic` —
`GamePlay.MapConsistencyEnforce` legitimately has the *value* `panic`. Clean up
with `git worktree remove --force`.

- [ ] **Step 7: Verify a real mob's stats did not move**

The boot test proves the world loads, not that a mob has the same stats. Wipe
instance saves, boot, and compare one mob in-game against its pre-migration
values — `sump_dweller` is the sharpest case (authored vitality 120 on a species
with vitality 120).

```bash
rm -rf _datafiles/world/dogmud/mobs.instances/* _datafiles/world/dogmud/rooms.instances/*
```

Do **not** touch `shops/`, `guilds/`, or `moderation/`.

- [ ] **Step 8: Update the schema doc**

`docs/schemas/mob.md` — the authoring convention has changed. Authored stats go
in `base:`; `training:` is reserved for runtime gains and **must not appear in a
template**. Say why, and name the gate test so a builder who trips it knows what
it is protecting.

- [ ] **Step 9: Commit**

Stage explicit paths — `git add -A` is a project trap.

```bash
git add tools/fold_mob_training_to_base.py internal/mobs/template_training_test.go docs/schemas/mob.md
git add _datafiles/world/dogmud/mobs/
git commit -m "refactor(u10b-0): fold authored mob training into base, 599 templates

The approved spec assumed a mob's Training came only from the runtime stat-pool
distribution. It does not: 599 of 641 mob files author training: directly under
character.stats, up to 188. Only six base: lines existed world-wide.

Left alone, MobStatTrainingCap would freeze the ~15 mobs whose authored value
already exceeds it, at spawn — the exact 'permanently frozen' failure the owner
ruling exists to remove — and every one of those mobs would start partway down
the decay curve instead of at rank 0.

Value-neutral by construction: base_new = species_base + authored_training, and
the loader's hydration correctly skips a nonzero Base. Species baselines are not
uniform (0 to 6000 across the roster), so the fold is per-species and per-stat.
training: 0 lines are deleted rather than pinned, so those stats keep tracking
species rebalances.

Line-based transform, no YAML library: a round-trip would reformat every file
and destroy its comments.

Adds a permanent gate: no mob template may carry authored Training. It fails if
a builder or the web editor reintroduces the convention.

Report: <paste the summary line counts from the CSV>"
```

---

## Task A4: Re-point the mob web editor at `Base`

The editor reads and writes the six `.Training` values. After Task A3 those are
zero on every template, so the editor would display zeroes and any save would
write authored `training:` straight back — reintroducing the convention the gate
test now forbids.

**Spec 13.6 ruled that cutting the write path was acceptable, on the grounds
that writing `Training` sets progression difficulty. Task A3 changes that
calculus: after the fold the editor writes `Base`, which is just a value.** So
re-point it rather than remove it, and keep the builder capability.

**Files:** Modify `modules/gmcp/gmcp.Mob.go`.

- [ ] **Step 1: Locate**

```bash
grep -n "Training" modules/gmcp/gmcp.Mob.go
```

- [ ] **Step 2: Re-point read and write**

Change both directions to `.Base`. Rename the request field from `StatTraining`
to `StatBase` so the wire format says what it means, and check whether the admin
page's JS reads that key:

```bash
grep -rn "stat_training\|statTraining\|StatTraining" _datafiles/html/ modules/ internal/
```

Update any client-side reference in the same commit.

- [ ] **Step 3: Fix the editor's own test**

```bash
grep -n "StatTraining\|Training" modules/gmcp/gmcp.Mob_test.go
```

It asserts the write round-trips into `.Training`. Re-point it at `.Base`. **This
is an expected failure — do not treat it as a regression.**

- [ ] **Step 4: Verify and commit**

```bash
go build ./... && go test ./modules/gmcp/ -v 2>&1 | tail -10
git add modules/gmcp/ _datafiles/html/
git commit -m "feat(u10b-0): mob web editor authors Base, not Training

After the training-to-base fold the editor would have displayed zeroes for every
mob and written authored training: straight back, reintroducing the convention
the new gate test forbids.

Spec 13.6 ruled cutting the write path acceptable because writing Training sets
progression difficulty. The fold changes that: the editor now writes Base, which
is just a value, so the builder capability is kept rather than removed."
```

---

## Task A5: Phase gates

- [ ] **Step 1: Formatting** — `gofmt -l internal/ modules/` must print nothing.
- [ ] **Step 2: Build and full suite**

```bash
go build ./... && go test ./... 2>&1 | grep -Ev "^ok|no test files"
```

Expected: no output. The suite is documented as known-green, so any failure is
real. Two expected failures were already handled in-task (the GMCP editor test in
A4, and the template gate in A3 which should now pass).

- [ ] **Step 3: Confirm nothing progression-related moved**

This phase must not change any progression rate. Sanity-check by inspection:
no file under `internal/characters/progression.go` was touched, and
`grep -n "UsesPerRank\|StatProgressionSoftCap" _datafiles/config.yaml` shows the
pre-phase values.

- [ ] **Step 4: Boot test** (already run in A3 Step 6 — re-run if anything
  changed since) and **instance-save wipe** before any smoke test.
- [ ] **Step 5: Adversarial playtest gate.** Per the content SOP — this phase
  touches 599 content files and three player-visible systems.

```bash
/playtest local --checkout C:/Users/Calabe\ Davis/workspace/DOGMud bug-finder 2026-08-03-prepush-sweep.yaml
```

Drive specifically: `assess` on both a mob corpse and a **player** corpse (the
band change in A2 Step 5), `raise` on a corpse, and a charm attempt. Those are
the three systems `StatPoolTotal` protects, and the only player-visible surface
this phase can break. Extract findings to memory — reports are gitignored.

- [ ] **Step 6: Ship**

```bash
git push -u origin feature/u10b-0-phase-a-groundwork
gh pr create --repo pruuk/DOGMud --base master --head feature/u10b-0-phase-a-groundwork --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
```

`gh` defaults to the fork **parent** — `--repo pruuk/DOGMud` is mandatory on
every command. A green check is not proof: runs pass while emitting annotations
and the lint gate is `only-new-issues`.

Hand over, do not merge — the arc's no-deploy policy holds.

---

## Self-Review

**Spec coverage:** 14.1 (`StatPoolTotal`, five consumers) → A2, A4 · 13.6 (editor
write path) → A4, with the ruling's rationale superseded and the change recorded
· the authored-`training:` discovery has no spec section yet — **it should be
written into the spec as section 15 when this phase lands**, because sections
13.2 and 4 still assert the false premise.

**Not in this phase, deliberately:** every rank site, every config value, the
spawn-pool move, and the caps. Phase A cannot change a progression rate; that is
its safety property and its verification story.

**Known-weak points, stated rather than hidden:**
- Task A3's script is the highest-risk artefact in the phase. Its `--verify`
  pass checks its own arithmetic, which is necessary but not sufficient — the
  boot test and the in-game stat comparison in Step 7 are what actually prove
  the world did not move.
- The `training: 0` handling (delete rather than pin) preserves species
  propagation for those stats but means the fold is not uniform across a file.
  That asymmetry is deliberate; the CSV report makes it visible.
- Stats that *did* have authored training stop tracking future species
  rebalances, because their value is now explicit. Unavoidable — the number has
  to live somewhere — but worth knowing before someone rebalances a species and
  wonders why 200 mobs did not move.
