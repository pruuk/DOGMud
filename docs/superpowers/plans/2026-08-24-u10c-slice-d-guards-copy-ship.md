# U10c Slice D — Guards, Player Copy, Playtest, Ship (v2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the arc. Restore a mechanic Slice B deleted by accident, ship the
owner-ruled duplication guard, fix a companion bug that predates the arc, replace
every player-facing sentence about charm across five files, sweep the stale docs,
and put the whole thing through an adversarial playtest before it merges.

**Architecture:** One restored mechanic, one one-line guard, one missing event, a
data test, a playtest profile that needs a Go registry edit, five content files,
two `context.md` sweeps, patch notes, and the playtest gate.

**Tech Stack:** Go, `internal/hooks`, `internal/mobs`, `internal/combat`, DOGMud
content YAML and help templates, the `mudagent` playtest harness.

---

## Why this is v2

v1 was rejected by three independent blind reviewers. It was wrong in four ways
that matter, and the corrections are the shape of this plan. Nobody should
re-derive v1's version of these.

1. **The `EverCharmed` save guard is an OWNER RULING with drafted code**
   (spec 11.3.5). v1 argued it away on the grounds that `EverCharmed` is
   `yaml:"-"` and therefore "cannot carry the guard." That is a category error:
   `SaveMobInstance` receives the **live in-memory** `*Mob`, `Charm()` sets the
   flag at `charminfo.go:48`, and `RemoveCharm()` never clears it. The flag is
   present at every save boundary for the mob's whole life. Non-persistence is
   the *design* — the spec's own comment says "nothing is written to disk, so a
   reboot clears it." **Implement it. Do not investigate it.**

2. **v1's Task 2 tests already exist.** `internal/mobs/instance_save_test.go:32`
   `TestSaveMobInstance_CharmedMobSkipsWrite` and `:70`
   `TestSaveMobInstance_UncharmedMobWritesFile`. v1 proposed writing them again
   in a new file, under a comment asserting in bold that nothing tested this.
   That package configures itself with `withMobProgressionEnabled(t)`
   (`instance_save_test.go:19`, built on `configs.AddOverlayOverrides`), **not**
   `configs.SetConfigForTest`.

3. **Spec 11.3.3's deliverable is a DATA test**, not what v1 wrote: assert every
   mob carrying a `shop:` block is `charm_immune` or `non_combatant`, so a
   *future* shopkeeper authored with neither fails at test time. 97 shop mobs
   today, 0 unprotected, so it passes on arrival.

4. **Playtest profiles are a hardcoded Go registry**, not file-discovered.
   `internal/playtestprofiles/types.go:5-13` `KnownTemplateIDs`, enforced at
   `playtestrun/binding.go:112`, `manifest.go:46`, `sanitize.go:16`. Dropping a
   YAML on disk fails at binding, before the container starts.

Smaller corrections carried in: the helpfile has **4** em dashes, not 6 (and
`grep -c` counts lines, not occurrences, so it cannot confirm either number);
there is **no spec section 16** (headings stop at 15.4); charm does **not**
reserve 280 — `CalcCompanionReserve` applies a skill reduction and
`SkillCostMultiplier`, giving **188** at manifestation 30.

---

## Task 1: Restore charm's in-combat penalty — OWNER RULING 2026-08-24

**Files:**
- Modify: `internal/hooks/spell_resolution.go` (`resolveAgainstMob`, line 343)
- Test: `internal/hooks/charm_in_combat_test.go` (create)

Spec 4.1's mechanics table lists the in-combat penalty as **"unchanged"**:
`×0.75` when the target is fighting the caster, `×0.85` when it is fighting
someone else. Slice B (`b567e527e`) deleted both along with `resolveCharmSpell`
and **replaced them with nothing**. Verified: `SituationalAttackMult`
(`internal/combat/situational.go:35-51`) returns a flat 1.0 for every channel
except `ChannelMelee`/`ChannelRanged`, and the defy score is Willpower + rhetoric
with no combat term. Charm is currently easier to land mid-fight than the spec
intends, while `charm.yaml`, `charm.template` and `hints.yaml:234` all still
promise the penalty exists.

