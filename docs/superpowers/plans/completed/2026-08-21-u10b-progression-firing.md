> # 🛑 ABANDONED — DO NOT EXECUTE THIS PLAN
>
> **This plan failed its blind adversarial review on 2026-08-21 and was never
> executed. It is kept as a record of an approach that did not work out, not as
> work to pick up.** The instruction block immediately below is the plan's
> original header and is no longer in force — ignore it.
>
> **What shipped instead:** U10b was re-cut on top of a new prerequisite slice
> and delivered as five sub-slices — U10b-0 (PRs #55–#60), U10b-1 (#70),
> U10b-1b (#74), U10b-2 (#75), U10b-3 (#76). The live design is
> `docs/superpowers/specs/completed/2026-08-26-u10b-1-progression-firing-convention-design.md`.
>
> **Why it did not work out.** Three blind reviewers converged on four blockers,
> each re-verified against source before being accepted:
>
> 1. **The uncontested class was inverted.** It tracked a full use but rolled at
>    10%, flooding the shared use counter that decays every other site on the
>    same stat — the opposite of the intended effect.
> 2. **The premise itself was then killed by the owner.** The whole design rested
>    on use counters driving progression rank. That was replaced by *training*
>    as the rank, which dissolved Tasks 2 and 4 outright and became the
>    prerequisite slice U10b-0.
> 3. **Four production sites bypass the seam entirely**, calling
>    `CheckSkillProgression` directly — including the *player* sneak failure
>    branch, which this plan would have left untouched while claiming full
>    coverage.
> 4. **Three of its test assets could not fail for the bugs they named.** Two
>    named fixtures did not exist in the repo at all, and it never mentioned
>    `internal/progression/seam_guard_test.go`, the closer prior-art guard its
>    own tasks would have left stale.
>
> The taxonomy it proposed — three firing classes — is also not what shipped.
> The settled rule is **best-of: one event per resolved action for the single
> highest-rolling candidate, full on a win and a fraction on a loss**, with
> crits and fumbles as a separate bonus layer.

# U10b — Progression Firing Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse eleven progression firing conventions into three classes —
contested (fires on a win), uncontested (low chance, high frequency), and the
existing crit/fumble bonus layer — with a guard test that pins every production
site to its class.

**Architecture:** Add one new seam (`OnStatUseUncontested` /
`OnSkillUseUncontested`) beside the existing `OnStatUse` / `OnSkillUseScaled`,
move the §3.2 sites onto it, gate the §3.1 sites on success, fold regen's
bespoke chance formula into the standard applier by passing its depletion
magnitude as the multiplier, delete three dead paths, then lock the whole
arrangement with an AST-walking class guard modelled on U6b's
`contestSiteOwners`.

**Tech Stack:** Go, `internal/characters` (progression core),
`internal/progression` (pure event package), `internal/actions` (Actor parity),
`internal/configs` (balance knobs), `go/ast` + `go/parser` for the guard test.

**Spec:** `docs/superpowers/specs/completed/2026-08-21-u10b-progression-firing-design.md`
(approved 2026-08-21). Read §2 (the rule), §3 (class assignment), §7 (owner
decisions) and §8 (done-when) before starting.

---

## Before you start — three things that will bite you

**1. `OnSkillUseScaled` rolls the primary stat at an UNSCALED 1.0.**
`internal/characters/progression.go:281-283` calls `c.OnStatUse(primaryStat,
userId)` with no multiplier. So you CANNOT implement "uncontested skill use" as
`OnSkillUseScaled(skill, uid, uncontestedRate)` — the skill would be damped and
its governing stat would still roll at the full contested rate. Task 2 builds a
separate method for this reason. If you find yourself writing
`OnSkillUseScaled(x, uid, rate)` for a §3.2 site, stop: that is the bug.

**2. The spec's §3 table is incomplete.** It was written from the August audit.
A fresh mechanical enumeration (Task 1) finds sites in `assess.go`,
`surprise_attack.go`, `mobcommands/flee.go`, `mobcommands/sneak.go`,
`characters/skills.go`, `NewRound_DoCombat_helpers.go:672`, and
`combat_throttle.go:185` that no §3 row names, plus THREE sites each in
`steal.go` and `plant.go` where §3 has one row. Task 1 reconciles this and is a
hard prerequisite for everything else. Do not skip it to "get to the real work".

**3. `config.yaml` has `skip-worktree` set.** Never build a commit from the copy
on disk — it desyncs both ways, and a stale disk copy has silently reverted
committed config before. Build the edit from `git show HEAD:_datafiles/config.yaml`.

Run `gofmt -l internal/ modules/` before every commit. It must print nothing.

---

### Task 1: Build the authoritative site inventory and reconcile it with §3

**Files:**
- Create: `docs/audits/2026-08-21-u10b-site-inventory.md`

This task writes no production code. It produces the list every later task
depends on, and it exists because the spec's §3 table under-counts.

- [ ] **Step 1: Enumerate every production progression site**

```bash
cd "C:/Users/Calabe Davis/workspace/DOGMud"
grep -rn "OnSkillUse\|OnStatUse\|OnSkillUseScaled\|ApplyProgression\|OnRegenTick\|OnFirstMobKill\|OnCritReceived" \
  --include=*.go internal/ modules/ \
  | grep -v "_test.go" \
  | grep -v "^internal/characters/progression.go" \
  | sort > /tmp/u10b-sites.txt
wc -l /tmp/u10b-sites.txt
```

Expected: roughly 100 lines, including lines where the symbol appears only in a
comment. Strip those by hand — a hit inside a `//` comment is documentation,
not a site.

- [ ] **Step 2: Write the inventory doc**

Create `docs/audits/2026-08-21-u10b-site-inventory.md` with this table. It is
already filled in from the enumeration run on 2026-08-21 at `1352caaad`;
re-run Step 1 and correct any line number that has moved.

```markdown
# U10b production progression site inventory (2026-08-21)

Mechanical enumeration at `1352caaad`. This supersedes the §3 table in the
U10b spec where they disagree — §3 was written from the August audit and
under-counts. "Class" is the spec §2 class; "Spec row" names the §3 row
that covers it, or NEW if §3 has none.

| File:line | Call | Class | Spec row |
|---|---|---|---|
| internal/actions/buy.go:786 | OnSkillUse(bartering) | contested | Bartering |
| internal/actions/buy.go:789 | OnStatUse(charisma) on shop mob | uncontested | Shop-mob charisma |
| internal/actions/sell.go:377 | OnSkillUse(Bartering) | contested | Bartering |
| internal/actions/sell.go:378 | OnStatUse(charisma) on shop mob | uncontested | Shop-mob charisma |
| internal/actions/combat_bash.go:142 | OnSkillUse(WeaponCombat) | contested | Special moves x11 |
| internal/actions/combat_drain.go:189 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_gore.go:144 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_grapple.go:127 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_hamstring.go:162 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_kick.go:205 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_maul.go:160 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_pounce.go:159 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_rake.go:159 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_throttle.go:214 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_trip.go:183 | OnSkillUse(UnarmedCombat) | contested | Special moves x11 |
| internal/actions/combat_throttle.go:185 | OnSkillUse(Spellcasting) on the THROTTLED target | ? | **NEW** |
| internal/actions/combat_taunt.go:200 | OnSkillUse(Rhetoric) on fumble | contested | Taunt |
| internal/actions/combat_taunt.go:277 | OnSkillUse(Rhetoric) on success | contested | Taunt |
| internal/actions/consider.go:27 | OnStatUse(perception) | uncontested | consider |
| internal/actions/defuse.go:129 | OnSkillUse(Skullduggery) | contested | Steal/plant/defuse |
| internal/actions/forage.go:142 | OnSkillUse(Search) | contested | Forage |
| internal/actions/mutation_venom_coat.go:34 | OnSkillUse(WeaponCombat) | uncontested | Venom-coat |
| internal/actions/plant.go:142 | OnSkillUse(Skullduggery) | contested | Steal/plant/defuse |
| internal/actions/plant.go:274 | OnSkillUse(Skullduggery) | contested | Steal/plant/defuse |
| internal/actions/plant.go:351 | OnSkillUse(Skullduggery) | contested | Steal/plant/defuse |
| internal/actions/salvage.go:166 | OnSkillUse(Salvage) | contested | Salvage |
| internal/actions/salvage.go:252 | OnSkillUse(Salvage) | contested | Salvage |
| internal/actions/search.go:243 | OnSkillUse(Search) | contested | Search |
| internal/actions/shadow.go:101 | OnSkillUse(Skullduggery) | contested | Shadow |
| internal/actions/shadow.go:150 | OnSkillUse(Skullduggery) | contested | Shadow |
| internal/actions/skill_helpers.go:102 | OnSkillUse(Rhetoric) warcry/rally | uncontested | Warcry/rally |
| internal/actions/steal.go:183 | OnSkillUse(Skullduggery) | contested | Steal/plant/defuse |
| internal/actions/steal.go:374 | OnSkillUse(Skullduggery) | contested | Steal/plant/defuse |
| internal/actions/steal.go:472 | OnSkillUse(Skullduggery) | contested | Steal/plant/defuse |
| internal/actions/surprise_attack.go:360 | OnSkillUse(Skullduggery) | contested | **NEW** (U10d owns the mechanic; the CLASS is U10b's) |
| internal/actions/track.go:128 | OnSkillUse(Search) | contested | Track |
| internal/characters/skills.go:87 | OnSkillUse(UnarmedCombat) | ? | **NEW** |
| internal/combat/defence_multiplier.go:166-171 | AwardDefenceProgression internals | contested | Melee/channel defence |
| internal/combat/defence_multiplier.go:541-542 | ApplyProgression bonus tier | bonus | §3.3 |
| internal/combat/skill_moves.go:193 | ApplyProgression defender | contested | Special moves x11 |
| internal/hooks/combat_shared_helpers.go:149 | ApplyProgression | contested | (verify) |
| internal/hooks/combat_shared_helpers.go:584 | ApplyProgression(Spellcasting) | contested | (verify) |
| internal/hooks/Death_MobKillCredit.go:61,86 | OnFirstMobKill | DELETED | §5 |
| internal/hooks/NewRound_AutoHeal.go:272,275,278 | OnRegenTick player x3 | uncontested | Regen ticks x6 |
| internal/hooks/NewRound_AutoHeal.go:368,371,374 | OnRegenTick mob x3 | uncontested | Regen ticks x6 |
| internal/hooks/NewRound_DoCombat_helpers.go:385,393 | player cast | contested | (player spell path) |
| internal/hooks/NewRound_DoCombat_helpers.go:544,550 | mob cast | contested | §6 mob-spell gates |
| internal/hooks/NewRound_DoCombat_helpers.go:672 | OnSkillUse(Skullduggery) | ? | **NEW** |
| internal/hooks/NewRound_DoCombat_unified.go:659-660 | emitAttackerStatGain x2 | contested | Melee attacker stat |
| internal/hooks/NewRound_DoCombat_unified.go:670,678 | attacker ordinary | contested | Melee attacker skill |
| internal/hooks/NewRound_DoCombat_unified.go:720-721 | bonus tier | bonus | §3.3 |
| internal/hooks/NewRound_MobRoundTick.go:496 | OnSkillUseScaled(craft) | contested | Crafting x4 |
| internal/hooks/NewRound_UserRoundTick.go:592 | OnSkillUseScaled(craft) | contested | Crafting x4 |
| internal/mobcommands/craft.go:54 | OnSkillUseScaled(craft) | contested | Crafting x4 |
| internal/mobcommands/flee.go:54 | OnSkillUse(Skullduggery) | ? | **NEW** |
| internal/mobcommands/sneak.go:19 | OnSkillUse(skullduggery) | contested | Sneak |
| internal/mobs/crafter.go:505 | OnSkillUse(recipe.Skill) UNSCALED | contested | crafter.go unscaled pair |
| internal/mobs/crafter.go:546 | OnSkillUse(recipe.Skill) UNSCALED | contested | crafter.go unscaled pair |
| internal/usercommands/assess.go:134 | OnSkillUse(Manifestation) | ? | **NEW** |
| internal/usercommands/craft.go:142 | OnSkillUseScaled(craft) | contested | Crafting x4 |
| internal/usercommands/go.go:388 | OnSkillUse(Search) movement | uncontested | Movement-trains-search |
| internal/usercommands/look.go:85 | OnStatUse(perception) | uncontested | look |
| internal/usercommands/shoot.go:197 | OnStatUse(perception) | contested | Shoot perception |
| internal/usercommands/shoot.go:199 | OnSkillUse(RangedCombat) | contested | Shoot skill |
| internal/usercommands/throw.go:454 | OnSkillUse(Skullduggery) | contested | Throw |

**Out of scope, named so the next audit does not rediscover them** (spec §1):
the no-roll grant paths `IncreaseStat` (`Quest_HandleQuestUpdate.go:363`,
`pack_scaling.go:81`, `bridge.go:273`), `TrainSkill`, and `SetSkill`. These are
rewards, not use. They move a stat with no roll, no cap check and no use
tracking, and that is deliberate.
```

- [ ] **Step 2b: Resolve the five rows marked `?`**

Open each file and decide its class using §2's rule: did a roll happen and did
the actor come out ahead (contested), or is it an action/tick with no
opposition (uncontested)? Replace the `?` with the answer plus a one-line
justification.

- `internal/actions/combat_throttle.go:185` — this awards Spellcasting to the
  THROTTLED target, not the attacker. Decide whether being interrupted is a
  contested loss (trains nothing) or an uncontested exercise of the skill.
  U10 shipped it; do not silently reverse a U10 decision without saying so.
- `internal/characters/skills.go:87` — read the comment at `:84`, which already
  argues about lost contests. It may already be correct.
- `internal/hooks/NewRound_DoCombat_helpers.go:672`
- `internal/mobcommands/flee.go:54`
- `internal/usercommands/assess.go:134`

- [ ] **Step 3: Commit**

```bash
git add docs/audits/2026-08-21-u10b-site-inventory.md
git commit -m "docs(u10b): authoritative production progression site inventory"
```

---

### Task 2: The uncontested seam

**Files:**
- Modify: `internal/configs/config.balance.go` (add one knob near line 386)
- Modify: `internal/configs/config.balance.progression.go` (default + validation)
- Modify: `internal/characters/progression.go` (two new methods after `OnSkillUseScaled`, which ends at line 292)
- Modify: `internal/actions/actor.go` (interface, after line 55)
- Modify: `internal/actions/actor_user.go` (after line 78)
- Modify: `internal/actions/actor_mob.go` (after line 79)
- Test: `internal/characters/progression_uncontested_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/characters/progression_uncontested_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// TestUncontestedScalesBothSkillAndStat is the load-bearing test of the whole
// uncontested class. OnSkillUseScaled rolls the skill's PRIMARY STAT at an
// unscaled 1.0 (progression.go:281-283), so implementing "uncontested skill
// use" as OnSkillUseScaled(skill, uid, rate) damps the skill and leaves the
// stat at the full contested rate. This test fails if anyone does that.
func TestUncontestedScalesBothSkillAndStat(t *testing.T) {
	repoRootChdir(t)
	configs.ReloadConfig()

	rate := float64(configs.GetBalanceConfig().UncontestedProgressionRate)
	if rate <= 0 || rate >= 1.0 {
		t.Fatalf("UncontestedProgressionRate must be in (0,1) for this test to mean anything, got %v", rate)
	}

	skill := string(skills.Search)
	primary := skills.GetSkillPrimaryStat(skill)
	if primary == "" {
		t.Fatalf("test needs a skill with a primary stat; %s has none", skill)
	}

	c := newTestCharacter(t)

	contestedSkill := c.skillProgressionChance(skill, 1.0)
	contestedStat := c.statProgressionChance(primary, 1.0)

	uncontestedSkill := c.skillProgressionChance(skill, rate)
	uncontestedStat := c.statProgressionChance(primary, rate)

	if got, want := uncontestedSkill, contestedSkill*rate; !floatNear(got, want) {
		t.Errorf("skill chance: got %v, want %v (contested %v x rate %v)", got, want, contestedSkill, rate)
	}
	if got, want := uncontestedStat, contestedStat*rate; !floatNear(got, want) {
		t.Errorf("stat chance: got %v, want %v (contested %v x rate %v)", got, want, contestedStat, rate)
	}
}

// TestOnSkillUseUncontestedTracksUse pins that the uncontested class still
// moves the virtual rank. A class that rolls but never tracks is the
// rank-independence half of the fyttyn exploit.
func TestOnSkillUseUncontestedTracksUse(t *testing.T) {
	repoRootChdir(t)
	configs.ReloadConfig()

	c := newTestCharacter(t)
	skill := string(skills.Search)
	primary := skills.GetSkillPrimaryStat(skill)

	beforeSkill := c.GetSkillUseCount(skill)
	beforeStat := c.GetStatUseCount(primary)

	c.OnSkillUseUncontested(skill, 0)

	if c.GetSkillUseCount(skill) != beforeSkill+1 {
		t.Errorf("skill use count: got %d, want %d", c.GetSkillUseCount(skill), beforeSkill+1)
	}
	if c.GetStatUseCount(primary) != beforeStat+1 {
		t.Errorf("primary stat use count: got %d, want %d", c.GetStatUseCount(primary), beforeStat+1)
	}
}

func TestOnStatUseUncontestedTracksUse(t *testing.T) {
	repoRootChdir(t)
	configs.ReloadConfig()

	c := newTestCharacter(t)
	before := c.GetStatUseCount("perception")
	c.OnStatUseUncontested("perception", 0)
	if c.GetStatUseCount("perception") != before+1 {
		t.Errorf("stat use count: got %d, want %d", c.GetStatUseCount("perception"), before+1)
	}
}

func floatNear(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
```

**Test fixtures:** `repoRootChdir` and `newTestCharacter` are this package's
existing helpers — Go test binaries run with CWD set to their package
directory, so the config load needs the repo root. Open
`internal/characters/progression_faucet_test.go` for the exact names in use and
reuse them; if they differ, use the real ones rather than adding duplicates.
Likewise check whether `floatNear` already exists in the package before adding it.

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/characters/ -run 'Uncontested' -v
```

Expected: FAIL to compile — `UncontestedProgressionRate` undefined,
`OnSkillUseUncontested` undefined, `OnStatUseUncontested` undefined,
`skillProgressionChance` undefined.

- [ ] **Step 3: Extract `skillProgressionChance`**

`CheckSkillProgression` (`internal/characters/progression.go:71`) computes its
chance inline. Extract it exactly as `statProgressionChance` (`:158`) already
is, so the test pins production's expression rather than a copy:

```go
// skillProgressionChance computes the probability (0.0-1.0) that a skill
// progression roll succeeds for skillName at bonusMultiplier. Extracted so the
// uncontested-class tests pin the expression production rolls rather than a
// hand-rolled duplicate that could silently drift from it. Mirrors
// statProgressionChance.
//
// A mob at or past MobSkillCap, or with mob progression disabled, returns 0.
func (c *Character) skillProgressionChance(skillName string, bonusMultiplier float64) float64 {
	// Move the chance derivation out of CheckSkillProgression VERBATIM --
	// including the mob gating, the per-skill multiplier and the mutation
	// multiplier -- and have CheckSkillProgression call this. This step is a
	// pure extraction: change no term, and the existing skill-progression
	// tests must stay green.
}
```

Then rewrite `CheckSkillProgression` to call it. Run the package suite before
continuing — a "pure extraction" that changes behaviour is a bug:

```bash
go test ./internal/characters/
```

Expected: PASS (extraction only).

- [ ] **Step 4: Add the knob**

`internal/configs/config.balance.go`, beside the other progression knobs
(near lines 386-392):

```go
	UncontestedProgressionRate ConfigFloat `yaml:"UncontestedProgressionRate"` // Multiplier for class-2 uncontested progression: no roll, no opposition, so low chance at high frequency (default 0.10)
```

`internal/configs/config.balance.progression.go`, beside the other defaults:

```go
	if b.UncontestedProgressionRate <= 0 || b.UncontestedProgressionRate > 1.0 {
		b.UncontestedProgressionRate = 0.10
	}
```

`_datafiles/config.yaml`, in the progression block near `RegenProgressionCurve`
(around line 1072). **Build this edit from `git show
HEAD:_datafiles/config.yaml`, not the working copy** — the file has
`skip-worktree` set and desyncs both ways:

```yaml
  # UncontestedProgressionRate: The class-2 multiplier (U10b). An uncontested
  # site is an action or a tick with no opposition and no roll to win: looking
  # at something, walking a room, shouting a warcry, coating a blade. These fire
  # constantly, so they progress at a fraction of the contested rate rather than
  # at parity. Lower = free actions matter less. Range: 0.001 to 1.0.
  UncontestedProgressionRate: 0.10
```

- [ ] **Step 5: Add the two methods**

`internal/characters/progression.go`, immediately after `OnSkillUseScaled`
(ends line 292):

```go
// OnStatUseUncontested is the class-2 entry point for a stat exercised by an
// action that had no opposition and no roll to win (U10b spec §2).
//
// It is OnStatUse damped by Balance.UncontestedProgressionRate. It still
// TRACKS the use: an untracked roll is rank-independent progression, which is
// the fyttyn mechanism (see internal/migration/0.16.0.go).
func (c *Character) OnStatUseUncontested(statName string, userId int) bool {
	c.TrackStatUse(statName)
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return false
	}
	rate := float64(configs.GetBalanceConfig().UncontestedProgressionRate)
	if rate <= 0 {
		return false
	}
	return c.CheckStatProgression(statName, userId, rate)
}

// OnSkillUseUncontested is the class-2 entry point for a skill exercised
// without opposition.
//
// It deliberately does NOT delegate to OnSkillUseScaled(skill, uid, rate).
// OnSkillUseScaled rolls the skill's primary stat through OnStatUse at an
// unscaled 1.0, so delegating would damp the skill and leave its governing stat
// at the full contested rate -- a silent hole in the class. Both halves are
// damped here instead. TestUncontestedScalesBothSkillAndStat pins this.
//
// Cluster affinity drift and the SkillUsed quest event still fire: the action
// genuinely happened, and neither is a progression roll.
func (c *Character) OnSkillUseUncontested(skillName string, userId int) bool {
	c.TrackSkillUse(skillName)

	if clusters := mutations.ClustersForSkill(skillName); clusters != nil {
		amt := float64(configs.GetBalanceConfig().MutationAffinityPerSkillUse)
		for _, cl := range clusters {
			c.AddClusterAffinity(cl, amt)
		}
	}

	rate := float64(configs.GetBalanceConfig().UncontestedProgressionRate)
	gained := false
	if configs.GetGamePlayConfig().UseSkillProgression && rate > 0 {
		gained = c.CheckSkillProgression(skillName, userId, rate)
	}

	if primaryStat := skills.GetSkillPrimaryStat(skillName); primaryStat != "" {
		c.OnStatUseUncontested(primaryStat, userId)
	}

	if userId > 0 {
		events.AddToQueue(events.SkillUsed{
			UserId: userId,
			Skill:  skills.SkillTag(skillName),
		})
	}

	return gained
}
```

- [ ] **Step 6: Add the Actor-parity wrappers**

`internal/actions/actor.go`, after the `OnStatUse` declaration at line 55:

```go
	// OnSkillUseUncontested / OnStatUseUncontested are the U10b class-2
	// entry points: an action with no opposition progresses at
	// Balance.UncontestedProgressionRate, not at the contested rate.
	OnSkillUseUncontested(skillName string) bool
	OnStatUseUncontested(statName string) bool
```

`internal/actions/actor_user.go`, after line 78:

```go
func (a *UserActor) OnSkillUseUncontested(skillName string) bool {
	return a.User.Character.OnSkillUseUncontested(skillName, a.User.UserId)
}

func (a *UserActor) OnStatUseUncontested(statName string) bool {
	return a.User.Character.OnStatUseUncontested(statName, a.User.UserId)
}
```

`internal/actions/actor_mob.go`, after line 79:

```go
func (a *MobActor) OnSkillUseUncontested(skillName string) bool {
	return a.Mob.Character.OnSkillUseUncontested(skillName, 0)
}

func (a *MobActor) OnStatUseUncontested(statName string) bool {
	return a.Mob.Character.OnStatUseUncontested(statName, 0)
}
```

Adding methods to `Actor` breaks every test fake implementing it. Find them:

```bash
go build ./... && go vet ./...
```

Fix each fake by adding the two methods. **Do not make them silent no-ops** if
the fake records `OnSkillUse`/`OnStatUse` calls — mirror whatever it does for
those, so uncontested calls are observable in tests that count progression.
Task 4 and Task 6 depend on this being done properly.

- [ ] **Step 7: Run the tests**

```bash
go test ./internal/characters/ -run 'Uncontested' -v
go test ./internal/characters/ ./internal/actions/ ./internal/configs/
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
gofmt -l internal/ modules/
git add internal/configs/config.balance.go internal/configs/config.balance.progression.go \
        internal/characters/progression.go internal/characters/progression_uncontested_test.go \
        internal/actions/actor.go internal/actions/actor_user.go internal/actions/actor_mob.go
git commit -m "feat(u10b): the uncontested class seam

OnStatUseUncontested / OnSkillUseUncontested, damped by the new
Balance.UncontestedProgressionRate (0.10).

OnSkillUseUncontested deliberately does NOT delegate to OnSkillUseScaled:
that rolls the skill's primary stat through OnStatUse at an unscaled 1.0,
so delegating would damp the skill and leave its governing stat at the
full contested rate. Both halves are damped here. Test pins it.

Also extracts skillProgressionChance beside the existing
statProgressionChance so the tests pin production's expression rather
than a hand-rolled copy."
```

Commit the `config.yaml` change separately, built from the `git show HEAD:` blob
per the skip-worktree SOP. Verify with `git diff HEAD -- _datafiles/config.yaml`
printing nothing after the commit.

---

### Task 3: Regen joins the uncontested class

**Files:**
- Modify: `internal/characters/progression.go` (`OnRegenTick` at 485; delete `regenDamperFactor` 409-425 and `CheckRegenProgression` 426-478)
- Modify: `internal/characters/progression_faucet_test.go` (drop `regenDecayFactorForTest`)
- Test: `internal/characters/progression_regen_parity_test.go`

**The parity multiplier is derivable exactly — do not guess it.**

Today one regen roll's probability is:

```
P_old = RegenProgressionBase x (1-ratio)^RegenProgressionCurve
        x statMult x mutMult
        x regenDamperFactor
where regenDamperFactor = CalculateProgressionChance(rank, softCap) / BaseProgressionChance
```

`CheckStatProgression` at multiplier `m` gives:

```
P_new = CalculateProgressionChance(rank, softCap)
        x statMult x mutMult x StatProgressionRate x m
```

Setting `P_new = P_old` and cancelling the shared terms:

```
m = RegenProgressionBase x (1-ratio)^RegenProgressionCurve
    / (BaseProgressionChance x StatProgressionRate)
```

So regen keeps pool depletion as its **magnitude** input exactly as §4 requires,
and passes it as the multiplier. This is algebraic parity, not a tuned guess.
Note this means **regen does not use `UncontestedProgressionRate`** — §4
explicitly permits the class two knobs rather than fudging one. The flat rate
governs the §3.2 *action* sites; regen's magnitude is its depletion.

**Confirm the two premises before trusting the algebra:** that
`CheckStatProgression` really does multiply by `StatProgressionRate` (read
`statProgressionChance`, `progression.go:158-192`, to the end), and that
`statMult`/`mutMult` are applied identically on both paths. If either differs,
re-derive rather than adjusting the test.

- [ ] **Step 1: Write the failing parity test**

Create `internal/characters/progression_regen_parity_test.go`:

```go
package characters

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
)

