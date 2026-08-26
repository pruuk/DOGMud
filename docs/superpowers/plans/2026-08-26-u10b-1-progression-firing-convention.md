# U10b-1 Progression Firing Convention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Progression resolves **Best-of**, like the defensive rolls. One resolved action produces one event, for the single highest-rolling candidate skill. Full on success, partial on failure. Crits and critical failures stay a separate channel. Progression follows the final outcome, floors included.

**Architecture:** FOUR seams, built in Phase A, so every later task is a one-line call. Nothing awards progression by hand after Phase A.

| Seam | Serves |
|---|---|
| `progression.Candidate` + `progression.BestOf` | The selection itself: one event, highest roll, tiebreak on skill level |
| `Character.AwardResolved(userId, won, candidates...)` | Every resolved action: search, track, forage, craft, salvage, skullduggery, concentration |
| `Actor.AwardResolved(won, candidates...)` | The `internal/actions` half, hiding the user/mob userId split as `OnSkillUse` does today |
| `combat.AwardDefenceProgression(c, userId, defenceType, multiplier)` | BOTH defence paths, melee and channel |

**Spec:** `docs/superpowers/specs/2026-08-26-u10b-1-progression-firing-convention-design.md`

---

## Read this before Task 1

Three plan versions failed blind adversarial review. These killed them.

1. **The seam is not the feature.** V1 converted fifteen roll sites and applied the rule to none. V2 added `Outcome.Defended` and never set it, left a defended melee swing awarding nothing, and never made salvage award. Phase B is the deliverable.
2. **`contest.Result.Success` means the ATTACKER won.** `!res.Success` is NOT "the defender won": under `side.ForceCrit` the attack wins with `Success == false`. A mirrored test fake passes either way.
3. **`contest.AgainstDifficulty` and `contest.Run` apply NO floor.** `combat.RunContest` is the only reader of `ContestFloor` and its doc comment forbids routing static-difficulty rolls there. Craft and salvage call `contest.RunWithFloors`.
4. **Both are on the root guard's watch list.** Every conversion adds `guardedRollExemptions` (file-keyed, **repo-root** `contest_floor_guard_test.go`, `package main`) and `contestSiteOwners` (`file:func`-keyed, `internal/combat/contest_site_guard_test.go`) entries **in the same step**.
5. **Go has no cross-package test helpers.** Task 1 creates them per package.
6. **A test binary sees a ZERO-VALUED config**, so `UseSkillProgression` is false and nothing progresses. `configs.SetConfigForTest` takes **two** arguments and does not validate.

### Facts verified against source; do not re-derive or contradict

| | |
|---|---|
| `Balance.SkillWeight` | **Already exists**, shipped **5.0**. Craft score uses it. Do not add a craft-specific weight. |
| `ApplyProgression` | `(events []progression.Event, side progression.Side, userId int, round uint64)` |
| `util.GetRoundCount()` | returns `uint64`, matching that parameter |
| `SetConfigForTest` | `(t *testing.T, c Config)`; `GetConfig() Config` |
| `Config` shape | `GamePlay` and `Balance` are BOTH top-level fields. There is no `Config.GamePlay.Balance`. |
| `contest.Result.Margin` | **stat-scale**, `AttackRoll.Value - DefenseRoll.Value`, attack-positive. NOT normalised. |
| `runBestOfAllDefense` | sets `best.defenseType = res.Winner` whenever `res.Contested`, so a quoted defence **exists on a loss** |
| `DefenseUsed` | stamped only inside `sendDefenseMessages`, i.e. only on a **won** defence |
| `DefenceSkillAndStat` | dodge→**unarmed-combat**/dex, parry→**weapon-combat**/dex, block→**weapon-combat**/str, quell→spellcasting/wil, defy→rhetoric/wil |
| `AwardDefenceProgression` | 2 production callers: `defence_multiplier.go:426`, `NewRound_DoCombat_helpers.go:46` |
| Crafting types | `RecipeSpec`; inventories are `[]items.Item`. There is no `Inventory` type. |
| `events.SkillUsed` | `{UserId int, Skill skills.SkillTag, Details string}`. `Skill` needs the conversion. |
| `search.go` | six `dice.RollStat(searchScore)`: 125, 125, 135, **135 hidden players**, **135 hidden mobs**, 175. One award at :243 gated on `rolledAgainstSomething`. |
| `track.go` | ONE roll at :124, compared at :131 (125), :159 (175), and **:253/:292 (175) and :256/:295 (135)** via a `rollValue` parameter. **Six comparisons, three thresholds.** `result.RollValue` is set at :125 and **never read**. |
| `salvage.go` | awards at :166 and :252; reads `SalvageMin/MaxChance` at :83-84 |
| `mobs/crafter.go` | awards at :505 and :546. The **legacy** path DOES gate on `SkillMinimum` (`pickEligibleRecipe`); the **shop** path (`EvaluateCraftOptions`) does not. |
| `go.go` | `if !isSneaking {` at :664; pursuit loop :668-697; `TryRoomBehavior` at :700 **and** an unrelated one at :357 |
| `CleanHit` gate | `NewRound_DoCombat_unified.go:666-667` |
| `OnCritReceived` | has **zero** production callers already; this slice neither deletes nor revives it. Leave `TestCritReceivedProgression_DecaysWithRank` alone. |

### Task 0: `pinConfigForTest`

`ProgressionChanceForStat` multiplies by `StatProgressionRate`, which is **0** in
a test binary, so an unpinned test progresses nothing and the plan's own
non-zero guards fire on correct code.

```go
func pinConfigForTest(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.UseSkillProgression = true
	cfg.Balance.ProgressionFailureFraction = 0.35
	cfg.Balance.BaseProgressionChance = 0.30
	cfg.Balance.StatProgressionRate = 2.25
	cfg.Balance.StatProgressionSoftCap = 50
	cfg.Balance.ProgressionDecayBelowCap = 3.0
	cfg.Balance.ProgressionDecayAboveCap = 2.0
	cfg.Balance.SkillWeight = 5.0
	configs.SetConfigForTest(t, cfg)
}
```