Owner ruling: **restore it.** (The owner was offered a config-knob version and
chose the plain restore, so use literals with a comment. Do not add a knob.)

- [ ] **Step 1: Write the failing tests**

```go
// Spec 4.1 lists this penalty as "unchanged" across the U10c rewrite. Slice B
// deleted it with resolveCharmSpell and replaced it with nothing, which is a
// silent balance change: nothing in the channel path penalises a social attack
// on a target already fighting, so charm got easier mid-combat while three
// separate files kept telling players it had got harder.
func TestCharmInCombat_TargetFightingCasterTakesSteepestPenalty(t *testing.T) {
	// Build an AttackSide with Mult 1.0, a mob whose CurrentCombatTarget().UserId
	// is the caster's, and assert charmInCombatMult returns 0.75.
}

func TestCharmInCombat_TargetFightingSomeoneElseTakesModeratePenalty(t *testing.T) {
	// Same, but the mob's combat target is a different userId. Want 0.85.
}

func TestCharmInCombat_IdleTargetIsUnpenalised(t *testing.T) {
	// Mob not in combat. Want 1.0.
}

// Mult 0 is the ZERO VALUE and AttackSide.score() reads it as "unset, 1.0"
// (defence_multiplier.go:78-83). So `side.Mult *= 0.75` on an unset side yields
// 0, which then reads back as 1.0 and the penalty VANISHES silently. This test
// exists because that bug is invisible: it produces a plausible number.
func TestCharmInCombat_UnsetMultIsNormalisedBeforeScaling(t *testing.T) {
	// side := combat.AttackSide{Stat: 100, Mult: 0}
	// apply the penalty against a target fighting the caster
	// assert side.Mult is 0.75, NOT 0
}
```

- [ ] **Step 2: Add the helper**

Put it in `internal/hooks/spell_resolution.go` next to its only caller.

```go
// charmInCombatMult is the attack-side penalty for charming a creature that is
// already fighting. A mind braced for violence is harder to reach, and a mind
// braced against YOU is hardest of all.
//
// Restored 2026-08-24. Spec 4.1 lists these two multipliers as unchanged across
// the U10c rewrite; Slice B deleted them along with resolveCharmSpell and put
// nothing in their place. Literals rather than balance knobs by owner ruling.
func charmInCombatMult(target *characters.Character, casterUserId int) float64 {
	if target == nil || !target.IsInCombat() {
		return 1.0
	}
	if target.CurrentCombatTarget().UserId == casterUserId {
		return 0.75 // fighting the caster -- steepest
	}
	return 0.85 // fighting someone else -- moderate
}
```

- [ ] **Step 3: Apply it in `resolveAgainstMob`**

`resolveAgainstMob` takes `side` **by value** and already mutates it for
`ForceCrit` at line 346, so this is the established seam. It is also the only
place with both the caster and the target — `spellAttackSideFor` has no target
parameter and is called once per cast before target selection, so the penalty
cannot live there.

```go
	// Task 17: the sleeping-victim forced crit reaches the spell channel.
	side.ForceCrit = combat.SleepingForceCrit(&mob.Character)

	// Charm alone carries an in-combat penalty (spec 4.1). Normalise Mult
	// FIRST: 0 is the zero value and score() reads it as "unset, 1.0", so
	// multiplying into an unset Mult yields 0, which reads back as 1.0 and
	// silently drops the penalty.
	if spellData.EffectType == "charm" {
		if side.Mult == 0 {
			side.Mult = 1.0
		}
		side.Mult *= charmInCombatMult(&mob.Character, user.UserId)
	}
```

- [ ] **Step 4: Run, then commit**

```bash
go test ./internal/hooks/ -run "TestCharmInCombat" -v
go test ./internal/hooks/ -count=1
git add internal/hooks/spell_resolution.go internal/hooks/charm_in_combat_test.go
git commit -m "fix(charm): restore the in-combat penalty Slice B deleted"
```

---