// TestRegenParityWithPreU10b pins spec §8 criterion 4: routing regen through
// the standard applier must not change its effective rate. The old expression
// is reproduced here ONCE, as the frozen d17836c64 baseline, and compared
// against what production now rolls. This is the only place the old formula is
// allowed to survive.
func TestRegenParityWithPreU10b(t *testing.T) {
	repoRootChdir(t)
	configs.ReloadConfig()
	b := configs.GetBalanceConfig()

	// Vitality explicitly, per criterion 4: with regen in the uncontested
	// class, vitality has exactly two sources and the toughen path is its only
	// active one, so it feels a rate change harder than any other stat. The
	// others are checked too, but vitality is the named one.
	stats := []string{"vitality", "willpower", "strength", "charisma", "dexterity", "perception"}
	ratios := []float64{0.0, 0.1, 0.25, 0.5, 0.75, 0.99}
	ranks := []int{0, 25, 150, 400}

	for _, stat := range stats {
		for _, ratio := range ratios {
			for _, rank := range ranks {
				c := newTestCharacter(t)
				// Drive the virtual rank by use count: rank = uses / UsesPerRank.
				c.StatUseCount = map[string]int{stat: rank * int(b.UsesPerRank)}

				old := legacyRegenChanceBaseline(c, stat, ratio)
				now := c.statProgressionChance(stat, regenProgressionMultiplier(ratio))

				if !floatNear(old, now) {
					t.Errorf("stat=%s ratio=%v rank=%d: old %.10f, new %.10f", stat, ratio, rank, old, now)
				}
			}
		}
	}
}

