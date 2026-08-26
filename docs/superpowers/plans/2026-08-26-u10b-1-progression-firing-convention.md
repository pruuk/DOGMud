# U10b-1: the progression firing convention

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use `- [ ]` for tracking.

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

- [ ] **Step 1:** failing test at each of the three triggers: a broken concentration awards spellcasting at the fraction
- [ ] **Step 2:** route all three through `AwardResolved`, won on `res.Success`
- [ ] **Step 3:** run, commit

⚠️ Concentration fires per damage instance, so up to ~4/round, and `spellcasting`
is fitted at 3.90 on the premise that casting is rare. Record the rate in the
commit.

---

## Task 13: The spell attacker

One cast is one resolved action: **one event, won if ANY target was hit.** Not a
per-target award.

- [ ] **Step 1:** failing tests: a cast every target defended awards the fraction; a cast that hit one of three awards full, once
- [ ] **Step 2:** route `NewRound_DoCombat_helpers.go:647` (mob twin `:806`) through `AwardResolved`
- [ ] **Step 3:** run, commit

---

# Phase C: the non-combat sites, on their CURRENT resolution

No conversions here. Each site already resolves; read its existing outcome.

## Task 14: `search`

⚠️ Awards a **FULL** event today, win or lose, so this is a **cut** on failure.

- [ ] **Step 1:** failing tests: a room with five hidden things awards the same as a room with one; a fruitless-but-resolved search awards the fraction
- [ ] **Step 2:** replace `:243` with `actor.AwardResolved(userId, foundAnything, char.CandidateFor(string(skills.Search)))`, setting `foundAnything` alongside each `result.*Found` append
- [ ] **Step 3:** run, commit

## Task 15: `track` and `forage`

⚠️ Forage's award is at **`internal/actions/forage.go:142`**, not in `internal/forager`.

- [ ] **Step 1:** failing tests for both: a resolved failure awards the fraction
- [ ] **Step 2:** route both through `AwardResolved`, won on the existing find/grade condition
- [ ] **Step 3:** run, commit

## Task 16: `craft` and `salvage`

Craft already resolves as `util.Rand(100) < chance`; salvage rolls per unit.
**Do not touch either roll.**

- [ ] **Step 1:** failing tests: a failed craft awards the fraction (**the case the slice is justified by**); a salvage recovering nothing awards the fraction; a salvage awards **once** per command, not per unit
- [ ] **Step 2:** award on both branches at all four craft sites and both salvage sites, via `AwardResolved`
- [ ] **Step 3:** run, commit

## Task 17: Mob crafters

- [ ] **Step 1:** failing test forcing a failed mob craft. **`pinConfigForTest` must set `MobProgressionEnabled`** or this asserts against a path that returns 0
- [ ] **Step 2:** award on both branches at `:505` and `:546`
- [ ] **Step 3:** run, commit

## Task 18: The sixteen skullduggery sites

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

- [ ] **Step 1:** failing tests: the two files no longer call `CheckSkillProgression`; a failed steal awards the fraction
- [ ] **Step 2:** convert each site
- [ ] **Step 3:** run everything, commit with **explicit paths**, never `git add internal/`

---

# Phase D: deletions

## Task 19: The stranded mob-follow roll

⚠️ Delete **only** the loop at `:668-697`. Keep the `if !isSneaking` wrapper and
the `TryRoomBehavior` at `:700`. A bare string assertion passes even if the
destination call is deleted, because of the unrelated one at `:357`; pin the
destination call specifically. Create `go_test.go`.

## Task 20: First-kill progression

Remove `OnFirstMobKill`, both call sites, and the message. **Keep `KD.AddMobKill`.**

## Task 21: Dead crit stubs

- [ ] Delete from **all nine** files; verify `grep -rl "func.*OnCritical" internal/` is empty
- [ ] Remove them from `progressionCalls`, the **DENY**-list, not `allowedDirectProgression`
- [ ] **`OnCritReceived` is NOT touched.** Leave `TestCritReceivedProgression_DecaysWithRank` alone

---

# Phase E: guard, docs, verify

## Task 22: The seam guard

- [ ] **Step 1:** add `OnStatUseScaled` and `AwardResolved` to `progressionCalls`, so a new raw call is caught
- [ ] **Step 2:** extend the guard so every production progression call routes through `AwardResolved` or `AwardDefenceProgression`, with a file-keyed allow-list carrying a reason: `actions_progression.go` (authored tutorial grant), `NewRound_AutoHeal.go` (regen, U10b-2)
- [ ] **Step 3:** prove the scanner is not blind before trusting a pass
- [ ] **Step 4:** run, commit

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