## Task 2: The `EverCharmed` instance-save guard — OWNER RULING, spec 11.3.5

**Files:**
- Modify: `internal/mobs/instance_save.go:84`
- Modify: `internal/mobs/instance_save_test.go` (append — **do not create a new file**)

The hazard, verified on both sides: `MobInstanceData` persists `Equipment`
(`instance_save.go:50`), `equipmentDiffers` is itself a save trigger (`:388`),
and the restore half re-equips it at `internal/mobs/mobs.go:531`
(`mob.Character.Equipment = *savedInstance.Equipment`). Once bonds expire, an
ex-companion is uncharmed **while still wearing the player's gear**, and the next
save pass bakes that gear into a world mob.

- [ ] **Step 1: Extend the guard**

```go
	// EverCharmed, not just IsCharmed: once a bond expires the ex-companion is
	// uncharmed while still wearing the equipment its owner handed it. Saving it
	// would bake player gear into a world mob permanently -- kill, loot,
	// re-charm, repeat. The betrayal stays real in-session (it fights you with
	// your own gear) but nothing is written to disk, so a reboot clears it.
	//
	// EverCharmed is yaml:"-" ON PURPOSE. It is read from the live character
	// here, never persisted, which is exactly the semantics this needs.
	if mob.Character.IsCharmed() || mob.Character.EverCharmed {
		return nil
	}
```

- [ ] **Step 2: Append the test to the EXISTING file**

`instance_save_test.go` already owns this. Use its config idiom,
`withMobProgressionEnabled(t)` (line 19), **not** `configs.SetConfigForTest`.

```go
// The ex-companion case. TestSaveMobInstance_CharmedMobSkipsWrite covers a mob
// that is charmed RIGHT NOW; this covers the window that opened when bonds
// started expiring: uncharmed, but still wearing the gear its owner handed it.
// Without the EverCharmed half, the next save pass writes that gear into
// mobs.instances/ and the restore at mobs.go:531 puts it back on a respawned
// world mob.
func TestSaveMobInstance_EverCharmedMobSkipsWrite(t *testing.T) {
	// Build the same fixture shape as TestSaveMobInstance_CharmedMobSkipsWrite,
	// then: mob.Character.Charm(42, 100, ""); mob.Character.RemoveCharm()
	// Assert mob.Character.EverCharmed is true and IsCharmed() is false,
	// then assert SaveMobInstance wrote NO file.
}
```

- [ ] **Step 3: Run and commit**

```bash
go test ./internal/mobs/ -run "TestSaveMobInstance" -v
git add internal/mobs/instance_save.go internal/mobs/instance_save_test.go
git commit -m "fix(charm): an ex-charmed mob never writes its owner's gear to disk"
```

---

## Task 3: The shop-mob data test — spec 11.3.3

**Files:**
- Create: `internal/mobs/shop_charm_safety_test.go`

Spec 11.3.3's deliverable, verbatim: *"assert that every mob carrying a `shop:`
block is `charm_immune` or `non_combatant`, so a future shopkeeper authored
without either is caught at test time."*

Today: 97 shop mobs, 0 unprotected, so this passes on arrival and its job is
purely to fail on the *next* one.

- [ ] **Step 1: Write it**

Walk `_datafiles/world/dogmud/mobs/`, anchored on `runtime.Caller` (test CWD in
this repo is not reliably the package dir — `internal/actions/economy_test.go`
chdirs to the repo root and all tests in a package share one binary). For each
YAML with a `shop:` key, require `charm_immune: true` or `non_combatant: true`.
Report **every** offender in one failure, not just the first, and name the file
path so the fix is obvious.

- [ ] **Step 2: Mutation check**

Temporarily strip both flags from
`_datafiles/world/dogmud/mobs/new_plymouth_common/9326-flower_seller.yaml`,
confirm the test names that exact file, then `git checkout` the file and re-run.

- [ ] **Step 3: Commit**

---

## Task 4: Companion death must republish vitals

**Files:**
- Modify: `internal/hooks/MobDeath_CompanionCleanup.go:46`
- Test: alongside