// legacyRegenChanceBaseline is the pre-U10b expression, frozen. It reproduces
// CheckRegenProgression x regenDamperFactor as they stood at d17836c64.
func legacyRegenChanceBaseline(c *Character, statName string, ratio float64) float64 {
	b := configs.GetBalanceConfig()
	chance := float64(b.RegenProgressionBase) * math.Pow(1.0-ratio, float64(b.RegenProgressionCurve))

	chance *= b.GetStatProgressionMultiplier(statName)
	chance *= 1.0 + mutations.GetStatProgressionMultiplier(c.Mutations)

	// regenDamperFactor
	bp := float64(b.BaseProgressionChance)
	if bp <= 0 {
		return 0
	}
	virtualRank := c.GetStatUseCount(statName) / int(b.UsesPerRank)
	if statVal := c.GetStatValue(statName); statVal > int(b.StatProgressionSoftCap) && statVal > virtualRank {
		virtualRank = statVal
	}
	chance *= CalculateProgressionChance(virtualRank, int(b.StatProgressionSoftCap)) / bp

	return chance
}
```

**If this test fails on the mob branch**, note that `CheckRegenProgression`
applied `MobProgressionRate` and `MobStatCap` itself and `statProgressionChance`
also applies both. Verify they are not now applied twice for a mob — add a
`c.IsMob = true` case to the matrix to prove it.

- [ ] **Step 2: Run it to verify it fails**

```bash
go test ./internal/characters/ -run RegenParity -v
```

Expected: FAIL — `regenProgressionMultiplier` undefined.

- [ ] **Step 3: Implement `regenProgressionMultiplier` and rewire `OnRegenTick`**

```go
// regenProgressionMultiplier converts a pool's depletion into the class-2
// multiplier that reproduces the pre-U10b regen chance exactly.
//
// Derivation (U10b plan Task 3): the old path rolled
//   RegenProgressionBase x (1-ratio)^RegenProgressionCurve x statMult x mutMult
//   x (CalculateProgressionChance(rank) / BaseProgressionChance)
// and CheckStatProgression at multiplier m rolls
//   CalculateProgressionChance(rank) x statMult x mutMult x StatProgressionRate x m.
// Cancelling the shared terms leaves the expression below. Depletion stays the
// MAGNITUDE input, exactly as spec §4 requires, but the roll, the cap checks
// and the rank floor now come from the one applier instead of a second copy.
func regenProgressionMultiplier(ratio float64) float64 {
	b := configs.GetBalanceConfig()
	base := float64(b.BaseProgressionChance)
	rate := float64(b.StatProgressionRate)
	if base <= 0 || rate <= 0 {
		return 0
	}
	depletion := float64(b.RegenProgressionBase) * math.Pow(1.0-ratio, float64(b.RegenProgressionCurve))
	return depletion / (base * rate)
}

// OnRegenTick is called every regen tick (every 3 rounds) for each resource
// pool. Pool depletion sets the magnitude; the roll itself is an ordinary
// class-2 uncontested stat use (U10b §4).
//
// Resource→stat mappings:
//
//	Health    → vitality, willpower
//	Stamina   → strength, vitality
//	Conviction→ willpower, charisma
func (c *Character) OnRegenTick(current, max int, relatedStats []string, userId int) {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return
	}
	if max <= 0 {
		return
	}

	ratio := float64(current) / float64(max)
	if ratio >= 1.0 {
		return // Pool is full, no progression chance
	}
	if ratio < 0 {
		ratio = 0
	}

	mult := regenProgressionMultiplier(ratio)
	if mult <= 0 {
		return
	}

	for _, statName := range relatedStats {
		// TRACK BEFORE ROLLING: the rank that decays the curve is derived from
		// the use count, so an untracked roll is rank-independent progression.
		c.TrackStatUse(statName)
		c.CheckStatProgression(statName, userId, mult)
	}
}
```

- [ ] **Step 4: Delete the dead pair**

Delete `regenDamperFactor` (`progression.go:409-425`) and
`CheckRegenProgression` (`:426-478`). Delete `regenDecayFactorForTest` from
`progression_faucet_test.go` and any test that called it. Let the compiler find
the rest:

```bash
go build ./... && go test ./internal/characters/
```

- [ ] **Step 5: Run the parity test**

```bash
go test ./internal/characters/ -run RegenParity -v
```

Expected: PASS at every stat/ratio/rank combination.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/characters/progression.go internal/characters/progression_faucet_test.go \
        internal/characters/progression_regen_parity_test.go
git commit -m "feat(u10b): regen joins the uncontested class, at exact parity

Pool depletion stays the MAGNITUDE input per spec §4, but it is now passed
as the class-2 multiplier to the one applier instead of driving a second
chance formula. The multiplier is derived algebraically, not tuned:

  m = RegenProgressionBase x (1-ratio)^curve / (BaseProgressionChance x StatProgressionRate)

which makes CheckStatProgression's roll identical to the old
CheckRegenProgression x regenDamperFactor product. The parity test pins it
across six stats, six depletion ratios and four virtual ranks, with
vitality named explicitly per criterion 4.

Deletes CheckRegenProgression and regenDamperFactor: the cap checks and
rank floor they reproduced by hand now arrive structurally.

Regen does NOT use UncontestedProgressionRate -- §4 permits the class two
knobs, and regen's magnitude IS its depletion."
```

---

### Task 4: Move the §3.2 action sites onto the uncontested seam

**Files:**
- Modify: `internal/actions/consider.go:27`
- Modify: `internal/usercommands/look.go:85`
- Modify: `internal/actions/mutation_venom_coat.go:34`
- Modify: `internal/actions/skill_helpers.go:99-104`
- Modify: `internal/usercommands/go.go:388`
- Modify: `internal/actions/buy.go:789`
- Modify: `internal/actions/sell.go:378`
- Test: `internal/actions/progression_class_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/actions/progression_class_test.go`:

