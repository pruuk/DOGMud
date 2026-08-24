# U10b-0 Phase F — Docs and ship (the last phase)

**Index:** `docs/superpowers/plans/2026-08-21-u10b-0-README.md`
**Spec:** `docs/superpowers/specs/2026-08-21-u10b-0-progression-rank-from-training-design.md`

**Predecessors:** A `1c5d10fd7`, B, C, D `4db7766f7`, E `1f983e779` — all merged.
Master is at `193f59c63`.

This phase writes no new behaviour. It makes the written record match the code,
exercises the one migration that has never run outside a unit test, and ships
the arc.

---

## Task 1 comes first, and it is destructive-order-sensitive

**The smoke-test SOP's `rm -rf mobs.instances/*` destroys exactly the saves
Phase C's migration exists to handle.** There are, right now, **265 legacy mob
instance saves** carrying `training:` with **zero** `schema_version:` markers,
plus **four user saves with companions** (`3`, `5`, `10`, `12`). Once wiped they
cannot be recreated — a fresh spawn writes a v1 save. Exercise the migration
against real legacy data **before any wipe, in this phase, as the first task.**

The migration has unit tests (`internal/mobs/migration_legacy_training_test.go`)
and they pass. What has never happened is the real load path running over real
files.

---

## Standing rules (unchanged, all seven still apply)

1. No absolute line numbers for code an earlier task shifts. Locate with `grep`
   at execution time and verify the grep matched the symbol you meant.
2. Go defaults move with shipped values. A test binary never loads `config.yaml`.
3. Safety defaults use `<= 0`; only genuine off-switches use `< 0`.
4. `characters.New()` calls `Validate()`. A disk-loaded fixture must be a raw
   `&characters.Character{}`.
5. Migrations must never assign `Base` without accounting for species hydration
   and for whether a fresh pool has already been rolled.
6. A soft cap bounds nothing.
7. `grep --include=*.go` cannot see `config.yaml`, `AGENTS.md`, or the 641 mob
   files.

And one this phase adds:

8. **`_datafiles/config.yaml` has `skip-worktree` set.** Build any commit that
   touches it from the `git show HEAD:` blob, never from disk — disk legitimately
   carries dev overrides. This has been caught twice in this arc already.

---

## Facts verified against master `193f59c63` before writing this plan

| Claim | Status |
|---|---|
| Legacy mob instance saves present locally | **265** carry `training:`, **0** carry `schema_version:`. One-shot. |
| User saves with companions | 4: `users/3.yaml`, `5`, `10`, `12`. |
| Migration entry point | `mobs.LegacyTrainingToGains(mobId, saved)`, `internal/mobs/migration_legacy_training.go`. `InstanceSchemaVersion = 1`. |
| Companion call site | `internal/hooks/PlayerSpawn_HandleJoin.go:123` — gated on `comp.SchemaVersion < mobs.InstanceSchemaVersion`. |
| Mob-instance call site | `internal/mobs/instance_save.go` — `SchemaVersion` field at `:27`, written at `:113`. |
| `MobStatCap` / `MobSkillCap` | Still exist as **legacy config knobs** (`config.balance.go:406,412`), superseded by the `*TrainingCap` pair and still validated. CLAUDE.md describes them as the live enforcement, which is stale. |
| Live cap enforcement | `MobSkillTrainingCap` at `progression.go:124` (inside `ProgressionChanceForSkill`), `MobStatTrainingCap` at `:241` (inside `ProgressionChanceForStat`) **and again at `:525`**. CLAUDE.md's "one per function, both gated on `c.IsMob`" is wrong on count and on location. |
| `CalculateProgressionChance` | `progression.go:85`. CLAUDE.md cites `:44-62`. Stale. |
| `Balance.UsesPerRank` readers | **Zero compute with it after Phase E.** Survivors: `internal/configs/smoke_test.go:92` asserting `> 0`, and a stale explanatory comment at `internal/usercommands/go.go:46`. |
| `internal/characters/context.md`, `internal/web/context.md` | Already updated by Phase E. Do not redo. |
| `internal/mobs/context.md` | **Not yet updated for the pool move or the schema version.** Verify and fix. |

---

## Task 1: Exercise the legacy migration on real data, before any wipe

**Do this before Task 6 touches `mobs.instances/`.** Nothing here modifies the
user's live data.

- [ ] **Step 1: Snapshot the legacy saves outside the repo.** They are the only
      copies and the SOP wipe is coming.

```bash
mkdir -p "$SCRATCH/u10b-legacy-saves"
cp -r _datafiles/world/dogmud/mobs.instances "$SCRATCH/u10b-legacy-saves/"
cp _datafiles/world/dogmud/users/3.yaml _datafiles/world/dogmud/users/5.yaml \
   _datafiles/world/dogmud/users/10.yaml _datafiles/world/dogmud/users/12.yaml \
   "$SCRATCH/u10b-legacy-saves/"
find "$SCRATCH/u10b-legacy-saves" -name '*.yaml' | wc -l    # expect 270
```