Pre-existing, affects **every** companion type, not just charm.
`MobDeath_CompanionCleanup.go` calls `RemoveCompanion` + `TrackCharmed(false)`
and then sends "Your X has fallen" — with no `RecalculateStats()` and no
`CharacterVitalsChanged`. Both other exits from a bond have it: `dismiss.go` via
`publishReleasedReservation`, and the expiry path at
`NewRound_MobRoundTick.go`. This is precisely the bug
`internal/usercommands/dismiss_vitals_test.go` documents at length: the live
readers (`status`, prompt bar) let go immediately, but `Char.Vitals` is
push-only, so the web client keeps showing a reservation for a companion that no
longer exists.

- [ ] **Step 1: Write the failing test.** Model it on
      `dismiss_vitals_test.go:65` — drain the queue, kill the companion, assert
      `events.DrainQueuedVitalsChangedForTest` is non-empty.

- [ ] **Step 2: Add the two lines** after `RemoveCompanion`, with a comment
      pointing at the dismiss precedent so the next reader knows it is a
      three-site invariant, not a local fix.

- [ ] **Step 3: Run, commit.**

---

## Task 5: The playtest profile — YAML **and** the Go registry

**Files:**
- Create: `tools/playtest/profiles/charmer.yaml`
- Modify: `internal/playtestprofiles/types.go` (`KnownTemplateIDs` + its "six tracked" comment)
- Modify: `internal/playtestprofiles/manifest.go` (the "six tracked templates" comment at ~line 57)
- Modify: `tools/playtest/profiles/README.md` and `tools/playtest/profiles/context.md`

**A bare YAML does not work.** `KnownTemplateIDs`
(`internal/playtestprofiles/types.go:5-13`) is a hardcoded list of six, enforced
at `playtestrun/binding.go:112` (`unknown profile %q`), `manifest.go:46` and
`sanitize.go:16`. `templates_repo_test.go:16` iterates it, so the new profile
picks up sanitize coverage automatically once registered.

- [ ] **Step 1: Add `"charmer"` to `KnownTemplateIDs`** and correct both "six
      tracked" comments to seven.

- [ ] **Step 2: Create the profile**

```yaml
role: user
username: template-charmer
character:
  name: Bindsong
  description: >
    A manifestation specialist built to exercise charm end to end: enough
    charisma to win the contest, enough conviction to pay the cost and hold
    the companion reservation, and a spare weapon to hand a charmed creature.
  roomid: 462
  zone: Thornwall City
  speciesid: 1
  stats:
    strength:
      base: 100
    dexterity:
      base: 110
    perception:
      base: 115
    vitality:
      base: 115
    willpower:
      base: 130
    charisma:
      base: 150
  health: 460
  stamina: 450
  conviction: 585
  gold: 400
  skills:
    manifestation: 30
    spellcasting: 15
    rhetoric: 10
    weapon-combat: 8
    search: 10
  spellbook:
    charm: 3
    heal: 2
  equipment:
    weapon:
      itemid: 10018
    body:
      itemid: 20008
    feet:
      itemid: 20003
  items:
    - itemid: 10018
```

The arithmetic, so nobody "simplifies" it into a profile that cannot charm:
`ConvictionMax = ConvictionBase + Cha×3 + Wil×1` = 5 + 450 + 130 = **585**
(`internal/characters/validate.go:109-111`; `config.yaml:974-976`). The cap is
`floor(585 × 0.66)` = **386** (`PoolReservationCapPct`, `config.yaml:1409`;
`reservationCapFor` floors, `reservation.go:47`).

**The reserve is 188, not 280.** `CalcCompanionReserve`
(`internal/characters/companions.go:270-278`) applies a skill reduction and
`SkillCostMultiplier`, so at manifestation 30 the flat `CompanionReserveDefault`
of 280 becomes `round(280 × 0.70 × 0.96)` = 188. (`CompanionReserveDefault` is
**absent** from `config.yaml` — that means "use the Go default 280", not zero.)
Either way it clears 386 comfortably. The reason to keep charisma high is the
contest, not the cap.

