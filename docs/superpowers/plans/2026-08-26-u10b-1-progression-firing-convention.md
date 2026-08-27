# U10b-1: the progression firing convention

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use `- [ ]` for tracking.

> ⚠️ **The task list grew on 2026-08-26.** Tasks **18b** and **18c** were added
> after a sweep found ~18 production progression sites that no task owned --
> the thirteen combat special moves, bartering x2, assess x2, and the unarmed
> fallback in `characters/skills.go`. Eleven of the thirteen special moves are
> SUCCESS-ONLY (`if result.Hit`), i.e. carrying the exact defect this slice
> exists to remove. **Task 22's guard would have gone red on all of them with
> nothing assigned to fix them**, so the plan as originally written could not
> reach its own "done when" item 1. Count the sites, not the tasks.

**Goal:** Progression resolves **Best-of**. One resolved action produces one event, for the single highest-rolling candidate skill. Full on success, partial on failure. Crits stay a separate channel. Progression follows the final outcome.

**Scope:** the firing rule only, wired to **every** site that resolves. **Every site keeps its CURRENT resolution.** The contest-core conversions, craft difficulty, material tier, salvage difficulty and the floors are **U10b-1b** and are not in this plan.

**Spec (decision record for both slices):** `docs/superpowers/specs/2026-08-26-u10b-1-progression-firing-convention-design.md`

---

## Facts verified against source, 2026-08-26. Do not contradict.

| | |
|---|---|
| `ApplyProgression` | `(events []progression.Event, side progression.Side, userId int, round uint64)` |
| `util.GetRoundCount()` | `uint64` |
| `SetConfigForTest` | `(t *testing.T, c Config)`. `GetConfig() Config` |
| `Config` | `GamePlay` and `Balance` are BOTH top-level. No `Config.GamePlay.Balance`. |
| Test-binary config | **NOT all zeroes.** `ensureConfigValidated` applies `<=0`-idiom defaults on first read. Only `<0`-idiom knobs and `ConfigBool`s stay zero, notably `UseSkillProgression` and `MobProgressionEnabled`. |
| `Balance.SkillWeight` | exists, ships **5.0** |
| `contest.Result.Success` | the **ATTACKER** won. `!Success` is NOT "the defender won" (`ForceCrit`). |
| `out.Defended` | assigned at `defence_multiplier.go:487`, **34 lines AFTER** the award at `:453`. At the award site the predicate is `!res.Success && !side.ForceCrit`. |
| `AwardDefenceProgression` | **Task 8 shipped** `(c, userId, defenceType string, won bool)`, an OUTCOME rather than a bare multiplier; it reads `ProgressionFailureFraction` itself. It makes **THREE** progression calls, not two (parry adds a strength roll) and all three scale. 2 production callers, both passing `true` until Task 9: `defence_multiplier.go:453`, `NewRound_DoCombat_helpers.go:238` (was `:46`; Tasks 9, 10 and 11 all grew that file above it) |
| `DefenceSkillAndStat` | dodge→**unarmed-combat**/dex, parry→**weapon-combat**/dex, block→**weapon-combat**/str, quell→spellcasting/wil, defy→rhetoric/wil |
| melee defender award | `processDefenderProgression` loops `defenceTypesUsed`, awarding once **per defence TYPE** (up to 3/round) |
| channel defender award | `defence_multiplier.go:453`, fires **win or lose** already |
| `runBestOfAllDefense` | sets `best.defenseType = res.Winner` whenever `res.Contested`, so a quoted defence **exists on a loss** |
| `DefenseUsed` | stamped only in `sendDefenseMessages`, i.e. only on a **won** defence |
| attacker gate | **Task 10 removed it.** Was `NewRound_DoCombat_unified.go:666-667`, `if !wh.CleanHit { continue }`; **Task 11 then removed the LOOP.** The award now lives in `processAttackerProgression` (`NewRound_DoCombat_helpers.go:89`), fires ONCE per round for a Best-of winner across skills, and awards win or lose |
| `WeaponHits` | **NEVER empty in production.** `collectAttackWeapons` contributes a fist per empty hand slot and ends with a `len == 0` fallback appending a bare fist; `buildAttackPlan` filters none of it; `calcSwingCount` has a minimum of 1; `calculateCombat:611` appends one entry per plan weapon unconditionally. So a bare-handed attacker produces **TWO** unarmed-combat entries, a 1H wielder produces weapon-combat **plus** an offhand unarmed-combat entry, and a 2H wielder produces one. The `len(WeaponHits) == 0 && res.CleanHit` fallback was **dead** and Task 10 deleted it |
| `CleanHit` | assigned inside `if res.hit` as `!res.defended`: **hit AND not defended**. A deflected swing is a Hit that is not a CleanHit. |
| clean-hit rate | **0.3856**, not 0.5752 (which is the hit rate). Fixed in `read_combat_analytics.py`, `987e7e872`. |
| `Actor` | **12 methods** after Task 7 (11 before it). 2 production implementers; field paths are `a.User.Character` / `a.Mob.Character`. **9 test fakes declare the interface in full** and need any new method added by hand: `internal/actions/{consider,economy,forage,salvage,scan,search,sleep,track}_test.go` + `internal/hooks/spell_foldanchor_test.go`. **A further 8 test types satisfy `Actor` by EMBEDDING and inherit new methods for free** -- `steal_test.go`'s `stubActorWithId` (embeds `stubActor`), and 7 that embed the `Actor` interface itself: `rhetoric_progression_test.go`'s `recordingActor` / `staleRhetoricTargetActor` / `rhetoricAdmissionRaceActor`, `combat_test.go`'s `staleCooldownActor`, `combat_reload_test.go`'s `staleRangedSecondaryActor`, and (in `internal/usercommands`) `shoot_test.go`'s `staleReloadPlayerActor` + `special_move_admission_side_effect_test.go`'s `staleWrapperCooldownActor`. **NOT `Actor` despite having `GetMobInstanceId`:** `internal/hooks/predator_hooks_test.go`'s `helpCallerActor` (local `partyActor`) and `internal/parties/actorkey_test.go`'s `fakeActor` (local `actorIdentity`) -- adding to those would be wrong. Verified by the compiler in Task 7: add to the interface, then `go test -run TestNothingZZZ ./...` and fix exactly what fails. |
| `events.SkillUsed` | `{UserId int, Skill skills.SkillTag, Details string}` |
| `OnSkillUseScaled` | also grants mutation cluster drift and emits `SkillUsed`, both unscaled |
| concentration | **THREE** sites: `combat_shared_helpers.go:141`/`:577` and `actions/combat_throttle.go:172` (award `:185`) |
| spell attacker award | `NewRound_DoCombat_helpers.go:647`, on `CastComplete`, gated only on `spellBonus > 0`, in no win/lose branch. The mob twin is `:806`. (Was `:385`; Tasks 9, 10 and 11 each grew that file above it, so re-derive rather than trusting this) |
| multi-candidate sites | **Task 11 CONVERTED BOTH.** The melee one is folded into `hooks.surpriseCandidate` (`NewRound_DoCombat_helpers.go:1329`) and the ranged one into `actions.awardFireProgression`; the "A SECOND Outcome is structurally required" comment no longer exists anywhere in the repo |
| skullduggery in a surprise attack | **never rolled**; read as a LEVEL at `crit_damage.go:74` |
| `search.go` | award at `:243` gated on `rolledAgainstSomething`, **full weight win or lose today** |
| `track.go` | award at `:128`, unconditional, full weight |
| `salvage.go` | awards at `:166` and `:252`, full weight |
| forage award | **`internal/actions/forage.go:142`**, not in `internal/forager` |
| `mobs/crafter.go` | awards at `:505`/`:546`, success-only |
| `go.go` | `if !isSneaking {` at `:664`; pursuit loop `:668-697`; `TryRoomBehavior` at `:700` **and** an unrelated one at `:357` |
| `go_test.go` | does not exist |
| `OnCritReceived` | zero production callers; **not touched by this slice**. Leave `TestCritReceivedProgression_DecaysWithRank` alone. |
| crit stub files | **nine** (same list as the `Actor` fakes) |
| seam guard maps | `progressionCalls` is the DENY-list; `allowedDirectProgression` is the allow-list |