⚠️ **Task 1 creates this**, in every package that needs it. Add whatever extra
knobs a given test reads; a missing knob is zero, not a default.

---

# Phase A: the seams

## Task 1: Test helpers, in every package that needs them

**Files:** create `testsupport_test.go` in `internal/progression`, `internal/actions`, `internal/forager`, `internal/hooks`, `internal/usercommands`, `internal/crafting`, `internal/configs`, `internal/items`, **`internal/characters`**, **`internal/combat`**, **`internal/mobs`**; create `internal/progression/census_test.go`.

⚠️ Tasks 4, 5, 7 write tests in `internal/characters` and Task 9 in
`internal/combat`; neither was on an earlier draft's list, so `pinConfigForTest`
was undefined at Task 4. `internal/combat` already has `repoRootForTest`
(`coup_de_grace_test.go:125`) so give it **only** `pinConfigForTest`.

- [ ] **Step 1: Write both helpers per package**

`repoRootForTest` anchors on `runtime.Caller(0)` and returns
`filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))`, because a
test binary does not reliably start in the package directory. `pinConfigForTest`
is Task 0's block verbatim.

⚠️ Match each package's existing `_test.go` package clause. `internal/progression`
has both: `event_test.go` is `package progression`, `seam_guard_test.go` is
`package progression_test`. Use `package progression_test`.

- [ ] **Step 2: Write the census test**

Walk `internal` and `modules` for non-test `.go`, count `OnSkillUseScaled(`,
`OnSkillUse(`, `OnStatUse(`, `OnStatUseScaled(`, `CheckSkillProgression(`,
`CheckStatProgression(`, `OnCritReceived(`, `TrackSkillUse(`, `TrackStatUse(`,
`OnRegenTick(`, `CheckRegenProgression(`, `AwardResolved(`, skipping comment
lines, assert against a pinned constant.

- [ ] **Step 3: Run, learn the number, set it, re-run**

⚠️ **The count moves in Tasks 4, 5, 7, 8, 9, 14, 15, 18, 19, 20, 21, 23 and 24.**
Each says so. A red census caused by your own previous task is expected.

- [ ] **Step 4: Verify all eleven packages compile, commit**

---

## Task 2: An AST recogniser for chained calls

Task 27's guard must see `x.Character.OnStatUse(...)`.
`internal/combat/contest_site_guard_test.go`'s walker asserts `v.X` is an
`*ast.Ident` and bails, so it sees only package-qualified calls.

⚠️ **Build it in `internal/progression`**, where Task 27's guard lives. Leave the
combat walker alone: matching on tail name alone there would classify every
`foo.Run(...)` as a contest site.

- [ ] **Step 1: Write the failing test** for `callNamesInSource`, asserting it sees `contest.AgainstDifficulty`, `v.Character.OnStatUse` and `c.OnSkillUse`
- [ ] **Step 2: Run to verify it fails**
- [ ] **Step 3: Implement**: walk `*ast.CallExpr`, take `sel.Sel.Name` from any `*ast.SelectorExpr` callee
- [ ] **Step 4: Run, commit**

⚠️ Declare the file `package progression_test` so Task 27 can use it.

---

## Task 3: The config knobs

⚠️ **Five knobs, not one.** `CraftFloor`, `CraftBaseDifficulty`,
`CraftSkillMinWeight` and `SalvageFloor` do not exist anywhere in
`internal/configs` today, and an earlier draft used all four without creating
any of them.

**Files:** `internal/configs/config.balance.go`, `config.balance.progression.go`, `config.balance.shops.go`, `config.balance.misc.go`, `configs.go`, `_datafiles/config.yaml`, create `internal/configs/u10b1_knobs_test.go`

| Knob | Default | Defaulted in |
|---|---|---|
| `ProgressionFailureFraction` | 0.35 | `config.balance.progression.go` |
| `CraftBaseDifficulty` | 100 | `config.balance.shops.go` |
| `CraftSkillMinWeight` | 5 | `config.balance.shops.go` |
| `CraftFloor` | 0.05 | `config.balance.shops.go` |
| `SalvageFloor` | 0.15 | `config.balance.misc.go` |

- [ ] **Step 1: Write the failing tests**

For `ProgressionFailureFraction` and both floors, **0 is a legal value** (an
explicit off-switch), so an absent key is indistinguishable from a deliberate
zero. Each needs a **pre-unmarshal `-1` sentinel**, seeded at
`tmpConfigData := Config{}` in `configs.go`, plus a test that the seed line is
actually present in production:

```go
if !strings.Contains(string(b), "ProgressionFailureFraction = -1") {
	t.Fatal("ReloadConfig does not seed the sentinel before unmarshal, so an " +
		"absent key is indistinguishable from an explicit 0")
}
```

⚠️ The assertion is the **assignment** form `= -1`, not the struct-literal `: -1`.
⚠️ The seed path is `tmpConfigData.Balance.X`, not `.GamePlay.Balance.X`.

- [ ] **Step 2: Run to verify they fail, then declare, seed, default, add to yaml**

⚠️ `_datafiles/config.yaml` has `skip-worktree`. Build the committed version from
the `git show HEAD:` blob, never `git add` it from disk.

- [ ] **Step 3: Run, commit**

---

## Task 4: `OnStatUseScaled`

**Files:** `internal/characters/progression.go`, create `progression_scaled_test.go`

- [ ] **Step 1: Write the failing test**: over 600 trials, multiplier 0.0 must not advance training and multiplier 1.0 must (both assertions; the first alone passes against an unpinned config that progresses nothing)
- [ ] **Step 2: Run to verify it fails, then implement**