- [ ] **Step 2: Record the before-state.** For a sample of legacy saves, capture
      mob id, room, and the six saved `training:` values. Pick at least one mob
      whose saved training is clearly large (real gains) and one that looks
      untouched, so both migration branches are covered — `gainsTotal <= 0` must
      land on **exactly zero**, not a small positive remainder.

- [ ] **Step 3: Boot an isolated detached worktree with the legacy saves in
      place** and let the real load path run. Non-default ports, fixed
      `boot-check.exe` path, per the pre-push SOP. `mobs.instances/` is
      gitignored, so it must be **copied in by hand** — a fresh worktree has
      none, and the migration would then be exercised against nothing while
      appearing to succeed.

- [ ] **Step 4: Confirm the migration actually ran and is idempotent.** After the
      boot, the re-written saves should carry `schema_version: 1` and gains-only
      `training:`. Then **boot a second time on the already-migrated files** and
      confirm the values do not move. The whole reason `InstanceSchemaVersion`
      exists is that a second run would subtract the pool again; prove it does
      not.

- [ ] **Step 5: Companions across a logout/login boundary.** This is the branch
      most likely to hide the `Base`-assignment class of bug, because companions
      respawn via `NewMobByIdFresh`, which rolls a fresh pool that the saved
      value would then be added on top of. A pre-slice companion must come back
      with **unchanged stat values, unchanged HP, and a training figure of 0**.

- [ ] **Step 6: Write down what you measured** in the plan or the commit body,
      with real numbers. "The migration ran" is not evidence; before-and-after
      values for named mobs are.

If any of this fails, **stop and fix before wiping anything.** The legacy saves
are the only test data that will ever exist for this code path.

---

## Task 2: `CLAUDE.md`

Three claims in the Stat & Progression System section are now false. Fix
exactly these; do not rewrite the section.

- [ ] **`UsesPerRank` no longer drives the display either.** The text says it is
      "retained for that display and drives nothing". After Phase E, nothing
      reads it at all. Say so, and point at the surviving smoke assertion so the
      next reader is not surprised to find one.

- [ ] **The curve's line reference.** `CalculateProgressionChance` is at
      `progression.go:85`, not `:44-62`. Prefer naming the function over citing
      a line range that drifts every phase.

- [ ] **The mob caps paragraph is wrong on name, count and location.** It claims
      "`MobStatCap` in `CheckStatProgression` (`progression.go:157`) and
      `MobSkillCap` in `CheckSkillProgression` (`progression.go:77`), one per
      function". The live guards are `MobStatTrainingCap` and
      `MobSkillTrainingCap`, they live in the two **chance** functions rather
      than the `Check*` functions, and `MobStatTrainingCap` is checked at **two**
      sites. `MobStatCap` / `MobSkillCap` still exist as legacy config knobs and
      are still validated, which is exactly why the stale text reads as
      plausible. Keep the true part: **players have no hard ceiling.**

- [ ] **Add the rule Phase E paid for.** The chance expression lives in exactly
      two functions, `Character.ProgressionChanceForStat` and
      `ProgressionChanceForSkill`. Production rolls them, the tests pin them, the
      dashboard displays them. Never recompute it anywhere: the dashboard did,
      silently dropped `StatProgressionRate` and every per-skill multiplier, and
      that is why a truncation bug which sealed two of a live character's stats
      read as a tuning problem for months. Mention `ProgressionRollThreshold` as
      the matching seam for the roll threshold, and that the denominator stays
      unexported deliberately.

---

## Task 3: `AGENTS.md`

Its progression bullets are already accurate apart from the same `UsesPerRank`
wording. Fix that, and add the never-recompute rule in one sentence — that file
is deliberately terse, so do not port CLAUDE.md's paragraph wholesale.

---

## Task 4: `internal/mobs/context.md`

**Verify every symbol before writing.** Phase E's `context.md` work is done;
this is the one file the arc still owes.

```powershell
Select-String -Path internal\mobs\*.go -Pattern '^(func|type|const|var)\s'
```

- [ ] The spawn stat pool lands in **`Base`**, not `Training`. `Training` is
      gains-only for mobs, exactly as it is for players.
- [ ] `InstanceSchemaVersion` and what a missing `schema_version:` means.
- [ ] `LegacyTrainingToGains` — its contract, and the two things it deliberately
      refuses to do: guess a per-stat pool split evenly (it distributes
      proportionally to preserve archetype shape) and migrate at all when the
      template or species record is missing (it returns the input unchanged,
      because refusing beats corrupting).
- [ ] `MobStatTrainingCap` / `MobSkillTrainingCap` supersede `MobStatCap` /
      `MobSkillCap`, and the caps are on **gains**, not on value — a mob authored
      at base 250 is no longer harder to train than one at 180.

Do not add "Future Enhancements" or "Performance Characteristics" sections.

---

## Task 5: The two inherited cleanups