### `pinConfigForTest`

```go
func pinConfigForTest(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.GamePlay.UseSkillProgression = true   // ConfigBool, stays false otherwise
	cfg.Balance.MobProgressionEnabled = true  // ConfigBool, else mobs never progress
	cfg.Balance.ProgressionFailureFraction = 0.35
	configs.SetConfigForTest(t, cfg)
}
```

⚠️ The two `ConfigBool`s are the ones that matter: everything else is defaulted
by `ensureConfigValidated`. Without `MobProgressionEnabled`, every mob-side test
asserts against a path that returns 0.

---

# Phase A: the seams

## Task 1: Test helpers

**Files:** create `testsupport_test.go` in `internal/progression`, `internal/characters`, `internal/actions`, `internal/hooks`, `internal/combat`, `internal/mobs`, `internal/usercommands`

- [ ] **Step 1:** `pinConfigForTest` (above) in each, plus `repoRootForTest` (anchored on `runtime.Caller(0)`) in each **except** `internal/combat`, which already has one
- [ ] **Step 2:** match each package's existing `_test.go` package clause. `internal/progression` has both; use `package progression_test`
- [ ] **Step 3:** `go test` each package, expect no output, commit

---

## Task 2: `ProgressionFailureFraction`

⚠️ **0 is a legal value** (an explicit off-switch) and an absent key unmarshals to
0, so `if x < 0 || x > 1.0` can never default it. Needs a **pre-unmarshal `-1`
sentinel** seeded at `tmpConfigData := Config{}` in `configs.go`, on
`tmpConfigData.Balance.ProgressionFailureFraction`.

- [ ] **Step 1:** write three failing tests: absent key gives 0.35; explicit 0 stays 0; and a **behavioural** test that loads a config omitting the key and asserts 0.35, not a source-text grep
- [ ] **Step 2:** declare, seed, default at 0.35, add to `_datafiles/config.yaml`
- [ ] **Step 3:** run, commit

⚠️ `_datafiles/config.yaml` has `skip-worktree`. Build the commit from the
`git show HEAD:` blob, never `git add` it from disk.

---

## Task 3: `OnStatUseScaled`

- [ ] **Step 1:** failing test, `pinConfigForTest` first: multiplier 0.0 must not advance training over 600 trials, and 1.0 must. **Both assertions**; the first alone passes against a function that progresses nothing
- [ ] **Step 2:** add `OnStatUseScaled(statName, userId, multiplier)`; `OnStatUse` becomes it at 1.0. Keep the existing `mudlog.Debug` trace
- [ ] **Step 3:** run, commit

⚠️ Do **not** yet change `OnSkillUseScaled`'s internal stat call; Task 4 does it
with the loss flag, because scaling it unconditionally would halve stat gain for
self-buff casts (`SelfCastProgressionMultiplier` is 0.5).

---

## Task 4: `Event.Lost`, and the two unscaled side effects

- [ ] **Step 1:** add `Lost bool` to `progression.Event`
- [ ] **Step 2:** widen `OnSkillUseScaled` to `(skillName string, userId int, bonusMultiplier float64, isLoss bool)`

Enumerate the callers first: `grep -rn "OnSkillUseScaled(" internal/ modules/ --include=*.go | grep -v _test.go`

- [ ] **Step 3:** failing tests: a loss grants strictly less cluster affinity than a win (with a non-zero guard on the win), and a loss emits **no** `SkillUsed`
- [ ] **Step 4:** scale mutation drift by `bonusMultiplier`; gate `SkillUsed` on `!isLoss`

⚠️ Gate on the **loss**, never on `multiplier < 1.0`, or self-buff casts silently
stop ticking `skill_use` quests. `SkillUsed.Skill` needs `skills.SkillTag(...)`.

- [ ] **Step 5:** make `OnSkillUseScaled`'s primary-stat call `OnStatUseScaled(primaryStat, userId, bonusMultiplier)`
- [ ] **Step 6:** run, commit

---

## Task 5: `Candidate`, `BestOf`, `Outcome.Defended`, `OrdinaryEventsScaled`

**Files:** `internal/progression/event.go`, create `bestof_test.go` and `event_scaled_test.go` (both `package progression_test`)

- [ ] **Step 1:** four failing `BestOf` tests: highest roll wins; equal rolls break on higher skill level; a full tie is deterministic across repeated calls; empty input reports **false** and awards nothing
- [ ] **Step 2:** two failing `OrdinaryEventsScaled` tests: attacker scaled when `Defended`, and the mirror when not. **Both**, or a stub returning `OrdinaryEvents` unchanged passes the first
- [ ] **Step 3:** implement

```go
// Candidate is one skill that could earn a resolved action's event.
//
// EVERY candidate is rolled the same way, dice.RollStat(stat + skill*SkillWeight).
// A candidate with no roll ties at zero and the tiebreak deletes it.
type Candidate struct {
	Skill string
	Stat  string  // empty means the skill's primary
	Roll  float64
	Level int
}

// BestOf picks the single Candidate that earns the event, as the defensive
// rolls pick a single defence. Highest Roll; ties on highest Level; a full tie
// on slice order, so callers keep that order fixed. Reports false when there is
// nothing to award: an empty Skill is not inert, CheckSkillProgression("")
// takes a roll and banners no skill.
func BestOf(cands []Candidate) (Candidate, bool)
```

`Outcome` gains `Defended bool`; `OrdinaryEventsScaled(o, frac)` sets
`Multiplier` and `Lost` on the losing side, decided by `o.Defended` **alone**.

- [ ] **Step 4:** run, commit

---

## Task 6: `Character.AwardResolved`

- [ ] **Step 1:** three failing assertions in one test: a win advances the skill; a loss advances it **strictly less**; a two-candidate call advances **only** the Best-of winner
- [ ] **Step 2:** implement

```go
// AwardResolved is THE entry point for the firing rule on a resolved action.
// ONE event, for the Best-of winner. Full on a win, ProgressionFailureFraction
// on a loss. The actor is the attacker side; Defended means the actor lost.
func (c *Character) AwardResolved(userId int, won bool, candidates ...progression.Candidate) {
	if c == nil {
		return
	}
	best, ok := progression.BestOf(candidates)
	if !ok {
		return
	}
	frac := float64(configs.GetBalanceConfig().ProgressionFailureFraction)
	o := progression.Outcome{AttackerSkill: best.Skill, AttackerStat: best.Stat, Defended: !won}
	c.ApplyProgression(progression.OrdinaryEventsScaled(o, frac),
		progression.SideAttacker, userId, util.GetRoundCount())
}
```

`Outcome.AttackerStat` exists (`event.go:70`) and `OrdinaryEvents` reads it at
`:151`, so passing the candidate's `Stat` through is correct. Leaving it empty
lets the skill's primary stat apply, which is what every current caller does.