```go
package actions

import "testing"

// TestUncontestedSitesUseTheUncontestedSeam is spec §8 criterion 3: no
// uncontested site fires at the contested rate. It uses a recording Actor and
// asserts the call landed on the uncontested method, which is the only
// observable difference at this layer.
func TestUncontestedSitesUseTheUncontestedSeam(t *testing.T) {
	t.Run("consider trains perception uncontested", func(t *testing.T) {
		actor := newRecordingActor(t)
		target := newRecordingActor(t)
		Consider(actor, target)

		if n := actor.contestedStatCalls["perception"]; n != 0 {
			t.Errorf("consider used the CONTESTED stat seam %d times; it is a class-2 site", n)
		}
		if n := actor.uncontestedStatCalls["perception"]; n != 1 {
			t.Errorf("consider uncontested perception calls: got %d, want 1", n)
		}
	})

	t.Run("warcry trains rhetoric uncontested with no coin flip", func(t *testing.T) {
		// awardRhetoricUse used to fire always in combat and 50% out of it.
		// One rule now: uncontested, every time.
		out := newRecordingActor(t)
		out.inCombat = false
		for i := 0; i < 50; i++ {
			awardRhetoricUse(out, out.GetCharacter())
		}
		if n := out.uncontestedSkillCalls["rhetoric"]; n != 50 {
			t.Errorf("out-of-combat warcry: got %d uncontested calls in 50 tries, want 50 (the 50%% coin flip is deleted)", n)
		}
		if n := out.contestedSkillCalls["rhetoric"]; n != 0 {
			t.Errorf("out-of-combat warcry used the contested seam %d times", n)
		}
	})
}
```

`newRecordingActor` is a test Actor recording which seam each call landed on.
Task 2 Step 6 already touched every fake in this package — extend the existing
one with `uncontestedSkillCalls` / `uncontestedStatCalls` maps and an `inCombat`
field rather than adding a second fake. If `Consider`'s signature differs from
the call above, use the real one (`internal/actions/consider.go:26`).

- [ ] **Step 2: Run it to verify it fails**

```bash
go test ./internal/actions/ -run UncontestedSites -v
```

Expected: FAIL — the sites still call the contested seam.

- [ ] **Step 3: Make the seven edits**

`internal/actions/consider.go:27`:

```go
	actor.OnStatUseUncontested("perception")
```

`internal/usercommands/look.go:85`:

```go
		user.Character.OnStatUseUncontested("perception", user.UserId)
```

`internal/actions/mutation_venom_coat.go:34`:

```go
	actor.OnSkillUseUncontested(string(skills.WeaponCombat))
```

`internal/actions/skill_helpers.go` — replace the whole helper. The
in-combat/out-of-combat split is exactly what §7 decision 1 collapses:

```go
// awardRhetoricUse grants Rhetoric progression for a shout-style special move
// (warcry, rally).
//
// U10b: this is a class-2 uncontested site. It used to fire at the full
// contested rate in combat and at a 50% coin flip out of it -- two firing
// conventions for one verb, and the coin flip was a bespoke anti-spam brake.
// The uncontested rate IS the anti-spam brake now, so the split is deleted and
// the site fires once, always, damped.
//
// This lives in actions/ so every Actor implementation gets it. Warcry and rally
// previously left progression to their callers -- the player wrappers implemented
// it and the mob wrappers did not, so mobs never built Rhetoric from either verb.
func awardRhetoricUse(actor Actor, c *characters.Character) {
	actor.OnSkillUseUncontested(string(skills.Rhetoric))
}
```

The `c *characters.Character` parameter is now unused. The lint gate is
`only-new-issues`, so this may pass — but drop the parameter and fix the call
sites if `go vet` or the linter complains.

`internal/usercommands/go.go:388`:

```go
			if movementTrainsSearch() {
				user.Character.OnSkillUseUncontested(string(skills.Search), user.UserId)
			}
```

`internal/actions/buy.go:789`:

```go
		shopMob.Character.OnStatUseUncontested("charisma", 0)
```

`internal/actions/sell.go:378`:

```go
	mob.Character.OnStatUseUncontested("charisma", 0)
```

- [ ] **Step 4: Collapse `movementTrainsSearch` into the class**

Spec §3.2 calls the movement site "the class prototype": it already had its own
config probability, which is a second uncontested rate. Delete the gate so the
class has one rate:

```go
			user.Character.OnSkillUseUncontested(string(skills.Search), user.UserId)
```

```bash
grep -rn "movementTrainsSearch" --include=*.go internal/
```

Remove the function, its knob declaration from `config.balance.go`, its default
from the matching `config.balance.*.go`, and its key from `config.yaml` (built
from the `git show HEAD:` blob). Record the removal in Task 13's patch notes —
a knob disappearing from a shipped config is worth naming.

**If the knob's shipped value implies a rate far from
`UncontestedProgressionRate`**, say so in the commit rather than silently
changing how often movement trains search. That is a real balance change and
the playtest gate should know to look at it.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/actions/ ./internal/usercommands/ ./internal/characters/
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/actions/consider.go internal/usercommands/look.go \
        internal/actions/mutation_venom_coat.go internal/actions/skill_helpers.go \
        internal/usercommands/go.go internal/actions/buy.go internal/actions/sell.go \
        internal/actions/progression_class_test.go internal/configs/
git commit -m "feat(u10b): move the free-action sites onto the uncontested seam

consider, look, venom coat, warcry/rally, movement-trains-search, and both
shop-mob charisma sites. Spamming look or consider was a cheesy way to
raise a stat (spec §7 decision 6); it now trickles.

Two bespoke brakes are deleted because the class rate replaces them:
warcry's in-combat/out-of-combat 50% coin flip, and the movement site's
own config probability."
```

---

### Task 5: Gate the two combat sites that fire on a loss

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_unified.go:657-660`
- Modify: `internal/combat/defence_multiplier.go:362-376`
- Test: `internal/combat/progression_win_only_test.go`

These are the two §3.1 rows with real behaviour change: the melee attacker's
stat fires **every swing** regardless of outcome, and the channel defence path
fires **win or lose** — the divergence `defence_multiplier.go:364-367` names in
source and defers to U10b by name.

- [ ] **Step 1: Establish the sense of `res.Success` BEFORE writing anything**

This is the highest-risk step in the plan. `contest.Result.Margin` is
ATTACK-positive on this path — `defenceDamageMultiplier` negates it at
`defence_multiplier.go:307`, and `awardChannelDefenceBonus` negates it again at
`:506`. Read `internal/contest/contest.go` and determine what `Success` means
for the caller here: attacker won, or defender won.

```bash
grep -n "Success" internal/contest/contest.go
sed -n '295,315p' internal/combat/defence_multiplier.go
```

Write the answer into the task notes. Getting this backwards inverts the entire
slice, and a test fake that mirrors the mistake would still pass.

- [ ] **Step 2: Write the failing test**

Create `internal/combat/progression_win_only_test.go`:

```go
package combat

import "testing"

// TestChannelDefenceProgressionIsWinOnly pins spec §8 criterion 2 for the
// channel path: the ordinary defence award fires only when a defence actually
// registered, converging on melee's shape. Pre-U10b this fired whenever the
// contest ran, win or lose.
func TestChannelDefenceProgressionIsWinOnly(t *testing.T) {
	t.Run("a won defence awards progression", func(t *testing.T) {
		defender := newProgressionRecordingCharacter(t)
		res := forcedContestResult(t, true /* defender wins */)
		awardOrdinaryChannelDefenceForTest(t, defender, res)
		if n := defender.skillCalls["dodge"]; n != 1 {
			t.Errorf("won defence: got %d dodge awards, want 1", n)
		}
	})

	t.Run("a lost defence awards nothing", func(t *testing.T) {
		defender := newProgressionRecordingCharacter(t)
		res := forcedContestResult(t, false /* defender loses */)
		awardOrdinaryChannelDefenceForTest(t, defender, res)
		if n := len(defender.skillCalls); n != 0 {
			t.Errorf("lost defence awarded %v; want nothing (spec §7 decision 2)", defender.skillCalls)
		}
	})
}

// TestChannelDefenceBonusTierIsNotGatedOnTheWin guards the mistake that the
// win-only change invites: a defensive crit or fumble is a class-3 event that
// rides an EXCEPTIONAL result, not a win (spec §2). Sweeping the bonus call
// inside the new gate would silently delete defensive crit progression.
func TestChannelDefenceBonusTierIsNotGatedOnTheWin(t *testing.T) {
	defender := newProgressionRecordingCharacter(t)
	attacker := newProgressionRecordingCharacter(t)
	res := forcedContestResultWithDefenceFumble(t)
	awardChannelDefenceBonus(ChannelMelee, AttackSide{}, attacker, defender, res, false, false)
	if len(defender.bonusEvents) == 0 {
		t.Error("a defence FUMBLE on a lost contest paid no bonus event; the bonus tier must not be gated on the win")
	}
}
```

`forcedContestResult` and the recording character: this package already has
contest-forcing helpers used by `contest_site_guard_test.go` and
`control_immune_test.go`. Reuse them rather than building new ones.
`awardOrdinaryChannelDefenceForTest` is a thin shim calling the production block
you are about to gate; if extracting that block into a named function makes the
shim honest, extract it.

- [ ] **Step 3: Run it to verify it fails**

```bash
go test ./internal/combat/ -run WinOnly -v
```

Expected: FAIL — the lost-defence case awards progression.

- [ ] **Step 4: Gate the channel defence path**

`internal/combat/defence_multiplier.go`, replacing lines 362-376. **Use
whichever sense Step 1 established** — if `Success` is attack-positive the gate
is `if !res.Success`:

```go
	out.DefenceType = res.Winner
	out.Cost = commitDefenceWinner(defender, candidates, res)

	// U10b: the ordinary defence award fires ONLY when the defence won.
	// Pre-U10b this path fired whenever the contest ran, win or lose, which was
	// the single largest firing divergence in the game -- melee has always
	// gated on a defence actually registering, and audit finding 3 named this
	// pair. Both shapes are "you learn by succeeding" now (spec §7 decision 2).
	// The consequence is accepted in §7 decision 6: losing stops training, and
	// the contest floor guarantees everyone wins sometimes.
	if defenceWon(res) {
		for _, candidate := range candidates {
			if candidate.entry.Name == res.Winner {
				AwardDefenceProgression(defender, defender.GetUserId(), res.Winner)
				break
			}
		}
	}

	// DELIBERATELY OUTSIDE the gate: a defensive crit or fumble is a class-3
	// event that rides an exceptional result, not a win (spec §2). Sweeping
	// this inside would silently delete defensive crit progression.
	awardChannelDefenceBonus(channel, side, attacker, defender, res, out.AttackerCrit, out.AttackerFumble)
```

Define `defenceWon(res contest.Result) bool` next to `defenceDamageMultiplier`
so the polarity is stated once, in one place, with a comment recording what
Step 1 found. Do not inline `res.Success` at the call site — the whole reason
this is risky is that the polarity is not obvious from the field name.

- [ ] **Step 5: Gate the melee attacker stat**

`internal/hooks/NewRound_DoCombat_unified.go`, replacing lines 657-660:

```go
	// Attacker stat progression keeps its quadrant-flavoured room messages, so
	// it stays on its own helper rather than becoming an event.
	//
	// U10b: gated on a clean hit. This fired on EVERY swing regardless of
	// outcome, so whiffing a full round trained strength and dexterity exactly
	// as well as landing one.
	if res.CleanHit {
		emitAttackerStatGain(atk, "strength", atkUid)
		emitAttackerStatGain(atk, "dexterity", atkUid)
	}
```

Confirm `res.CleanHit` is in scope and means what the weapon loop at `:668`
means by `wh.CleanHit`. If the round-level and per-weapon flags differ, gate on
whichever represents "at least one swing landed".

- [ ] **Step 6: Run the tests**

```bash
go test ./internal/combat/ ./internal/hooks/
```