- [ ] **`Balance.UsesPerRank`.** Nothing computes with it. Recommendation: keep
      the knob and the smoke assertion, and make both say plainly that it is
      retained only so an existing `config.yaml` does not fail validation.
      Removing it is a config-schema change for zero benefit and would break any
      deployed `config.yaml` that still lists it. Also fix the stale comment at
      `internal/usercommands/go.go:46`, which still explains the retired
      `virtualRank = useCount / UsesPerRank` model as if it were live. **Flag the
      keep-or-remove choice to the owner rather than deciding it silently.**

- [ ] **`config.yaml`'s vitality comment contradicts its own value.** The block
      above `vitality: 2.2` still reads "NOT retuned by Phase D ... Known gap: it
      reads ~52%/use and needs its own slice", directly above an inline comment
      saying the vitality slice solved it against the damage-taken faucet. Delete
      the stale block. **Build this commit from the `git show HEAD:` blob**, per
      standing rule 8.

- [ ] **Confirm all six keys are present in `config.yaml`**, since absence falls
      back to a Go default and is meaningful: `StatProgressionSoftCap`,
      `ProgressionChanceFloor`, `MobStatTrainingCap`, `MobSkillSoftCap`,
      `MobSkillTrainingCap`, `ObservedCritProgressionBonus`. Check the **`git
      show HEAD:` blob**, not disk — a key present only on disk is not shipped.

---

## Task 6: Pre-push gates

Only now is the instance-save wipe safe.

- [ ] `gofmt -l internal/ modules/` prints nothing.
- [ ] `go build ./...`, then `go test ./...` for the arc's packages at minimum:
      `internal/characters`, `internal/mobs`, `internal/web`, `internal/configs`,
      `internal/hooks`, `internal/skills`.
- [ ] `golangci-lint run` on touched packages; confirm no **new** finding. The
      gate is `only-new-issues`, and the repo carries pre-existing findings that
      are not this phase's to fix.
- [ ] Instance-save wipe **after Task 1 is complete and its evidence recorded**,
      then a clean boot in an isolated detached worktree on non-default ports.
      `Server Ready` = 1, panic-pattern count = 0. **Exit 124 is the success
      case.** Never grep the bare word `panic` — `MapConsistencyEnforce`
      legitimately has the *value* `panic`.
- [ ] `docs/PATCH_NOTES.md` — the arc's player-facing entry. The 08-22 entry
      already covers the pace retune and the toughness faucet, and 08-24 covers
      the dashboard. Judge whether F needs its own line or whether the arc is
      already described; **do not pad the notes with a duplicate.**

---

## Task 7: The adversarial playtest gate

Per the content SOP. The arc changed how every character in the game improves,
so a boot-clean check proves nothing about whether it *feels* right.

```text
/playtest local --checkout <abs> bug-finder <goals>.yaml
```

Drive specifically, beyond the personality's own agenda:

- **A companion across a logout/login boundary** — the migration branch most
  likely to hide a `Base`-assignment bug.
- **`assess` on a corpse, and `raise`** — the two other systems `StatPoolTotal`
  protects, and where Phase A's own accepted change produced a real defect.
- **Enough combat to trip a progression banner**, and confirm the banner text
  reads correctly. Grep for `SKILL ADVANCEMENT` and `STATISTIC INCREASED` —
  **not** "SKILL INCREASED", which silently matches nothing.

Fix what it finds, re-run if needed, and only then hand over.

---

## Task 8: Ship

- [ ] Push, `gh pr create --repo pruuk/DOGMud --base master --head
      feature/u10b-0-phase-f-ship --fill`. **`gh` defaults to the fork parent;
      `--repo pruuk/DOGMud` is mandatory on every command that can target a
      repo.**
- [ ] Watch the checks. A green check is not proof on its own — confirm each job
      actually executed and carries zero annotations.
- [ ] Merge with `--merge`, not `--squash`. Delete the branch. Remove a stray
      `refs/tags/master` if it re-seeds on origin.
- [ ] Update the phase index README to mark the arc complete.

**Merging is not deploying.** The arc's standing no-deploy policy holds until the
whole arc is playtested, including the mandatory owner run: fighting the
Elemental Queen with Meirok, per `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`. The
crib-sheet pass is an **end-of-arc pre-deploy gate**, and this is the end of the
arc — so it is now due.

---

## Open questions for the owner

1. **Expected Rank saturation** (from Phase E). Against real data, five of six
   stats pin at the soft cap while trained points sit at 12 to 35, so all six
   read as behind permanently. Fixing it properly needs a uses-since-last-gain
   counter, which is a save-schema change. Currently documented on the panel
   rather than fixed. In scope, or a separate slice?
2. **`UsesPerRank`**: keep as a documented-dead knob, or remove it and the smoke
      assertion together?
3. **The vitality slice is still unplaytested.** It needs a profile that can take
   real hits *and* reach summons and a merchant, which `mid.yaml` cannot. Does
   the Task 7 run cover it, or does it need its own?
