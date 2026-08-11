# Skill and Crit Rebalance — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Skill governs whether you connect and how often and how hard you crit; damage magnitude stays mostly stats, gear and spell choice.

**Architecture:** Six independently shippable slices. `SkillWeight` ships and plays **alone** first, so a combat-feel regression is attributable to one change. Crit then moves from a self-relative z-score to the normalized opposed-roll margin, gains floors, and gains a skill-scaled damage multiplier.

**Tech Stack:** Go, `internal/combat`, `internal/characters`, `internal/configs`, `_datafiles/config.yaml`.

**Spec:** `docs/superpowers/specs/2026-08-11-skill-and-crit-rebalance-design.md` — **read the Traps section before writing any code.**

**Branch:** `feature/5.11-skill-and-crit-rebalance`, created before Task 1.

---

## The six traps, restated

Every one is silent. They are why this has a spec.

| # | Trap |
|---|---|
| T1 | `best.margin` is **defence-positive** (`defenseRoll.Value - attackRoll.Value`). Attacker margin is its **negation**. An inverted sign compiles and puts crit on the losing side. |
| T2 | `best.margin` is `math.Inf(-1)` when **no defence was attempted**. Detect via `best.defenseType == ""`, never by testing the margin. |
| T3 | Both rolls use `atkStdDev`, so the normaliser is `atkStdDev * math.Sqrt2`, available as `best.hitRoll.StdDev`. |
| T4 | `forceCrit` mutates `best.hitRoll.ZScore`. Once crit stops reading that field it becomes a silent no-op. |
| T5 | The crit floor must run **after** the hit is final. An attack crit forces a hit, so an earlier floor becomes a hit-floor bypass leaking through `MinDefenseChance`. |
| T6 | Never floor the **margin**. Floor saves already carry margin ±1 per the 5.9 convention; flooring it corrupts every margin-scaled effect. |

**Fumbles stay on the self-relative z-score.** `attackFumble := best.hitRoll.ZScore <= fumbleThreshold` is deliberately unchanged. Fumbles have the same architectural quirk as crits did, but moving them is out of scope and would change failure rates nobody asked to change. Do not "fix" them while you are in there.

---

## File Structure

- Modify: `_datafiles/config.yaml` — `SkillWeight`; new crit knobs
- Modify: `internal/configs/config.balance.go` — new knob fields
- Modify: `internal/configs/config.balance.combat.go` — defaults + validation
- Modify: `internal/combat/combat_helpers.go` — `calcCritThreshold`, `calcAttackScore`, `resolveDefenseOutcome`, `calcHitDamage`
- Modify: `internal/characters/combat.go` — defence scores (grapple positional)
- Test: `internal/combat/*_test.go`
- Docs: `internal/combat/context.md`, `internal/dice/context.md`, `CLAUDE.md`, `docs/PATCH_NOTES.md`

---

### Task 1 (5.11b): `SkillWeight` 2.0 -> 5.0

**Files:** `_datafiles/config.yaml:673`

**`config.yaml` has `skip-worktree` set.** Staging it errors misleadingly about
sparse-checkout. Unset, stage, re-set:
`git update-index --no-skip-worktree _datafiles/config.yaml`, stage, then
`git update-index --skip-worktree _datafiles/config.yaml`.

- [ ] **Step 1: Change the value**

```yaml
  # SkillWeight: Global multiplier on skill contributions in additive formulas
  # (hit rolls, defense scores, spell attack, etc.). Higher = skills matter more.
  #
  # Raised 2.0 -> 5.0 by roadmap chunk 5.11b. Measured: prod Meirok (weapon-combat
  # 69, the highest on the server) landed exactly MinAttackHitChance against a
  # 325g Elemental King -- the 5.9 floor was the only thing keeping him in the
  # fight, because skill contributed +138 against the King's Dexterity of 417.
  #
  # NOTE this is asymmetric in practice: defence is also stat + skill*SkillWeight,
  # and every mob has combat skill 1 (no skills block; GetCombatSkillLevel floors
  # at 1). So it lifts players on offence AND defence while giving mobs +3.
  # Deliberate. See tools/balance/matrix.py.
  SkillWeight: 5.0
```