`OnStatUseScaled(statName, userId, multiplier)` tracks the use and calls
`CheckStatProgression(statName, userId, multiplier)`; `OnStatUse` becomes it at
1.0. Keep the existing `mudlog.Debug("Progression", "event", "stat_use", ...)`
line, which is the only stat-progression trace in the log.

- [ ] **Step 3: Scope the `OnSkillUseScaled` change to LOSSES only**

⚠️ Replacing its trailing `OnStatUse(primaryStat, ...)` with an unconditionally
scaled call halves stat gain on already-scaled paths:
`SelfCastProgressionMultiplier` is 0.5 shipped, so a self-buff caster's willpower
roll would drop to 0.5 with no ruling behind it. Scale the stat by the
**loss fraction only when the event is a loss** (Task 5 threads that flag), and
otherwise pass the caller's multiplier through unchanged.

- [ ] **Step 4: Run, update the census, commit**

⚠️ Bonus events do **not** reach here (`ApplyProgression` routes
`Class.IsBonus()` to `applyBonusProgression`), and `actGrantProgression` calls
`CheckSkillProgression` directly with 1000.0. An earlier draft warned of a 1000x
stat roll and a crit double-dip; **both were false.**

---

## Task 5: Thread `Lost`; scale mutation drift; gate the quest event

⚠️ An earlier draft wrote `if !isLoss { ... }` inside `OnSkillUseScaled` with no
plumbing to deliver `isLoss`. It needs a real path: `Event` gains the flag,
`OrdinaryEventsScaled` sets it, `ApplyProgression` passes it down.

**Files:** `internal/progression/event.go`, `internal/characters/progression.go`, create `progression_side_effects_test.go`

- [ ] **Step 1: Add `Lost bool` to `progression.Event`**, set by `OrdinaryEventsScaled` on the losing side
- [ ] **Step 2: Widen `OnSkillUseScaled` to take `isLoss bool`**

Enumerate and update every caller first:

Run: `grep -rn "OnSkillUseScaled(" internal/ modules/ --include=*.go | grep -v _test.go`

- [ ] **Step 3: Write the failing tests**: a 0.35 award grants strictly less cluster affinity than a 1.0 award (with a non-zero guard on the 1.0 case), and a LOSS emits no `SkillUsed`
- [ ] **Step 4: Scale the drift**

```go
	amt := float64(configs.GetBalanceConfig().MutationAffinityPerSkillUse) * bonusMultiplier
```

- [ ] **Step 5: Gate the quest event on the LOSS flag**

```go
	// Quest skill_use counters are integers, so a fractional award cannot pay a
	// fractional tick. A resolved LOSS does not advance "use this skill N
	// times", or such a quest becomes "fail at it N times".
	if !isLoss {
		events.AddToQueue(events.SkillUsed{UserId: userId, Skill: skills.SkillTag(skillName)})
	}
```

⚠️ Gate on the **loss**, never on `multiplier < 1.0`.
`SelfCastProgressionMultiplier` 0.5 would otherwise silently stop self-buff casts
ticking `skill_use`.

- [ ] **Step 6: Run, update the census, commit**

---

## Task 6: `Candidate`, `BestOf`, `Outcome.Defended`, `OrdinaryEventsScaled`

This is the rule itself.

**Files:** `internal/progression/event.go`, create `internal/progression/bestof_test.go` and `event_scaled_test.go` (both `package progression_test`)

- [ ] **Step 1: Write the failing tests**

```go
// TestBestOfPicksHighestRoll pins the rule: ONE event per resolved action, for
// the single highest-rolling candidate.
func TestBestOfPicksHighestRoll(t *testing.T) {
	got, ok := progression.BestOf([]progression.Candidate{
		{Skill: "weapon-combat", Roll: 120, Level: 40},
		{Skill: "skullduggery", Roll: 155, Level: 10},
	})
	if !ok || got.Skill != "skullduggery" {
		t.Fatalf("BestOf picked %+v, want skullduggery (higher roll)", got)
	}
}

// TestBestOfTiebreaksOnSkillLevel pins the tiebreak.
func TestBestOfTiebreaksOnSkillLevel(t *testing.T) {
	got, ok := progression.BestOf([]progression.Candidate{
		{Skill: "weapon-combat", Roll: 140, Level: 40},
		{Skill: "skullduggery", Roll: 140, Level: 10},
	})
	if !ok || got.Skill != "weapon-combat" {
		t.Fatalf("BestOf picked %+v, want weapon-combat (equal roll, higher level)", got)
	}
}

// TestBestOfIsDeterministicOnAFullTie pins that a total tie resolves by slice
// order, so the same inputs always produce the same award.
func TestBestOfIsDeterministicOnAFullTie(t *testing.T) {
	cands := []progression.Candidate{
		{Skill: "weapon-combat", Roll: 140, Level: 10},
		{Skill: "skullduggery", Roll: 140, Level: 10},
	}
	first, _ := progression.BestOf(cands)
	for i := 0; i < 20; i++ {
		got, _ := progression.BestOf(cands)
		if got.Skill != first.Skill {
			t.Fatalf("tie resolved to %q then %q; must be deterministic",
				first.Skill, got.Skill)
		}
	}
}

// TestBestOfEmptyAwardsNothing pins that no candidates means no event, never an
// empty Skill: CheckSkillProgression("") takes a roll and banners no skill.
func TestBestOfEmptyAwardsNothing(t *testing.T) {
	if _, ok := progression.BestOf(nil); ok {
		t.Fatal("BestOf(nil) reported a winner")
	}
}
```

Plus the two `OrdinaryEventsScaled` tests: attacker scaled when `Defended`, and
the mirror when not. **Both are needed**; the first alone passes against a stub
returning `OrdinaryEvents` unchanged, since that hardcodes 1.0 on the defender.

- [ ] **Step 2: Run to verify they fail, then implement**

