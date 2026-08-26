# U10b-1 Progression Firing Convention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One firing rule for progression (one event per resolved command, a loss pays a fraction, crit and fumble stay a bonus layer), wired into every path that awards, with the melee and channel defence paths unified.

**Architecture:** THREE unifying seams, built first, so every later task is a one-line call rather than a bespoke fix. Nothing awards progression by hand after Phase A.

| Seam | Serves |
|---|---|
| `Character.AwardResolved(skill, userId, won)` | Every resolved non-combat command: search, track, forage, craft, salvage, skullduggery |
| `Actor.AwardResolved(skill, won)` | The `internal/actions` half, hiding the user/mob userId difference exactly as `OnSkillUse` does today |
| `combat.AwardDefenceProgression(c, userId, defenceType, multiplier)` | BOTH defence paths, melee and channel, unified |

**Tech Stack:** Go, `internal/contest`, `internal/combat`, `internal/progression`, `internal/characters`, `internal/actions`, `internal/crafting`, `internal/configs`.

**Spec:** `docs/superpowers/specs/2026-08-26-u10b-1-progression-firing-convention-design.md`

---

## Read this before Task 1

This plan is the second rewrite. Two previous versions failed blind adversarial review.

1. **The seam is not the feature.** Version one converted fifteen roll sites and applied the rule to none. Version two added `Outcome.Defended` and never set it, left a defended melee swing awarding nothing, and never made salvage award at all. Phase B is the deliverable.
2. **`contest.Result.Success` means the ATTACKER won.** `!res.Success` is NOT "the defender won": under `side.ForceCrit` the attack wins with `Success == false`. Gate on `Defended`. A mirrored test fake passes either way.
3. **`contest.AgainstDifficulty` and `contest.Run` apply NO floor.** `combat.RunContest` is the only reader of `ContestFloor` and its doc comment forbids routing static-difficulty rolls there. Craft and salvage call `contest.RunWithFloors` directly.
4. **Both are on the root guard's watch list.** Every conversion adds `guardedRollExemptions` (file-keyed, in the **repo-root** `contest_floor_guard_test.go`, `package main`) and `contestSiteOwners` (`file:func`-keyed, `internal/combat/contest_site_guard_test.go`) entries **in the same step**.
5. **Go has no cross-package test helpers.** Task 1 creates `repoRootForTest` per package. A helper in `internal/combat` cannot serve a guard in `internal/progression`.
6. **A test binary sees a ZERO-VALUED config.** `configs.SetConfigForTest` takes **two** arguments and does not validate.

### Task 0: the config idiom every behavioural test in this plan uses

`GetBalanceConfig()` in a test binary returns zeroes, and `UseSkillProgression`
is `false`, so no progression rolls happen at all. Pin it first:

```go
func pinConfigForTest(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.UseSkillProgression = true
	cfg.Balance.ProgressionFailureFraction = 0.35
	cfg.Balance.BaseProgressionChance = 0.30
	cfg.Balance.StatProgressionSoftCap = 50
	configs.SetConfigForTest(t, cfg)
}
```

⚠️ Confirm each field's real path before pasting. `Balance` is a **top-level**
field of `Config`, not nested under `GamePlay`. Add whichever extra knobs a
given test needs. Where this plan says `pinConfigForTest(t)`, it means this.

---

# Phase A: the unifying seams

## Task 1: Package-local `repoRootForTest`, and the census

**Files:** create `testsupport_test.go` in `internal/progression`, `internal/actions`, `internal/forager`, `internal/hooks`, `internal/usercommands`, `internal/crafting`, `internal/configs`, `internal/items`; create `internal/progression/census_test.go`.

- [ ] **Step 1: Write the helper once per package**

```go
// repoRootForTest resolves the repository root from this file's own location.
//
// Test binaries do NOT reliably start in the package directory: all tests share
// one binary, so a relative path passes or fails depending on which package ran
// first. Anchor on runtime.Caller instead.
//
// Duplicated per package because Go test helpers are not visible across
// packages. internal/combat already has one; do not add a second there.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}
```

⚠️ Match each package's existing `_test.go` package clause (`x` versus
`x_test`) or the file will not compile alongside them. `internal/progression`
has both: `event_test.go` is `package progression`, `seam_guard_test.go` is
`package progression_test`. Put the helper in `package progression_test`.

- [ ] **Step 2: Write the census test**

Walk `internal` and `modules` for non-test `.go` files, count occurrences of
`OnSkillUseScaled(`, `OnSkillUse(`, `OnStatUse(`, `OnStatUseScaled(`,
`CheckSkillProgression(`, `CheckStatProgression(`, `OnCritReceived(`,
`TrackSkillUse(`, `TrackStatUse(`, `OnRegenTick(`, `CheckRegenProgression(`,
`AwardResolved(`, skipping lines whose trimmed form starts with `//`, and assert
the total equals a pinned constant.

Failure message: *"If you added a site deliberately, classify it against spec
section 3.1 and update this number in the same commit."*

- [ ] **Step 3: Run to learn the number, set it, re-run**

Run: `go test ./internal/progression/ -run Census -v`
Expected: FAIL first (reporting the count), PASS after.

⚠️ **This number moves in Tasks 4, 5, 7, 9, 14, 15, 18, 20, 21 and 24.** Each
says so and updates it in the same commit. A red census whose cause is your own
previous task is expected; read the diff before assuming a regression.

- [ ] **Step 4: Verify all eight compile**