- [ ] **Step 3:** add a `func (c *Character) CandidateFor(skill string) progression.Candidate` helper that builds a candidate with the standard roll, so no site hand-rolls one (shipped as a METHOD, not a free function, to match `AwardResolved` and the package's own style)
- [ ] **Step 4:** run, commit

---

## Task 7: `Actor.AwardResolved`, and the nine declaring fakes

- [ ] **Step 1:** failing test that both production actors satisfy an interface including `AwardResolved(won bool, cands ...progression.Candidate)`
- [ ] **Step 2:** add to the interface and both implementations, using `a.User.Character` / `a.Mob.Character`
- [ ] **Step 3:** add the method to the **nine fakes that DECLARE the interface in full** (see the `Actor` row above), recording the award where later tests need to observe it. The 8 embedding types inherit it and must not be edited; the 2 lookalikes in `internal/hooks/predator_hooks_test.go` and `internal/parties/` implement different local interfaces and must not be touched. **Let the compiler enumerate** -- `go test -run TestNothingZZZ ./...` -- rather than working from a list
- [ ] **Step 4:** `go build ./... && go test ./internal/actions/ ./internal/hooks/`, commit

**Landed as `awardRecorder`** in `internal/actions/testsupport_test.go`: an
embeddable recorder with an `awards []recordedAward` slice and an
`awardedCandidate(skill)` helper, so Tasks 8-14 can assert what a converted
site offered without re-editing eight fakes. `internal/hooks` is a separate
package and carries its own copy on `fakeActor`.

---

## Task 8: `AwardDefenceProgression` takes the outcome ✅

Shipped as `won bool`, **not** a bare multiplier. `OnSkillUseScaled` needs a
multiplier *and* an `isLoss` flag, and they mean different things — a WINNING
action can carry a sub-1.0 multiplier (self-buff casts ship 0.5), so a caller
holding a bare multiplier would have to keep a second flag in agreement and a
mismatch would award a reduced roll that still ticked `skill_use` quests. The
function reads `Balance.ProgressionFailureFraction` itself; `internal/combat`
already reads balance config freely, so no purity constraint is broken
(`internal/progression` stays untouched).

Note it makes **three** progression calls, not two: parry is the one two-stat
defence and its strength roll scales too.

- [x] **Step 1:** failing tests (`internal/combat/defence_progression_award_test.go`), assertion-level RED against a stub that ignored `won`
- [x] **Step 2:** add `won bool`, scaling all **three** calls; both production callers pass `true` so the task is a provable no-op
- [x] **Step 3:** run, commit

The loss weight is pinned by a **deterministic bracket** rather than a trial
count: at `ProgressionFailureFraction` 0 a lost defence advances nothing (the
chance short-circuits before any roll) and at 1.0 it advances with certainty,
which proves the multiplier is the knob rather than a hardcoded constant. The
shipped 0.35 sits strictly between by construction. Asserting on 0.35's
*outcome* directly could only ever be statistical; the quest-event half of the
loss path (`SkillUsed` suppressed) is exact at 0.35 and is asserted there.

---

# Phase B: wire combat

## Task 9: Both defence paths, Best-of, win or lose ✅

Shipped. The melee half needed a new per-swing record, because the field it used
to key on cannot carry a loss and is not reliably per-swing either:
`AttackResult.DefenseUsed` is stamped only by `sendDefenseMessages` (a WON
defence) and is never reset between swings, so `SwingEvent.DefenseUsed` copies
the ROUND-level value and over-reports once any swing has been defended. That
bug is pre-existing, is left alone, and is now moot for progression.

What landed instead: `combat.SwingDefence{Defence, Roll, Won}` and
`AttackResult.SwingDefences`, appended once per CONTESTED swing in
`calculateCombat` from `best.defenseType` / `best.defRoll.Value` /
`res.defenceWon()` (a new `hitResolution` method: `defended || defenseCrit`,
the union the crit floor can promote between).
`hooks.processDefenderProgression` builds one `progression.Candidate` per entry
with the roll that ALREADY happened -- no re-roll -- and hands the choice to
`progression.BestOf`, recovering the winning entry by value (each defence type
has a distinct Skill/Stat pair, so equal candidates can only name the same
defence).

🔴 **OPEN DIVERGENCE, CARRY TO U10b-1b: the two paths disagree on FUMBLES.**
Task 9 closed the win/lose divergence but NOT this one, and an earlier draft of
the `defenceWon` docstring wrongly claimed parity. Probe both paths with one
situation -- attacker rolls z = -3.0, defence takes the margin:

| | verdict | award |
|---|---|---|
| melee | `hit=false fumble=true defended=false` | `ProgressionFailureFraction` |
| channel | `AttackerFumble=true Defended=true` | **full weight** |

Cause: melee's three fumble branches (`combat_helpers.go:1042`, `:1057`,
`:1068`) all `return res` BEFORE `attackWon := best.margin <= 0` at `:1103`, so
`defended` is never set whatever the margin said. The channel path has no fumble
branch before its award at `defence_multiplier.go:464-470`; it derives
`out.AttackerFumble` but the predicate never reads it.

Deliberately NOT fixed here. Whether a fumble is nobody's contest win or the
margin still decides is a RESOLUTION question, and resolution is 1b's whole
remit. Blast radius is roughly the 2.3% attacker-fumble rate, and both paths
already beat pre-U10b-1, which paid such a swing nothing.

⚠️ **The channel path has NO lost branch.** `defence_multiplier.go:453` awards
the winning candidate regardless of `res.Success`. Do **not** add a second call;
make the existing one's `won` argument conditional.

⚠️ **`out.Defended` is not assigned until `:487`**, after two `return out` exits.
At the award site use `!res.Success && !side.ForceCrit`, or hoist the assignment
above the loop. Do not reach for bare `!res.Success`.

⚠️ **Melee awards once per defence TYPE.** Best-of collapses it to one. That is a
**cut** for a defender with several defences and a **gain** for one with a single
defence.

- [x] **Step 1:** read both paths (`helpers.go:30-70`, `defence_multiplier.go:442-459`)
- [x] **Step 2:** failing tests: a melee round where nothing lands awards the best-quoted defence at the fraction; a melee round with three defence types awards **once**; a lost channel defence awards the fraction and is **not** awarded twice
- [x] **Step 3:** melee, expose the quoted defence from `runBestOfAllDefense` (a new field on `SwingEvent` or `AttackResult`) and replace `processDefenderProgression`'s loop with a single award. When `getAvailableDefenses` is empty the contest is uncontested, `defenseType` is `""`, and nothing is awarded; note that in the commit
- [x] **Step 4:** channel, replace the `true` literal on the one existing `AwardDefenceProgression` call with the real outcome. `won` here is the DEFENCE's win, i.e. `!res.Success && !side.ForceCrit`, not bare `!res.Success`. Do not touch the `BonusEvents` line
- [ ] **Step 5:** run, commit

---

## Task 10: A defended melee swing awards the attacker

⚠️ Attacker events sit inside `if !wh.CleanHit { continue }`. Until that changes,
no `Outcome` is built for a defended swing at all. **Missed by three previous
plan versions.**

- [x] **Step 1:** failing test: a round whose swings are all defended awards the attacker's weapon skill at the fraction
- [x] **Step 2:** replace the `continue` with a scaled award, clean hit at 1.0 and defended at the fraction. A MISS awards too, on the same terms: `!CleanHit` covers both a deflection and an outright miss, and a missed swing is a contest that resolved and lost
- [x] **Step 3:** run, commit

**Shipped as `processAttackerProgression`** (`NewRound_DoCombat_helpers.go:89`),
extracted from `applyCombatProgression` so the firing condition has a seam the
tests can drive without the rest of Phase 5.

**Rate change to carry into the `SkillProgressionMultipliers` re-solve:**
awards per ROUND go from `P(clean hit)` = **0.3856** to **1.0**, roughly
**2.6x**. The design spec's risk table says "+26%"; that was computed from
0.5752, the **hit** rate mislabelled as the clean-hit rate, and is wrong.

⚠️ **Task 10 shipped a per-WEAPON award and Task 11 replaced it with a
per-ROUND one.** The per-weapon rule paid per HAND SLOT, giving 1 award to a
two-hander, 2 to a dual-wielder or a bare-handed fighter, 2 to a 1H wielder
(one of them unarmed-combat, off the empty hand) and **6** to extra-arms L4 --
while weapon SPEED contributed nothing at all. Owner ruling 2026-08-26: one
award per round. See Task 11.