- [ ] **Step 2: Verify the knob is actually read from config, not a default**

Run: `grep -rn "SkillWeight" internal/configs/config.balance*.go`
Confirm the field exists and note its Go default. A knob absent from
`config.yaml` silently falls back to the default — here it is present, so the
5.0 takes effect.

- [ ] **Step 3: Build and test**

Run: `go build ./... && go test ./internal/combat/... ./internal/characters/...`
Expected: PASS. **If a test asserting a specific hit rate or power score fails,
do not weaken the assertion** — report it. A hardcoded expectation that moves
with `SkillWeight` is a finding about that test, and the correct fix is usually
to derive the expectation from the config value.

- [ ] **Step 4: Boot test**

Per the pre-push SOP, in an isolated detached worktree on non-default ports.
Copy the skip-worktree config in by hand. **Exit code 124 is success.** Do not
grep for the bare word `panic` — `GamePlay.MapConsistencyEnforce` has the
*value* `panic`.

- [ ] **Step 5: Commit**

```bash
git update-index --no-skip-worktree _datafiles/config.yaml
git add _datafiles/config.yaml
git commit -m "balance(combat): raise SkillWeight 2.0 -> 5.0 (chunk 5.11b)

Prod Meirok holds the highest combat skill on the server (weapon-combat 69) and
landed exactly MinAttackHitChance against a 325g Elemental King. The 5.9 floor
was the only thing keeping him in that fight: skill contributed +138 while the
King's Dexterity alone is 417.

Modelled four tunings on a full player x enemy matrix (tools/balance/matrix.py).
SW 5 gives a clean diagonal -- each tier handles its own content at 60-66% and
stays correctly hopeless above it. Nerfing mob stat pools instead flattens
everything toward 50/50 and would mean re-tuning every already-balanced instance
mob; doing both overcorrects into a stomp.

Asymmetric by construction: defence is also stat + skill*SkillWeight and every
mob has combat skill 1, so this lifts players on offence and defence while
giving mobs +3. Accepted deliberately.

No code change."
git update-index --skip-worktree _datafiles/config.yaml
```

- [ ] **Step 6: STOP — hand to the user for play**

This slice ships alone on purpose. The model predicts Meirok's total damage
output against the Elemental King rises **~5x** (3.95x hit rate x 1.25x damage
per hit once Task 5 lands; this task delivers the hit-rate half). Do not start
Task 2 until the user has played it.

---

### Task 2 (5.11c): Move grapple positional modifiers into the scores

**Files:** `internal/combat/combat_helpers.go` (`calcCritThreshold`, `calcAttackScore`), `internal/characters/combat.go`

Prone is already a score multiplier (`ProneAttackMultiplier`,
`ProneVulnerabilityMultiplier`). Grapple is a crit-threshold subtraction. Same
category of effect, two mechanisms. Unify on scores.

**This is not numerically behaviour-preserving.** A threshold shift and a score
shift are different transforms. Expect a delta and document it in the commit.

- [ ] **Step 1: Write failing tests first**

Assert that a grapple controller's **attack score** is higher than the same
character not controlling, and that a ground-grappled defender's **defence
score** is lower. Both must fail before the change.

- [ ] **Step 2: Remove the positional block from `calcCritThreshold`**

Delete the `IsController` / `IsGroundGrapple` / `IsStandingGrapple` block and
the target-ground-grapple `+= 0.4`. **Keep** the `Accuracy` and `Blink` buff
modifiers and both floors — buffs are "this character crits more readily",
which is what a threshold expresses. Positional effects are not.

- [ ] **Step 3: Add equivalent multipliers to the scores**

Add balance knobs mirroring the prone ones (`GrappleControllerAttackMultiplier`,
`GrappleGroundedDefenseMultiplier`, etc.) in `config.balance.go`, defaulted in
`config.balance.combat.go`, and apply them in `calcAttackScore` next to the
prone block and in the defense-score path in `characters/combat.go`.

**Identifier spelling:** the codebase uses American `Defense`
(`MinDefenseChance`, `defenseScore`, `DefenseDodge`). Match it in knob and
variable names regardless of the prose spelling in the spec.

- [ ] **Step 4: Run tests, then commit**

`go build ./... && go test ./internal/combat/... ./internal/characters/...`