Run: `go test ./internal/progression/ ./internal/actions/ ./internal/forager/ ./internal/hooks/ ./internal/usercommands/ ./internal/crafting/ ./internal/configs/ ./internal/items/ 2>&1 | grep -v "^ok"`
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git commit -m "test(u10b-1): package-local repoRootForTest and the progression census"
```

---

## Task 2: An AST recogniser for chained calls, in the package that needs it

Task 26's guard must see `x.Character.OnStatUse(...)`, a selector on a selector.
`internal/combat/contest_site_guard_test.go`'s walker asserts `v.X` is an
`*ast.Ident` and bails, so it sees only package-qualified calls.

⚠️ **Build this in `internal/progression`, not `internal/combat`.** Task 26's
guard lives there and cannot import another package's test helper. Leave the
combat walker alone: it is correct for its job, and making it match on tail name
alone would classify every `foo.Run(...)` in the tree as a contest site.

**Files:** create `internal/progression/astscan_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestChainedCallRecogniser pins that the scanner sees a method reached through
// a field. x.Character.OnStatUse(...) is the dominant progression call shape,
// and a walker asserting v.X is an *ast.Ident misses every one, shipping green
// while enforcing nothing.
func TestChainedCallRecogniser(t *testing.T) {
	src := `package p
func f(v x) {
	contest.AgainstDifficulty(1, 2)
	v.Character.OnStatUse("dexterity", 1)
	c.OnSkillUse("weapon-combat", 1)
}
`
	got := map[string]bool{}
	for _, name := range callNamesInSource(t, src) {
		got[name] = true
	}
	for _, want := range []string{"AgainstDifficulty", "OnStatUse", "OnSkillUse"} {
		if !got[want] {
			t.Errorf("recogniser missed %q", want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/progression/ -run TestChainedCallRecogniser -v`
Expected: FAIL, "undefined: callNamesInSource"

- [ ] **Step 3: Implement**

```go
// callNamesInSource returns the tail name of every call expression, whether
// package-qualified (contest.AgainstDifficulty) or reached through a field
// (v.Character.OnStatUse). Tail-name matching is safe here because callers pair
// it with an explicit set of names they are looking for.
func callNamesInSource(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			out = append(out, sel.Sel.Name)
		}
		return true
	})
	return out
}
```

- [ ] **Step 4: Run, commit**

Run: `go test ./internal/progression/ -v`
Expected: PASS

---

## Task 3: `ProgressionFailureFraction`, with a pre-unmarshal sentinel

An absent YAML key unmarshals to `0`, a **legal** value here (an explicit
off-switch), so `if x < 0 || x > 1.0` can never default it.

**Files:** `internal/configs/config.balance.go`, `config.balance.progression.go`, `configs.go`, `_datafiles/config.yaml`, create `internal/configs/progression_failure_fraction_test.go`

- [ ] **Step 1: Locate the pre-unmarshal construction point**

Run: `grep -n "tmpConfigData := Config{}" internal/configs/configs.go`

⚠️ The seed line is `tmpConfigData.Balance.ProgressionFailureFraction = -1`.
There is no `Config.GamePlay.Balance`.

- [ ] **Step 2: Write three failing tests**

```go
func seedAndLoad(t *testing.T, doc string) Balance {
	t.Helper()
	b := Balance{ProgressionFailureFraction: -1}
	if err := yaml.Unmarshal([]byte(doc), &b); err != nil {
		t.Fatal(err)
	}
	b.Validate()
	return b
}

func TestFailureFractionDefaultsWhenAbsent(t *testing.T) {
	if got := float64(seedAndLoad(t, "RollSpread: 0.15\n").ProgressionFailureFraction); got != 0.35 {
		t.Fatalf("key ABSENT = %v, want 0.35", got)
	}
}

func TestFailureFractionZeroIsHonoured(t *testing.T) {
	if got := float64(seedAndLoad(t, "ProgressionFailureFraction: 0\n").ProgressionFailureFraction); got != 0 {
		t.Fatalf("explicit 0 became %v; the off-switch must be honoured", got)
	}
}

// TestReloadConfigSeedsTheSentinel proves PRODUCTION is wired. The two above
// only prove Validate() behaves given a seeded struct; without this they both
// pass while ReloadConfig never seeds and the knob ships inert.
func TestReloadConfigSeedsTheSentinel(t *testing.T) {
	root := repoRootForTest(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "configs", "configs.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "ProgressionFailureFraction = -1") {
		t.Fatal("ReloadConfig does not seed the sentinel before unmarshal, so " +
			"an absent key is indistinguishable from an explicit 0")
	}
}
```

⚠️ The assertion is `ProgressionFailureFraction = -1`, an **assignment**, not
the struct-literal form `: -1`. An earlier draft asserted the literal against an
assignment implementation, so the test failed on correct code.

⚠️ Import `os`, `path/filepath`, `strings`, `testing`, `gopkg.in/yaml.v2`.

- [ ] **Step 3: Declare, seed, default**

Validator:

```go
// -1 is the "unset" sentinel seeded by ReloadConfig. Anything in [0,1] is taken
// at face value, including an explicit 0.
if b.ProgressionFailureFraction < 0 || b.ProgressionFailureFraction > 1.0 {
	b.ProgressionFailureFraction = 0.35
}
```

- [ ] **Step 4: Add to `_datafiles/config.yaml`, run, commit**

⚠️ That file has `skip-worktree`. Never `git add` it from disk; build the
committed version from the `git show HEAD:` blob per CLAUDE.md.

---

## Task 4: `OnStatUseScaled`

**Files:** `internal/characters/progression.go`, create `internal/characters/progression_scaled_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestOnStatUseScaledRespectsMultiplier(t *testing.T) {
	pinConfigForTest(t)
	const trials = 600

	zero := newProgressionTestCharacter(t)
	full := newProgressionTestCharacter(t)
	for i := 0; i < trials; i++ {
		zero.OnStatUseScaled("dexterity", 0, 0.0)
		full.OnStatUseScaled("dexterity", 0, 1.0)
	}

	if got := zero.Stats.Dexterity.Training; got != 0 {
		t.Fatalf("multiplier 0.0 advanced training to %d; multiplier ignored", got)
	}
	if got := full.Stats.Dexterity.Training; got == 0 {
		t.Fatal("multiplier 1.0 never advanced training over 600 trials; the " +
			"config is not pinned and progression is switched off")
	}
}
```

⚠️ Both assertions matter. The first alone passes against a function that never
progresses anything, which is exactly what an unpinned config produces.

- [ ] **Step 2: Run to verify it fails, then implement**

```go
// OnStatUseScaled is OnStatUse with an explicit progression multiplier.
//
// It exists because an ordinary progression event carries a Multiplier and the
// stat half used to ignore it: a resolved LOSS paid a fractional skill roll and
// a full-rate stat roll.
func (c *Character) OnStatUseScaled(statName string, userId int, multiplier float64) bool {
	c.TrackStatUse(statName)
	if configs.GetGamePlayConfig().UseSkillProgression {
		return c.CheckStatProgression(statName, userId, multiplier)
	}
	return false
}

// OnStatUse is OnStatUseScaled at full weight.
func (c *Character) OnStatUse(statName string, userId int) bool {
	return c.OnStatUseScaled(statName, userId, 1.0)
}
```

- [ ] **Step 3: Make `OnSkillUseScaled`'s primary-stat call honour the multiplier**

Replace its trailing `c.OnStatUse(primaryStat, userId)` with
`c.OnStatUseScaled(primaryStat, userId, bonusMultiplier)`.

⚠️ Bonus events do **not** reach here: `ApplyProgression` routes
`Class.IsBonus()` to `applyBonusProgression`. And `actGrantProgression` calls
`CheckSkillProgression` directly with 1000.0, also bypassing this. An earlier
draft warned of a 1000x stat roll and a crit double-dip; **both were false.** No
gating is needed.

- [ ] **Step 4: Run, update the census, commit**

---

## Task 5: Scale mutation drift; gate the quest event on a LOSS

**Files:** `internal/characters/progression.go`, create `internal/characters/progression_side_effects_test.go`

- [ ] **Step 1: Write the failing test**

Assert a 0.35 award grants strictly less cluster affinity than a 1.0 award, and
that the 1.0 award grants more than zero. Confirm the field first:

Run: `grep -n "AddClusterAffinity" -A 4 internal/characters/*.go | head -20`

- [ ] **Step 2: Scale the drift**

```go
	// At the SAME weight as the event. A resolved loss must not buy a full
	// tick, or the failure fraction roughly doubles mutation acquisition.
	amt := float64(configs.GetBalanceConfig().MutationAffinityPerSkillUse) * bonusMultiplier
```

- [ ] **Step 3: Gate the quest event on LOSS, not on the multiplier**

```go
	// Quest skill_use counters are integers, so a fractional award cannot pay a
	// fractional tick. A resolved LOSS does not advance "use this skill N
	// times", or such a quest becomes "fail at it N times".
	if !isLoss {
		events.AddToQueue(events.SkillUsed{UserId: userId, Skill: skills.SkillTag(skillName)})
	}
```

⚠️ `SkillUsed.Skill` is a `skills.SkillTag`, not a `string`; the conversion is
required.

⚠️ **Gate on the loss, not on `multiplier >= 1.0`.**
`SelfCastProgressionMultiplier` is 0.5 shipped, so a self-only buff cast passes
a multiplier below 1.0 and a multiplier-based gate would silently stop emitting
`SkillUsed` for self-buffs, which they emit today. No shipped quest uses
`skill_use` (grep `_datafiles/world/dogmud/quests/`), so it is latent, but it is
a trap for the next quest author. Thread `isLoss` from the caller.

- [ ] **Step 4: Run, update the census, commit**

---

## Task 6: `Outcome.Defended` and `OrdinaryEventsScaled`

**Files:** `internal/progression/event.go`, create `internal/progression/event_scaled_test.go` (`package progression_test`)

⚠️ `event_test.go` is `package progression`; a new file is needed for the
qualified references.

- [ ] **Step 1: Write BOTH failing tests**

One asserting the attacker is scaled when `Defended` is true and the defender is
not; one asserting the mirror when `Defended` is false. **Both are needed**: the
first alone passes against a stub returning `OrdinaryEvents` unchanged, since
that hardcodes 1.0 on the defender.

- [ ] **Step 2: Add the field and the function**

```go
	// Defended reports that the DEFENDER won this contest.
	//
	// Carried rather than derived because contest.Result.Success means the
	// ATTACKER won, and !Success is NOT "the defender won": under
	// side.ForceCrit the attack wins with Success == false. Every attempt to
	// reconstruct this at a call site has been a bug, and a mirrored test fake
	// passes either way.
	Defended bool
```

```go
// OrdinaryEventsScaled is OrdinaryEvents with the U10b-1 firing rule applied:
// the side that LOST a resolved contest earns failureFraction of an event.
// The winner is decided by o.Defended alone.
func OrdinaryEventsScaled(o Outcome, failureFraction float64) []Event {
	evs := OrdinaryEvents(o)
	for i := range evs {
		lost := (evs[i].Side == SideAttacker && o.Defended) ||
			(evs[i].Side == SideDefender && !o.Defended)
		if lost {
			evs[i].Multiplier = failureFraction
		}
	}
	return evs
}
```

- [ ] **Step 3: Run, commit**

---

## Task 7: `Character.AwardResolved`, the single non-combat award seam

Every resolved command in Phase C calls this and nothing else. Building it now
turns five later tasks into one-liners and gives Task 26's guard one name to
look for.

**Files:** `internal/characters/progression.go`, create `internal/characters/award_resolved_test.go`

- [ ] **Step 1: Write the failing test**

```go
// TestAwardResolvedPaysFractionOnALoss pins the firing rule at its single entry
// point. Every resolved non-combat command in the game routes here.
func TestAwardResolvedPaysFractionOnALoss(t *testing.T) {
	pinConfigForTest(t)
	const trials = 800

	won := newProgressionTestCharacter(t)
	lost := newProgressionTestCharacter(t)
	for i := 0; i < trials; i++ {
		won.AwardResolved("search", 0, true)
		lost.AwardResolved("search", 0, false)
	}

	wonLvl := won.GetSkillLevel("search")
	lostLvl := lost.GetSkillLevel("search")

	if wonLvl == 0 {
		t.Fatal("a winning command never advanced search; config not pinned")
	}
	if lostLvl == 0 {
		t.Fatal("a losing command never advanced search; the fraction is not " +
			"awarded, which is the whole slice")
	}
	if lostLvl >= wonLvl {
		t.Fatalf("loss advanced to %d vs win %d; the fraction is not applied",
			lostLvl, wonLvl)
	}
}
```

⚠️ All three assertions are load-bearing: unpinned config, "loss awards nothing"
(the version-two failure), and "loss awards full weight".

- [ ] **Step 2: Run to verify it fails, then implement**

```go
// AwardResolved is THE entry point for the U10b-1 firing rule on a resolved
// non-combat command: search, track, forage, craft, salvage, skullduggery.
//
// ONE event per resolved COMMAND, not per internal roll. A command that ran
// several rolls (search runs six, salvage rolls per unit) calls this once. A
// command that resolved and won nothing pays ProgressionFailureFraction.
//
// The actor is the attacker side; Defended means "the actor lost", which is how
// a one-sided command expresses a loss in the shared Outcome shape.
func (c *Character) AwardResolved(skillName string, userId int, won bool) {
	if c == nil || skillName == "" {
		return
	}
	frac := float64(configs.GetBalanceConfig().ProgressionFailureFraction)
	o := progression.Outcome{AttackerSkill: skillName, Defended: !won}
	c.ApplyProgression(
		progression.OrdinaryEventsScaled(o, frac),
		progression.SideAttacker,
		userId,
		util.GetRoundCount(),
	)
}
```

⚠️ Confirm `ApplyProgression`'s real signature before pasting; it is
`(events []progression.Event, side progression.Side, userId int, round uint64)`.

- [ ] **Step 3: Run, update the census, commit**

---

## Task 8: `Actor.AwardResolved`, the interface half

`internal/actions` reaches progression through the `Actor` interface, whose
`OnSkillUse(skillName string) bool` hides the userId difference between user and
mob actors. Search, track, forage and salvage all live behind it.

**Files:** `internal/actions/actor.go`, `actor_user.go`, `actor_mob.go`, create `internal/actions/actor_award_test.go`

- [ ] **Step 1: Write the failing test**

Assert both `UserActor` and `MobActor` satisfy an interface including
`AwardResolved(skillName string, won bool)`, and that a losing call reaches the
character's skill.

- [ ] **Step 2: Add to the interface and both implementations**

```go
	// AwardResolved fires the U10b-1 firing rule for a resolved command: full
	// weight on a win, ProgressionFailureFraction on a loss. Prefer it over
	// OnSkillUse for anything that ROLLED; OnSkillUse stays correct for actions
	// with no roll at all.
	AwardResolved(skillName string, won bool)
```

`UserActor` calls `a.Character.AwardResolved(skillName, a.UserId, won)`;
`MobActor` calls `a.Character.AwardResolved(skillName, 0, won)`, mirroring how
each implements `OnSkillUse` today.

- [ ] **Step 3: Run, commit**

---

## Task 9: `AwardDefenceProgression` gains a multiplier, the unified defence seam

It is already **the** mapping for all five defences in one place. Adding the
multiplier makes it the single award for both defence paths, which is what spec
5.5.0 unifies.

**Files:** `internal/combat/defence_multiplier.go`, create `internal/combat/defence_award_test.go`

- [ ] **Step 1: Write the failing test**

Assert a 0.35 call advances the defence skill strictly less than a 1.0 call over
many trials, with a non-zero guard on the 1.0 case.

- [ ] **Step 2: Add the parameter**

```go
func AwardDefenceProgression(c *characters.Character, userId int, defenceType string, multiplier float64) {
	if c == nil {
		return
	}
	skill, stat := DefenceSkillAndStat(defenceType)
	if skill == "" {
		return // unrecognised defence awards nothing rather than guessing
	}
	c.OnSkillUseScaled(skill, userId, multiplier)
	c.OnStatUseScaled(stat, userId, multiplier)
	// ... parry's second stat, unchanged, also scaled
}
```

Update every existing caller to pass `1.0` **in this task**, so it is a provable
no-op until Task 10 wires the fraction.

Run: `grep -rn "AwardDefenceProgression(" internal/ --include=*.go | grep -v _test.go`

- [ ] **Step 3: Run, update the census, commit**

---

# Phase B: wire combat, and unify the two defence paths

## Task 10: Populate `Defended`; unify both defence paths onto best-quoted

**Files:** `internal/combat/defence_multiplier.go`, `internal/hooks/NewRound_DoCombat_helpers.go`, create `internal/hooks/defence_unification_test.go`

- [ ] **Step 1: Read what each path does now**

Run: `sed -n '30,70p' internal/hooks/NewRound_DoCombat_helpers.go`
Run: `sed -n '415,430p' internal/combat/defence_multiplier.go`

⚠️ `defenceTypesUsed` collects `se.DefenseUsed != DefenseNone`, stamped only in
the defended-**win** branches. In a round where the defender was hit by
everything it returns an **empty slice**, so there is no skill to name and
`progression.Event` forbids an empty `Skill`. That is precisely why the unified
rule is best-**quoted** rather than best-won: `runBestOfAllDefense` rolls every
available defence and picks a best even on a loss.

⚠️ **Directions of travel are opposite.** The channel path paid a FULL event win
or lose, so it takes a **cut**. Melee awarded only on a win, so it takes a
**gain**. Do not describe either as "the" change.

- [ ] **Step 2: Write both failing tests**

```go
// TestMeleeDefenceAwardsOnALostRound pins the unification. Melee used to award
// only a defence that WON, so a round where the defender was hit by everything
// taught nothing and had no skill to name.
func TestMeleeDefenceAwardsOnALostRound(t *testing.T) {
	// Drive a round in which no defence lands; assert the best-quoted defence
	// receives an award carrying ProgressionFailureFraction.
}

// TestChannelDefenceAwardsFractionOnALoss pins the other direction: this path
// paid a FULL event win or lose, so this is a cut.
func TestChannelDefenceAwardsFractionOnALoss(t *testing.T) {
	// Drive a lost channel defence; assert the award carries the fraction.
}
```

`internal/hooks/NewRound_DoCombat_parity_test.go` builds combatants and drives
rounds; follow its setup rather than inventing one.

- [ ] **Step 3: Melee awards the best-quoted defence**

Expose the quoted defence from `runBestOfAllDefense` so it is available even on
a loss, and have `processDefenderProgression` award it at `1.0` when it won and
the fraction when it did not.

- [ ] **Step 4: Channel passes the fraction on a loss**

⚠️ The Outcome-building function in `defence_multiplier.go` awards
`progression.BonusEvents`, **not** `OrdinaryEvents`, with a comment explaining
that asking for ordinary events there would double-award. The ordinary defender
award is the `AwardDefenceProgression` call in the defence-won branch. Pass
`1.0` there and add a matching call on the **lost** branch with the fraction.
**Do not touch the `BonusEvents` line.**

- [ ] **Step 5: Populate `Defended` on every production Outcome**

Run: `grep -rn "progression.Outcome{" internal/ --include=*.go | grep -v _test.go`

Each either sets `Defended` or is justified in the commit.
`internal/actions/combat_fire.go`'s is a second, skullduggery-only Outcome on
the shoot path; `NewRound_DoCombat_unified.go`'s `bonusOut` is a bonus-tier
Outcome where `Defended` is meaningless.

- [ ] **Step 6: Run, commit**

---

## Task 11: A defended melee swing awards the attacker

**Files:** `internal/hooks/NewRound_DoCombat_unified.go`, create `internal/hooks/attacker_fraction_test.go`

⚠️ The attacker's ordinary events sit inside
`for _, wh := range res.WeaponHits { if !wh.CleanHit { continue } ... }`. Until
that gate changes, setting `Defended` changes nothing, because no Outcome is
built for a defended swing at all. **This is the single most-missed defect
across both previous plan versions.**

- [ ] **Step 1: Write the failing test**

```go
// TestDefendedSwingAwardsTheAttackerTheFraction is the end-to-end proof that
// the headline rule is wired on the melee attacker path. Two previous plan
// versions shipped this unwired with every gate green.
func TestDefendedSwingAwardsTheAttackerTheFraction(t *testing.T) {
	// Drive a round whose swings are all defended; assert the attacker's weapon
	// skill receives an award carrying ProgressionFailureFraction.
}
```

- [ ] **Step 2: Replace the `continue` with a scaled award**

Clean hit at 1.0, defended swing at the fraction, preserving the per-weapon-hit
shape so a two-weapon round still awards per hit.

- [ ] **Step 3: Run, commit**

---

# Phase C: static-difficulty conversions

Each task: convert the roll, add BOTH guard entries in the same step, award
through `Actor.AwardResolved`.

## Task 12: `search.go`'s four difficulty checks

**Files:** `internal/actions/search.go`, `contest_floor_guard_test.go`, `internal/combat/contest_site_guard_test.go`, create `internal/actions/search_contest_test.go`

- [ ] **Step 1: Write the failing test**

Assert `contest.AgainstDifficulty(` appears **4** times and
`dice.RollStat(searchScore)` appears **2** times (the hidden-actor sites, which
Task 13 converts).

⚠️ Task 13 drives that second count to 0 and **must update this assertion in the
same commit**. An earlier draft left this test failing after Task 13 and still
claimed PASS.

- [ ] **Step 2: Convert the four**

Secret exits (125), hidden containers (125), stashed items (135), hidden nouns
(175):

```go
		rolledAgainstSomething = true
		if contest.AgainstDifficulty(searchScore, 125.0).Success {
```

- [ ] **Step 3: Add BOTH guard entries in this step**

Learn the exact `contestSiteOwners` keys by running the guard:

Run: `go test . ./internal/combat/ -run "Floored|Owned" -v`

- [ ] **Step 4: Run all three, commit**

---

## Task 13: A hider's skill counts when someone searches for them

**Files:** `internal/actions/search.go`, `internal/actions/search_contest_test.go`, create `internal/actions/search_hidden_test.go`

⚠️ **Do not invent a score helper.** `internal/actions` already exports
`CalcDetectionScore` and `CalcSneakScoreVsObserver`, and `usercommands/go.go`'s
four hidden-detection sites use them. Reuse those; a third composition would
recreate the disagreement this task exists to end.

Run: `grep -n "CalcDetectionScore\|CalcSneakScoreVsObserver" internal/actions/*.go internal/usercommands/go.go`

- [ ] **Step 1: Write the failing test**

Assert `search.go` no longer contains `dice.RollStat(searchScore)`, and that a
high-sneak hider is spotted strictly less often than a low-sneak one **through
the real search path**, not through a bare `RunContest` call.

- [ ] **Step 2: Convert both sites to opposed contests** using the existing helpers

⚠️ Opposed, so `combat.RunContest` is correct and needs **no**
`guardedRollExemptions` entry. `contestSiteOwners` entries still apply.

- [ ] **Step 3: Update Task 12's count assertion to 0 in this commit**
- [ ] **Step 4: Run, commit**

Run: `go test ./internal/actions/ ./internal/usercommands/ ./internal/behaviortree/ ./internal/combat/ -v`

---

## Task 14: `search` awards once per command, through the seam

**Files:** `internal/actions/search.go`, create `internal/actions/search_award_test.go`

- [ ] **Step 1: Write both failing tests**

A room with five hidden things awards the same number of events as a room with
one; a fruitless-but-resolved search awards the fraction rather than nothing.

- [ ] **Step 2: Replace the award**

```go
	// ONE event per resolved COMMAND, not per internal roll: search runs up to
	// six checks and a rich room must not pay six times a bare one. A command
	// that rolled and won nothing pays the failure fraction.
	if rolledAgainstSomething {
		actor.AwardResolved(string(skills.Search), foundAnything)
	}
```

Set `foundAnything` alongside each existing `result.*Found` append.

- [ ] **Step 3: Run, update the census, commit**

---

## Task 15: `track` and `forage`

**Files:** `internal/actions/track.go`, `internal/forager/forage_core.go`, both guard files, create two test files

- [ ] **Step 1: Write the failing tests**: bare threshold gone, `AgainstDifficulty` reached, `NOTE(unassigned` removed, resolved failure awards the fraction

- [ ] **Step 2: Convert `track.go` with ONE roll**

⚠️ `track.go` grades a trail against **two** thresholds (125 and 175) from a
**single** roll, stored in `result.RollValue` and read at `track.go:253` and
`:292` against `175.0`. Take one `contest.AgainstDifficulty(searchScore, 125.0)`
and grade the tier from the same result; rolling twice changes the tier
distribution silently.

⚠️ `contest.Result.Margin` is a **normalised** margin, not a stat-scale number,
so feeding it to those `175.0` comparisons makes them always-false. Either keep
a stat-scale value in `RollValue` or convert both comparison sites here.

- [ ] **Step 3: Convert `forage_core.go`**, keeping the per-biome difficulty unchanged
- [ ] **Step 4: Award through `AwardResolved` in both**
- [ ] **Step 5: Guard entries, run, update the census, commit**

---

## Task 16: The authored material tier

**Files:** `internal/items/itemspec.go`, create `internal/items/material_tier.go` and two test files, `internal/configs/*`

- [ ] **Step 1: Write the failing tests**

Tier 1 gives 0.75, tier 3 gives 1.0, tier 5 gives 1.25. Untiered (0) gives
**1.0, neutral**, never the cheapest.

- [ ] **Step 2: Add `ItemSpec.MaterialTier` and `MaterialTierMultiplier`**

```go
MaterialTier int `yaml:"material_tier,omitempty"` // 1 (common) to 5 (rarest). Modifies craft difficulty. 0 = untiered, neutral. NOT RarityTier, which is a vendor stock cap where higher means MORE common.
```

- [ ] **Step 3: Guard new materials, grandfathering existing ones**

Scan **all** item directories, not just `materials-40000`: `MaterialTier` is on
`ItemSpec`, so a new material under `consumables-30000` would otherwise escape.

Run: `grep -rL "material_tier:" _datafiles/world/dogmud/items/**/*.yaml | xargs -n1 basename | sort`

⚠️ On day one **all** materials are untiered, so every multiplier is 1.0 and
difficulty is `100 + SkillMinimum * 5`. That is a coherent shipping state
because `SkillMinimum` carries difficulty. Say so in the playtest goals so a
tester does not read "materials do not matter yet" as a broken design.

- [ ] **Step 4: Run, commit**

---

## Task 17: Deterministic ingredient resolution

⚠️ Real types: the recipe type is **`RecipeSpec`**, inventories are
**`[]items.Item`**, and there is no `Inventory` type:

```go
func HasIngredients(inv []items.Item, componentInv []items.Item, recipe *RecipeSpec) (bool, string)
func ConsumeIngredients(inv []items.Item, componentInv []items.Item, recipe *RecipeSpec) ([]items.Item, []items.Item)
```

**Files:** `internal/crafting/crafting.go`, create `internal/crafting/ingredient_selection_test.go`

- [ ] **Step 1: Write the failing test**: resolve the same recipe against the same inventory fifty times, assert identical item ids every time

- [ ] **Step 2: Implement**

```go
// SelectIngredientItems resolves a recipe's ingredient tags to the CONCRETE
// items that will be consumed, deterministically.
//
// Determinism matters twice. FindSpecByComponentTag iterates a Go map, and four
// items share component_tag "bottle", so tag lookup would re-roll craft
// difficulty every attempt. And difficulty must ride on what the player
// actually consumes, not on the recipe's declared tag.
//
// Ties break on the LOWEST ItemId so the choice is stable across runs.
func SelectIngredientItems(inv []items.Item, componentInv []items.Item, recipe *RecipeSpec) []items.Item
```

- [ ] **Step 3: Add the tier selector**

```go
// HighestIngredientTier returns the material-tier multiplier of the
// HIGHEST-TIERED item among those actually selected.
//
// Tier, not gold value: gold value was measured across all 126 recipes and
// found uncorrelated with recipe tier.
func HighestIngredientTier(selected []items.Item) float64
```

⚠️ Named for **tier**, not "dearest". An earlier draft called it
`DearestIngredientTier`, a price word that would have re-admitted gold value
through the back door.

- [ ] **Step 4: Have `ConsumeIngredients` take the selection**, so the roll and the consumption cannot disagree. Enumerate and update its callers.
- [ ] **Step 5: Run, commit**

---

## Task 18: Craft resolves as a floored contest and awards both ways

**Files:** `internal/crafting/crafting.go`, `internal/hooks/NewRound_UserRoundTick.go`, `NewRound_MobRoundTick.go`, `internal/mobs/crafter.go`, three existing test files, both guard files, create `internal/crafting/craft_contest_test.go`

- [ ] **Step 1: Write three failing tests**

```go
// TestCraftAtRecipeMinimumIsFiftyFifty pins the anchor. CraftBaseDifficulty is
// 100 because 100 is the human stat baseline, so difficulty reads as "a
// baseline human holding exactly the recipe's minimum skill", and such a
// crafter is 50/50 -- today's shipped CraftingBaseSuccessChance.
func TestCraftAtRecipeMinimumIsFiftyFifty(t *testing.T) {
	pinConfigForTest(t)

	score := CraftScore(100, 10)           // stat 100, skill 10
	difficulty := CraftDifficulty(10, 1.0) // skill_minimum 10, neutral tier

	if score == 0 || difficulty == 0 {
		t.Fatal("score or difficulty is zero; the config knobs are not wired")
	}
	if score != difficulty {
		t.Fatalf("score %v != difficulty %v at the minimum", score, difficulty)
	}
}

// TestAdvancedRecipesNeedMoreMastery pins the owner's intended curve: nine
// levels above the minimum reads ~93% on a novice recipe and ~73% on a master
// one, because success depends on the RATIO of difficulty to score.
func TestAdvancedRecipesNeedMoreMastery(t *testing.T) {
	pinConfigForTest(t)

	novice := CraftDifficulty(0, 1.0) / CraftScore(100, 9)
	master := CraftDifficulty(40, 1.0) / CraftScore(100, 49)

	if novice == 0 || master == 0 {
		t.Fatal("ratios are zero; the config knobs are not wired")
	}
	if !(novice < master) {
		t.Fatalf("novice ratio %v is not below master %v; an advanced recipe "+
			"must need more mastery to reach the same reliability", novice, master)
	}
}

// TestFailedCraftAwardsTheFraction is the case the whole slice is justified by.
func TestFailedCraftAwardsTheFraction(t *testing.T) {
	// Force a failed craft; assert the recipe skill received an award carrying
	// ProgressionFailureFraction.
}
```

⚠️ The zero guards are load-bearing: without them the formula tests pass against
unwired knobs, since `0 == 0`.

- [ ] **Step 2: Implement the formulas**

```go
// CraftScore composes the crafter's side the way EVERY other score in the game
// is composed: stat + skill * SkillWeight. Do not invent a bespoke formula; an
// earlier draft did, and it cancelled the recipe term out of the odds and
// ignored the crafter's stat entirely.
func CraftScore(primaryStat float64, craftSkill int) float64

// CraftDifficulty reads as "a baseline human holding exactly this recipe's
// minimum skill", scaled by the tier of the materials in hand.
//
// CraftBaseDifficulty is 100 because 100 is the human stat baseline.
func CraftDifficulty(recipeSkillMinimum int, materialTierMult float64) float64
```

Confirm the shared skill-weight knob's real name before using it; if there is no
shared one, add `CraftSkillWeight` defaulting to 5 and say why in its comment.

- [ ] **Step 3: Route the four call sites through a floored contest**

```go
selected := crafting.SelectIngredientItems(user.Character.Items, user.Character.ComponentItems, recipe)
res := contest.RunWithFloors(
	crafting.CraftScore(primaryStat, sl),
	[]contest.Entry{{Score: crafting.CraftDifficulty(recipe.SkillMinimum, crafting.HighestIngredientTier(selected))}},
	float64(configs.GetBalanceConfig().CraftFloor),
)
if res.Success {
```

⚠️ `RunWithFloors`, not `AgainstDifficulty`: craft needs the mercy band its
`[5%, 95%]` clamp gives today. The extremes match exactly; the interior
compresses by `(1 - 2f)`, so a true 80% reads as 77%.

- [ ] **Step 4: Award on BOTH branches via `AwardResolved`**
- [ ] **Step 5: Retire `CalcSuccessChance` and fix its THREE test callers**

- `internal/crafting/crafting_test.go` (`TestCalcSuccessChance`)
- `internal/crafting/integration_crafting_test.go` (four calls)
- `internal/mobs/crafter_test.go`, where `forceCraftSuccess`/`forceCraftFailure`
  drive `CraftingBaseSuccessChance`/`Min`/`Max` **by string key** through
  `configs.AddOverlayOverrides`. **Map keys are strings, so this compiles and
  silently stops working.** Replace the lever with `CraftFloor` plus an extreme
  `CraftBaseDifficulty`, and assert the forced outcome actually occurs.

- [ ] **Step 6: Guard entries for THREE new caller files**

The `RunWithFloors` calls live at the call sites, so
`internal/hooks/NewRound_UserRoundTick.go`, `internal/hooks/NewRound_MobRoundTick.go`
and `internal/mobs/crafter.go` each need an exemption and owner entries.

- [ ] **Step 7: Run, update the census, commit**

---

## Task 19: Mob crafters award under the same rule

⚠️ Mob crafters **bypass the `SkillMinimum` attempt gate** that
`usercommands/craft.go` applies, and ship at skill 1 against recipes with
minimums up to 65. With `stat + skill*5` the score stays positive there, which
is one reason the retired bespoke formula was wrong (it went negative).

- [ ] **Step 1: Write the failing test**, forcing a failed mob craft
- [ ] **Step 2: Award on both branches at both sites** (`crafter.go:505`, `:546`)
- [ ] **Step 3: Record the restock effect**

Run `go test ./internal/mobs/ ./internal/shops/ -v` and note in the commit the
new success rate at skill 1 against a neutral tier for the `skill_minimum` 3 to
8 band. Dynamic pricing keys on the stock-to-restock ratio, so a throughput drop
pins those goods near the price ceiling world-wide.

---

## Task 20: Salvage resolves as a floored contest and awards through the seam

⚠️ `SalvageMinChance`/`SalvageMaxChance` default in **`config.balance.misc.go`**
and are read at `internal/actions/salvage.go:83-84`. The award sites are
`salvage.go:166` and `:252`, and **no previous plan version touched them**.

**Files:** `internal/crafting/salvage.go`, `internal/actions/salvage.go`, `internal/configs/*`, both guard files, create `internal/crafting/salvage_contest_test.go`

- [ ] **Step 1: Write the failing tests**

Dear materials are harder to reclaim than cheap ones; per-unit recovery at high
skill stays in a **band** around today's 85% ceiling; a salvage that recovers
nothing awards the fraction.

⚠️ Assert the ceiling as a band (say 0.80 to 0.88), not merely `<= 0.90`. A
one-sided bound passes against an implementation with no ceiling logic that
happens to lose.

- [ ] **Step 2: Implement, mirroring craft**

`SalvageScore(primaryStat, salvageSkill)`, `SalvageDifficulty(tierMult)`, and
`RollSalvageUnit` calling `contest.RunWithFloors` with `SalvageFloor` 0.15.

⚠️ `RollSalvageReturns(ingredients []RecipeIngredient, chance float64)` and
`RollSalvageReturnsFromSpec(returns []items.SalvageReturn, chance float64)` take
a precomputed chance, and `RecipeIngredient` carries only `ItemTag`/`Quantity`.
Both signatures change. **Do not resolve tiers from a tag inside these
functions**. That is `FindSpecByComponentTag` again, the map-order function
Task 17 exists to forbid. Resolve concretely at the call site and pass items in.

- [ ] **Step 3: Award ONCE per salvage command via `AwardResolved`**, won if anything was recovered
- [ ] **Step 4: Retire the two clamps**, keeping `SalvageSoftCap`, `SalvageGoldPerRound`, `SalvageMaxRounds`
- [ ] **Step 5: Guard entries, run, update the census, commit**

⚠️ **Check the material sink.** Today retention per craft-then-salvage cycle is
about 0.81 at skill 50. Compute the new figure at skill 0, 15, 25 and 50 and
record all four in the commit. The endpoints matching is **not** sufficient: the
old curve was `sqrt` in skill and the new one is a normal CDF of a ratio, which
saturates earlier, so the mid band can weaken while both ends look right.

---

# Phase D: remaining sites

## Task 21: The sixteen skullduggery sites

`actions/steal.go` x3, `actions/plant.go` x3, `actions/shadow.go` x2,
`usercommands/skill.skullduggery.sneak.go` x2, `usercommands/picklock.go` x2,
`actions/defuse.go`, `usercommands/throw.go`, `mobcommands/flee.go`,
`hooks/NewRound_DoCombat_helpers.go`.

Four bypass every entry point via direct `CheckSkillProgression`: both sneak
sites (one is the **failure** branch) and both picklock sites.

⚠️ **`picklock` is not a contest.** Both sites fire on success of a pin
minigame, not an opposed roll, so there is no resolved loss to pay a fraction
to. Route it through the seam as a win-only `AwardResolved(skill, true)` and say
so in the commit.

- [ ] **Step 1: Write the failing tests**: a source assertion that the two files no longer call `CheckSkillProgression`, and a behavioural one that a failed steal awards the fraction
- [ ] **Step 2: Convert each site**

⚠️ Two traps: `Outcome` holds exactly **one** `AttackerSkill`, so a site
awarding both a combat skill and skullduggery needs **two** Outcomes; and
`SkillPrimaryStats["skullduggery"] == "dexterity"`, the same as weapon-combat,
so awarding both rolls dexterity twice unless the second Outcome's `Stat` is
left empty.

- [ ] **Step 3: Run everything, update the census, commit with EXPLICIT paths**

⚠️ Never `git add internal/`. Sixteen code-touching tasks precede this one.

---

## Task 22: The mob spell path adopts the player path's gates

Player: self-cast penalty, area-cast-with-no-targets zeroing, `spellBonus > 0`.
Mob: none of the three.

- [ ] **Step 1: Write three BEHAVIOURAL tests**, one per gate

⚠️ Do **not** assert the mob block contains the gate literals. Step 2 extracts
them into a shared helper, after which the block contains a call, so such a test
fails on a correct implementation.

- [ ] **Step 2: Extract ONE helper, call it from both paths.** Do not copy the gates.
- [ ] **Step 3: Run, commit**

---

# Phase E: deletions

## Task 23: Delete the stranded mob-follow roll

⚠️ The `if !isSneaking {` block at `go.go:664` contains **both** the
`GetMobs(FindFightingPlayer)` pursuit loop (668-697) **and** a
`behaviortree.TryRoomBehavior` call (700-705). Delete **only the loop**.

⚠️ `go.go` has a second `TryRoomBehavior` call at line 357, so an assertion that
merely greps for the string passes even if the destination one is deleted. Pin
the destination call specifically.

⚠️ `internal/usercommands/go_test.go` does not exist; create a new file.

---

## Task 24: Delete first-kill progression

Remove `Character.OnFirstMobKill`, both call sites in `Death_MobKillCredit.go`,
and the message "Defeating a new foe hones your combat instincts!". **Keep
`KD.AddMobKill`**, which feeds the bestiary. Update the census.

---

## Task 25: Dead crit stubs and stale comments

- [ ] **Step 1: Delete the stubs from ALL NINE files**

```
internal/actions/{consider,economy,forage,salvage,scan,search,sleep,track}_test.go
internal/hooks/spell_foldanchor_test.go
```

Verify: `grep -rl "func.*OnCritical" internal/` returns nothing.

- [ ] **Step 2: Remove them from the DENY-list**

They sit in `progressionCalls` in `internal/progression/seam_guard_test.go`,
**not** in `allowedDirectProgression`. Removing rows from the allow-list instead
would silently break the U9 guard.

- [ ] **Step 3: Fix the two stale `defence_multiplier.go` comments**

One wrongly claims a forced-crit defence "was still progressed exactly as on the
melee path" (the melee path returns before `DefenseUsed` is stamped). The other
cites a `Margin` negation at a line holding an unrelated function. Prefer
describing behaviour to citing lines.

⚠️ **`OnCritReceived` is NOT deleted by this slice** and stays a live entry
point. An earlier draft told the implementer to re-anchor
`TestCritReceivedProgression_DecaysWithRank` because "this task removes
`OnCritReceived`". A false premise. **Leave that test alone.**

---

# Phase F: guard, docs, verify

## Task 26: The standing guard

- [ ] **Step 1: Scope the scanner narrowly**

⚠️ `util.Rand(` has **210** production sites; `dice.RollStat(` has 14. A scanner
flagging every `util.Rand` turns disposition into a multi-day triage of loot
tables, spawn rolls and weather. Scope to: a roll whose result is compared
against a threshold **to decide whether an action succeeded**. Use Task 2's
`callNamesInSource`.

- [ ] **Step 2: Write the guard** with a file-keyed allow-list carrying a reason per entry: `actions_progression.go` (authored tutorial grant), `item_procs.go` (spec 7), `NewRound_AutoHeal.go` (regen, U10b-2), `NewRound_DoCombat_helpers.go` and `usercommands/target.go` (target switch, U12).

- [ ] **Step 3: Prove the scanner is not blind BEFORE trusting a pass**

Feed it a source string containing a known bare threshold roll and assert it is
reported. **If the guard passes on its first run before this exists, assume the
scanner is blind, not the tree clean.**

- [ ] **Step 4: Disposition each offender, run, commit**

---

## Task 27: Breadcrumbs and the roadmap

- [ ] **Step 1: Breadcrumb `item_procs.go` and `handleMobWeaponPickup`**
- [ ] **Step 2: `NOTE(U12)` at both `ChanceToSwitchTarget` roll sites**
- [ ] **Step 3: ADD a Category B row to the roadmap**

⚠️ There is **no "Category B" row** in `UNIFIED_RESOLUTION_ROADMAP.md` today;
only "Category C" appears, inside U10b's prose cell. Verify with
`grep -in "category b" docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` and **add**
a row rather than editing one. Record: 20 sites, not 8; U10b-1 converted 15; U12
owns 2; 3 stay off-core deliberately; and that `contest.AgainstDifficulty`
having no callers was never a gap to close for its own sake, since
static-difficulty rolls are deliberately unfloored.

---

## Task 28: Re-solve every fitted multiplier

⚠️ **The direction differs per path**, so a single headline number is wrong:

| Path | Direction |
|---|---|
| Melee attacker | +26% (0.5752 to 0.7239 events per swing) |
| Melee defence | a gain, shrinking with swing count (roughly +17% at two swings, +8% at three, since it fires once per ROUND) |
| Channel defence | a **cut**: it paid a full event win or lose before |

⚠️ `combat-analytics.jsonl` is **combat-only**, so search, track, forage,
salvage and skullduggery have **no measurement basis**. Re-solve what the data
covers; for the rest record the reasoning and defer tuning to the playtest.

⚠️ `TrackSkillUse`/`TrackStatUse` still fire at full weight on a fractional
event, so the use counters inflate faster than the rolls do. They are telemetry
now, but `u10b_solve_v3.py` was fitted against them; check whether the solver
reads counters or rolls before trusting its output.

---

## Task 29: Docs, `context.md`, patch notes

`internal/progression` gains `OrdinaryEventsScaled` and `Outcome.Defended`;
`internal/characters` gains `AwardResolved` and `OnStatUseScaled`;
`internal/actions` gains `Actor.AwardResolved`; `internal/crafting` gains
`CraftScore`, `CraftDifficulty`, `SelectIngredientItems`,
`HighestIngredientTier`, `SalvageScore`, `SalvageDifficulty` and loses
`CalcSuccessChance`; `internal/items` gains `MaterialTierMultiplier` and
`ItemSpec.MaterialTier`; `internal/combat`'s `AwardDefenceProgression` changes
signature.

⚠️ Verify every symbol exists before naming it.

Patch notes: player-facing, no raw numbers, no em dashes, 80 columns. Failing at
something teaches you a little; searching, tracking, foraging, crafting and
salvaging resolve like everything else; how hard a craft is depends on the
recipe and the materials; someone good at hiding is harder to find. **Do not
promise that mobs will chase you.** Task 23 deletes a roll and adds no pursuit.

---

## Task 30: Pre-push verification and the adversarial playtest

- [ ] **Step 1:** `gofmt -l internal/ modules/` prints nothing
- [ ] **Step 2:** `go build ./... && go test ./... 2>&1 | grep -v "^ok" | head -40`
- [ ] **Step 3:** `Logging.LogToFile: false`, built from the `git show HEAD:` blob
- [ ] **Step 4: Boot test in an isolated detached worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

⚠️ **Exit code 124 is the success case.** Never grep for the bare word `panic`;
`MapConsistencyEnforce` legitimately has the value `panic`.

- [ ] **Step 5: Adversarial playtest, reporting FIVE signals separately**

1. **The convention.** Does failing at something visibly teach a little?
2. **Defence unification.** Do dodge, parry, block, quell and defy still improve
   at a reasonable pace? Channel defences took a **cut** here.
3. **Crafting feel.** Does difficulty track the recipe? Does an advanced recipe
   need more mastery than a simple one? **No material is tiered yet**, so the
   material half is neutral until the backfill lands.
4. **Search, track and forage odds.** Weak searchers improve sharply; experts
   lose near-certainty.
5. **The economy.** The craft-then-salvage material sink, and shop restock with
   mob crafters at skill 1.

⚠️ **The AI port caps 3 commands per round and silently discards the overflow
after echoing it.** Send one command per batch and verify output.

- [ ] **Step 6: Fix findings, extract them to memory** (reports are gitignored)
- [ ] **Step 7: Open the PR** with `--repo pruuk/DOGMud` on every `gh` call