```go
// Candidate is one skill that could earn a resolved action's progression event.
type Candidate struct {
	Skill string  // skill tag; empty candidates are ignored
	Stat  string  // optional override; empty means the skill's primary stat
	Roll  float64 // the roll this candidate made
	Level int     // skill level, the tiebreak
}

// BestOf picks the single Candidate that earns the event, exactly as the
// defensive rolls pick a single defence.
//
// Highest Roll wins; ties break on highest Level; a full tie breaks on slice
// order, so callers MUST keep that order fixed for the award to be
// reproducible. Reports false when there is nothing to award, which callers
// must honour: an empty Skill is not inert, since CheckSkillProgression("")
// takes a roll and a success banners no skill at all.
func BestOf(cands []Candidate) (Candidate, bool)
```

Add `Defended bool` to `Outcome` and `OrdinaryEventsScaled(o, failureFraction)`
setting `Multiplier` and `Lost` on the losing side, decided by `o.Defended`
alone.

⚠️ `Defended` is **carried, never derived.** `contest.Result.Success` means the
attacker won, and `!Success` is not "the defender won" under `ForceCrit`.

- [ ] **Step 3: Run, commit**

---

## Task 7: `Character.AwardResolved`

**Files:** `internal/characters/progression.go`, create `award_resolved_test.go`

- [ ] **Step 1: Write the failing tests**

Over 800 trials: a win advances the skill; a loss also advances it but strictly
less; and a two-candidate call advances **only** the best-of winner, never both.
That third assertion is the one that pins the double-stat fix.

- [ ] **Step 2: Run to verify they fail, then implement**

```go
// AwardResolved is THE entry point for the firing rule on a resolved action.
//
// ONE event, for the Best-of winner among the candidates. Full weight on a win,
// ProgressionFailureFraction on a loss. Crits and critical failures are a
// separate channel and never appear here.
//
// Best-of is why a site awarding both a combat skill and skullduggery cannot
// double-roll dexterity: only one candidate wins, so the shared primary stat is
// rolled once.
//
// The actor is the attacker side; Defended means "the actor lost", which is how
// a one-sided action expresses a loss in the shared Outcome shape.
func (c *Character) AwardResolved(userId int, won bool, candidates ...progression.Candidate) {
	if c == nil {
		return
	}
	best, ok := progression.BestOf(candidates)
	if !ok {
		return
	}
	frac := float64(configs.GetBalanceConfig().ProgressionFailureFraction)
	o := progression.Outcome{
		AttackerSkill: best.Skill,
		AttackerStat:  best.Stat,
		Defended:      !won,
	}
	c.ApplyProgression(
		progression.OrdinaryEventsScaled(o, frac),
		progression.SideAttacker,
		userId,
		util.GetRoundCount(),
	)
}
```

- [ ] **Step 3: Run, update the census, commit**

---

## Task 8: `Actor.AwardResolved`, and the NINE test fakes

⚠️ `Actor` is a 10-method interface with **two** production implementers and
**nine** full test-fake implementers. Adding a method breaks all nine, and
`internal/hooks/spell_foldanchor_test.go` has an explicit
`var _ actions.Actor = (*fakeActor)(nil)`, so that package stops compiling
immediately. This is "2 production types plus 9 fakes", not "both implementations".