---

## Task 11: The two multi-candidate sites

`NewRound_DoCombat_unified.go` and `actions/combat_fire.go`, both then carrying
"A SECOND Outcome is structurally required". **Scope grew on the owner's
ruling:** the melee attacker also collapsed from per-weapon to one award per
round, Best-of across SKILLS, selected on each skill's best ACTUAL attack roll.

⚠️ **Skullduggery is never rolled in a surprise attack**, so its candidate is
the one synthesised roll in the melee set (`CandidateFor`). It is also
out-drawn by construction: a weapon candidate's roll is the MAX across 1-4
swings, skullduggery is a single draw, which alone gives the weapon 65-80% at
equal scores. Skullduggery needs roughly **four skill levels above the combat
skill** to reach a coin flip.

- [x] **Step 1:** failing test: both skills are passed as candidates and **only one** advances
- [x] **Step 2:** replace both two-`Outcome` blocks with one `AwardResolved` call each
- [x] **Step 3:** run, commit

**Shipped** in `f1dc4cfca` + `0e97a4c83` + the review round.

⚠️ **`dexterity` is still rolled TWICE per melee round** -- once by
`emitAttackerStatGain` (`unified.go:660`) and once as weapon-combat's primary
inside `ApplyProgression`. Pre-existing, NOT introduced here, but Task 10 made
it fire every round instead of only on a clean hit. It is the same double-roll
Task 11 deleted on the ranged side. **Carry it into the re-solve.**

⚠️ **`0e97a4c83` partly closed a U10b-2 faucet.** Removing an `IsPlayer()` gate
means mob archers now train ranged-combat (at `MobProgressionRate` 0.5) where
they previously trained nothing. U10b-2's remaining work there is measuring the
rate, not wiring the faucet.

🔴 **Open for U10b-1b, alongside the melee/channel fumble divergence:** the
attacker's `BestRoll` cannot discriminate between two different SKILLS.
`calcAttackScore` takes its skill term from `GetCombatSkillLevel`, which
resolves the **main-hand** weapon's tag for every entry, so an offhand fist
rolls on a score built from weapon-combat's rank. Which of two different skills
trains is decided by the dual-wield penalty, not by either skill's own rank.
Within one skill the max is exactly right.
- [ ] **Step 3:** run, commit

---

## Task 12: Concentration, all three sites

- [x] **Step 1:** failing test at each of the three triggers: a broken concentration awards spellcasting at the fraction
- [x] **Step 2:** route all three through `AwardResolved`, won on `res.Success`
- [x] **Step 3:** run, commit

**Shipped** as the shared `hooks.awardConcentration` for the two hooks-side
triggers (damage and position); the throttle site calls
`Character.AwardResolved` directly because it lives in `internal/actions`.

⚠️ Concentration fires per damage instance, so up to ~4/round, and `spellcasting`
is fitted at 3.90 on the premise that casting is rare.

**MEASURED, and it is the largest rate change in the slice.** At 30% damage
(difficulty 300) a fresh caster holds only about **10 of 200** contests, so
events go from ~10 to 200: **20x in COUNT**, or roughly **7.6x in full-weight
equivalents** once the 0.35 fraction is applied. Multiply by up to four damage
instances a round. Carry both into the re-solve.

The position trigger's award sits BEFORE the win/lose branch on purpose --
the loss arm returns, so an award written inside it would never fire.
`TestProcessFoldRound_PositionBreakStillAwards` is sabotage-proven against
exactly that mistake.

---

## Task 13: The spell attacker

One cast is one resolved action: **one event, won if ANY target was hit.** Not a
per-target award.

- [x] **Step 1:** failing tests: a cast every target defended awards the fraction; a cast that hit one of three awards full, once
- [x] **Step 2:** route `NewRound_DoCombat_helpers.go:647` (mob twin `:806`) through `AwardResolved`

**Shipped.** The award already fired once per cast, not per target; what it
lacked was an OUTCOME. `resolveSpell` / `resolveMobSpell` now return
`anyLanded`, plumbed out of the four per-target resolvers (which gained a
`landed` return beside their existing `fumbled` one). `landed` means the
contest was WON OUTRIGHT -- a defended cast is not landed even though it
still deals partial damage, matching `SkillMoveResult.Hit`. Uncontested
casts (identify, fold-anchor, fold-recall, purge-affliction, a no-defence
help spell, a cooperative mob heal) count as landed: there was no defence to
beat. A failed `validateFoldRecall` does not.

🔴 **A REGRESSION WAS NEARLY SHIPPED HERE, and the guard against it is
`TestAwardResolvedScaled_BonusAppliesOnAWinNotJustALoss`.** Routing this site
through the plain `AwardResolved` silently DROPS `spellBonus`, the
`1 + Difficulty*SpellDifficultyProgressionScale` scaling -- so hard spells
would have started training exactly like trivial ones. The first fix was also
wrong: folding the bonus into `frac` and passing `frac*bonus` to
`OrdinaryEventsScaled` does nothing on a WIN, because that function only
touches the LOSING side's `Multiplier` and leaves the winner's at the 1.0
`OrdinaryEvents` hardcodes. New `Character.AwardResolvedScaled` applies the
bonus to the EVENT instead. **Any later task converting a site that carries a
difficulty or cost multiplier -- craft especially -- must use
`AwardResolvedScaled`, not `AwardResolved`.**
- [x] **Step 3:** run, commit

---

# Phase C: the non-combat sites, on their CURRENT resolution

No conversions here. Each site already resolves; read its existing outcome.

## Task 14: `search`

⚠️ Awards a **FULL** event today, win or lose, so this is a **cut** on failure.

- [x] **Step 1:** failing tests: a room with five hidden things awards the same as a room with one; a fruitless-but-resolved search awards the fraction
- [x] **Step 2:** replace `:243` with `actor.AwardResolved(userId, foundAnything, char.CandidateFor(string(skills.Search)))`, setting `foundAnything` alongside each `result.*Found` append
- [x] **Step 3:** run, commit

**Shipped.** ⚠️ The plan's Step 2 signature was stale: `Actor.AwardResolved`
takes `(won bool, cands ...Candidate)` with **no userId** -- the actor supplies
its own (Task 7). Shipped as
`actor.AwardResolved(result.FoundAnything(), char.CandidateFor(string(skills.Search)))`.

`foundAnything` is DERIVED as `SearchResult.FoundAnything()` rather than tracked
by a flag set beside each of the six append sites: one predicate in one place
cannot fall out of step with five of its six siblings. ⚠️ **A new tier must add
its slice to that method**, or its finds read as failures.

The `rolledAgainstSomething` anti-botting gate is UNCHANGED and is deliberately
not the firing rule: an empty room resolved no contest, so there is no loss to
pay a fraction on. Without it, `search` in a bare corridor would have become a
free progression tick the moment losing started paying.


## Task 15: `track` and `forage`

⚠️ Forage's award is at **`internal/actions/forage.go:142`**, not in `internal/forager`.

- [x] **Step 1:** failing tests for both: a resolved failure awards the fraction
- [x] **Step 2:** route both through `AwardResolved`, won on the existing find/grade condition
- [x] **Step 3:** run, commit

**Shipped, and the two sites move in OPPOSITE directions.**

- **`track` is a CUT.** It awarded a FULL event on every fired roll, so a
  tracker who read nothing trained as much as one who picked up a trail. The
  threshold is MODE-DEPENDENT and the award now uses the same numbers the
  branches gate on: **125** for a trail-scan, **175** for an active track on a
  named target. A single threshold would have reported a WIN on a 125-174 roll
  while telling the player their tracking "isn't sharp enough".