---

### Task 3 (5.11d): Margin-derived crit

**Files:** `internal/combat/combat_helpers.go` (`resolveDefenseOutcome`)

The highest-risk task. Traps T1–T4 all live here.

- [ ] **Step 1: Write the failing tests FIRST — asymmetric, per T1**

```go
// A decisive ATTACKER win must crit. A decisive DEFENDER win must not.
// These MUST be asymmetric: a symmetric test passes under a sign inversion,
// which is exactly the T1 bug.
func TestMarginCrit_AttackerDominance_Crits(t *testing.T)   { /* atk >> def */ }
func TestMarginCrit_DefenderDominance_DoesNotCrit(t *testing.T) { /* def >> atk */ }

// T2: a defender with zero stamina attempts no defence, leaving best.margin at
// math.Inf(-1). That must NOT produce a guaranteed crit.
func TestMarginCrit_NoDefenceAttempted_DoesNotAlwaysCrit(t *testing.T)

// T3: at parity the crit rate must measure ~2.3% over a large sample. A missing
// sqrt(2) shifts this detectably.
func TestMarginCrit_ParityRateMatchesLegacy(t *testing.T)
```

Run them. All must FAIL.

- [ ] **Step 2: Replace the crit criterion**

In `resolveDefenseOutcome`, replace
`attackCrit := best.hitRoll.ZScore >= critThreshold` with a normalized-margin
computation:

- attacker margin is `-best.margin` (**T1**)
- normaliser is `best.hitRoll.StdDev * math.Sqrt2` (**T3**)
- guard `best.hitRoll.StdDev <= 0` to avoid a divide-by-zero
- when `best.defenseType == ""` (**T2**), **fall back to the existing
  self-relative z-score check**. Rationale: with no defence attempted there is
  no contest and therefore no margin to derive from, and this preserves current
  behaviour exactly on that path. Document it in a comment so the next reader
  does not "fix" it. Do **not** try to synthesise a margin from
  `math.Inf(-1)`.

Mirror it for `defenseCrit` using `+best.margin` against `defCritThreshold`.

**Threshold 2.0 is unchanged and is calibrated by construction:** at parity the
normalised margin is standard normal, so it reproduces today's 2.3%.

- [ ] **Step 3: Rework `forceCrit` (T4)**

The `best.hitRoll.ZScore = critThreshold + 0.5` hack no longer influences crit.
Replace it with an explicit boolean that short-circuits the crit determination,
and delete the ZScore mutation. Leaving it in place is worse than removing it —
it looks load-bearing and is not.

- [ ] **Step 4: Confirm all Step 1 tests now pass, and the rest of the suite too**

Run: `go test ./internal/combat/...`

- [ ] **Step 5: Commit**

Message must state the sign convention (T1) and the no-defence fallback (T2)
explicitly — those are the two things a future reader will get wrong.

---

### Task 4 (5.11e): Crit floors, 1% of hits, both directions

**Files:** `internal/configs/config.balance.go`, `config.balance.combat.go`, `internal/combat/combat_helpers.go`

- [ ] **Step 1: Add two knobs**

`MinAttackCritChance` and `MinDefenseCritChance`, both defaulting to `0.01`,
separately tunable, mirroring how 5.9 split its floors. Clamp to `[0, 0.5]`.

- [ ] **Step 2: Write the failing ordering test (T5) — the important one**

```go
// The crit floor must NEVER convert a miss into a hit. With the floor forced to
// 1.0, the measured HIT rate must be unchanged.
func TestCritFloor_NeverTurnsAMissIntoAHit(t *testing.T)
```

This is the test that catches the hit-floor-bypass coupling. It must fail
against an implementation that floors before hit resolution.

- [ ] **Step 3: Apply the floors after the hit outcome is final**

Only on swings that already hit, and only when not already a crit. Never mutate
`res.hit`. Do not floor the margin (**T6**).

- [ ] **Step 4: Test and commit**

---

### Task 5 (5.11f): Skill-scaled crit damage multiplier

**Files:** `internal/configs/config.balance.go`, `config.balance.combat.go`, `internal/combat/combat_helpers.go` (`calcHitDamage`)

- [ ] **Step 1: Add the knobs**