Expected: PASS. Existing tests asserting progression on a miss will fail — that
is the intended change, so update those assertions rather than reverting the
gate. **Read each one before changing it**: if a test asserts progression on a
miss for a reason unrelated to §3.1, raise it instead of editing it away.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/ modules/
git add internal/combat/defence_multiplier.go internal/hooks/NewRound_DoCombat_unified.go \
        internal/combat/progression_win_only_test.go
git commit -m "feat(u10b): the two combat sites that trained on a loss now gate on the win

Channel defence fired whenever the contest ran, win or lose, while melee
has always gated on a defence registering -- audit finding 3, named in
source at defence_multiplier.go:364-367 and deferred to this slice. Both
are win-only now, behind a named defenceWon() helper so the contest
result's polarity is stated once rather than inlined.

The melee attacker's stat fired on every swing, so whiffing a whole round
trained strength and dexterity as well as landing one. Gated on CleanHit.

The bonus tier stays OUTSIDE both gates and has its own guard test: a crit
or fumble is a class-3 event that rides an exceptional result, not a win."
```

---

### Task 6: Taunt, throw and shoot

**Files:**
- Modify: `internal/actions/combat_taunt.go:200` (delete), `:93` (comment)
- Modify: `internal/usercommands/throw.go:454`
- Modify: `internal/usercommands/shoot.go:196-200`
- Test: `internal/actions/taunt_progression_test.go`

- [ ] **Step 1: Write the failing test**

```go
package actions

import "testing"

// TestTauntProgressionIsSuccessOnly pins spec §3.1's taunt row: pre-U10b
// rhetoric was awarded on all three outcomes (fumble, miss, hit).
func TestTauntProgressionIsSuccessOnly(t *testing.T) {
	t.Run("a fumbled taunt trains nothing", func(t *testing.T) {
		actor := newRecordingActor(t)
		forceTauntOutcome(t, actor, outcomeFumble)
		if n := actor.contestedSkillCalls["rhetoric"]; n != 0 {
			t.Errorf("fumbled taunt trained rhetoric %d times, want 0", n)
		}
	})
	t.Run("a landed taunt trains once", func(t *testing.T) {
		actor := newRecordingActor(t)
		forceTauntOutcome(t, actor, outcomeHit)
		if n := actor.contestedSkillCalls["rhetoric"]; n != 1 {
			t.Errorf("landed taunt trained rhetoric %d times, want 1", n)
		}
	})
}
```

`forceTauntOutcome` drives `ExecuteTaunt` with a stubbed channel attack.
`internal/actions/taunt_collapse_test.go` already forces crits through such a
seam — reuse that mechanism rather than inventing a second one.

- [ ] **Step 2: Run it to verify it fails**

```bash
go test ./internal/actions/ -run TauntProgressionIsSuccessOnly -v
```

Expected: FAIL — the fumble branch trains rhetoric.

- [ ] **Step 3: Delete the fumble-branch award**

`internal/actions/combat_taunt.go`: remove
`actor.OnSkillUse(string(skills.Rhetoric))` at line 200, inside the
`if out.AttackerFumble` block. Update the doc comment at line 93:

```go
//   - OnSkillUse progression trigger (success only, U10b; was all outcomes)
```

The award at line 277 (the success path) stays exactly as-is.

**Check the miss path too.** §3.1 says taunt fired on "all three outcomes", but
only two awards are visible (`:200` fumble, `:277` success). Trace what happens
when the taunt is simply defended: if `ExecuteTaunt` returns before `:277`, the
miss case is already correct and §3.1 over-counted. Record which it was.

- [ ] **Step 4: Gate throw and shoot**

`internal/usercommands/throw.go:454` — `fumbled` and `hitMobs` are already in
scope for the engage decision two lines below:

```go
	// U10b: skullduggery trains only on a throw that connected. This fired
	// unconditionally, so a miss trained exactly as well as a hit.
	if !fumbled && len(hitMobs) > 0 {
		user.Character.OnSkillUse(string(skills.Skullduggery), user.UserId)
	}
```

`internal/usercommands/shoot.go:196-200` — fold perception into the existing
`hit` gate:

```go
	// --- Progression: both halves gate on the hit (U10b) ---
	// Perception fired unconditionally here, which trained the shooter's aim
	// stat for missing. The old comment claimed it mirrored melee; melee's
	// attacker stat is gated on CleanHit as of U10b Task 5, so it now does.
	if hit {
		user.Character.OnStatUse("perception", user.UserId)
		user.Character.OnSkillUse(string(skills.RangedCombat), user.UserId)
	}
```

Note `shoot` uses Perception for both hit and damage (it is a deliberate-move
action, not an auto-attack swing) — that is why perception rather than
dexterity appears here, and it is correct.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/actions/ ./internal/usercommands/
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/actions/combat_taunt.go internal/usercommands/throw.go \
        internal/usercommands/shoot.go internal/actions/taunt_progression_test.go
git commit -m "feat(u10b): taunt, throw and shoot gate on the success

Taunt awarded rhetoric from its fumble branch. Throw awarded skullduggery
unconditionally. Shoot awarded perception unconditionally while gating
only the skill on the hit -- its comment claimed it mirrored melee, and
after Task 5 it finally does."
```

---

### Task 7: The skullduggery family

**Files:**
- Modify: `internal/actions/steal.go:183,374,472`
- Modify: `internal/actions/plant.go:142,274,351`
- Modify: `internal/actions/defuse.go:129`
- Modify: `internal/actions/shadow.go:101,150`
- Modify: `internal/actions/search.go:243`
- Modify: `internal/actions/track.go:128`
- Modify: `internal/mobcommands/sneak.go:19`
- Modify: `internal/mobcommands/flee.go:54`
- Modify: `internal/actions/surprise_attack.go:360`
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:672`
- Test: `internal/actions/skullduggery_progression_test.go`

Spec §7 decision 3: defuse, plant and steal stop training on failed attempts.
§3.1 adds that these three **fire BEFORE the roll** today, so this is a move,
not just a gate.

- [ ] **Step 1: Read every site before editing any of them**

```bash
sed -n '175,195p;366,382p;464,480p' internal/actions/steal.go
sed -n '134,150p;266,282p;343,359p' internal/actions/plant.go
sed -n '120,136p' internal/actions/defuse.go
sed -n '93,109p;142,158p' internal/actions/shadow.go
sed -n '235,251p' internal/actions/search.go
sed -n '120,136p' internal/actions/track.go
sed -n '12,26p' internal/mobcommands/sneak.go
sed -n '46,62p' internal/mobcommands/flee.go
sed -n '352,368p' internal/actions/surprise_attack.go
sed -n '664,680p' internal/hooks/NewRound_DoCombat_helpers.go
```

For each, record in the task notes: (a) does the award sit before or after the
resolution, and (b) what local holds the success verdict. You need both before
you can gate correctly, and some sites have no success variable in scope where
the award currently sits. **If a site needs the verdict threaded down from a
caller, do that in a separate commit** so the mechanical gating stays reviewable.

- [ ] **Step 2: Write the failing test**

```go
package actions

import "testing"