The second `itemid: 10018` under `items:` is the weapon to hand a charmed
creature, so the tester need not disarm itself.

- [ ] **Step 3: Prove it boots with charm known.** Log in as the profile and type
      `spells`. A profile that loads but grants nothing wastes the whole run.

- [ ] **Step 4: `go test ./internal/playtestprofiles/ -count=1`. Commit.**

---

## Task 6: Rewrite `charm.yaml`'s description

**Files:**
- Modify: `_datafiles/world/dogmud/spells/charm.yaml`

Note where this actually shows: **not** the `spells` command (which lists
SpellId/Name/Target/Cost only) and **not** `help charm` (which resolves to the
dedicated template). Its one render path is `help spell charm` via
`help/spell.template`. Task 8 deals with that template's own problems.

Two current claims are false: *"Stronger creatures resist more fiercely"* (spec
3.6 removed the power term) and, after Task 1 restores the penalty, *"much
harder against creatures already in combat"* becomes true again and may stay.

- [ ] **Step 1: Replace `description:`**

```yaml
description: |
  You reach into the mind of a hostile creature and bend its
  will to yours. A stubborn mind fights you hardest. Raw
  strength is no defence at all, so the biggest thing in the
  room can be the easiest to take, and the worst to lose hold
  of. A creature already fighting is harder to reach.
  What you bind will follow you and fight for you, but not for
  long. There is no warning and no sign of the hold weakening.
  It simply ends. The more completely you win, the longer you
  keep it.
  When it ends, if you are standing beside it, it attacks you.
```

Deliberate choices, do not undo them: sentence two is the spec 10.1
strength-versus-will requirement; *"There is no warning and no sign of the hold
weakening"* replaces a v1 draft that said the mind "begins to return", which
described the re-roll ladder this arc deleted; and the last line states the
betrayal plainly rather than as *"and it remembers"*, which an ESL reader parses
as atmosphere rather than as **it attacks you**.

- [ ] **Step 2: Verify it parses** (`go build ./...` proves nothing here — a
      malformed spell YAML panics at *startup*):

```bash
python -c "import yaml,io; yaml.safe_load(io.open('_datafiles/world/dogmud/spells/charm.yaml',encoding='utf-8')); print('YAML_OK')"
```

- [ ] **Step 3: Commit.**

---