`CritDamageBase` default **2.0**, `CritDamagePerSkill` default **0.05**.

- [ ] **Step 2: Write the failing test**

```go
// Linear in skill: 2.05x at rank 1, 4.50x at rank 50, 5.45x at rank 69.
// Linear NOT sqrt is deliberate -- a sqrt curve puts rank 25 at 4.12x, 75% of
// rank 69's value for 36% of the skill, which is the chunk 5.7 complaint again.
func TestCritDamageMultiplier_LinearInSkill(t *testing.T)

// The multiplier stacks ON TOP of the existing mitigation bypass.
func TestCritDamage_StacksOnMitigationBypass(t *testing.T)
```

- [ ] **Step 3: Apply it in `calcHitDamage`**

The crit branch currently rolls `sdp.rawDmgForCrit` (unmitigated). Multiply that
by `CritDamageBase + CritDamagePerSkill*skillRank`. **Keep the bypass** — it is
retained deliberately, because defence is best-of-all so a skill difference
already bites harder on the defensive side, and the bypass is what lets a crit
answer heavy armour.

- [ ] **Step 4: Test and commit**

---

### Task 6 (5.11g): Docs

- [ ] **Step 1: `internal/combat/context.md`** — crit is margin-derived; the T1
  sign convention; the T2 no-defence fallback; fumbles deliberately still
  z-score-based; positional effects live in scores, buffs in the threshold.
- [ ] **Step 2: `internal/dice/context.md`** — note that `RollResult.Margin` now
  feeds crit determination, so the 5.9 bare-success convention is load-bearing
  for more than effect scaling.
- [ ] **Step 3: `CLAUDE.md`** — update the combat sections. The
  "Z-score thresholds: ZScore >= 2.0 = crit" line is now **wrong for attacks**
  and must say margin-derived.
- [ ] **Step 4: `docs/PATCH_NOTES.md`** — dated, player-facing, no raw numbers,
  no em dashes. Skilled characters now land more blows and their critical hits
  hit far harder.
- [ ] **Step 5: Commit**

---

### Task 7: Adversarial playtest gate

**Required by CLAUDE.md before handoff. Not optional.**

- [ ] **Step 1: Harness sweep**

```text
/playtest local --checkout C:/Users/Calabe Davis/workspace/DOGMud bug-finder 2026-08-03-prepush-sweep.yaml
```

The goals file must already carry `ephemeral:` — `2026-08-03-prepush-sweep.yaml`
does. Local runs always start a disposable Docker checkout via `playtestrun` and
do **not** read endpoint or credentials from `targets.yaml`.

- [ ] **Step 2: Report to the user for manual play**

Specifically ask them to check: does a crit *feel* like the skill payoff; is
combat narration readable when nearly every swing crits against weak content;
and does the Elemental King fight land at "hard" rather than "trivial" — the
model predicts ~5x total damage output versus today.

- [ ] **Step 3: Fix what it finds, re-run, then hand over**

---

### Task 8: Ship

- [ ] **Step 1: Full verification**

```bash
gofmt -l internal/ modules/    # want: no output
go build ./...
go test ./...                  # internal/relationships fails locally on Defender; CI runs it
```

- [ ] **Step 2: Roadmap + PR**

Mark 5.11b-g Done. Then:

```bash
git push -u origin feature/5.11-skill-and-crit-rebalance
gh pr create --repo pruuk/DOGMud --base master --head feature/5.11-skill-and-crit-rebalance --fill
```

**`gh` defaults to the fork PARENT — every command needs `--repo pruuk/DOGMud`.**

**Do not stage `_datafiles/world/dogmud/rooms/thornwall_city/473.yaml`** or any
untracked file. The user is testing admin builder saves. Stage named paths only,
never `git add -A`.

---

## Notes for the implementer

- Every new test must be **mutation-verified** — confirmed to fail against code
  lacking the fix. A test that has never failed is not known to work.
- Tasks 1 and 2 change combat feel. If an existing test's expectation must move,
  say so rather than quietly adjusting it — a hardcoded expectation that tracks
  `SkillWeight` is itself a finding.
- Prefer codegraph MCP over grep for symbol verification; counts and line
  numbers here were taken 2026-08-11.