- **`forage` is a GAIN, and the only one in Phase C.** It was SUCCESS-ONLY --
  the award sat inside the found-an-item path -- so a fruitless forage trained
  nothing at all. The award moved up to just after `result.RollHappened`, so
  every early exit (wrong biome, cooldown) still awards nothing.

⚠️ `won` for forage is **`coreResult.Found`**, not `result.Found`. The
"crumbles in your hands" branch is a found item that failed to construct: a
data problem, not a lost contest. **This is not academic -- a test binary
seeds no item registry, so in tests that branch is the COMMON outcome**, and a
first draft asserting `won == result.Found` failed for exactly that reason.

✅ **FIXED, not deferred (owner ruling 2026-08-26).** `track`'s cooldown checks
run AFTER the roll, and the award used to fire before both -- so spamming
`track` paid every time, and it got WORSE under this slice because once losing
pays there is no roll outcome that fails to award. The award now fires at each
resolved EXIT rather than once beside the roll; a cooldown refusal is not a
resolved contest and awards nothing. Pinned by
`TestTrack_ACooldownRefusalAwardsNothing`, sabotage-proven.

🔴 **STILL OPEN, and it makes this task's loss branch nearly dead for a
developed character: `track`'s 125/175 thresholds are STATIC while the score
is not.** `CalcSearchScore` is `Perception + SkillMultiplier(rank)*25`, so:

| Perception | search rank | score | P(>=125) | P(>=175) |
|---|---|---|---|---|
| 100 | 0 | 125 | 50% | 0.4% |
| 100 | 25 | 160 | 93% | 27% |
| 100 | 50 | 175 | 97% | 50% |
| 150 | 25 | 210 | **99.7%** | 87% |

The trail-scan tier is a formality for anyone competent, so the failure
fraction this task added almost never fires there. `track.go` already carries
a NOTE that this is "a static difficulty check still off the contest core" and
that `contest.AgainstDifficulty` was built for it and has ZERO production
callers. **Converting it is U10b-1b's charter** (this slice keeps every site's
current resolution), and it is not a mechanical call swap -- it needs a
difficulty MODEL decision (static? scaled by trail age? by target?).


## Task 16: `craft` and `salvage`

Craft already resolves as `util.Rand(100) < chance`; salvage rolls per unit.
**Do not touch either roll.**

- [x] **Step 1:** failing tests: a failed craft awards the fraction (**the case the slice is justified by**); a salvage recovering nothing awards the fraction; a salvage awards **once** per command, not per unit
- [x] **Step 2:** award on both branches at all four craft sites and both salvage sites, via `AwardResolved`
- [x] **Step 3:** run, commit

**Shipped.** ⚠️ The plan said "all four craft sites"; they are not equivalent.
Only **TWO** call `crafting.CalcSuccessChance` and can therefore FAIL --
`NewRound_UserRoundTick` and `NewRound_MobRoundTick`, the multi-round paths.
The two immediate sites (`usercommands/craft.go`, `mobcommands/craft.go`) reach
`ImmediateComplete`, which means `TimeRounds <= 0` and `InitiateCraft` finished
without rolling. **An instant recipe cannot fail**, so those pass `won: true`.