## Task 7: Rewrite `help charm` — OWNER-REQUIRED FOR COMPLETION

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/charm.template`

The owner named this a required completion step. The slice is not done without
it.

Wrong lines, and why:

| Line | Why |
|---|---|
| "Your hold gradually loosens over time." | The ladder is deleted. It does not loosen; it ends. |
| "The stronger your charisma and the higher your manifestation skill, the longer the charm endures." | False. Duration is bought with the contest margin. |
| "Defense: Mental (opposed by target's willpower)" | Answered by **defy** now. The willpower half survives. |
| "Duration: Scales with charisma and manifestation skill" | Same falsehood. |
| "the creature's willpower and **mental fortitude**" | The defending skill is **rhetoric**. |

House rules: **4** em dashes (U+2014), 0 en dashes — count them with Python, not
`grep -c`, which counts lines. **Preserve CRLF**: 422 of 453 templates in that
directory are CRLF, so a lone LF file would be a new inconsistency, not a repair.

Content that must appear:
- It becomes a companion and fights for you.
- **The hold ends**, with no warning and no sign of weakening.
- **How completely you won decides how long you keep it** — not your charisma,
  not your rank. Plain words, no numbers.
- **A creature caught asleep is held longest.** A real tactic worth rewarding.
- When it ends and you are there, **it turns on you**. If you are elsewhere it
  simply reverts, and your conviction comes back either way.
- **Logging out destroys the creature** and anything you gave it.
- **`dismiss` is not a peaceful parting** — it turns on you at once. Free a slot
  *before* you need one, not mid-fight. (Charm is `base_folds: 36`, the longest
  channel in the game, so "dismiss then immediately re-charm" is a trap.)
- It keeps the gear you hand it and grows stronger while it serves you.
- Not usable on players, merchants, or non-fighters.
- **A powerful creature is not harder to charm, only far worse to lose hold of.**
- Corrected labels: defence is **defy** (willpower and its skill at arguing
  back); duration is **how completely you won**.

**Never state the formula or the rounds remaining.** Spec 3.3: the uncertainty is
the mechanic.

- [ ] **Steps:** read the current file and a clean recent exemplar
      (`help/progression.template`, written 2026-08-24), rewrite keeping the
      section skeleton, verify over raw telnet on port 33333 that no `<ansi>`
      leaks and nothing passes column 80, commit.

---

## Task 8: The other four charm surfaces

**Files:**
- Modify: `_datafiles/world/dogmud/templates/help/companion.template`
- Modify: `_datafiles/world/dogmud/templates/help/dismiss.template`
- Modify: `_datafiles/world/dogmud/templates/help/manifestation.template` (~line 52)
- Modify: `_datafiles/world/dogmud/templates/help/spell.template`

Rewriting `charm.template` alone leaves four documents contradicting it.

- [ ] **`companion.template`** says *"Charmed companions persist as long as the
      magical bond holds"* under Persistence. Spec 13 rules that logout
      **destroys** them, which Tasks 7 and 9 both make a headline. This is the
      file `help charm`'s own "See also" points at, so the contradiction is one
      keystroke away.

- [ ] **`dismiss.template`** — three defects:
      - *"the charm cannot be reapplied to the same creature"* — **false**, spec
        3.7 explicitly allows re-charming and `RemoveCharm()` sets nothing that
        would block it.
      - *"**ALL** companions present in the same room will immediately turn
        hostile"* — **false**. `dismiss.go` has exactly one `SetAggro`, on the
        dismissed mob. Likely pre-existing.
      - Slice C added a **presence gate** to that line: the betrayal now fires
        only if you are in the same room. Documented nowhere player-facing.
      - Also carries 2 em dashes.

- [ ] **`manifestation.template:52`** — *"Charmed — An existing creature bent to
      your will. Retains its original capabilities."* Silent on the clock and
      the betrayal, so the school helpfile still teaches the pre-arc model.

- [ ] **`spell.template`** — the generic fallback that renders `help spell charm`.
      Two problems, both newly live:
      - It prints **three raw numbers** (Base Folds, Conv. Cost, Wait Time),
        against the no-hard-numbers rule, on the one screen that shows the
        description Task 6 rewrote.
      - Slice B adding `target_defense_type: social` switched on its
        `Resisted by: {{ .TargetDefenseType }} defense` line. Charm is the
        **only** `social` spell in the game (survey: mental ×9, none ×13,
        physical ×11, social ×1), so a player now reads "social defense" here
        and "defy" in `charm.template`.
      - **Minimum:** map `TargetDefenseType` to the player-facing defence name so
        the two screens agree. The raw numbers are a wider problem than charm —
        if fixing them touches every spell, file it as a follow-up and say so
        rather than silently leaving three numbers on the page.

- [ ] Verify each over telnet. Commit as one content commit.

---

## Task 9: Patch notes

**Files:**
- Modify: `docs/PATCH_NOTES.md`

One entry for the arc, newest at the top. Use **the real date on the day you
write it**.

```markdown
## 2026-08-24: Charm is a gamble now, not a purchase

Bending a creature to your will used to be permanent, which made it a
straightforward trade: pay the conviction, keep the creature. It is not that
any more.

The hold now ends. How long you keep it depends on how completely you won the
contest of wills, and nothing tells you how long that is. A creature you barely
overpowered can turn on you almost at once, sometimes before the fight you
charmed it for is finished. One you dominated outright will stay with you far
longer. You will not know which you have until it stops.

Catch something asleep and you will hold it longest.

When the hold breaks and you are standing beside it, it comes for you. If you
are somewhere else, it simply goes back to being what it was, and your
conviction comes back to you either way.

This cuts both ways with everything you invest in it. A charmed creature keeps
the gear you hand it and grows stronger fighting at your side, so the more you
put into one, the worse the moment it remembers itself.