// TestSkullduggeryTrainsOnlyOnSuccess pins spec §7 decision 3. These three
// verbs fired BEFORE the roll pre-U10b, so a failed steal trained exactly as
// well as a successful one -- and the playtest cribsheet row is "a failed
// steal trains nothing".
func TestSkullduggeryTrainsOnlyOnSuccess(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, a *recordingActor, succeed bool)
	}{
		{"steal", forceStealOutcome},
		{"plant", forcePlantOutcome},
		{"defuse", forceDefuseOutcome},
	}
	for _, tc := range cases {
		t.Run(tc.name+" failure trains nothing", func(t *testing.T) {
			a := newRecordingActor(t)
			tc.run(t, a, false)
			if n := a.contestedSkillCalls["skullduggery"]; n != 0 {
				t.Errorf("%s failure trained skullduggery %d times, want 0", tc.name, n)
			}
		})
		t.Run(tc.name+" success trains once", func(t *testing.T) {
			a := newRecordingActor(t)
			tc.run(t, a, true)
			if n := a.contestedSkillCalls["skullduggery"]; n != 1 {
				t.Errorf("%s success trained skullduggery %d times, want 1", tc.name, n)
			}
		})
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

```bash
go test ./internal/actions/ -run SkullduggeryTrainsOnlyOnSuccess -v
```

Expected: FAIL on all three failure cases.

- [ ] **Step 4: Move each award after its resolution and gate it**

Apply this transformation at each of the ten files. The shape is identical
everywhere; only the success variable's name changes:

```go
// BEFORE (award sits before the roll, or after it but ungated):
actor.OnSkillUse(string(skills.Skullduggery))
... resolution ...

// AFTER:
... resolution ...
// U10b: skullduggery trains on the success only (spec §7 decision 3). This
// award used to fire before the roll, so a failed attempt trained as well as
// a successful one.
if <successVar> {
    actor.OnSkillUse(string(skills.Skullduggery))
}
```

Per-site verdicts, from §3.1: `search.go:243` gates on a find,
`track.go:128` on a successful track, `shadow.go:101,150` on the shadow
succeeding rather than on begin. `mobcommands/sneak.go:19` and
`mobcommands/flee.go:54` are mob-side twins of player verbs — gate them the
same way so mob and player share one convention.

`surprise_attack.go:360` — U10d owns redesigning the mechanic; its progression
CLASS is U10b's. Gate on the attack landing and change nothing else. Say so in
source, so U10d's implementer does not read the gate as settled design:

```go
	// U10b owns the CLASS; U10d owns the mechanic. Gate only.
	if landed {
		actor.OnSkillUse(string(skills.Skullduggery))
	}
```

**One caution on `steal`:** a failed steal has its own consequences (the mark
wakes, aggro, a crime record). Gating progression must not accidentally move
inside a branch that also skips those. Gate only the `OnSkillUse` line.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/actions/ ./internal/mobcommands/ ./internal/hooks/
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/actions/steal.go internal/actions/plant.go internal/actions/defuse.go \
        internal/actions/shadow.go internal/actions/search.go internal/actions/track.go \
        internal/mobcommands/sneak.go internal/mobcommands/flee.go \
        internal/actions/surprise_attack.go internal/hooks/NewRound_DoCombat_helpers.go \
        internal/actions/skullduggery_progression_test.go
git commit -m "feat(u10b): the skullduggery family trains on the success only

steal, plant and defuse awarded skullduggery BEFORE the roll -- a failed
steal trained exactly as well as a successful one. Each award moves after
its resolution and gates on the verdict. shadow, sneak, search, track,
surprise_attack and both mobcommands twins get the same treatment so the
mob and player sides share one convention.

surprise_attack's MECHANIC belongs to U10d; only its progression class is
claimed here, and the source says so."
```

---

### Task 8: Salvage and the crafter's unscaled pair

**Files:**
- Modify: `internal/actions/salvage.go:166,252`
- Modify: `internal/mobs/crafter.go:505,546`
- Test: `internal/actions/salvage_progression_test.go`

- [ ] **Step 1: Write the failing test**

```go
package actions

import "testing"

// TestSalvageTrainsOnlyOnRecovery pins spec §3.1's salvage row: the award
// fired "always once committed", so a salvage that recovered nothing trained
// as well as one that recovered a full set.
func TestSalvageTrainsOnlyOnRecovery(t *testing.T) {
	t.Run("recovering nothing trains nothing", func(t *testing.T) {
		a := newRecordingActor(t)
		forceSalvageRecovery(t, a, 0)
		if n := a.contestedSkillCalls["salvage"]; n != 0 {
			t.Errorf("empty salvage trained %d times, want 0", n)
		}
	})
	t.Run("recovering one material trains once", func(t *testing.T) {
		a := newRecordingActor(t)
		forceSalvageRecovery(t, a, 1)
		if n := a.contestedSkillCalls["salvage"]; n != 1 {
			t.Errorf("successful salvage trained %d times, want 1", n)
		}
	})
}
```

- [ ] **Step 2: Run it to verify it fails**

```bash
go test ./internal/actions/ -run SalvageTrainsOnlyOnRecovery -v
```

Expected: FAIL on the empty case.

- [ ] **Step 3: Gate both salvage sites**

At `salvage.go:166` and `:252`:

```go
	// U10b: salvage trains on recovering at least one material. The item is
	// still consumed on an empty result -- that is salvage's own design, not a
	// progression rule.
	if recovered > 0 {
		actor.OnSkillUse(string(skills.Salvage))
	}
```

Use whichever local holds the recovered count at each site; if neither has one
in scope, compute it from the returned material slice rather than threading a
new parameter through.

**The two sites are different phases** — one is the begin/commit path and one
is the per-round or completion path. Read both before gating: if `:166` fires
at commit time when no recovery has happened yet, the correct fix may be to
delete that award entirely rather than gate it, since `:252` already covers
completion. Decide and say which in the commit.

- [ ] **Step 4: Fix the crafter's unscaled pair**

`internal/mobs/crafter.go:505` and `:546` are the only crafting sites calling
`OnSkillUse` instead of `OnSkillUseScaled` — audit finding 4, still live. Both
already sit inside a success branch, so only the call changes:

```go
		mob.Character.OnSkillUseScaled(recipe.Skill, 0, craftBonus)
```

Read how `craftBonus` is derived at `internal/hooks/NewRound_MobRoundTick.go:496`
and compute it identically from `recipe.SkillMinimum` and the mob's skill level.
**Do not invent a second formula.** If the derivation is more than two lines,
extract a shared helper into `internal/crafting` and have all six crafting sites
call it — six copies of a difficulty formula is the next audit finding.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/actions/ ./internal/mobs/ ./internal/hooks/ ./internal/crafting/
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/actions/salvage.go internal/mobs/crafter.go \
        internal/actions/salvage_progression_test.go internal/crafting/
git commit -m "feat(u10b): salvage gates on recovery; the crafter joins the scaled cluster

Salvage awarded on commit, so recovering nothing trained as well as
recovering a full set. The item is still consumed either way -- that is
salvage's design, not a progression rule.

crafter.go:505,546 were the last two crafting sites calling the unscaled
OnSkillUse while the other four use OnSkillUseScaled (audit finding 4).
Both now derive the same difficulty bonus from the same helper."
```

---

### Task 9: The mob-spell gate asymmetry

**Files:**
- Modify: `internal/hooks/NewRound_DoCombat_helpers.go:357-396` and `:531-552`
- Test: `internal/hooks/mob_spell_progression_test.go`

Spec §6: the player path applies a self-cast penalty, zeroes progression for an
area cast that found no targets, and gates on `spellBonus > 0`. The mob path has
none of the three and fires unconditionally on `CastComplete`.

- [ ] **Step 1: Read the player path and write its three gates down verbatim**

```bash
sed -n '357,396p' internal/hooks/NewRound_DoCombat_helpers.go
sed -n '531,552p' internal/hooks/NewRound_DoCombat_helpers.go
```

You are going to reproduce these exactly. A paraphrase is how the asymmetry
comes back.

- [ ] **Step 2: Write the failing test**

```go
package hooks

import "testing"

// TestMobSpellProgressionMatchesPlayerGates pins spec §6. The mob path fired
// unconditionally on CastComplete while the player path applied three gates.
func TestMobSpellProgressionMatchesPlayerGates(t *testing.T) {
	t.Run("area cast with no targets trains nothing", func(t *testing.T) {
		mob := newProgressionRecordingMob(t)
		forceMobCast(t, mob, castOptions{area: true, targets: 0, spellBonus: 1.0})
		if n := mob.skillCalls["spellcasting"]; n != 0 {
			t.Errorf("empty area cast trained %d times, want 0", n)
		}
	})
	t.Run("self cast is penalised exactly like the player path", func(t *testing.T) {
		mobBonus := mobCastBonusForTest(t, castOptions{selfCast: true, targets: 1, spellBonus: 1.0})
		playerBonus := playerCastBonusForTest(t, castOptions{selfCast: true, targets: 1, spellBonus: 1.0})
		if mobBonus != playerBonus {
			t.Errorf("mob self-cast bonus %v, player %v -- they must match", mobBonus, playerBonus)
		}
	})
	t.Run("zero spellBonus trains nothing", func(t *testing.T) {
		mob := newProgressionRecordingMob(t)
		forceMobCast(t, mob, castOptions{targets: 1, spellBonus: 0})
		if n := mob.skillCalls["spellcasting"]; n != 0 {
			t.Errorf("zero-bonus cast trained %d times, want 0", n)
		}
	})
}
```

- [ ] **Step 3: Run it to verify it fails**

```bash
go test ./internal/hooks/ -run MobSpellProgressionMatchesPlayerGates -v
```

Expected: FAIL on all three.

- [ ] **Step 4: Extract the gates into one helper and call it from both paths**

Do not copy the three conditions into the mob path — two copies that drift is
the exact bug being fixed.

```go
// spellProgressionBonus returns the progression multiplier a completed cast
// earns, or 0 when the cast earns none.
//
// U10b §6: the player and mob paths applied different gates -- the mob path
// applied none at all and fired unconditionally on CastComplete. One helper,
// both callers, so they cannot drift again.
func spellProgressionBonus(selfCast bool, targetCount int, isArea bool, spellBonus float64) float64 {
	// Reproduce the player path's three gates from :357-396 VERBATIM here.
}
```

Both `:385` and `:544` become:

```go
	if bonus := spellProgressionBonus(selfCast, targetCount, isArea, spellBonus); bonus > 0 {
		<caster>.Character.OnSkillUseScaled(string(castSkill), <uid>, bonus)
	}
```

The `OnStatUse` calls beside them (`:393`, `:550`) are the primarystat override
and must move inside the same gate — the comments at `:377-379` and `:537-539`
explain why they exist at all, and an ungated stat roll beside a gated skill
roll is the U10b bug in miniature.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/hooks/
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/ modules/
git add internal/hooks/NewRound_DoCombat_helpers.go internal/hooks/mob_spell_progression_test.go
git commit -m "fix(u10b): the mob spell path adopts the player path's three gates

The player path applies a self-cast penalty, zeroes progression for an
area cast that found no targets, and gates on spellBonus > 0. The mob path
applied none of the three and fired unconditionally on CastComplete.

Extracted into one spellProgressionBonus helper called by both, rather
than copied -- two copies is what let them diverge. The primarystat
override rolls move inside the same gate."
```

---

### Task 10: Deletions, and re-anchoring the test that survives them

**Files:**
- Modify: `internal/characters/progression_faucet_test.go` (rename + re-anchor)
- Modify: `internal/characters/progression.go` (delete `OnFirstMobKill` 322-332, `OnCritReceived` 352-378)
- Modify: `internal/hooks/Death_MobKillCredit.go:61,86`
- Modify: nine `_test.go` files carrying stale fakes

- [ ] **Step 1: Re-anchor the faucet test FIRST, before deleting anything**

Spec §3.4 observation 6: `TestCritReceivedProgression_DecaysWithRank`
(`progression_faucet_test.go:47`) pins production's real expression — its helper
calls `statProgressionChance` — but its name and doc comment describe
`OnCritReceived`. Delete the method first and the test reads as dead
scaffolding. Re-anchor it first, in its own commit, while the reason is visible.

Rename `critReceivedChanceForTest` → `toughenChanceForTest`,
`TestCritReceivedProgression_DecaysWithRank` → `TestToughenProgression_DecaysWithRank`,
`TestCritReceivedProgression_RatesAtThreeRanks` → `TestToughenProgression_RatesAtThreeRanks`,
and replace the doc comment:

```go
// toughenChanceForTest computes the chance the TOUGHEN PATH rolls against for
// statName, without tracking or rolling.
//
// The live path is the seam's bonus tier: BonusEvents emits {SideDefender,
// DefenderSkill, ToughenStat, ClassObserved} on an ExcAttackCrit, and
// applyBonusProgression tracks the use then calls CheckStatProgression with
// Balance.ObservedCritProgressionBonus. This helper calls statProgressionChance
// -- the SAME expression CheckStatProgression rolls against -- so it pins
// production's formula rather than a hand-rolled duplicate.
//
// It was named after OnCritReceived, this path's dead predecessor, which U10b
// deletes. The test is NOT scaffolding for that corpse: it is the only thing
// pinning that toughening runs on the decayed curve, which is the owner's
// stated condition for keeping the mechanic crit-only (spec §7 decision 8).
// Do not delete it with its former namesake.
func toughenChanceForTest(c *Character, statName string) float64 {
	mult := float64(configs.GetBalanceConfig().ObservedCritProgressionBonus)
	if mult <= 0 {
		return 0
	}
	return c.statProgressionChance(statName, mult)
}
```

```bash
go test ./internal/characters/ -run Toughen -v
gofmt -l internal/ modules/
git add internal/characters/progression_faucet_test.go
git commit -m "test(u10b): re-anchor the toughen curve test to the live seam

It pins production's real expression but was named and documented after
OnCritReceived, which the next commit deletes. Renamed and re-documented
FIRST, while the reason is still visible, so the corpse does not take the
only test of the owner's decay condition with it (spec §3.4 obs 6)."
```

- [ ] **Step 2: Delete `OnFirstMobKill` and its call sites**

Delete the method (`progression.go:322-332`). In
`internal/hooks/Death_MobKillCredit.go`, delete the calls at `:61` (killer) and
`:86` (party members), and the player-facing message *"Defeating a new foe hones
your combat instincts!"*.

**Keep `KD.AddMobKill`** and all kill tracking — it feeds the kill/bestiary
displays and is not progression. Verify:

```bash
grep -rn "AddMobKill" --include=*.go internal/ | grep -v "_test.go"
```

Expected: still present.

- [ ] **Step 3: Delete `OnCritReceived`**

Delete `progression.go:352-378`, then confirm nothing referenced it:

```bash
grep -rn "OnCritReceived" --include=*.go internal/ modules/
```

Expected: no hits. Clean up the stale comment mention at
`NewRound_DoCombat_unified.go:614` as well.

- [ ] **Step 4: Delete the nine stale test fakes**

`Character.OnCriticalSuccess` and `OnCriticalFailure` stopped existing in U9,
but no-op implementations survive on test fakes:

```bash
grep -rln "OnCriticalSuccess\|OnCriticalFailure" --include=*_test.go internal/ modules/
```

Delete those methods from each file named. Expect nine files; if the count
differs, say so rather than assuming the spec was wrong.

- [ ] **Step 5: Verify and commit**

```bash
go build ./... && go test ./internal/characters/ ./internal/hooks/
gofmt -l internal/ modules/
git add -u internal/
git commit -m "refactor(u10b): delete the three dead progression paths

- OnFirstMobKill + both call sites + its player message. First-kill-of-a-
  type progression is deleted outright (spec §5, owner). KD.AddMobKill and
  all kill tracking STAY -- that feeds the bestiary, not progression.
- OnCritReceived: zero production callers since U9 replaced it with
  Outcome.ToughenStat. The mechanic is alive; only the corpse goes.
- Nine test fakes implementing OnCriticalSuccess/OnCriticalFailure for
  methods that stopped existing in U9."
```

Note `git add -u internal/` is safe here because this step only deletes. Do not
use `git add -A` anywhere in this plan — there are permanently untracked items
in this working tree.

---

### Task 11: Pin the toughen path (spec §8 criterion 5)

**Files:**
- Test: `internal/combat/toughen_path_test.go`
- Test: `internal/progression/event_toughen_test.go`
- Test: `internal/characters/progression_toughen_test.go`

No production change is expected. Criterion 5 has the base pin plus (a)-(d), and
§3.4 observations 7-9 name three refactor hazards.

- [ ] **Step 1: Write the channel-mapping guard**

Create `internal/combat/toughen_path_test.go`:

```go
package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// allAttackChannels must list every AttackChannel constant declared in
// defence_sets.go. TestChannelConstantsAreAllListed fails if one is missing.
var allAttackChannels = []AttackChannel{
	ChannelMelee,
	ChannelRanged,
	ChannelSpellPhysical,
	ChannelSpellMental,
	ChannelSocial,
}

// TestEveryChannelMapsToADamageChannel is spec §8 criterion 5(d) and §3.4
// observation 8.
//
// channelDamageChannel's default arm returns "", ToughenStatFor("") returns "",
// and BonusEvents then falls back to o.DefenderStat -- so an unmapped channel
// DEGRADES SILENTLY to training the ordinary defence stat instead of failing.
// A new channel added without a mapping would quietly stop toughening and no
// test would notice. This one does.
func TestEveryChannelMapsToADamageChannel(t *testing.T) {
	for _, ch := range allAttackChannels {
		dmgChannel := channelDamageChannel(ch)
		if dmgChannel == "" {
			t.Errorf("channel %q maps to no damage channel; toughening will silently fall back to the ordinary defence stat", ch)
			continue
		}
		if stat := characters.ToughenStatFor(dmgChannel); stat == "" {
			t.Errorf("channel %q maps to damage channel %q, which ToughenStatFor does not know", ch, dmgChannel)
		}
	}
}

// TestChannelConstantsAreAllListed catches what the table cannot: someone adds
// a constant and does not add it to allAttackChannels.
func TestChannelConstantsAreAllListed(t *testing.T) {
	declared := attackChannelConstantsFromSource(t) // parses defence_sets.go with go/ast
	if len(declared) != len(allAttackChannels) {
		t.Fatalf("defence_sets.go declares %d AttackChannel constants but allAttackChannels lists %d; add the new one and decide its toughen mapping", len(declared), len(allAttackChannels))
	}
}

// TestToughenStatPerChannel is criterion 5's per-channel pin.
func TestToughenStatPerChannel(t *testing.T) {
	cases := []struct {
		channel AttackChannel
		want    string
	}{
		{ChannelMelee, "vitality"},
		{ChannelRanged, "vitality"},
		// §3.4 observation 7: a physical-DEFENDED spell still toughens
		// willpower, because its damage is cast off willpower. This mapping
		// looks like a bug and is not. Asserted by name so a refactor cannot
		// "fix" it into vitality.
		{ChannelSpellPhysical, "willpower"},
		{ChannelSpellMental, "willpower"},
		{ChannelSocial, "charisma"},
	}
	for _, tc := range cases {
		got := characters.ToughenStatFor(channelDamageChannel(tc.channel))
		if got != tc.want {
			t.Errorf("channel %q toughens %q, want %q", tc.channel, got, tc.want)
		}
	}
}

// TestMeleeHardcodedChannelMatchesTheMapper is §3.4 observation 9.
// NewRound_DoCombat_unified.go:709 hardcodes ToughenStatFor("physical") instead
// of calling the mapper, which is exactly the drift ToughenStatFor was exported
// to prevent. Pin the two against each other.
func TestMeleeHardcodedChannelMatchesTheMapper(t *testing.T) {
	got := characters.ToughenStatFor("physical")
	want := characters.ToughenStatFor(channelDamageChannel(ChannelMelee))
	if got != want {
		t.Errorf("melee's hardcoded %q disagrees with the mapper's %q", got, want)
	}
}
```

`attackChannelConstantsFromSource` parses `defence_sets.go` and counts
`AttackChannel`-typed constants. Lift the `go/ast` walk from
`internal/combat/contest_site_guard_test.go`, which already parses this tree.

- [ ] **Step 2: Write the event-layer pins**

Create `internal/progression/event_toughen_test.go`:

```go
package progression

import "testing"

// TestToughenFiresOnRealCritOnly is criterion 5's base pin plus 5(c).
func TestToughenFiresOnRealCritOnly(t *testing.T) {
	base := Outcome{
		AttackerSkill: "weapon-combat", AttackerStat: "strength",
		DefenderSkill: "dodge", DefenderStat: "dexterity",
		ToughenStat: "vitality",
	}
	bonuses := Bonuses{Doing: 2.0, Observing: 0.5}

	t.Run("a real attack crit toughens the defender once", func(t *testing.T) {
		o := base
		o.Exceptional = ExcAttackCrit
		var got []Event
		for _, e := range BonusEvents(o, bonuses) {
			if e.Side == SideDefender {
				got = append(got, e)
			}
		}
		if len(got) != 1 {
			t.Fatalf("got %d defender events, want 1", len(got))
		}
		if got[0].Stat != "vitality" || got[0].Class != ClassObserved {
			t.Errorf("got {stat:%q class:%v}, want {vitality ClassObserved}", got[0].Stat, got[0].Class)
		}
	})

	t.Run("a floor-granted crit toughens nobody", func(t *testing.T) {
		o := base
		o.Exceptional = ExcAttackCrit
		o.Floored = true
		if evs := BonusEvents(o, bonuses); len(evs) != 0 {
			t.Errorf("floored crit emitted %d events, want 0", len(evs))
		}
	})

	// §3.4 observation 3: a fumble gives the defender their ORDINARY stat, not
	// the toughen stat. Coherent (nobody toughens from a flailing miss) and
	// pinned so a future refactor does not "symmetrise" it.
	t.Run("a fumble does not toughen", func(t *testing.T) {
		o := base
		o.Exceptional = ExcAttackFumble
		for _, e := range BonusEvents(o, bonuses) {
			if e.Side == SideDefender && e.Stat == "vitality" {
				t.Errorf("fumble toughened the defender; want the ordinary defence stat %q", o.DefenderStat)
			}
		}
	})

	// Criterion 5(c): magnitude does not gate. Outcome carries no damage amount
	// at all, which IS the assertion -- if someone adds one, this test stops
	// compiling and they have to come read this comment.
	t.Run("damage magnitude does not gate toughening", func(t *testing.T) {
		o := base
		o.Exceptional = ExcAttackCrit
		if len(BonusEvents(o, bonuses)) == 0 {
			t.Error("a crit carrying no damage information emitted nothing; toughening must not depend on magnitude (spec §7 decision 8)")
		}
	})

	// §3.4's empty-channel hazard at the event layer: ToughenStat unset falls
	// back to DefenderStat rather than emitting nothing. Pinned so the fallback
	// is a decision, not a surprise.
	t.Run("an unset ToughenStat falls back to the ordinary defence stat", func(t *testing.T) {
		o := base
		o.ToughenStat = ""
		o.Exceptional = ExcAttackCrit
		for _, e := range BonusEvents(o, bonuses) {
			if e.Side == SideDefender && e.Stat != o.DefenderStat {
				t.Errorf("unset ToughenStat produced stat %q, want the fallback %q", e.Stat, o.DefenderStat)
			}
		}
	})
}
```

- [ ] **Step 3: Write the curve and tracking pins**

Create `internal/characters/progression_toughen_test.go`:

```go
package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/progression"
)