All four use **`AwardResolvedScaled`**, never plain `AwardResolved` -- the
plain form silently drops `craftBonus` (Task 13's near-miss).

`salvage` is a CUT: both sites paid a FULL event whether or not anything came
back. One award per COMMAND, not per unit.

### 🐛 Two lookup bugs found while in the file, fixed here (owner, mid-task)

**Salvage could only find targets in the BACKPACK.** `StoreItem` auto-routes
potions and throwables into an equipped bandolier and `is_component` items into
a component bag, but `salvageItem` scanned `char.Items` alone and
`usercommands/salvage.go` refused any source that was not literally
`"in your backpack"` -- telling the player to "remove" something they were not
wearing. `RemoveItem` already handled all three slices; only the LOOKUPS were
narrow.

⚠️ **NOT the spoiled-potion case**, which is the obvious guess and is wrong:
`NewRound_AutoHeal` auto-ejects `PhaseSpoiled` potions to the backpack. The
live cases are a **DECLINING** potion (salvage accepts `PhaseDeclining` too,
and only Spoiled is ejected) and a **THROWABLE** (bandolier-routed, never
age-ejected).

🔴 **STILL OPEN:** `Character.FindItem` does not search `c.ComponentItems` at
all, so a crafted `is_component` item auto-routed to the component bag cannot
be NAMED by any command using that helper. That is a FindItem gap, wider than
salvage; `actions.salvageItem` now searches all three containers, so the
salvage half is closed.


## Task 17: Mob crafters

- [x] **Step 1:** failing test forcing a failed mob craft. **`pinConfigForTest` must set `MobProgressionEnabled`** or this asserts against a path that returns 0
- [x] **Step 2:** award on both branches at `:505` and `:546`
- [x] **Step 3:** run, commit

**Shipped.** Both sites are `executeCraft` (shop-inventory path) and
`executeCraftLegacy` (backpack path); both were success-only and now award
above the branch. Ingredients are consumed regardless of the roll, so a failed
mob craft already cost materials and taught nothing.

✅ **THE DIFFICULTY BONUS NOW APPLIES HERE TOO.** These used bare `OnSkillUse`
and had NEVER scaled by `1 + SkillMinimum*CraftDifficultyProgressionScale`, so
a shopkeeper crafting a demanding recipe trained exactly as fast as one
crafting a trivial one.

A first pass left that alone, reasoning that closing it was "a rate change
wearing a firing-convention change's clothes". **That was wrong.** The bonus is
part of the AWARD, not the resolution, so an award dropping a multiplier every
sibling applies is a firing-rule inconsistency -- which is what the tin says
this slice is: *"the firing rule only, wired to **every** site that resolves."*
The resolution (`crafting.CalcSuccessChance`) is untouched, so "every site
keeps its CURRENT resolution" still holds.

⚠️ **RATE CHANGE for shopkeeper crafters**, on top of the loss award: their
progression now scales with recipe `SkillMinimum` where it never did. The
re-solve must price both.

**All six craft sites are now identical in shape:**
`AwardResolvedScaled(uid, won, craftBonus, CandidateFor(recipe.Skill))`.

**Three BEHAVIOR PATHS call the one crafting mechanism**, and owner ruling
2026-08-26 is that this shape is fine: `actions.InitiateCraft` (immediate, both
actor kinds), the Activity multi-round tick (`NewRound_{User,Mob}RoundTick`),
and `mobs.TickMobCraft` (shopkeeper idle economy). One mechanism --
`crafting.GetRecipe` / `CalcSuccessChance` / `ConsumeIngredients` -- with three
triggers is normal, not duplication.

🔴 **The finding worth carrying forward is about AUDIT COVERAGE.** The
least-travelled of those paths diverged in TWO independent ways at once and
nobody noticed: `TickMobCraft` awarded success-only AND without the difficulty
bonus. When several behavior paths call one mechanism, audit every CALL SITE,
not the mechanism -- a grep of the shared code finds the library and misses
exactly this.

The plan's `MobProgressionEnabled` warning is now an explicit `t.Fatal`
precondition in the test rather than a comment, and it is sabotage-proven:
removing the pin fails on the precondition instead of silently asserting
against a path that returns 0.


## Task 18: The ~~sixteen~~ SEVENTEEN skullduggery sites

`steal.go` x3, `plant.go` x3, `shadow.go` x2, `sneak.go` x2, `picklock.go` x2,
`defuse.go`, `throw.go`, `mobcommands/flee.go`, `NewRound_DoCombat_helpers.go`.

Four bypass every entry point via direct `CheckSkillProgression`: both sneak
sites (one is the **failure** branch) and both picklock sites.

⚠️ **`picklock` is win-only** (a pin minigame, not a contest): pass `won: true`.
⚠️ **Four sites have no obvious `won`**: `shadow.go:101` (mob target, no
contest), `shadowPlayer:150` (the roll sets an informational flag only),
`throw.go:454` (multi-target), `defuse.go:129`. Decide each explicitly and record
the reasoning in the commit; do not leave one implicit.
⚠️ `sneak.go:88` and `shadow.go:114` **emit `SkillUsed` explicitly**; routing
through `OnSkillUseScaled` would emit it twice. Remove the explicit emission.

- [x] **Step 1:** failing tests: the two files no longer call `CheckSkillProgression`; a failed steal awards the fraction
- [x] **Step 2:** convert each site
- [x] **Step 3:** run everything, commit with **explicit paths**, never `git add internal/`

---

**Shipped. Two corrections to this task's own text, both found by sweeping
every production progression call rather than trusting the enumeration:**

1. 🔴 **SEVENTEEN sites, not sixteen.** `internal/mobcommands/sneak.go:19` is
   absent from the list above. It was success-only.
2. 🔴 **FOUR explicit `SkillUsed` emissions, not two.** The task names
   `sneak.go:88` and `shadow.go:114`; it misses **`shadow.go:163`** and
   **`defuse.go:132`**. All four are removed -- routing through
   `OnSkillUseScaled` emits the quest event, so keeping them would fire it
   twice per action.

⚠️ **`SkillUsed.Details` is DEAD.** Those four emissions set it (`"sneak"`,
`"shadow"`, `"defuse"`), and **nothing reads it**: `SkillUseQuestNotify` passes
only `UserId` and `Skill` to the quest engine, and a repo-wide grep finds no
other consumer. Removing the explicit emissions loses nothing.

**Rulings on the four sites with no obvious `won`, each verified against
source rather than guessed:**

| Site | Ruling | Why |
|---|---|---|
| `shadow.go:101` (mob target) | `true` | runs NO contest -- the buff applies and the shadow begins. Nothing to lose. |
| `shadow.go:150` (player target) | `!detected` | the plan called this "informational only". True of the `ShadowResult` FLAG, false of the CONTEST: `RunContest` is called with the target's search score as attacker, so `detected` means the shadower was spotted. |
| `throw.go:454` | `!fumbled` | untargeted room AoE, so "did it hit" has no single answer. A fumble is the one unambiguous loss. |
| `defuse.go:129` | `success` | it has a real contest 20 lines below; the award simply fired before it. |

Plus: **picklock x2 = `true`** (pin minigame, no opposing score), **both flee
sites = `blocker == nil`** ("got away"), **sneak = per branch**.

Every existing gate was preserved and none of them is the firing rule:
`result.RollHappened` (sneak), `contested` / `includeSkill` (flee). An action
that resolved no contest still awards nothing.


## Task 18b: The ~~thirteen~~ FOURTEEN special-move sites

**Added 2026-08-26 (owner). These were in NO task, and the slice could not have
reached its own "done when" without them** -- Task 22's guard requires every
production progression call to route through `AwardResolved` or
`AwardDefenceProgression`, and these eighteen (18b + 18c) would have turned it
red with no task assigned to convert them. Allow-listing them instead would gut
the guard.

They are also the EASIEST conversions in the slice: every one already has a
clean `won` sitting on the line above it.

**Eleven are success-only today** -- a failed bash trains nothing, exactly the
defect this slice exists to remove:

| File | Gate | `won` |
|---|---|---|
| `combat_bash.go:142` | `if result.Hit` | `result.Hit` |
| `combat_drain.go:189` | `if result.Hit` | `result.Hit` |
| `combat_gore.go:144` | `if result.Hit` | `result.Hit` |
| `combat_hamstring.go:162` | `if result.Hit` | `result.Hit` |
| `combat_kick.go:205` | `if result.Hit` | `result.Hit` |
| `combat_maul.go:160` | `if result.Hit` | `result.Hit` |
| `combat_pounce.go:159` | `if result.Hit` | `result.Hit` |
| `combat_rake.go:159` | `if result.Hit` | `result.Hit` |
| `combat_throttle.go:220` | `if result.Hit` | `result.Hit` |
| `combat_trip.go:183` | `if result.Hit` | `result.Hit` |
| `combat_grapple.go:127` | `if result.Success` | `result.Success` |

⚠️ `combat_throttle.go:220` is the MOVE's own award. Do not confuse it with the
CONCENTRATION award at `:176`, which Task 12 already converted.

**Three are ungated and need an explicit decision each:**

- **`combat_taunt.go:200`** is the BACKFIRE branch -- it already fires on a
  FAILED taunt, at full weight. Pass `won: false`. This one is a **cut**.
- **`combat_taunt.go:277`** is the resolved path; `out` is in scope, so
  `won: !out.Defended`, matching the `RecordSpecialMove` call directly below it.
- **`mutation_venom_coat.go:34`** is a mutation TRIGGER, not a contest. Nothing
  can be lost, so `won: true` -- the same treatment `ImmediateComplete` gets in
  Task 16.

- [x] **Step 1:** failing tests: a MISSED bash awards the fraction; a backfired taunt awards the fraction; a landed move still awards full
- [x] **Step 2:** convert all thirteen to `actor.AwardResolved(won, char.CandidateFor(skill))`
- [x] **Step 3:** run, commit

⚠️ These share ONE shared special-move cooldown across 18 verbs. Check whether
any site awards on a COOLDOWN-REFUSED attempt, as `track` did (Task 15's
follow-up); a refusal is not a resolved contest.

---

**Shipped.** ⚠️ **FOURTEEN, not thirteen** -- the table above lists eleven
gated sites and three ungated, which I mis-totalled when writing this task two
days after making the same error on Task 18's "sixteen". Count the rows.

The eleven gated sites went from `if result.Hit { OnSkillUse }` to
`AwardResolved(result.Hit, ...)`: the gate became the WEIGHT rather than a
precondition.

**Cooldown-refusal hazard checked and CLEAN.** Every one of these returns
`OnCooldown` from `ExecuteX` *before* reaching the award, so a refused move
never resolved and never awards. That is the trap `track` fell into (Task 15's
follow-up); it does not exist here.

**Guarded structurally, not behaviourally.** These are call sites in eleven
files and the property is structural -- no progression award may sit inside a
hit gate -- so
`TestSpecialMoves_NoProgressionAwardSitsInsideAHitGate` asserts it across all
eleven at once and fails the moment one is re-wrapped. A behavioural test would
have covered one verb. Sabotage-proven by re-wrapping `combat_bash.go`.


## Task 18c: bartering, assess, and the unarmed fallback

**Added 2026-08-26 (owner), same reason as 18b.**

| Site | Skill | Notes |
|---|---|---|
| `actions/buy.go:796` | bartering | inside `postSuccessBookkeeping`, gated on an `awardProgression` parameter |
| `actions/sell.go:384` | bartering | same shape |
| `usercommands/assess.go:86` | manifestation | |
| `usercommands/assess.go:147` | manifestation | |
| `characters/skills.go:87` | unarmed-combat | |

⚠️ **Bartering has no obvious loser.** A completed trade is a success by
construction -- the refusal paths return before the award. Decide explicitly
whether a REFUSED haggle is a resolved contest that should pay the fraction, or
whether bartering is uncontested like an instant craft, and record the
reasoning. Do not leave it implicit.

⚠️ `characters/skills.go:87` is in the `characters` package, which cannot import
`actions`. It has a `*Character`, so use `AwardResolved` directly.

- [x] **Step 1:** failing tests per decision above
- [x] **Step 2:** convert
- [x] **Step 3:** run, commit

---

**Shipped. Four of the five are UNCONTESTED and pass `won: true`; only one had
a real contest.**

- **bartering x2** -- the question this task raised ("is a refused haggle a
  resolved contest?") resolves to NO. Both awards sit inside
  `postSuccessBookkeeping` / the post-sale block, and every refusal path (no
  stock, cannot afford, carry capacity) returns before them. **Haggling is not
  a contest in this economy; it is a price lookup.** The `awardProgression`
  gate is untouched and is not the firing rule -- it exists so `buy 200 x`
  fires ONE award rather than 200.
- **assess x2** -- ⚠️ **`assess` contains no dice roll at all.** Verified:
  `grep RollStat|RunContest|AgainstDifficulty|util.Rand` over the file returns
  only the two progression lines. It is a reading, not a contest, so nothing
  can be lost. Same treatment `venom_coat` and an instant recipe get.
- **`characters/skills.go:87`** -- the ONE real contest here, and a CUT. A lost
  recovery roll fired nothing, so a character who failed to scramble to their
  feet learned nothing from the attempt -- which is exactly the situation that
  teaches you to. Still inside the `contestWin != nil` guard: a FREE stand ran
  no contest and awards nothing.

⚠️ **The shop mob's `OnStatUse("charisma", 0)` in `buy.go`/`sell.go` is NOT the
stray-stat-roll pattern Task 22 deletes.** It belongs to a DIFFERENT character
(the merchant), not to the actor receiving the bartering award, and it is the
merchant's only progression from trading. Checked here so Task 22 does not have
to rediscover it.


# Phase D: deletions

## Task 19: The stranded mob-follow roll

⚠️ Delete **only** the loop at `:668-697`. Keep the `if !isSneaking` wrapper and
the `TryRoomBehavior` at `:700`. A bare string assertion passes even if the
destination call is deleted, because of the unrelated one at `:357`; pin the
destination call specifically. Create `go_test.go`.

- [x] **Shipped.** Deleted the loop only; the `if !isSneaking` wrapper and the
  destination `TryRoomBehavior` are kept, both pinned by the new `go_test.go`.

⚠️ **This is a BALANCE CHANGE, not a cleanup.** The roll decided whether an
engaged mob chased a leaving player. With it gone and pursuit not yet authored
anywhere, **walking out of a fight is strictly safer than it was.** That is the
arc's ruling (mob pursuit is authored behaviour, no roll) but the authoring is
the behavior unification arc's work, so there is a window where nothing chases.

⚠️ **One compile fallout worth knowing:** the deleted loop DECLARED
`mobInstanceIds`, and the ambush block further down reused it with `=`. It now
declares its own.

The plan's warning about the bare string assertion was exactly right and is now
pinned in both directions: `TestGo_TheDestinationRoomEnterBehaviourSurvives`
matches on `TryRoomBehavior(destRoom.RoomId` and additionally asserts the file
carries **two or more** such calls, so the reason the weaker assertion would
have been useless is itself part of the test.

⚠️ **My own tombstone comment broke the guard first.** It quoted the deleted
formula verbatim, so the "the roll is gone" test grepped for `speedDelta` and
found my explanation of it. Reworded rather than weakening the test.



## Task 20: First-kill progression

Remove `OnFirstMobKill`, both call sites, and the message. **Keep `KD.AddMobKill`.**

- [x] **Shipped.** Function, both call sites and the message deleted;
  `KD.AddMobKill` kept at both sites.

⚠️ **It progressed a skill named `"combat"` THAT DOES NOT EXIST.** There is no
`Combat` SkillTag, `"combat"` is absent from `skills.SkillPrimaryStats`, and
`skillNameMap` is empty so nothing aliased it. **Verified against the archived
prod saves: none of the 34 carrying a skills block has a `combat:` entry**, so
the phantom skill never reached player data and **no save cleanup is owed.**
That check was the point of looking -- deleting the code would not have cleaned
the saves if it had been writing to them.

The deletion has a two-layer guard, and the first layer is the compiler:
re-adding the call no longer COMPILES, because the function is gone.
`TestFirstMobKillProgression_StaysDeleted` catches the case where someone
re-adds both, and separately pins that `KD.AddMobKill` survived.

⚠️ **My tombstone comment tripped the guard, exactly as in Task 19** -- it named
the deleted symbol, so the "it stays deleted" walk found my explanation of it.
Reworded rather than weakening the test. Two for two; a tombstone naming its
own corpse is a recurring hazard in this slice.



## Task 21: Dead crit stubs

- [x] Delete from **all nine** files; verify `grep -rl "func.*OnCritical" internal/` is empty
- [x] Remove them from `progressionCalls`, the **DENY**-list, not `allowedDirectProgression`
- [x] **`OnCritReceived` is NOT touched.** Leave `TestCritReceivedProgression_DecaysWithRank` alone

---

**Shipped.** `grep -rl "func.*OnCritical" internal/` is now empty. The nine
were all test fakes -- the `Actor` interface dropped these back in U9, so they
were vestigial methods satisfying nothing.

⚠️ **Three other files mention `OnCritical` and were correctly LEFT ALONE:**
`NewRound_DoCombat_helpers.go:363` and `NewRound_DoCombat_parity_test.go:144`
are historical COMMENTS describing what U9 replaced, and the third was the
deny-list itself. The plan's "nine" counts `func` DEFINITIONS, not mentions --
worth stating because a naive `grep -rl OnCritical` returns twelve.

Removing the two names from `progressionCalls` is not just tidiness: a
deny-list naming a method that exists nowhere reads as though the guard covers
more than it does.

`OnCritReceived` stays in the map and
`TestCritReceivedProgression_DecaysWithRank` is untouched and still passing.



# Phase E: guard, docs, verify

## Task 22: The seam guard, and DELETE the stray stat rolls

**Owner ruling 2026-08-26: delete the bare stat rolls that sit beside a
progression award. They do not fit the design and are leftovers.** A stat roll
IS progression, so a full-weight one that ignores the action's outcome is a
firing-rule violation exactly like a skill roll would be.

### Step 0: delete `emitAttackerStatGain`'s rolls

`internal/hooks/NewRound_DoCombat_unified.go:739`. Called twice per melee round
(`:659-660`, strength then dexterity) and calls `OnStatUse(statName, uid)` --
**full weight, unconditional, win or lose**, on top of the award
`processAttackerProgression` already makes. Consequences today:

- **dexterity is rolled TWICE** every round: once here, once as
  weapon-combat's primary inside the award.
- **strength is rolled once but unconditionally**, so a round in which every
  swing whiffed trains strength exactly as much as a round that landed.

This is the same defect Task 11 deleted on the ranged side, where an explicit
`OnStatUse("perception")` sat beside a ranged-combat award whose primary is
perception.

⚠️ The function ALSO emits the mob stat-gain flavour text
(`characters.MobStatGainMessages`) on a gain. That is player-visible and must
survive. Either keep the emote path and drive it from the award, or move it;
do not delete the emote with the roll.

### Step 0b: the sweep the ruling asked for

Verified 2026-08-26. Every direct `OnStatUse` / `OnSkillUse` in production:

| Site | Shape | Verdict |
|---|---|---|
| `unified.go:740` (`emitAttackerStatGain`) | unconditional stat roll beside an award | **DELETE** (Step 0) |
| `NewRound_DoCombat_helpers.go:655` (player cast), `:812` (mob twin) | stat roll **gated** on `spellData.PrimaryStat != "" && != skill's primary` | **NOT the same defect.** It is a declared-override hook, not a double-roll -- and it is currently dead, because no shipped spell sets `PrimaryStat`. Deleting it closes the separately-filed inert-`PrimaryStat` item. **Decide explicitly; do not sweep it away silently.** |
| `actions/buy.go:800`, `actions/sell.go:385` | `OnStatUse("charisma", 0)` on a shop mob trade | **Check before touching.** `buy.go:796` awards `bartering` whose primary IS charisma, so this looks like the same double-roll -- but confirm the mob path actually reaches the bartering award first. |
| `actions/combat_{bash,drain,gore,grapple,hamstring,kick,maul,pounce,rake,taunt,throttle,trip}.go`, `defuse.go` | `actor.OnSkillUse(...)`, success-only | **NOT strays.** These are the special-move seam conversions the earlier tasks own; they become `AwardResolved` calls, they do not get deleted. |

**Special attacks carry no stray stat rolls at all** -- the grep is clean. Only
the melee round and (conditionally) the shop path have them.

### Then the guard

- [x] **Step 1:** add `OnStatUseScaled` and `AwardResolved` to `progressionCalls`, so a new raw call is caught
- [x] **Step 2:** extend the guard so every production progression call routes through `AwardResolved` or `AwardDefenceProgression`, with a file-keyed allow-list carrying a reason: `actions_progression.go` (authored tutorial grant), `NewRound_AutoHeal.go` (regen, U10b-2)
- [x] **Step 3:** prove the scanner is not blind before trusting a pass
- [x] **Step 4:** run, commit

**Shipped.**

🔴 **Step 1 asked for `AwardResolved` in `progressionCalls` and that was WRONG.**
That map is the DENY-list of raw primitives; `AwardResolved` is the SEAM every
converted site routes through. Listing it would flag the code this slice spent
twenty commits producing. `OnStatUseScaled` was added; `AwardResolved` was not,
and the map now records why.

**The walk WIDENED from 2 packages to 6** (`combat`, `hooks`, `actions`,
`usercommands`, `mobcommands`, `mobs`). `internal/characters` stays out: it is
the applier's home.

🔴 **Widening immediately found FOUR sites no sweep had caught** --
`skill_helpers.go`'s warcry/rally award and three in `go.go`. Two converted here;
the two hidden-detection sites are allow-listed with an explicit
**"remove this row in U10b-1b"** marker, because the settled decisions assign the
hidden-detection FIX to that slice by name.

**Step 3, proven not assumed:** a probe of `a.GetCharacter().OnStatUse(...)` --
selector-on-selector, the shape the plan warns a naive helper misses -- IS caught.
That warning applies to `contest_site_guard_test.go`, which bails on
`v.X.(*ast.Ident)`; this walker matches `sel.Sel.Name`.

⚠️ The `scanned < 20` floor was calibrated on two large packages and failed on
`internal/mobs` (19 files) while scanning it fine. Now 5 per package plus a
**total floor of 150**.

### Step 0: the stray stat rolls, and what replaced the faucet

`emitAttackerStatGain`'s two bare `OnStatUse` calls are deleted. They fired every
round, full weight, win or lose, on top of the award: **dexterity was rolled
TWICE a round** and **strength trained as much on a whiffed round as a landed
one**. The mob stat-gain EMOTE survives, now driven by a real gain
(`mobStatSnapshot` before, `emitMobStatGains` after).

🔴 **CONSEQUENCE, ruled on rather than shipped quietly:** strength's only
skill-primary is `blacksmithing`, and **strength is the melee DAMAGE stat**
(`combat_helpers.go:429`). Owner ruling 2026-08-26, two replacements, both landed:

1. **Stamina regen trains STRENGTH ONLY** (was `strength, vitality`). Vitality
   already draws from health regen.
2. **Grappling trains strength** via `Candidate.Stat: "strength"` -- the same
   shape `DefenceSkillAndStat` uses for block and defy.

Rejected: a second strength roll alongside the swing, which would re-create the
defect the deletion removed. Also considered and not taken: strength as the
two-hander primary, strength-based bash knockdown defence, folding strength into
dexterity/vitality.
## Task 23: Re-solve

⚠️ Use the **corrected** clean-hit rate **0.3856** (`987e7e872`), never 0.5752.

⚠️ Directions differ per path: melee attacker **up**; melee defender **up** with
one defence type but **down** with three; channel defender **down**; search,
track, steal, sneak and salvage **down** because they award full today.

⚠️ `combat-analytics.jsonl` is **combat-only**, so search, track, forage, salvage
and skullduggery have no measurement basis. Set by judgement, confirm at
playtest, and say so in the commit.

⚠️ The shipped multipliers in `internal/skills/skills.go` were themselves fitted
on the mislabelled rate. Re-derive them here or record why not.

### DONE. `tools/balance/u10b1_solve_v4.py` supersedes v3.

Re-derived all 16 skill and 5 stat multipliers in `config.yaml` **and** the Go
map in `internal/skills/skills.go` (which is what test binaries see).

Three findings worth carrying forward:

1. **The correction SORTS the two rates rather than just lowering one.** A melee
   attacker award's `won` is `CleanHit` (**0.3856**); a special move's `won` is
   `result.Hit` (**0.5752**). v3 used one number for both, so the special-move
   family was solved on the right number by accident and melee on the wrong one.

2. **The build now matters and it did not before.** Under per-entry awards
   weapon-combat's rate spanned only 1.09x across 1H+fist / 2H / 1H+shield.
   Under Best-of the offhand fist steals the single award about two rounds in
   three, so the spread is now **2.48x**. Each combat skill is therefore solved
   at its own **concentrating build**, matching the owner's "concerted effort to
   grind X or Y" framing that already governs every other track. Solving
   weapon-combat on the fist build (its *minimum*) gave 3.33 and would have made
   a two-hander train ~2.5x too fast.

3. **The convention removed the empty-offhand advantage structurally.** Total
   events per round were 2.480 (1H+fist) against 1.623 (2H) and are now 1.606 /
   1.504 / 1.505. No per-skill multiplier could have done that, which is why
   v3's "unarmed sits BELOW weapon" asymmetry was compensation for a problem
   that no longer exists.

**Control row:** `bartering` is unchanged at 2.07. Buy and sell award with
`won=true`, so the convention could not move its rate, and the solve reproduces
its shipped value exactly. `perception` (1.03x rate, 1.06x multiplier) and
`charisma` (1.04x / 1.07x) agree the same way. Any future solve that shifts
bartering has a bug in it.

**Vitality is deliberately NOT retuned** even though Task 22 took it off the
stamina regen tick: `tools/balance/u10b_vitality_solve.py` puts regen at 0.33/hr
against crit-toughen's 2.67/hr, *before* the U10b-0 damage-taken faucet grew its
event count further. Losing one of two regen pools is low single digits of its
rate, and that file's own conclusion is that the multiplier is the wrong lever
for vitality.

⚠️ **Judgement, not measurement:** `combat-analytics.jsonl` is combat-only, so
every `p_win` for search, track, salvage, barter, craft and skullduggery is set
by judgement, as is the concentration model and the shield-carry fraction. The
solver labels every input M / D / R / J. These are what Task 25's playtest is
for.

## Task 24: Docs

`context.md` for `internal/progression`, `internal/characters`,
`internal/actions`, `internal/combat`. ⚠️ `internal/actions/context.md`'s
`Actor` block was **already corrected in Task 7** -- it listed
`OnCriticalSuccess`/`OnCriticalFailure`, deleted in U9, and omitted
`AwardResolved`. Do not re-file that as an open defect here.

Patch notes: failing at something now teaches you a little. No raw numbers, no em
dashes, 80 columns. **Do not promise that mobs will chase you.**

## Task 25: Verify and playtest

- [ ] `gofmt -l internal/ modules/` prints nothing
- [ ] `go build ./... && go test ./...`
- [ ] Boot test in an isolated detached worktree. **Exit 124 is success.** Never grep the bare word `panic`
- [ ] Adversarial playtest, **one signal**: does failing at something visibly teach you a little? Watch defence pacing (channel and geared-melee defenders took a cut) and passive defence training, which now pays every round at zero resource cost
- [ ] PR with `--repo pruuk/DOGMud` on every `gh` call