Size is no protection against you and no comfort either. A stubborn mind is
what resists you; a big one is simply worse news later.

Letting one go is not a peaceful parting. Dismiss a charmed creature and it
turns on you at once, so free a slot before you need one rather than in the
middle of a fight.

As before, logging out destroys anything you have charmed along with whatever
it was carrying. Take back anything you lent it before you go.

One smaller thing. Charm now refuses to target other players, which the help
file always claimed it did and which nothing actually enforced.
```

Three things this deliberately avoids, all of which v1 got wrong:
- **No "within the hour" / "all evening".** Real time is 2 to 30 minutes at
  `RoundSeconds: 4`. "Within the hour" reads as *plenty of time* and manufactures
  the unfair surprise the rest of the arc is trying to prevent.
- **No "bring it home before you go."** There is no home;
  `PlayerSpawn_HandleJoin.go:56-63` strips the record wherever the mob stood. The
  real mitigation is taking your gear back.
- **No "no longer hunts you across zones."** Charm was permanent before this arc,
  so no bond ever lapsed and nothing ever hunted anyone. Do not claim credit for
  repairing something that never happened.

- [ ] Commit.

---

## Task 10: Sweep the stale docs

**Files:**
- Modify: `internal/combat/context.md` (~line 888)
- Modify: `internal/hooks/context.md` (~lines 1067, 1339-1340, 1414-1420)
- Modify: `docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md` (~line 185)

Spec 11.4.8 named the two `context.md` files by hand, and CLAUDE.md makes this
mandatory: *"any work that reshapes an existing package's API, data model, or
file list MUST update it."* Nothing in slices A–C touched them.

- [ ] `internal/combat/context.md:888` lists `hooks.tickMobCharmState` (charm
      reroll) and `hooks.resolveCharmSpell` as live direct `RunContest` sites.
      Both are deleted.
- [ ] `internal/hooks/context.md:1067` repeats it; `:1339-1340` still says
      *"charmed companions are permanently Active"*; `:1414-1420` describes
      `resolveCharmSpell`'s gates as they were before Slice B.
- [ ] Also record the arc's real API changes: `AttackerNormalizedMargin` on
      `ChannelDefenceResult`, the deleted `CompanionInfo.CharmDuration` /
      `CharmRerolls`, and charm's move onto `ChannelSocial`.
- [ ] `UNIFIED_RESOLUTION_ROADMAP.md:185` still describes the pre-arc state in
      the present tense (*"scores `Charisma + Manifestation × 25`… the ladder is
      dead code… charmed for 99999 rounds"*) and still asks *"Also decide charm's
      DEFENCE stat"*, which was decided as defy. **Verify every symbol you write
      exists** before committing — a `context.md` describing an invented API is
      worse than none.

- [ ] Commit.

---

## Task 11: The adversarial playtest gate — DO NOT SKIP

**Files:**
- Create: `tools/playtest/goals/2026-08-24-u10c-charm-arc.yaml`

Nobody has watched a charm resolve in a live game since the Slice B rewrite.
Boot-clean verifies the system; it has never verified the experience.

- [ ] **Step 1: Make a bond observable inside the run.** Shipped durations are
      `CharmDurationMinRounds: 30` / `MaxRounds: 450` at `RoundSeconds: 4`, i.e.
      **2 to 30 real minutes**, against a 30-minute wall-clock budget — and the
      charmer profile's high charisma guarantees large margins and therefore long
      bonds. **As written, the run would almost certainly end before any bond
      lapsed**, and the grudge, the break message and the conviction release
      would ship unobserved.

      The goals schema has no config-override mechanism (checked
      `internal/playtestrun/`; only `MaxAIConnections`). So edit
      `_datafiles/config.yaml` **inside the ephemeral checkout** to
      `CharmDurationMinRounds: 3` / `CharmDurationMaxRounds: 8` before the run,
      and **state in the report that this run cannot validate duration tuning.**

- [ ] **Step 2: Write the goals file** with a top-level `ephemeral:` block
      (required by local `playtestrun`); copy the block shape from
      `tools/playtest/goals/2026-08-03-prepush-sweep.yaml`. Use
      `profile: charmer`.

Goals:
1. Cast charm and read every line. **One contest, one verdict** — a resist line
   and a success line for the same cast is the double narration Slice B removed.
2. Cast at a **player** — clean refusal, not a swallowed command.
3. Cast at a **shopkeeper** — the harm-target refusal.
4. Cast at a **sleeping** creature. Assert only that it *lands*; with the
   shortened durations you cannot verify "holds longest". Most sleepers are
   `non_combatant` townspeople and therefore charm-proof, so give the tester a
   route: `106-city_guard.yaml` and `261-farmer_hesta.yaml` are combatant
   sleepers.
5. Hand it a weapon; confirm it uses it.
6. **Wait out a bond in an idle room** and watch the break.
7. **Wait out a bond mid-fight with a third creature.** This is the case that
   matters: two enemies and one red line explaining why. Does it scroll past?
8. `dismiss` and confirm it turns on you.
9. **Walk two rooms away, wait out the bond, come back.** Then try to `dismiss`
   it and to charm something else. (This is where the stranded-companion bug
   lived; the fix is `b0b85bfec` and this goal is its live check.)
10. Confirm conviction returns on **break, dismiss, and companion death**.
11. Charm a creature another player already charmed — the refusal says *"You
    can't target a companion with a harmful spell"* about a wolf that looks
    wild. Does that read as sensible or as a bug?
12. **Get interrupted mid-channel.** Conviction is spent incrementally and only
    the unspent part is refunded, so a knockdown at fold 30 of 36 burns most of
    120 conviction. Is the player told anything?
13. Charm at the companion cap, and with the reservation over the cap. Confirm
    **no conviction is charged** and the messages name the real reason.
14. A **second player in the room** — can a bystander tell what happened?
15. `help charm`, `help spell charm`, `help companion`, `help dismiss` and
    `spells`, read as a confused human.

- [ ] **Step 3: Run adversarially.**

```text
/playtest local --checkout C:/Users/Calabe Davis/workspace/DOGMud bug-finder 2026-08-24-u10c-charm-arc.yaml
```

- [ ] **Step 4: Extract findings to memory immediately** — reports are gitignored.
- [ ] **Step 5: Fix what it finds; re-run if the fixes are behavioural.**

---

## Task 12: Gates, PR, merge

- [ ] `gofmt -l internal/ modules/` prints nothing.
- [ ] `go build ./...`.
- [ ] `go test ./internal/hooks/ ./internal/mobs/ ./internal/combat/ ./internal/characters/ ./internal/actions/ ./internal/usercommands/ ./internal/playtestprofiles/ -count=1`
      — note `playtestprofiles`, which Task 5 touches and v1 omitted.
- [ ] `golangci-lint run` — no **new** finding on a touched file (73 pre-existing
      across these packages).
- [ ] Boot in an isolated detached worktree. `Server Ready` = 1, panic patterns
      = 0, **exit 124 is success**, never grep the bare word `panic`. This matters
      more than usual: Tasks 6, 7 and 8 edit five content files and content YAML
      panics at *startup*, not at build.
- [ ] Add a **section 16** to the spec (it does not exist yet; headings stop at
      15.4) recording the in-combat restore and the `EverCharmed` guard as
      shipped.
- [ ] File spec section 12's follow-up: the instance-save audit the owner asked
      for. It exists only as a spec paragraph today.
- [ ] PR with `--repo pruuk/DOGMud`. Confirm each job ran with **zero
      annotations** — a green check is not proof.
- [ ] Merge `--merge`, never `--squash`.
- [ ] Update the U10c memory topic file and mark the arc closed.

---

## For whoever executes this

The tests are small. **Tasks 7, 8 and 11 are the slice**, and Task 1 is a
merged balance regression that should land first so the copy written in 6 and 7
is true when it ships.

If a `grep` or `sed` finds something this document did not predict, **stop and
report**. v1 of this plan was rejected by three reviewers; the "Why this is v2"
section exists because its four most confident claims were its four wrong ones.