// TestToughenTracksBeforeRolling is criterion 5(b). Untracked, vitality's rank
// sits at 0 forever regardless of its VALUE, which is a flat rank-independent
// chance -- the fyttyn exploit.
func TestToughenTracksBeforeRolling(t *testing.T) {
	repoRootChdir(t)
	configs.ReloadConfig()

	c := newTestCharacter(t)
	before := c.GetStatUseCount("vitality")

	c.applyBonusProgression(progression.Event{
		Side: progression.SideDefender, Skill: "dodge", Stat: "vitality",
		Class: progression.ClassObserved, Multiplier: 0.5,
	}, 0)

	if got := c.GetStatUseCount("vitality"); got != before+1 {
		t.Errorf("vitality use count %d, want %d -- ClassObserved must track", got, before+1)
	}
}

// TestToughenDecaysWithRank is criterion 5(a): the roll runs on the shared
// decayed curve, so a high-rank defender's chance is strictly below a rank-0
// defender's. This is the owner's stated condition for keeping toughening
// crit-only (spec §7 decision 8).
func TestToughenDecaysWithRank(t *testing.T) {
	repoRootChdir(t)
	configs.ReloadConfig()
	b := configs.GetBalanceConfig()

	low := newTestCharacter(t)
	low.StatUseCount = map[string]int{"vitality": 0}

	high := newTestCharacter(t)
	high.StatUseCount = map[string]int{"vitality": 150 * int(b.UsesPerRank)}

	lo := toughenChanceForTest(low, "vitality")
	hi := toughenChanceForTest(high, "vitality")

	if !(hi < lo) {
		t.Errorf("toughen chance did not decay: rank0 %.6f, rank150 %.6f", lo, hi)
	}
}
```

- [ ] **Step 4: Run everything**

```bash
go test ./internal/combat/ ./internal/progression/ ./internal/characters/ -v -run 'Toughen|Channel'
```

Expected: PASS. If `TestChannelConstantsAreAllListed` fails, a channel exists
that `allAttackChannels` does not name — add it AND decide its toughen mapping.

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ modules/
git add internal/combat/toughen_path_test.go internal/progression/event_toughen_test.go \
        internal/characters/progression_toughen_test.go
git commit -m "test(u10b): pin the toughen path, including the three mapper hazards

Criterion 5 and its four sub-pins: fires on a real crit only, per channel
(physical vitality / magical willpower / conviction charisma), on the
shared decayed curve, tracking before it rolls, and NOT gated on damage
magnitude.

Plus the §3.4 refactor hazards:
- ChannelSpellPhysical deliberately toughens WILLPOWER (its damage is cast
  off willpower even though dodge/block defend it). Asserted by name.
- channelDamageChannel's default arm degrades silently: BonusEvents falls
  back to DefenderStat, so a new channel would quietly stop toughening.
  The guard drives off the constants and fails instead.
- Melee hardcodes ToughenStatFor(physical) rather than calling the mapper.
  The two are now pinned against each other."
```

---

### Task 12: The class guard (spec §8 criterion 1)

**Files:**
- Create: `internal/characters/progression_class_guard_test.go`

Model this on `internal/combat/contest_site_guard_test.go` — same `go/ast` walk,
same file:func keying, same "name the owner, TODO is not an owner" discipline.
**Read that file before writing this one.**

- [ ] **Step 1: Write the guard**