The nine: `internal/actions/{consider,economy,forage,salvage,scan,search,sleep,track}_test.go` and `internal/hooks/spell_foldanchor_test.go`.
(`rhetoric_progression_test.go`'s `recordingActor` embeds the interface and gets the method free.)

**Files:** `internal/actions/actor.go`, `actor_user.go`, `actor_mob.go`, the nine test files, create `internal/actions/actor_award_test.go`

- [ ] **Step 1: Write the failing test** asserting both production actors satisfy an interface including `AwardResolved(won bool, candidates ...progression.Candidate)`, and that a losing call reaches the character's skill

- [ ] **Step 2: Add to the interface and both implementations**

⚠️ The field paths are `a.User.Character` and `a.Mob.Character`. Neither struct
has a bare `Character` or `UserId` field:

```go
func (a *UserActor) AwardResolved(won bool, cands ...progression.Candidate) {
	a.User.Character.AwardResolved(a.User.UserId, won, cands...)
}
func (a *MobActor) AwardResolved(won bool, cands ...progression.Candidate) {
	a.Mob.Character.AwardResolved(0, won, cands...)
}
```

- [ ] **Step 3: Add the method to all NINE fakes**, recording the award where a later task's test needs to observe it
- [ ] **Step 4: Run `go build ./... && go test ./internal/actions/ ./internal/hooks/`**, update the census, commit

---

## Task 9: `AwardDefenceProgression` gains a multiplier

**Files:** `internal/combat/defence_multiplier.go`, create `defence_award_test.go`

- [ ] **Step 1: Write the failing test**: 0.35 advances strictly less than 1.0, with a non-zero guard
- [ ] **Step 2: Add the parameter**, scaling both the skill and the stat calls, and update **both** production callers (`defence_multiplier.go:426`, `NewRound_DoCombat_helpers.go:46`) to pass `1.0` so this task is a provable no-op
- [ ] **Step 3: Run, update the census, commit**

---

# Phase B: wire combat

## Task 10: Both defence paths, Best-of, win or lose

**Files:** `internal/combat/defence_multiplier.go`, `internal/hooks/NewRound_DoCombat_helpers.go`, create `internal/hooks/defence_unification_test.go`

⚠️ **The channel path has NO lost branch.** `defence_multiplier.go:423-429` awards
the winning candidate name regardless of `res.Success`, and the comment above it
says so: *"whenever the contest ran, win or lose."* An earlier draft told the
implementer to "add a matching call on the lost branch", which would
**double-award** every lost channel defence. The correct edit is a **conditional
multiplier on the one existing call**.

⚠️ **Melee awards once per defence TYPE**, looping `defenceTypesUsed`, so a
defender with dodge, parry and block takes three events a round, two of which
train weapon-combat. Best-of collapses that to one. This is a **cut** for geared
fighters and a **gain** for a defender with one defence type.

- [ ] **Step 1: Read both paths**

Run: `sed -n '30,70p' internal/hooks/NewRound_DoCombat_helpers.go`
Run: `sed -n '415,432p' internal/combat/defence_multiplier.go`

- [ ] **Step 2: Write the failing tests**

A melee round in which nothing lands awards the best-quoted defence at the
fraction; a melee round with three defence types awards **once**, not three
times; a lost channel defence awards the fraction, not a full event, and is not
awarded twice.

- [ ] **Step 3: Melee passes ONE Best-of candidate**

`runBestOfAllDefense` sets `best.defenseType = res.Winner` whenever
`res.Contested`, so a quoted defence exists on a loss. Expose it, and replace
`processDefenderProgression`'s loop with a single award: full weight if that
defence won, the fraction if not.

⚠️ When `getAvailableDefenses` is empty the contest is uncontested and
`defenseType` is `""`. `BestOf` reports false and nothing is awarded. That is
correct; note it in the commit so it is not mistaken for a gap.

- [ ] **Step 4: Channel passes a conditional multiplier on the ONE existing call**

Do not add a second call. Do not touch the `BonusEvents` line, which is
deliberately separate and whose comment explains that asking for ordinary events
there would double-award.

- [ ] **Step 5: Populate `Defended` on every production Outcome**

Run: `grep -rn "progression.Outcome{" internal/ --include=*.go | grep -v _test.go`

- [ ] **Step 6: Run, update the census, commit**

---

## Task 11: A defended melee swing awards the attacker

⚠️ Attacker events sit inside `for _, wh := range res.WeaponHits { if !wh.CleanHit { continue }` (`NewRound_DoCombat_unified.go:666-667`). Until that gate changes, setting `Defended` changes nothing, because no Outcome is built for a defended swing at all. **This is the single most-missed defect across all three previous versions.**

- [ ] **Step 1: Write the failing test**: a round whose swings are all defended awards the attacker's weapon skill at the fraction
- [ ] **Step 2: Replace the `continue`** with a scaled award, clean hit at 1.0 and defended at the fraction, preserving the per-weapon-hit shape
- [ ] **Step 3: Run, commit**

---

## Task 12: Concentration comes under the rule

Its THREE sites award spellcasting only on success, so a broken
concentration teaches nothing.

⚠️ **THREE sites, not two.** An earlier draft said two and missed the throttle
trigger, which the roadmap already names ("concentration's three original
triggers plus throttle"):

| Site | Award |
|---|---|
| `internal/hooks/combat_shared_helpers.go:141` | `:149` |
| `internal/hooks/combat_shared_helpers.go:577` | `:584` |
| **`internal/actions/combat_throttle.go:172`** | **`:185`**, `OnSkillUse(Spellcasting)` inside the success branch |

**Files:** the three above, create `internal/hooks/concentration_progression_test.go`

- [ ] **Step 1: Write the failing test**: a broken concentration awards spellcasting at the fraction, driven at **each** of the three triggers
- [ ] **Step 2: Route all three through `AwardResolved`**, won on `res.Success`

⚠️ Concentration fires **per damage instance**, so up to roughly four times a
round. `spellcasting` is fitted at 3.90 on the premise that casting is rare and
cooldown-gated. Record the resulting rate in the commit; this is the densest new
spellcasting faucet in the slice.

- [ ] **Step 3: Run, update the census, commit**

---

# Phase C: static-difficulty conversions

## Task 13: `search.go`'s four difficulty checks

- [ ] **Step 1: Write the failing test**: `contest.AgainstDifficulty(` appears 4 times, `dice.RollStat(searchScore)` twice (the hidden-actor sites, converted in Task 14)

⚠️ Task 14 drives that second count to 0 and **must update this assertion in the
same commit.**

- [ ] **Step 2: Convert the four** (125, 125, 135, 175)
- [ ] **Step 3: Add BOTH guard entries in this step.** Learn the `contestSiteOwners` keys by running `go test . ./internal/combat/ -run "Floored|Owned" -v`
- [ ] **Step 4: Run all three, commit**

---

## Task 14: A hider's skill counts

⚠️ **Do not invent a score helper.** `internal/actions` exports
`CalcDetectionScore(c)` and `CalcSneakScoreVsObserver(sneaker, observer, room)`,
and `go.go` uses them at six call sites. Reuse them.

- [ ] **Step 1: Write the failing test**: `search.go` no longer contains `dice.RollStat(searchScore)`, and a high-sneak hider is spotted strictly less often **through the real search path**
- [ ] **Step 2: Convert both sites to `combat.RunContest`** using the existing helpers

⚠️ Opposed, so **no** `guardedRollExemptions` entry; `contestSiteOwners` still applies.

- [ ] **Step 3: Update Task 13's count assertion to 0 in this commit**
- [ ] **Step 4: Run `./internal/actions/ ./internal/usercommands/ ./internal/behaviortree/ ./internal/combat/`, commit**

---

## Task 15: `search` awards once, through the seam

- [ ] **Step 1: Write the failing tests**: a room with five hidden things awards the same as a room with one; a fruitless-but-resolved search awards the fraction

⚠️ `search.go:243` awards a **FULL** event today, win or lose. This is a **cut**
on failure, not a gain. Do not write a test comment claiming otherwise.

- [ ] **Step 2: Replace the award**

```go
	if rolledAgainstSomething {
		actor.AwardResolved(foundAnything, progression.Candidate{
			Skill: string(skills.Search),
			Roll:  bestRoll,
			Level: actor.GetCharacter().GetSkillLevel(skills.Search),
		})
	}
```

Track `bestRoll` as the highest margin across the six checks, and
`foundAnything` alongside each existing `result.*Found` append.

- [ ] **Step 3: Run, update the census, commit**

---

## Task 16: `track` and `forage`

⚠️ **`track.go` has SIX comparisons over THREE thresholds**, not two: `:131`
(125), `:159` (175), `:253`/`:292` (175), `:256`/`:295` (135). They read a
`rollValue` **parameter** threaded from the call site at `:151`, not
`result.RollValue`, which is set at `:125` and **never read**.

⚠️ `contest.Result.Margin` is **stat-scale**, not normalised, so
`res.AttackRoll.Value` is the drop-in for `roll.Value` and no conversion is
needed. An earlier draft claimed the opposite.

⚠️ **Forage's award is not in `internal/forager`.** The roll is
`forage_core.go:129`; the award is `actor.OnSkillUse(string(skills.Search))` at
**`internal/actions/forage.go:142`**.

- [ ] **Step 1: Write the failing tests** for both
- [ ] **Step 2: Convert `track.go` with ONE roll**, keeping all six comparisons working on a stat-scale value
- [ ] **Step 3: Convert `forage_core.go`**, keeping the per-biome difficulty
- [ ] **Step 4: Award through `AwardResolved` in both**
- [ ] **Step 5: Guard entries, run, update the census, commit**

---

## Task 17: The authored material tier

- [ ] **Step 1: Write the failing tests**: tier 1 gives 0.75, tier 3 gives 1.0, tier 5 gives 1.25; **untiered (0) gives 1.0, neutral**, never the cheapest
- [ ] **Step 2: Add `ItemSpec.MaterialTier` and `MaterialTierMultiplier`**
- [ ] **Step 3: Guard NEW materials, grandfathering existing ones**

⚠️ Scan **all** item directories: `MaterialTier` is on `ItemSpec`, so a new
material under `consumables-30000` would escape a a `materials-40000`-only scan.

Run: `grep -rL "material_tier:" _datafiles/world/dogmud/items/ --include='*.yaml'`

⚠️ `**` does not recurse in bash without `globstar`; use `-r` with `--include`.

⚠️ On day one **every** material is untiered, so the multiplier is 1.0 and
difficulty is `100 + SkillMinimum * 5`. Coherent, because `SkillMinimum` carries
difficulty. **The backfill is owed before U11** and the guard is its completion
check: empty the grandfathered set and it turns green.

- [ ] **Step 4: Run, commit**

---

## Task 18: Deterministic ingredient resolution

⚠️ **Do NOT tiebreak on lowest ItemId.** Bottle ids are Glass Vial **40006**,
Clay Flask 40043, Sealed Phial 40044, Crystalline Decanter 40045, and their
aging multipliers run 1.0 / 3.0 / 0.5 / 0.25. Lowest-id makes the Glass Vial
always win and orders the rest **inverse to quality**, so the Sealed Phial and
Crystalline Decanter become unusable by anyone carrying a Vial. That deletes the
potion aging axis as a side effect of a determinism fix.

**Use inventory order**, which `ConsumeIngredients` already uses (component bag,
then backpack) and which is already deterministic. Determinism was the goal, not
a particular ordering.

- [ ] **Step 1: Write the failing test**: the same recipe against the same inventory resolves to identical item ids fifty times running, **and** a player holding a Crystalline Decanter and a Glass Vial can still consume the Decanter
- [ ] **Step 2: Implement**

```go
func SelectIngredientItems(inv []items.Item, componentInv []items.Item, recipe *RecipeSpec) []items.Item
func HighestIngredientTier(selected []items.Item) float64
```

⚠️ Named for **tier**, not "dearest": a price word would re-admit gold value,
which was measured as noise against recipe tier.

- [ ] **Step 3: Have `ConsumeIngredients` take the selection**, enumerating and updating its callers
- [ ] **Step 4: Run, commit**

---

## Task 19: Craft resolves as a floored contest and awards both ways

- [ ] **Step 1: Write three failing tests**

50/50 at the minimum (`CraftScore(100, 10) == CraftDifficulty(10, 1.0)`, with
non-zero guards, since `0 == 0` passes against unwired knobs); an advanced recipe
needs more mastery than a novice one (compare ratios); and **a failed craft
awards the fraction**, which is the case the slice is justified by.

- [ ] **Step 2: Implement the formulas**

```go
// CraftScore composes the crafter's side the way every other score in the game
// is composed: stat + skill * SkillWeight. Balance.SkillWeight already exists
// and ships at 5.0; do not add a craft-specific weight.
func CraftScore(primaryStat float64, craftSkill int) float64

// CraftDifficulty reads as "a baseline human holding exactly this recipe's
// minimum skill", scaled by the tier of the materials in hand.
// CraftBaseDifficulty is 100 because 100 is the human stat baseline.
func CraftDifficulty(recipeSkillMinimum int, materialTierMult float64) float64
```

- [ ] **Step 3: Route the four call sites through `contest.RunWithFloors`** with `CraftFloor`

⚠️ `RunWithFloors`, not `AgainstDifficulty`: craft needs the mercy band its
`[5%, 95%]` clamp gives today. Extremes match; the interior compresses by
`(1 - 2f)`, so a true 80% reads as 77%.

- [ ] **Step 4: Award on BOTH branches via `AwardResolved`**
- [ ] **Step 5: Retire `CalcSuccessChance` and fix its THREE test callers**

`crafting_test.go`, `integration_crafting_test.go`, and `mobs/crafter_test.go`,
whose `forceCraftSuccess`/`forceCraftFailure` drive the deleted knobs **by string
key** through `configs.AddOverlayOverrides`. **Map keys are strings, so that
compiles and silently stops working.** Replace the lever and assert the forced
outcome actually occurs.

- [ ] **Step 6: Guard entries for THREE caller files** (`NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go`, `mobs/crafter.go`, the last of which has **two** sites, so two owner rows)
- [ ] **Step 7: Run, update the census, commit**

---

## Task 20: Mob crafters award under the same rule

⚠️ **Only the SHOP path bypasses the `SkillMinimum` gate.** The legacy path's
`pickEligibleRecipe` gates explicitly (`if skillLevel < recipe.SkillMinimum`).
Scope the claim, or the "skill 1 against a minimum of 65" example will not
reproduce.

⚠️ **Throughput goes UP, not down.** Crafter mobs ship at stat 104 to 115 with
`blacksmithing: 1`, so their success rate rises roughly 1.4x to 3x in the
`skill_minimum` 3 to 10 band. The economy risk is crafted goods drifting toward
the **0.25x price floor**, not the ceiling. Record the measured rates in the
commit.

- [ ] **Step 1: Write the failing test** forcing a failed mob craft
- [ ] **Step 2: Award on both branches at both sites** (`:505`, `:546`)
- [ ] **Step 3: Run `./internal/mobs/ ./internal/shops/`, update the census, commit**

---

## Task 21: Salvage

⚠️ Award sites `salvage.go:166` and `:252`; clamps read at `:83-84` and defaulted
in **`config.balance.misc.go`**. **No previous plan version touched the awards.**

⚠️ **`SalvageDifficulty` needs a base and nobody has picked one.** Mirroring
`CraftBaseDifficulty` at 100 triples the material sink's leakage at skill 0
(retention 14% today versus 44%). Set the base so that per-unit recovery at
skill 0 lands near today's 15% and at skill 50 near today's 85%, and **record the
retention figures at skill 0, 15, 25 and 50 in the commit.** Endpoints matching
is not sufficient: the old curve was `sqrt` in skill and the new one is a normal
CDF of a ratio, which saturates earlier.

- [ ] **Step 1: Write the failing tests**: dear materials are harder to reclaim; per-unit recovery at high skill sits in a **band** around 0.80 to 0.88 (not merely `<= 0.90`, which passes against no ceiling logic at all); a salvage recovering nothing awards the fraction
- [ ] **Step 2: Implement** `SalvageScore`, `SalvageDifficulty`, `RollSalvageUnit` on `contest.RunWithFloors` with `SalvageFloor`

⚠️ Both `RollSalvageReturns` signatures change. **Do not resolve tiers from a tag
inside them**. That is `FindSpecByComponentTag`, the map-order function Task 18
exists to forbid. Resolve concretely at the call site.

- [ ] **Step 3: Award ONCE per salvage command via `AwardResolved`**, won if anything was recovered
- [ ] **Step 4: Retire the two clamps**, keeping `SalvageSoftCap`, `SalvageGoldPerRound`, `SalvageMaxRounds`
- [ ] **Step 5: Guard entries, run, update the census, commit**

---

# Phase D: remaining sites

## Task 22: The sixteen skullduggery sites

`actions/steal.go` x3, `plant.go` x3, `shadow.go` x2,
`usercommands/skill.skullduggery.sneak.go` x2, `picklock.go` x2, `defuse.go`,
`throw.go`, `mobcommands/flee.go`, `hooks/NewRound_DoCombat_helpers.go`.

Four bypass every entry point via direct `CheckSkillProgression`: both sneak
sites (one is the **failure** branch) and both picklock sites.

🔴 **The two MULTI-CANDIDATE sites are not in the list above, and without them
Best-of ships inert.** Every site named above awards exactly one skill, as do
search, track, forage, craft, salvage, concentration and both defence paths. If
this task only converts those, **every production `AwardResolved` call passes a
single candidate** and `BestOf` is exercised only by Task 6's unit tests, while
the double-dexterity roll the architecture exists to fix survives untouched.

Add both, and treat them as the point of the task:

| Site | Today |
|---|---|
| `internal/hooks/NewRound_DoCombat_unified.go:699` | a **second** `Outcome` awarding skullduggery, alongside the weapon-skill award at `:670`. The source comment at `:692` says "A SECOND Outcome is structurally required" |
| `internal/actions/combat_fire.go:406` | the ranged twin, same shape |

✅ **Best-of removes the trap those two comments describe.** Each passes **both
skills as candidates** to one `AwardResolved` call. Only one wins, so the shared
dexterity primary stat is rolled once. No two-`Outcome` pattern, no suppression
flag.

⚠️ **Skullduggery is never rolled in a surprise attack.** It is read as a LEVEL
at `internal/combat/crit_damage.go:74` to scale the crit multiplier. Per spec
3.0.1a, **roll it for selection** as
`dice.RollStat(dexterity + skullduggeryLevel * SkillWeight)`, the same shape as
the weapon candidate. A candidate with no roll ties at zero and the level
tiebreak deletes it deterministically.

⚠️ **`picklock` is not a contest**: both sites fire on success of a pin minigame,
so there is no resolved loss. Pass `won: true` and say so in the commit.

⚠️ Several of these award a **FULL** event win or lose today (`steal.go` x3, the
sneak failure branch), so the rule is a **cut** on failure there.

- [ ] **Step 1: Write the failing tests**: the two files no longer call `CheckSkillProgression`; a failed steal awards the fraction; a site with two candidates advances only one
- [ ] **Step 2: Convert each site**
- [ ] **Step 3: Run everything, update the census, commit with EXPLICIT paths**

⚠️ Never `git add internal/`.

---

## Task 23: The mob spell path adopts the player path's gates

- [ ] **Step 1: Write three BEHAVIOURAL tests**, one per gate (self-cast penalty, empty-area zeroing, `spellBonus > 0`)

⚠️ Do not assert the mob block contains the gate literals; Step 2 extracts them
into a shared helper, after which it contains a call.

- [ ] **Step 2: Extract ONE helper, call it from both paths**
- [ ] **Step 3: Run, update the census, commit**

---

# Phase E: deletions

## Task 24: Delete the stranded mob-follow roll

⚠️ Delete **only the loop** (`go.go:668-697`). The `if !isSneaking {` block at
`:664` also contains `TryRoomBehavior` at `:700`, and there is a second,
unrelated `TryRoomBehavior` at `:357`, so a bare string assertion passes even if
the destination one is deleted. Pin the destination call specifically.

⚠️ `internal/usercommands/go_test.go` does not exist; create it.

---

## Task 25: Delete first-kill progression

Remove `Character.OnFirstMobKill`, both call sites, and the message. **Keep
`KD.AddMobKill`**, which feeds the bestiary. Update the census.

---

## Task 26: Dead crit stubs and stale comments

- [ ] **Step 1: Delete the stubs from ALL NINE files** (the same nine as Task 8). Verify: `grep -rl "func.*OnCritical" internal/` returns nothing
- [ ] **Step 2: Remove them from the DENY-list `progressionCalls`**, not from `allowedDirectProgression`, which would silently break the U9 guard
- [ ] **Step 3: Fix the two stale `defence_multiplier.go` comments**

⚠️ **`OnCritReceived` is NOT touched by this slice.** Leave
`TestCritReceivedProgression_DecaysWithRank` alone. An earlier draft told the
implementer to re-anchor it on a false premise.

---

# Phase F: guard, docs, verify

## Task 27: The standing guard

⚠️ `util.Rand(` has **210** production sites; `dice.RollStat(` has 14. Scope the
scanner to a roll compared against a threshold **to decide whether an action
succeeded**, using Task 2's `callNamesInSource`.

- [ ] **Step 1: Define the scanner** with a concrete result type (`File`, `Line`, `Mechanism`)
- [ ] **Step 2: Write the guard** with a file-keyed allow-list carrying a reason per entry
- [ ] **Step 3: Prove the scanner is not blind BEFORE trusting a pass**: feed it a known bare threshold roll and assert it is reported. **A first-run pass means the scanner is blind, not the tree clean.**
- [ ] **Step 4: Disposition each offender, run, commit**

---

## Task 28: Breadcrumbs and the roadmap

- [ ] **Step 1: Breadcrumb `item_procs.go` and `handleMobWeaponPickup`**
- [ ] **Step 2: `NOTE(U12)` at both `ChanceToSwitchTarget` roll sites**
- [ ] **Step 3: ADD a Category B row**: there is **no** such row today, only "Category C" inside U10b's prose cell. Verify with `grep -in "category b"` and **add**, do not edit
- [ ] **Step 4: Record the material-tier backfill as OWED BEFORE U11** on U11's row, as a test rather than prose

---

## Task 29: Re-solve the fitted multipliers

⚠️ **Direction differs per path**; a single headline number is wrong. Melee
attacker **+26%**; melee defender **+47%** with one defence type but roughly
**−21%** with three; channel defender a **cut**; search, track, steal, sneak and
salvage a **cut** of 32 to 57% because they award a full event today.

⚠️ Re-solve **unarmed-combat** (which is what dodge trains), weapon-combat
(parry and block), spellcasting (quell), rhetoric (defy), skullduggery, salvage
and search.

⚠️ `combat-analytics.jsonl` is **combat-only**, so the non-combat multipliers are
set by judgement and confirmed at playtest, not solved. Say so in the commit.

⚠️ `TrackSkillUse`/`TrackStatUse` still fire at full weight on a fractional
event, so counters inflate faster than rolls. Check whether the solver reads
counters or rolls before trusting it.

---

## Task 30: Docs, `context.md`, patch notes

New symbols: `progression.Candidate`, `progression.BestOf`,
`progression.OrdinaryEventsScaled`, `Outcome.Defended`, `Event.Lost`,
`Character.AwardResolved`, `Character.OnStatUseScaled`, `Actor.AwardResolved`,
`crafting.CraftScore` / `CraftDifficulty` / `SelectIngredientItems` /
`HighestIngredientTier` / `SalvageScore` / `SalvageDifficulty`,
`items.MaterialTierMultiplier`, `ItemSpec.MaterialTier`. Removed:
`CalcSuccessChance`, `OnFirstMobKill`. Changed: `AwardDefenceProgression`,
`OnSkillUseScaled`.

⚠️ Verify every symbol exists before naming it.

Patch notes: no raw numbers, no em dashes, 80 columns. Failing at something
teaches you a little; searching, tracking, foraging, crafting and salvaging
resolve like everything else; how hard a craft is depends on the recipe and the
materials; someone good at hiding is harder to find. **Do not promise that mobs
will chase you.**

---

## Task 31: Pre-push verification and the adversarial playtest

- [ ] **Step 1:** `gofmt -l internal/ modules/` prints nothing
- [ ] **Step 2:** `go build ./... && go test ./... 2>&1 | grep -v "^ok" | head -40`
- [ ] **Step 3:** `Logging.LogToFile: false`, from the `git show HEAD:` blob
- [ ] **Step 4: Boot test in an isolated detached worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

⚠️ **Exit 124 is the success case.** Never grep the bare word `panic`.

- [ ] **Step 5: Adversarial playtest, FIVE signals reported separately**

1. **The convention.** Does failing at something visibly teach a little?
2. **Defence.** Do dodge, parry, block, quell and defy improve at a reasonable
   pace? Channel defences took a **cut**, and so did geared melee defenders.
3. **Crafting.** Does difficulty track the recipe? Does an advanced recipe need
   more mastery? **No material is tiered yet**, so the material half is neutral.
4. **Search, track, forage.** Weak searchers improve sharply; experts lose
   near-certainty. Failure now pays **less** than it does today.
5. **The economy.** The craft-then-salvage material sink, and shop restock, where
   throughput goes **up**.

⚠️ **The AI port caps 3 commands per round and silently discards the overflow
after echoing it.** One command per batch; verify output.

- [ ] **Step 6: Fix findings, extract them to memory** (reports are gitignored)
- [ ] **Step 7: Open the PR** with `--repo pruuk/DOGMud` on every `gh` call