```go
package characters

// The U10b class guard (spec §8 criterion 1). U6b's lesson was that a guard
// enumerating CATEGORIES only protects categories somebody remembered to name,
// so this one enumerates CALL SITES: every production reference to a
// progression entry point must appear below with the class it belongs to.
//
// Keys are file:func, not file -- a file-level allowlist hides new unowned
// sites inside allowlisted files, and steal.go and plant.go hold three each.

import (
	"sort"
	"strings"
	"testing"
)

type progressionClass string

const (
	classContested   progressionClass = "contested"   // fires on a win
	classUncontested progressionClass = "uncontested" // action or tick, no opposition
	classBonus       progressionClass = "bonus"       // rides an exceptional result
	classGrant       progressionClass = "grant"       // no roll at all; deliberately out of scope
)

// progressionSiteClasses is the allowlist. Populate it from every row of
// docs/audits/2026-08-21-u10b-site-inventory.md (Task 1), with the class that
// row settled on. A site not listed here fails the test.
var progressionSiteClasses = map[string]progressionClass{
	"internal/actions/consider.go:Consider":                  classUncontested,
	"internal/usercommands/look.go:Look":                     classUncontested,
	"internal/actions/skill_helpers.go:awardRhetoricUse":     classUncontested,
	"internal/usercommands/go.go:Go":                         classUncontested,
	"internal/hooks/NewRound_AutoHeal.go:AutoHealHandler":    classUncontested,
	"internal/actions/steal.go:Steal":                        classContested,
	"internal/combat/defence_multiplier.go:awardChannelDefenceBonus": classBonus,
	// ... every remaining row from the Task 1 inventory ...
}

// progressionEntryPoints are the symbols a site is recognised by.
var progressionEntryPoints = map[string]bool{
	"OnSkillUse": true, "OnSkillUseScaled": true, "OnStatUse": true,
	"OnSkillUseUncontested": true, "OnStatUseUncontested": true,
	"ApplyProgression": true, "OnRegenTick": true,
}

// noRollGrantPaths are named EXPLICITLY so the next audit does not rediscover
// them as gaps (spec §1 and §8 criterion 1). They move a stat with no roll, no
// cap check and no use tracking. That is deliberate: they are rewards, not use.
var noRollGrantPaths = map[string]progressionClass{
	"internal/hooks/Quest_HandleQuestUpdate.go": classGrant, // IncreaseStat, quest reward
	"internal/mobs/pack_scaling.go":             classGrant, // IncreaseStat, pack scaling
	"internal/quests/bridge.go":                 classGrant, // IncreaseStat, quest engine
	// TrainSkill and SetSkill are the other two no-roll paths; add their sites
	// here once walkProgressionSites is extended to recognise them.
}

func TestEveryProgressionSiteHasAClass(t *testing.T) {
	sites := walkProgressionSites(t, "..")
	var unowned []string
	for _, site := range sites {
		if _, ok := progressionSiteClasses[site]; ok {
			continue
		}
		if _, ok := noRollGrantPaths[fileOf(site)]; ok {
			continue
		}
		unowned = append(unowned, site)
	}
	sort.Strings(unowned)
	if len(unowned) > 0 {
		t.Errorf("%d progression site(s) have no class. Assign each to contested / uncontested / bonus in progressionSiteClasses, or to noRollGrantPaths if it is a no-roll grant:\n  %s",
			len(unowned), strings.Join(unowned, "\n  "))
	}
}

func TestNoStaleClassAssignments(t *testing.T) {
	sites := map[string]bool{}
	for _, s := range walkProgressionSites(t, "..") {
		sites[s] = true
	}
	for key := range progressionSiteClasses {
		if !sites[key] {
			t.Errorf("progressionSiteClasses names %q, which no longer exists. Delete the row.", key)
		}
	}
}

// TestUncontestedSitesCallTheUncontestedSeam is criterion 3's structural half:
// a site classed uncontested must not reference the contested entry points.
func TestUncontestedSitesCallTheUncontestedSeam(t *testing.T) {
	for site, class := range progressionSiteClasses {
		if class != classUncontested {
			continue
		}
		for _, call := range entryPointsCalledAt(t, "..", site) {
			switch call {
			case "OnSkillUse", "OnSkillUseScaled", "OnStatUse":
				t.Errorf("%s is classed uncontested but calls the contested %s", site, call)
			}
		}
	}
}
```

`walkProgressionSites`, `entryPointsCalledAt` and `fileOf` are the AST helpers.
Lift their structure from `contest_site_guard_test.go` — it already parses a
tree, tracks the enclosing `FuncDecl`, and keys `file:func` including the
`file:var <name>` form for package-level initialisers. The `".."` root is
`internal/` relative to this package; adjust if the helper wants the repo root.

**A caution the `TestUncontestedSitesCallTheUncontestedSeam` check inherits:**
`buy.go:Buy` and `sell.go:Sell` each contain BOTH a contested bartering award
and an uncontested shop-mob charisma award, so a function-level class cannot
describe them. Either split the shop-mob award into its own small function so
the key is honest, or add a documented exception list. Do not weaken the check
to make them pass.

- [ ] **Step 2: Run it and let it tell you what is missing**

```bash
go test ./internal/characters/ -run EveryProgressionSiteHasAClass -v
```

Expected: FAIL, listing every unassigned site. Add each from the Task 1
inventory and iterate until green. **Do not populate the map by pasting the
failure output** — a guard filled that way pins whatever the code happens to do,
which is the opposite of its job. Open each site and confirm the class.

- [ ] **Step 3: Run the whole suite**

```bash
go test ./...
```

Expected: PASS. `go test ./...` exits 0 with no known failures on this repo, so
any failure here is real and yours. If Windows Defender false-positives on the
test binary, set `GOTMPDIR=C:\gotmp`.

- [ ] **Step 4: Commit**

```bash
gofmt -l internal/ modules/
git add internal/characters/progression_class_guard_test.go
git commit -m "test(u10b): the class guard - every production site is assigned

Enumerates progression CALL SITES, not categories, keyed file:func --
U6b's lesson was that a category guard only protects categories somebody
remembered to name, and steal.go and plant.go hold three sites each.

Fails on an unassigned site, on a stale row naming a site that no longer
exists, and on an uncontested-classed site that calls a contested entry
point.

Names the no-roll grant paths (IncreaseStat via quest rewards, pack
scaling, the quest bridge) as deliberately out of scope, so the next audit
does not rediscover them as gaps."
```

---

### Task 13: Documentation and the pre-push gate

**Files:**
- Modify: `internal/characters/context.md`
- Modify: `internal/progression/context.md`
- Modify: `internal/actions/context.md`
- Modify: `docs/PATCH_NOTES.md`
- Modify: `docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`

- [ ] **Step 1: Update the three `context.md` files**

Work that reshapes a package's API MUST update its `context.md`. Verify every
symbol you name actually exists:

```powershell
Select-String -Path internal\characters\*.go -Pattern '^(func|type|const|var)\s'
```

`internal/characters/context.md`: add `OnStatUseUncontested` /
`OnSkillUseUncontested` to the Public API section, and a Gotchas entry saying
`OnSkillUseUncontested` does NOT delegate to `OnSkillUseScaled`, and why.
Remove `CheckRegenProgression`, `regenDamperFactor`, `OnFirstMobKill` and
`OnCritReceived` if they are named there.

`internal/actions/context.md`: the two new `Actor` interface methods.

`internal/progression/context.md`: note that the toughen path is the one cell
that swaps the stat, and that an unset `ToughenStat` falls back to
`DefenderStat` rather than emitting nothing.

- [ ] **Step 2: Write the patch note**

`docs/PATCH_NOTES.md`, dated entry. Player-facing framing, no raw numbers, no
em dashes, wrapped at 80:

```markdown
## 2026-08-21

Training now follows one rule. You improve by succeeding, not by trying.
A failed pickpocket, a missed swing, a taunt that falls flat, and a
salvage that recovers nothing no longer build skill the way a success
does.

Actions that were never a contest in the first place, like looking at
something or walking between rooms, still train you, but slowly.

Resting and recovering trains your body exactly as much as it always did.

Surviving a devastating blow still toughens you.
```

- [ ] **Step 3: Add the cribsheet rows**

`docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`, the four rows criterion 9 names:

```markdown
| U10b | A failed steal trains nothing | Steal and fail repeatedly; skullduggery must not advance |
| U10b | Resting trains at the old rate | Rest from low health; vitality gains should feel unchanged |
| U10b | consider spam no longer trains perception | Spam consider; perception should crawl, not climb |
| U10b | Taking a critical hit still toughens | Take a hard crit; vitality should still move |
```

Match the existing table's column layout rather than assuming this one.

- [ ] **Step 4: Update the roadmap row**

`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md:184`: mark U10b delivered, note
that the crit/critical-failure bonus layer owed for U10's sites landed here, and
record that `surprise_attack`'s progression class was claimed while its mechanic
stays with U10d.

- [ ] **Step 5: Run the full pre-push SOP**

```bash
gofmt -l internal/ modules/          # must print nothing
go build ./...
go test ./...
```

Confirm `Logging.LogToFile: false` in `_datafiles/config.yaml`. Then the
isolated boot test:

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                           # want 1
```

Exit code 124 is the SUCCESS case — the timeout fired because the server stayed
up. Do NOT grep for the bare word `panic`: `GamePlay.MapConsistencyEnforce`
legitimately has the value `panic`. Clean up with
`git worktree remove --force C:/tmp/dogmud-boot-check`.

- [ ] **Step 6: Commit**

```bash
git add internal/characters/context.md internal/progression/context.md \
        internal/actions/context.md docs/PATCH_NOTES.md \
        docs/PRE_DEPLOY_PLAYTEST_CRIBSHEET.md docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md
git commit -m "docs(u10b): context.md sweep, patch notes, cribsheet rows, roadmap"
```

---

### Task 14: The adversarial playtest gate (spec §8 criterion 9)

This slice changes how the game *feels* to train, which no unit test can
verify. Per the CLAUDE.md content playtest-review gate and the U6b lesson, this
task is required, not optional.

- [ ] **Step 1: Confirm the harness exists**

It is EXTERNAL (`../gomud-playtest-harness`, or `GOMUD_HARNESS_DIR`) and has
been deleted before. Check first.

- [ ] **Step 2: Write the goals file**

Create `tools/playtest/goals/2026-08-21-u10b-progression.yaml` with
`ephemeral:` set (local runs require it). Cover the four cribsheet rows plus one
open-ended probe: *does training feel arbitrary or unfair now that losing
teaches nothing?*

- [ ] **Step 3: Wipe instance saves, then run**

```bash
rm -rf _datafiles/world/dogmud/mobs.instances/* _datafiles/world/dogmud/rooms.instances/*
```

Do NOT wipe `shops/`, `guilds/` or `moderation/` — those are persistent living
state.

```text
/playtest local --checkout C:/Users/Calabe Davis/workspace/DOGMud bug-finder 2026-08-21-u10b-progression.yaml
```

Run it with an explicitly critical, adversarial mandate.

- [ ] **Step 4: Extract findings to memory**

Playtest reports are gitignored. Extract every finding into a memory topic file
before moving on, or it is lost.

- [ ] **Step 5: Fix what it finds, re-run, then hand to the user**

Do not claim the slice done on a clean boot alone. Note that
`playtestrun stop` leaves the container running — tear down with
`docker compose -p dogmud-playtest-<compose_id> -f .run/<compose_id>/compose.resolved.yml down -v`.

---

## Self-review notes

**Spec coverage.** §8 criterion 1 → Task 12. Criterion 2 → Tasks 5, 6, 7, 8.
Criterion 3 → Tasks 2, 4, and the guard's structural half in Task 12.
Criterion 4 → Task 3. Criterion 5 → Task 11. Criterion 6 → Task 10.
Criterion 7 → Task 9. Criterion 8 → Task 8. Criterion 9 → Task 14.
§4 → Task 3. §5 → Task 10. §6 → Task 9. §3.4 observations 6-9 → Tasks 10, 11.

**Known soft spots, flagged rather than hidden:**

1. **Task 5's `res.Success` polarity.** I could not confirm from source whether
   `Success` is attack-positive on the channel path, and `Margin` demonstrably
   is (it gets negated twice). Step 1 exists to settle it before any code is
   written, and Step 4 routes it through a named `defenceWon()` helper so the
   answer lives in one place. This is the highest-risk step in the plan: getting
   it backwards inverts the slice, and a mirrored test fake would still pass.
2. **Task 7's ten sites** are given as a transformation shape rather than
   before/after code, because some have no success variable in scope where the
   award currently sits and the fix differs per site. Step 1 surfaces that
   before any edit. This is a deliberate deviation from "complete code in every
   step" — writing invented before/after for ten unread sites would be worse.
3. **`UncontestedProgressionRate: 0.10` is a first guess.** Unlike Task 3's
   regen multiplier, which is derived algebraically against a parity target,
   this one has no target: the sites it governs are deliberately being made
   weaker. Expect the playtest gate to move it, and do not treat the shipped
   value as load-bearing.
4. **Task 12's function-level keying cannot describe `buy.go` / `sell.go`,**
   which hold a contested and an uncontested award in the same function. The
   task says to split the function rather than weaken the check, but that is a
   real refactor the executing engineer should confirm is wanted.
